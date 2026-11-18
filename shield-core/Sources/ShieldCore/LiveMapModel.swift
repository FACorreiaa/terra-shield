import Foundation
import Observation

/// State behind the live map (DESIGN.md §10.1): what the viewport shows, when it was last
/// loaded, and what went wrong. Owns no UI, so the rules here are unit-tested.
///
/// Rules:
/// - Viewport changes are debounced; a newer request cancels an older one, so a slow response
///   never overwrites a newer view.
/// - A failure keeps the last good data on screen and says how old it is. It never blanks the
///   map and never shows sample data in place of live data (DESIGN.md §15).
/// - Detections load beside events, and their failure does not fail the events.
@Observable @MainActor
public final class LiveMapModel {
    public enum Phase: Equatable, Sendable {
        /// Nothing loaded yet.
        case loading
        case loaded
        /// The last load failed; `snapshot` still holds the previous good data, if any.
        case failed(LoadFailure)
    }

    public let provider: any HazardEventProvider
    public private(set) var snapshot: MapSnapshot?
    public private(set) var detections: DetectionLayer = .empty
    /// Detections were asked for and could not be loaded.
    public private(set) var detectionsFailed = false
    public private(set) var phase: Phase = .loading
    /// When `snapshot` was received.
    public private(set) var lastSuccess: Date?
    public private(set) var viewport: MapViewport = .world
    /// From `/v1/sources`, for per-source staleness (DESIGN.md §5). Kept from the last success.
    public private(set) var sources: [String: Source] = [:]

    /// How often the map reloads while visible. 60 s is the shortest source poll interval.
    public static let refreshInterval: Duration = .seconds(60)

    public var query = EventQuery() {
        didSet { if query != oldValue { reload() } }
    }

    /// The FIRMS layer; off by default (DESIGN.md §5).
    public var showsDetections = false {
        didSet {
            guard showsDetections != oldValue else { return }
            if !showsDetections { detections = .empty; detectionsFailed = false }
            reload()
        }
    }

    public var selectedEventID: UUID? {
        didSet {
            guard selectedEventID != oldValue else { return }
            loadSelectedArea()
        }
    }

    /// The selected event's footprint, drawn under the markers (DESIGN.md §5). `nil` while
    /// loading, for a plain point, or if the detail could not be loaded: the marker and card
    /// still show, so a failure here is not worth a banner.
    public private(set) var selectedGeometry: EventGeometry?

    /// The event's source has missed three polls (DECISIONS #8): fade it, don't hide it.
    public func isStale(_ event: HazardEvent) -> Bool {
        sources[event.source]?.isStale(now: now()) ?? false
    }

    /// The map itself has not refreshed for three refresh intervals, e.g. after failures.
    public var dataIsStale: Bool {
        guard let lastSuccess else { return false }
        return now().timeIntervalSince(lastSuccess) > 3 * Self.refreshInterval.seconds
    }

    public var selectedEvent: HazardEvent? {
        guard let selectedEventID else { return nil }
        return snapshot?.events.first { $0.id == selectedEventID }
    }

    private let debounce: Duration
    private let now: @Sendable () -> Date
    private var loadTask: Task<Void, Never>?
    private var areaTask: Task<Void, Never>?
    private var generation = 0

    public init(
        provider: any HazardEventProvider,
        debounce: Duration = .milliseconds(350),
        now: @escaping @Sendable () -> Date = { .now }
    ) {
        self.provider = provider
        self.debounce = debounce
        self.now = now
    }

    /// The map settled on a new region. Loads after `debounce` unless another change follows.
    public func viewportDidChange(_ viewport: MapViewport) {
        guard viewport != self.viewport || snapshot == nil else { return }
        self.viewport = viewport
        schedule(after: debounce)
    }

    /// Loads now, replacing any pending or running load.
    public func reload() {
        schedule(after: .zero)
    }

    /// Reloads every `interval` until the calling task is cancelled (attach to the view's
    /// lifetime). 60 s matches the shortest source poll interval; the server caches 15 s.
    public func keepFresh(every interval: Duration = LiveMapModel.refreshInterval) async {
        while !Task.isCancelled {
            try? await Task.sleep(for: interval)
            if Task.isCancelled { return }
            reload()
        }
    }

    /// Waits for the current load, if any (tests, pull-to-refresh).
    public func settle() async {
        await loadTask?.value
    }

