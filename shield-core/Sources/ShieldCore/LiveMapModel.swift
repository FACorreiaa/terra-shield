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
    /// The same sources in the API's order, for the Sources screen (DESIGN.md §10.7).
    public private(set) var sourceList: [Source] = []
    /// The Sources screen's last refresh failed; the list above is the last one that loaded.
    public private(set) var sourcesRefreshFailed = false

    /// Connected to `/v1/stream` and caught up (S3.6), and the stream has spoken recently.
    /// While live the data is current without a reload: the stream pushes every change and a
    /// heartbeat every 20 s. A half-open socket (sleep, network change) goes quiet, so silence
    /// past `liveSilence` ends it, without waiting for the connection to fail.
    public var isLive: Bool {
        guard streamReady, let lastStreamActivity else { return false }
        return now().timeIntervalSince(lastStreamActivity) < Self.liveSilence.seconds
    }

    /// Two missed heartbeats, plus slack.
    public static let liveSilence: Duration = .seconds(45)

    /// When the stream last delivered anything, a heartbeat included: the data was confirmed
    /// current then.
    public private(set) var lastStreamActivity: Date?
    private var streamReady = false

    /// When the map's data was last known current: a load, or a live stream's latest word.
    public var lastUpdate: Date? {
        guard isLive, let lastStreamActivity else { return lastSuccess }
        return max(lastSuccess ?? lastStreamActivity, lastStreamActivity)
    }
    /// The last stream cursor seen, sent back as `Last-Event-ID` on reconnect.
    public private(set) var streamCursor: String?

    /// How often the map reloads while visible. 60 s is the shortest source poll interval.
    public static let refreshInterval: Duration = .seconds(60)

    /// The filters (DESIGN.md §10.6), saved across launches when the model has a store.
    /// A query that shows nothing (no type or no kind) is refused: the API reads an empty list as
    /// "all", and the sheet never offers one. Heat detections stay out of `kinds`; they are
    /// `showsDetections`.
    public var query = EventQuery() {
        didSet {
            var q = query
            q.kinds.remove(.detection)
            if q.types.isEmpty || q.kinds.isEmpty { q = oldValue }
            if q != query { query = q } // no observer re-entry: didSet doesn't fire for this
            guard query != oldValue, !restoring else { return }
            save()
            reload()
        }
    }

    /// Armed conflicts in view (ADR 27): their own snapshot, so they never join the hazard
    /// counts, clusters or list. Nil while the layer is off.
    public private(set) var conflicts: MapSnapshot?
    /// The conflicts layer's last load failed; the hazards are unaffected.
    public private(set) var conflictsFailed = false

    /// The conflicts layer; off by default. Saved with the filters.
    public var showsConflicts = false {
        didSet {
            guard showsConflicts != oldValue, !restoring else { return }
            if !showsConflicts { conflicts = nil; conflictsFailed = false }
            save()
            reload()
        }
    }

    /// Every military base listed on Wikidata (ADR 29), loaded worldwide once: a static layer, so
    /// a pan never asks again. Nil while off or before the first answer.
    public private(set) var bases: BaseLayer?
    /// The bases layer's last load failed; retried by a map load a minute or more later.
    public private(set) var basesFailed = false
    private var basesTask: Task<Void, Never>?
    private var basesGeneration = 0
    private var basesFailedAt: Date?

    /// The bases layer; off by default. Saved with the filters (no quota behind it).
    public var showsBases = false {
        didSet {
            guard showsBases != oldValue, !restoring else { return }
            save()
            // Hazards are not reloaded: the layer is independent of them.
            if showsBases {
                startBases()
            } else {
                basesTask?.cancel()
                basesTask = nil
                basesGeneration += 1
                bases = nil
                basesFailed = false
                basesFailedAt = nil
                selectedBaseID = nil
            }
        }
    }

    /// The base shown in the sheet or inspector, by Wikidata ID.
    public var selectedBaseID: String? {
        didSet {
            guard selectedBaseID != oldValue, selectedBaseID != nil else { return }
            clearSelections(except: .base)
        }
    }

    public var selectedBase: MilitaryBase? {
        guard let selectedBaseID else { return nil }
        return bases?.bases.first { $0.id == selectedBaseID }
    }

    /// Historic sites in view (ADR 30): cells below zoom 8, points from zoom 8. Nil while off or
    /// before the first answer; a failed request keeps the last answer on screen.
    public private(set) var historic: HistoricLayer?
    /// The last historic request failed; retried by a map load a minute or more after it was asked.
    public private(set) var historicFailed = false
    private var historicTask: Task<Void, Never>?
    private var historicGeneration = 0
    /// What `historic` answers, so a view inside it is not asked again (the layer changes weekly).
    private var historicAnswered: HistoricRequest?
    /// The request in flight, if any.
    private var historicAsking: HistoricRequest?
    private var historicFailedAt: Date?

    /// The historic layer; off by default. Saved with the filters (no quota behind it).
    public var showsHistoric = false {
        didSet {
            guard showsHistoric != oldValue, !restoring else { return }
            save()
            // Hazards are not reloaded: the layer is independent of them.
            if showsHistoric {
                startHistoric()
            } else {
                resetHistoric()
                selectedHistoricID = nil
            }
        }
    }

    /// The groups shown, never none: the API reads an empty `kind` as every kind, so turning the
    /// last one off is refused. Saved with the filters.
    public var historicKinds: Set<HistoricKind> = Set(HistoricKind.allCases) {
        didSet {
            if historicKinds.isEmpty { historicKinds = oldValue } // no observer re-entry
            guard historicKinds != oldValue, !restoring else { return }
            if let kind = selectedHistoricSite?.kind, !historicKinds.contains(kind) { selectedHistoricID = nil }
            save()
            guard showsHistoric else { return }
            // A deliberate change: asked at once, even within a minute of a failure.
            historicFailedAt = nil
            startHistoric()
        }
    }

    /// The site shown in the sheet or inspector, by Wikidata ID.
    public var selectedHistoricID: String? {
        didSet {
            guard selectedHistoricID != oldValue else { return }
            if selectedHistoricID != nil { clearSelections(except: .historic) }
            chosenSite = selectedHistoricID.flatMap { id in historic?.sites.first { $0.id == id } }
        }
    }

    /// The selected site, kept as last seen while the answer no longer holds it: a pan replaces the
    /// answer, zooming out turns it into a grid, and the iPhone sheet shrinks the view over it.
    public var selectedHistoricSite: HistoricSite? {
        guard let selectedHistoricID else { return nil }
        return historic?.sites.first { $0.id == selectedHistoricID }
            ?? chosenSite.flatMap { $0.id == selectedHistoricID ? $0 : nil }
    }

    private var chosenSite: HistoricSite?
    /// The map has reported a view; until then `viewport` is only the default.
    private var hasViewport = false

    private enum SelectionKind { case event, aircraft, base, historic }

    /// One thing selected at a time: an event, an aircraft, a base or a historic site.
    private func clearSelections(except kept: SelectionKind) {
        if kept != .event, selectedEventID != nil { selectedEventID = nil }
        if kept != .aircraft, selectedAircraftHex != nil { selectedAircraftHex = nil }
        if kept != .base, selectedBaseID != nil { selectedBaseID = nil }
        if kept != .historic, selectedHistoricID != nil { selectedHistoricID = nil }
    }

    /// Live flights in view (ADR 28); nil while off, before the first answer, or while the view
    /// is too large.
    public private(set) var flights: AircraftLayer?
    /// The last flights request failed; the positions shown, if any, are the previous answer's.
    public private(set) var flightsFailed = false
    /// The view touches more than 4 cells of 5°: the layer asks to zoom in instead of asking.
    public private(set) var flightsNeedZoom = false
    /// The box last asked for, answered or not (a failure, a refusal): a reload of the same view
    /// doesn't ask again; only the minute timer does (`keepFlightsFresh`), so hazard polls,
    /// Refresh and stream bursts never spend the feed's shared quota.
    private var flightsBox: GeoBox?
    /// The box `flights` answers for.
    private var flightsShownBox: GeoBox?
    /// When the last flights request started: the timer waits a full minute from it.
    private var flightsAskedAt: Date?
    /// The selected aircraft as last seen, kept while it is out of the answer (the iPhone sheet
    /// shrinks the view over it) until it is closed or the layer goes off.
    private var chosenAircraft: Aircraft?
    private var flightsTask: Task<Void, Never>?
    private var flightsGeneration = 0
    /// No faster than once a minute for the same view (ADR 28; the server refreshes each cell at
    /// most once a minute anyway).
    public static let flightsInterval: Duration = .seconds(60)

    /// The flights layer; off by default and at every launch: never saved, so a run can't start
    /// spending the feed's daily quota on its own.
    public var showsFlights = false {
        didSet {
            guard showsFlights != oldValue else { return }
            // Hazards are not reloaded: the layer is independent of them.
            if showsFlights {
                startFlights(for: viewport, force: false)
            } else {
                flightsTask?.cancel()
                flightsGeneration += 1
                flights = nil
                flightsFailed = false
                flightsNeedZoom = false
                flightsBox = nil
                flightsShownBox = nil
                flightsAskedAt = nil
                selectedAircraftHex = nil
            }
        }
    }

    /// The aircraft shown in the sheet or inspector, by address. Selecting one clears the event,
    /// and the other way round.
    public var selectedAircraftHex: String? {
        didSet {
            if selectedAircraftHex != nil { clearSelections(except: .aircraft) }
            chosenAircraft = selectedAircraftHex.flatMap { hex in flights?.aircraft.first { $0.hex == hex } }
        }
    }

    public var selectedAircraft: Aircraft? {
        guard let selectedAircraftHex else { return nil }
        return flights?.aircraft.first { $0.hex == selectedAircraftHex }
            ?? chosenAircraft.flatMap { $0.hex == selectedAircraftHex ? $0 : nil }
    }

    /// The FIRMS layer; off by default (DESIGN.md §5). Saved with the filters.
    public var showsDetections = false {
        didSet {
            guard showsDetections != oldValue, !restoring else { return }
            if !showsDetections { detections = .empty; detectionsFailed = false }
            save()
            reload()
        }
    }

    public var selectedEventID: UUID? {
        didSet {
            guard selectedEventID != oldValue else { return }
            if selectedEventID != nil { clearSelections(except: .event) }
            loadSelectedArea()
        }
    }

    /// What the event detail (DESIGN.md §10.3) shows for the selection; nil with nothing selected.
    public enum DetailState: Sendable, Hashable {
        case loading
        case loaded(EventDetail)
        /// The source no longer publishes it: a 404, or retracted on the stream. Carries the
        /// event as last seen, if it was, so the detail can still say what it was.
        case gone(HazardEvent?)
        /// The first load failed. A failed refresh keeps the last detail instead.
        case failed
    }

    public private(set) var selectedDetail: DetailState?

    /// The selected event's footprint, drawn under the markers (DESIGN.md §5). `nil` while
    /// loading, for a plain point, or if the detail could not be loaded: the marker and card
    /// still show, so a failure here is not worth a banner.
    public var selectedGeometry: EventGeometry? {
        guard case .loaded(let detail) = selectedDetail, detail.geometry.hasShape else { return nil }
        return detail.geometry
    }

    /// The event's source has missed three polls (DECISIONS #8): fade it, don't hide it.
    public func isStale(_ event: HazardEvent) -> Bool {
        sources[event.source]?.isStale(now: now()) ?? false
    }

    /// The map itself has not refreshed for three refresh intervals, e.g. after failures.
    public var dataIsStale: Bool {
        if isLive { return false }
        guard let lastSuccess else { return false }
        return now().timeIntervalSince(lastSuccess) > 3 * Self.refreshInterval.seconds
    }

    public var selectedEvent: HazardEvent? {
        guard let selectedEventID else { return nil }
        if let shown = snapshot?.events.first(where: { $0.id == selectedEventID }) { return shown }
        if let conflict = conflicts?.events.first(where: { $0.id == selectedEventID }) { return conflict }
        if chosenEvent?.id == selectedEventID { return chosenEvent }
        // Out of the snapshot (panned away, joined a cluster, opened by id, or gone): the detail's
        // copy keeps it on screen.
        switch selectedDetail {
        case .loaded(let detail): return detail.event
        case .gone(let last?): return last
        default: return nil
        }
    }

    /// An event chosen outside the map's own list (a country's overview), kept so it can be shown
    /// when a cluster or the map's filters leave it out of the snapshot. The map's copy wins
    /// whenever there is one, since it is the fresher.
    private var chosenEvent: HazardEvent?

    /// Selects `event` even if the map doesn't show it.
    public func select(_ event: HazardEvent) {
        chosenEvent = event
        selectedEventID = event.id
    }

    private let debounce: Duration
    private let now: @Sendable () -> Date
    private var loadTask: Task<Void, Never>?
    private var areaTask: Task<Void, Never>?
    private var generation = 0

    /// `filters`: where the person's filters are kept; `nil` keeps them for this run only.
    public init(
        provider: any HazardEventProvider,
        filters: FilterStore? = nil,
        debounce: Duration = .milliseconds(350),
        now: @escaping @Sendable () -> Date = { .now }
    ) {
        self.provider = provider
        self.filterStore = filters
        self.debounce = debounce
        self.now = now
        // @Observable runs observers even in init, so `restoring` keeps this from saving back
        // or loading before the map has reported a viewport.
        if let saved = filters?.load() {
            restoring = true
            query = saved.query
            showsDetections = saved.showsDetections
            showsConflicts = saved.showsConflicts
            // Not where the provider has none (sample data): its toggle is hidden there, so the
            // legend could never be closed.
            showsBases = saved.showsBases && provider.supportsBases
            showsHistoric = saved.showsHistoric && provider.supportsHistoric
            historicKinds = saved.historicKinds
            restoring = false
        }
    }

    private let filterStore: FilterStore?
    @ObservationIgnored private var restoring = false

    private func save() {
        filterStore?.save(SavedFilters(query: query, showsDetections: showsDetections, showsConflicts: showsConflicts,
                                       showsBases: showsBases, showsHistoric: showsHistoric,
                                       historicKinds: historicKinds))
    }

    /// The map settled on a new region. Loads after `debounce` unless another change follows.
    public func viewportDidChange(_ viewport: MapViewport) {
        guard viewport != self.viewport || snapshot == nil else { return }
        self.viewport = viewport
        hasViewport = true
        schedule(after: debounce)
    }

    /// Loads now, replacing any pending or running load.
    public func reload() {
        schedule(after: .zero)
    }

    /// Keeps the map current until the calling task is cancelled (attach to the view's
    /// lifetime): follows the live stream when the provider has one, and reloads every
    /// `interval` regardless, so a silently dead stream can never freeze the map. 60 s matches
    /// the shortest source poll interval; the server caches 15 s.
    public func keepFresh(every interval: Duration = LiveMapModel.refreshInterval) async {
        // The stream task ends with this one: cancelling the view's task ends the poll loop,
        // and the defer cancels the stream.
        let stream: Task<Void, Never>? = provider.supportsLiveChanges ? Task { await self.followChanges() } : nil
        defer { stream?.cancel() }
        while !Task.isCancelled {
            try? await Task.sleep(for: interval)
            if Task.isCancelled { return }
            reload()
        }
    }

    /// Follows `/v1/stream`, reconnecting with the last cursor until cancelled.
    ///
    /// Backoff after a failure: the server's `Retry-After` when it gave one (429, 503), else
    /// its `retry:` hint (5 s) doubling per consecutive failure up to 60 s. Never a banner:
    /// polling carries the map meanwhile.
    func followChanges(
        sleep: @escaping @Sendable (Duration) async -> Void = { try? await Task.sleep(for: $0) },
        jitter: @escaping @Sendable () -> Double = { .random(in: 0.8...1.2) }
    ) async {
        var failures = 0
        while !Task.isCancelled {
            var retryAfter: Duration?
            do {
                for try await update in provider.changes(after: streamCursor) {
                    if let cursor = update.cursor { streamCursor = cursor }
                    lastStreamActivity = now()
                    failures = 0
                    if let change = update.change { apply(change) }
                }
            } catch is CancellationError {
                break
            } catch let unavailable as StreamUnavailable {
                retryAfter = unavailable.retryAfter
            } catch {
                if Task.isCancelled { break }
            }
            streamReady = false
            if Task.isCancelled { break }
            failures += 1
            // ±20 % so clients dropped together (a pod shutting down) don't return together.
            // The server's Retry-After is taken as given.
            await sleep(retryAfter ?? Self.streamBackoff(failures: failures) * jitter())
        }
        streamReady = false
    }

    /// 5 s, 10 s, 20 s, 40 s, then 60 s.
    nonisolated static func streamBackoff(failures: Int) -> Duration {
        let seconds = 5 * pow(2, Double(max(failures, 1) - 1))
        return .seconds(min(seconds, 60))
    }

    /// Applies one change. Patched in place only when the snapshot is the complete, unclustered
    /// set of events in view: zoom 6 or more, and not truncated. Otherwise a change in view
    /// reloads (debounced, so a burst is one request), because only the server knows:
    /// - below zoom 6, whether the event joins a cluster (even where none exists yet);
    /// - when truncated, whether the event makes the cut.
    func apply(_ change: LiveChange) {
        switch change {
        case .ready:
            streamReady = true
            lastStreamActivity = now()
            // The stream is open before this load, so no change can fall between the two.
            reload()
        case .reset:
            reload()
        case .created(let event):
            if streamReady { noteArrival(event) }
            apply(.upsert(event))
        case .upsert(let event):
            // An event chosen outside the map's list keeps up too: it may never be in the snapshot.
            if chosenEvent?.id == event.id { chosenEvent = event }
            if event.id == selectedEventID { loadSelectedArea(keepingCurrent: true) }
            guard let snapshot else { return }
            let shown = snapshot.events.contains { $0.id == event.id }
            let inView = viewport.box.contains(event.centroid)
            guard shown || inView else { return }
            guard canPatch(snapshot), !inBurst() else {
                schedule(after: debounce)
                return
            }
            var events = snapshot.events.filter { $0.id != event.id }
            if inView, query.matches(event, now: now()) { events.append(event) }
            self.snapshot = snapshot.replacing(events: events.sortedForDisplay())
        case .removed(let id):
            // An open detail stays open and says so (DESIGN.md §10.3 "Gone"); closing it under
            // the reader would hide why the marker went.
            if id == selectedEventID {
                areaTask?.cancel()
                selectedDetail = .gone(selectedEvent)
            }
            guard let snapshot else { return }
            let shown = snapshot.events.contains { $0.id == id }
            if canPatch(snapshot), !(shown && inBurst()) {
                if shown { self.snapshot = snapshot.replacing(events: snapshot.events.filter { $0.id != id }) }
            } else if shown || !snapshot.clusters.isEmpty || snapshot.truncated {
                // It may be a cluster member, or its removal lets a cut event in.
                schedule(after: debounce)
            }
        }
    }

    // MARK: Arrivals (DESIGN.md §10.1 "New event", §13 `arrive`)

    /// Live arrivals in view the map hasn't animated yet; it calls `arrived(_:)` once it has.
    public private(set) var arrivals: Set<UUID> = []
    /// The newest severe or extreme arrival in view, until the view dismisses it.
    public private(set) var toast: EventToast?
    @ObservationIgnored private var arrivalTimes: [UUID: Date] = [:]

    public func arrived(_ id: UUID) {
        arrivals.remove(id)
        arrivalTimes[id] = nil
    }

    public func dismissToast() { toast = nil }

    #if DEBUG
    /// Screenshot automation: a change as if the stream sent it.
    public func simulate(_ change: LiveChange) { apply(change) }
    #endif

    private func noteArrival(_ event: HazardEvent) {
        let t = now()
        guard viewport.box.contains(event.centroid), query.matches(event, now: t) else { return }
        // An arrival the map never drew (a reload left it in a cluster) is forgotten after a few
        // seconds, so the set can't grow.
        for (id, at) in arrivalTimes where t.timeIntervalSince(at) > 5 { arrived(id) }
        arrivals.insert(event.id)
        arrivalTimes[event.id] = t
        if event.severity >= .severe { toast = EventToast(event: event) }
    }

    private func canPatch(_ snapshot: MapSnapshot) -> Bool {
        viewport.zoom >= MapViewport.detailZoom && !snapshot.truncated
    }

    /// More patches than this within a second is a burst: a source's first load (Meteoalarm's
    /// sends ~3,900 at once) or a backlog after a reconnect. Each patch re-sorts the snapshot
    /// and makes the map diff its annotations, so a burst would hold the main actor for
    /// seconds; past the limit the changes fold into one debounced reload instead, which the
    /// server answers with the same state in a single response.
    static let burstLimit = 50

    @ObservationIgnored private var burstStart = Date.distantPast
    @ObservationIgnored private var burstCount = 0

    /// Counts a patch about to happen; true once this second has seen more than `burstLimit`.
    private func inBurst() -> Bool {
        let t = now()
        if t.timeIntervalSince(burstStart) > 1 {
            burstStart = t
            burstCount = 0
        }
        burstCount += 1
        return burstCount > Self.burstLimit
    }

    /// The Sources screen asks on open and on pull-to-refresh, outside the map's load.
    public func refreshSources() async {
        do {
            setSources(try await provider.sources())
            sourcesRefreshFailed = false
        } catch {
            if LoadFailure.isCancellation(error) { return }
            sourcesRefreshFailed = true
        }
    }

    private func setSources(_ list: [Source]) {
        sourceList = list
        sources = Dictionary(list.map { ($0.id, $0) }, uniquingKeysWith: { a, _ in a })
    }

    /// Waits for the current load, if any (tests, pull-to-refresh).
    public func settle() async {
        await loadTask?.value
        await flightsTask?.value
        await basesTask?.value
        await historicTask?.value
    }

    /// Waits for the map's load only, not the flights (tests).
    func settleMap() async {
        await loadTask?.value
    }

    private func schedule(after delay: Duration) {
        loadTask?.cancel()
        generation += 1
        let generation = generation
        loadTask = Task { [weak self] in
            if delay > .zero { try? await Task.sleep(for: delay) }
            // Replaced before it started (a later reload in the same turn): skip the request.
            if Task.isCancelled { return }
            await self?.load(generation: generation)
        }
    }

    private func load(generation: Int) async {
        let viewport = viewport, query = query, wantsDetections = showsDetections, provider = provider
        let wantsConflicts = showsConflicts
        // Flights run on their own: the server waits on the feed, and hazards must not.
        startFlights(for: viewport, force: false)
        // A layer restored as on loads with the first map load; a failure waits a minute.
        if showsBases, bases == nil, basesTask == nil { startBases() }
        // Asks only when the view has left what the last answer covers.
        startHistoric()
        async let events = Self.capture { try await provider.snapshot(matching: query, in: viewport) }
        async let fires = Self.detections(from: provider, in: viewport, if: wantsDetections)
        async let conflictList = Self.conflicts(from: provider, in: viewport, if: wantsConflicts)
        async let sourceList = Self.capture { try await provider.sources() }
        let (eventResult, fireResult, conflictResult, sourceResult) = await (events, fires, conflictList, sourceList)
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
        if case .success(let list) = sourceResult { setSources(list) }
        switch conflictResult {
        case .success(let layer)?:
            // Off again while it loaded: drop it.
            if showsConflicts { conflicts = layer }
            conflictsFailed = false
        case .failure(let error)?:
            if !LoadFailure.isCancellation(error) { conflictsFailed = true }
        case nil:
            break
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

    /// Fetches the selected event's detail. A newer selection cancels the older fetch, and a
    /// late answer for an event no longer selected is dropped.
    private func loadSelectedArea(keepingCurrent: Bool = false) {
        areaTask?.cancel()
        guard let id = selectedEventID else {
            selectedDetail = nil
            return
        }
        // A refetch for the same event (a live update) keeps what is shown until the new answer
        // arrives, so nothing flickers; a new selection starts from loading.
        if !keepingCurrent || selectedDetail == nil { selectedDetail = .loading }
        let provider = provider
        areaTask = Task { [weak self] in
            let result = await Self.capture { try await provider.detail(id: id) }
            guard let self, !Task.isCancelled, self.selectedEventID == id else { return }
            switch result {
            case .success(let detail?):
                if self.chosenEvent?.id == id { self.chosenEvent = detail.event }
                self.selectedDetail = .loaded(detail)
            case .success(nil):
                // Gone needs the event as last seen to say what it was; an id never seen (a
                // stale link) has nothing to show, so the selection goes.
                if let last = self.selectedEvent {
                    self.selectedDetail = .gone(last)
                } else {
                    self.selectedEventID = nil
                }
            case .failure:
                if case .loaded = self.selectedDetail { return }
                self.selectedDetail = .failed
            }
        }
    }

    /// Waits for an in-flight area fetch (tests).
    func settleArea() async {
        await areaTask?.value
    }

    private func resetHistoric() {
        historicTask?.cancel()
        historicTask = nil
        historicGeneration += 1
        historic = nil
        historicFailed = false
        historicFailedAt = nil
        historicAnswered = nil
        historicAsking = nil
    }

    /// Asks for the current view's sites unless the last answer, or the request in flight, already
    /// covers it. A failure waits a minute from when it was asked, as the bases layer does.
    private func startHistoric() {
        guard showsHistoric, provider.supportsHistoric else { return }
        // Not before the map has reported a view: the default is the whole world at zoom 1.
        guard hasViewport else { return }
        if let historicAnswered, let historic,
           historicAnswered.serves(viewport, kinds: historicKinds, truncated: historic.isTruncated,
                                   sites: historic.sites.count) { return }
        if historicTask != nil, let historicAsking,
           historicAsking.serves(viewport, kinds: historicKinds, truncated: false) { return }
        if let historicFailedAt, now().timeIntervalSince(historicFailedAt) < 60 { return }
        historicTask?.cancel()
        historicGeneration += 1
        let generation = historicGeneration, provider = provider, askedAt = now()
        let wanted = HistoricRequest(viewport, kinds: historicKinds)
        historicAsking = wanted
        historicTask = Task { [weak self] in
            let result = await Self.capture { try await provider.historic(wanted) }
            guard let self, generation == self.historicGeneration, !Task.isCancelled, self.showsHistoric else { return }
            self.historicTask = nil
            self.historicAsking = nil
            switch result {
            case .success(let layer):
                self.historic = layer
                self.historicAnswered = wanted
                self.historicFailed = false
                self.historicFailedAt = nil
            case .failure(let error):
                if LoadFailure.isCancellation(error) { return }
                self.historicFailed = true
                self.historicFailedAt = askedAt
            }
        }
    }

    private func startBases() {
        guard provider.supportsBases else { return }
        if let basesFailedAt, now().timeIntervalSince(basesFailedAt) < 60 { return }
        basesTask?.cancel()
        basesGeneration += 1
        // A failure is dated from the request, so the map's minute poll is the one that retries.
        let generation = basesGeneration, provider = provider, askedAt = now()
        basesTask = Task { [weak self] in
            let result = await Self.capture { try await provider.bases() }
            guard let self, generation == self.basesGeneration, !Task.isCancelled, self.showsBases else { return }
            self.basesTask = nil
            switch result {
            case .success(let layer):
                self.bases = layer
                self.basesFailed = false
                self.basesFailedAt = nil
            case .failure(let error):
                if LoadFailure.isCancellation(error) { return }
                self.basesFailed = true
                self.basesFailedAt = askedAt
            }
        }
    }

    /// Asks for the flights in view now, the same view included (the minute timer, tests).
    public func refreshFlights() async {
        startFlights(for: viewport, force: true)
        await flightsTask?.value
    }

    /// Keeps the flights current while the layer is on: one request a minute for the view on
    /// screen (ADR 28). Attach to the view with the layer's state as its id.
    public func keepFlightsFresh(
        sleep: @escaping @Sendable (Duration) async -> Void = { try? await Task.sleep(for: $0) }
    ) async {
        let interval = TimeInterval(Self.flightsInterval.components.seconds)
        func elapsed() -> TimeInterval {
            flightsAskedAt.map { now().timeIntervalSince($0) } ?? interval
        }
        while showsFlights, !Task.isCancelled {
            // A view change asks at once and restarts the minute, so wait from the last request;
            // with nothing asked yet (a view too large), a whole minute.
            let wait = flightsAskedAt == nil ? interval : min(interval, max(1, (interval - elapsed()).rounded(.up)))
            await sleep(.seconds(wait))
            guard showsFlights, !Task.isCancelled else { return }
            if elapsed() >= interval { await refreshFlights() }
        }
    }

    /// Starts a flights request unless the layer is off, the view is too large (which clears the
    /// layer), or — without `force` — this box was already asked for.
    private func startFlights(for viewport: MapViewport, force: Bool) {
        guard showsFlights else { return }
        let box = viewport.box.asSent
        guard AircraftArea.fits(box) else {
            needZoomForFlights()
            return
        }
        // Same view: keep its state, "zoom in" from the server included, until the timer asks.
        if !force, box == flightsBox { return }
        flightsNeedZoom = false
        flightsTask?.cancel()
        flightsGeneration += 1
        let generation = flightsGeneration, provider = provider
        flightsBox = box
        flightsAskedAt = now()
        flightsTask = Task { [weak self] in
            let result = await Self.capture { try await provider.aircraft(in: box) }
            guard let self, generation == self.flightsGeneration, !Task.isCancelled, self.showsFlights else { return }
            switch result {
            case .success(let layer):
                self.flights = layer
                self.flightsShownBox = box
                self.flightsFailed = false
                if let hex = self.selectedAircraftHex, let seen = layer.aircraft.first(where: { $0.hex == hex }) {
                    self.chosenAircraft = seen
                }
            case .failure(let error) where error is AircraftArea.TooLarge:
                self.needZoomForFlights()
                // Refused: not asked again for this view until the timer.
                self.flightsBox = box
            case .failure(let error):
                if LoadFailure.isCancellation(error) { return }
                self.flightsFailed = true
                // Another view's positions would be counted as this one's.
                if self.flightsShownBox != box {
                    self.flights = nil
                    self.flightsShownBox = nil
                }
            }
        }
    }

    private func needZoomForFlights() {
        flightsTask?.cancel()
        flightsGeneration += 1
        flightsNeedZoom = true
        flights = nil
        flightsShownBox = nil
        flightsFailed = false
        flightsBox = nil
        selectedAircraftHex = nil
    }

    private nonisolated static func conflicts(
        from provider: any HazardEventProvider, in viewport: MapViewport, if wanted: Bool
    ) async -> Result<MapSnapshot, any Error>? {
        guard wanted else { return nil }
        return await capture { try await provider.snapshot(matching: .conflicts, in: viewport) }
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
