import Foundation

/// What the UI asks for. Mirrors the planned `GET /v1/events` filters (ARCHITECTURE §4).
public struct EventQuery: Sendable, Hashable {
    public var types: Set<HazardType>
    public var minimumSeverity: Severity
    public var kinds: Set<EventKind>
    public var includeResolved: Bool
    public var window: TimeInterval?
    public var country: String?

    /// The default view (DESIGN.md §10.6): every type, minor and up, detections off, last 24 h.
    public init(
        types: Set<HazardType> = Set(HazardType.hazards),
        minimumSeverity: Severity = .minor,
        kinds: Set<EventKind> = [.observed, .warning, .forecast],
        includeResolved: Bool = false,
        window: TimeInterval? = 24 * 3600,
        country: String? = nil
    ) {
        self.types = types
        self.minimumSeverity = minimumSeverity
        self.kinds = kinds
        self.includeResolved = includeResolved
        self.window = window
        self.country = country
    }

    /// The conflicts layer (ADR 27): every conflict the server still shows. No time window: the
    /// server keeps each one 90 days after its end date, and the dates are months old.
    public static let conflicts = EventQuery(types: [.conflict], kinds: [.observed], window: nil)

    public func matches(_ e: HazardEvent, now: Date = .now) -> Bool {
        guard types.contains(e.type), e.severity >= minimumSeverity, kinds.contains(e.kind) else { return false }
        if !includeResolved, e.status == .resolved { return false }
        // A resolved event by when it ended (else its last update), an active one by its last
        // update: the server's rule (PR #23), so a live patch agrees with the next load.
        let windowed = e.isResolved ? (e.endedAt ?? e.updatedAt) : e.updatedAt
        if let window, now.timeIntervalSince(windowed) > window { return false }
        if let country, !e.countries.contains(where: { $0.iso2 == country }) { return false }
        return true
    }
}

/// The seam between UI and data. Today `SampleEventProvider`; after BACKLOG S2.1 an
/// implementation over the generated `shield-api` client. Swapping is a one-line change.
public protocol HazardEventProvider: Sendable {
    /// True when everything this provider returns is sample data. The UI must then show
    /// `SampleDataBadge` (DESIGN.md §15).
    var isSampleData: Bool { get }
    func events(matching query: EventQuery) async throws -> [HazardEvent]
    /// The map's view: single events plus server clusters below zoom 6 (ADR 9). Sorted for
    /// display (most severe, then most recently updated).
    func snapshot(matching query: EventQuery, in viewport: MapViewport) async throws -> MapSnapshot
    func event(id: UUID) async throws -> HazardEvent?
    /// The event with its full geometry, for drawing its area when selected.
    func detail(id: UUID) async throws -> EventDetail?
    func sources() async throws -> [Source]
    /// Every country with its count of active events, by name (S3.4).
    func countries() async throws -> [Country]
    /// One country's overview; nil when the server has no boundary for `iso2` (404).
    func countrySummary(iso2: String) async throws -> CountrySummary?
    /// Satellite thermal detections in view: grid cells below zoom 6, points from zoom 6.
    func detections(in viewport: MapViewport) async throws -> DetectionLayer
    /// Live aircraft in `box` (ADR 28); the box must touch at most 4 cells of 5°
    /// (`AircraftArea.fits`). Defaults to none.
    func aircraft(in box: GeoBox) async throws -> AircraftLayer
    /// Whether `aircraft(in:)` has anything behind it; the map offers the layer only then.
    var supportsAircraft: Bool { get }
    /// Whether `changes(after:)` streams anything (S3.6). Defaults to false.
    var supportsLiveChanges: Bool { get }
    /// Live updates after `cursor`. Ends or throws when the connection drops; the caller
    /// reconnects with the last cursor it saw. Throws `StreamUnavailable` on 429/503.
    func changes(after cursor: String?) -> AsyncThrowingStream<LiveUpdate, any Error>
}

extension HazardEventProvider {
    public func aircraft(in box: GeoBox) async throws -> AircraftLayer { .empty }
    public var supportsAircraft: Bool { false }
}