    private func schedule(after delay: Duration) {
        loadTask?.cancel()
        generation += 1
        let generation = generation
        loadTask = Task { [weak self] in
            if delay > .zero {
                try? await Task.sleep(for: delay)
                if Task.isCancelled { return }
            }
            await self?.load(generation: generation)
        }
    }

    private func load(generation: Int) async {
        let viewport = viewport, query = query, wantsDetections = showsDetections, provider = provider
        async let events = Self.capture { try await provider.snapshot(matching: query, in: viewport) }
        async let fires = Self.detections(from: provider, in: viewport, if: wantsDetections)
        async let sourceList = Self.capture { try await provider.sources() }
        let (eventResult, fireResult, sourceResult) = await (events, fires, sourceList)
        // A newer load started meanwhile: its answer is the one to show.
        guard generation == self.generation, !Task.isCancelled else { return }

        switch eventResult {
        case .success(let snapshot):
            self.snapshot = snapshot
            lastSuccess = now()
            phase = .loaded
        case .failure(let error):
            if LoadFailure.isCancellation(error) { return }
            phase = .failed(LoadFailure(error))
        }
        if case .success(let list) = sourceResult {
            sources = Dictionary(list.map { ($0.id, $0) }, uniquingKeysWith: { a, _ in a })
        }
        switch fireResult {
        case .success(let layer)?:
            detections = layer
            detectionsFailed = false
        case .failure(let error)?:
            if !LoadFailure.isCancellation(error) { detectionsFailed = true }
        case nil:
            break
        }
    }

    private nonisolated static func capture<T: Sendable>(
        _ body: @Sendable () async throws -> T
    ) async -> Result<T, any Error> {
        do { return .success(try await body()) } catch { return .failure(error) }
    }

    /// Fetches the selected event's geometry. A newer selection cancels the older fetch, and a
    /// late answer for an event no longer selected is dropped.
    private func loadSelectedArea() {
        areaTask?.cancel()
        selectedGeometry = nil
        guard let id = selectedEventID else { return }
        let provider = provider
        areaTask = Task { [weak self] in
            let result = await Self.capture { try await provider.detail(id: id) }
            guard let self, !Task.isCancelled, self.selectedEventID == id else { return }
            if case .success(let detail?) = result, detail.geometry.hasShape {
                self.selectedGeometry = detail.geometry
            }
        }
    }

    /// Waits for an in-flight area fetch (tests).
    func settleArea() async {
        await areaTask?.value
    }

    private nonisolated static func detections(
        from provider: any HazardEventProvider, in viewport: MapViewport, if wanted: Bool
    ) async -> Result<DetectionLayer, any Error>? {
        guard wanted else { return nil }
        return await capture { try await provider.detections(in: viewport) }
    }
}

/// An error whose message was written for the person using the app.
public protocol UserFacingError: Error {
    var userMessage: String { get }
}

/// Why a load failed, in words for the user.
public enum LoadFailure: Equatable, Sendable {
    /// The device has no network.
    case offline
    /// The network works but Shield's API could not be reached or answered with an error.
    case unreachable(String)

    public init(_ error: any Error) {
        if let url = error as? URLError {
            switch url.code {
            case .notConnectedToInternet, .networkConnectionLost, .dataNotAllowed,
                 .internationalRoamingOff, .callIsActive:
                self = .offline
                return
            case .cannotConnectToHost, .cannotFindHost, .timedOut, .dnsLookupFailed:
                self = .unreachable("Shield’s server isn’t responding.")
                return
            default:
                break
            }
        }
        // Only words written for people. Library errors (decoding, generated client) describe
        // themselves with debug dumps, which must never reach the banner.
        if let described = (error as? any UserFacingError)?.userMessage {
            self = .unreachable(described)
        } else {
            self = .unreachable("Something went wrong loading events.")
        }
    }

    public var message: String {
        switch self {
        case .offline: "You’re offline."
        case .unreachable(let reason): reason
        }
    }

    static func isCancellation(_ error: any Error) -> Bool {
        error is CancellationError || (error as? URLError)?.code == .cancelled
    }
}

extension Duration {
    var seconds: TimeInterval {
        let (s, atto) = components
        return TimeInterval(s) + TimeInterval(atto) / 1e18
    }
}
