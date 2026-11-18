import Foundation
import Testing
@testable import ShieldCore

/// A provider whose answers the test controls: each viewport gets the events it was told to
/// return, optionally after a delay, or an error.
private final class ScriptedProvider: HazardEventProvider, @unchecked Sendable {
    // @unchecked: test-only; `lock` guards the mutable state.
    private let lock = NSLock()
    private var _requests: [MapViewport] = []
    private var _fireRequests = 0
    var delays: [Int: Duration] = [:]           // by zoom
    var areaDelays: [UUID: Duration] = [:]
    var areaFailure: (any Error)?
    var failure: (any Error)?
    var fireFailure: (any Error)?

    var requests: [MapViewport] { lock.withLock { _requests } }
    var fireRequests: Int { lock.withLock { _fireRequests } }

    let isSampleData = false

    func events(matching query: EventQuery) async throws -> [HazardEvent] { [] }
    func event(id: UUID) async throws -> HazardEvent? { nil }
    func sources() async throws -> [Source] {
        // EONET is stale in the sample sources; USGS is fresh.
        SampleData.sources(now: Date(timeIntervalSince1970: 1_794_838_800))
    }

    func snapshot(matching query: EventQuery, in viewport: MapViewport) async throws -> MapSnapshot {
        lock.withLock { _requests.append(viewport) }
        if let delay = delays[viewport.zoom] { try await Task.sleep(for: delay) }
        if let failure { throw failure }
        // The zoom is echoed through `truncated`/count so tests can tell answers apart.
        let events = Array(SampleData.events(now: .init(timeIntervalSince1970: 0)).prefix(viewport.zoom))
        return MapSnapshot(events: events, clusters: [], truncated: false, generatedAt: .init(timeIntervalSince1970: 0))
    }

    /// A square around each event's centroid, so tests can tell answers apart.
    func detail(id: UUID) async throws -> EventDetail? {
        if let delay = areaDelays[id] { try await Task.sleep(for: delay) }
        if let areaFailure { throw areaFailure }
        guard let e = SampleData.events(now: .init(timeIntervalSince1970: 0)).first(where: { $0.id == id }) else { return nil }
        let c = e.centroid
        let ring = [(0.0, 0.0), (1, 0), (1, 1), (0, 1), (0, 0)].map {
            Coordinate(latitude: c.latitude + $0.1, longitude: c.longitude + $0.0)
        }
        return EventDetail(event: e, geometry: .polygons([[ring]]))
    }

    func detections(in viewport: MapViewport) async throws -> DetectionLayer {
        lock.withLock { _fireRequests += 1 }
        if let fireFailure { throw fireFailure }
        return .points([], truncated: true)
    }
}

private func viewport(zoom: Int) -> MapViewport {
    MapViewport(box: .world, zoom: zoom)
}

@MainActor @Suite struct LiveMapModelTests {
    let at = Date(timeIntervalSince1970: 1_794_838_800)

    @Test func debouncesViewportChangesIntoOneRequest() async {
        let p = ScriptedProvider()
        let m = LiveMapModel(provider: p, debounce: .milliseconds(50), now: { [at] in at })
        m.viewportDidChange(viewport(zoom: 2))
        m.viewportDidChange(viewport(zoom: 3))
        m.viewportDidChange(viewport(zoom: 4))
        await m.settle()
        #expect(p.requests.map(\.zoom) == [4])
        #expect(m.snapshot?.events.count == 4)
        #expect(m.phase == .loaded)
        #expect(m.lastSuccess == at)
    }

    @Test func aSlowOlderResponseNeverOverwritesANewerView() async {
        let p = ScriptedProvider()
        p.delays[2] = .milliseconds(300)
        let m = LiveMapModel(provider: p, debounce: .zero)
        m.viewportDidChange(viewport(zoom: 2))
        try? await Task.sleep(for: .milliseconds(20)) // zoom 2 is in flight
        m.viewportDidChange(viewport(zoom: 3))
        await m.settle()
        try? await Task.sleep(for: .milliseconds(350)) // let the old request finish, if it would
        #expect(m.snapshot?.events.count == 3)
    }

    @Test func aFailureKeepsTheLastGoodDataAndSaysWhy() async {
        let p = ScriptedProvider()
        let m = LiveMapModel(provider: p, debounce: .zero)
        m.viewportDidChange(viewport(zoom: 2))
        await m.settle()
        let good = m.lastSuccess

        p.failure = URLError(.notConnectedToInternet)
        m.reload()
        await m.settle()
        #expect(m.phase == .failed(.offline))
        #expect(m.snapshot?.events.count == 2)
        #expect(m.lastSuccess == good)

        p.failure = nil
        m.reload()
        await m.settle()
        #expect(m.phase == .loaded)
    }

    @Test func detectionsOnlyLoadWhenShownAndTheirFailureLeavesEventsAlone() async {
        let p = ScriptedProvider()
        let m = LiveMapModel(provider: p, debounce: .zero)
        m.viewportDidChange(viewport(zoom: 2))
        await m.settle()
        #expect(p.fireRequests == 0)

        p.fireFailure = URLError(.badServerResponse)
        m.showsDetections = true
        await m.settle()
        #expect(p.fireRequests == 1)
        #expect(m.phase == .loaded)
        #expect(m.detectionsFailed)

        p.fireFailure = nil
        m.reload()
        await m.settle()
        #expect(!m.detectionsFailed)
        #expect(m.detections == .points([], truncated: true))

        m.showsDetections = false
        #expect(m.detections == .empty)
    }

    @Test func staleSourcesFadeTheirEventsAndOldDataIsStale() async throws {
        var clock = at
        let p = ScriptedProvider()
        let m = LiveMapModel(provider: p, debounce: .zero, now: { [clock] in clock })
        m.viewportDidChange(viewport(zoom: 9))
        await m.settle()
        let events = try #require(m.snapshot?.events)
        let eonet = try #require(events.first { $0.source == "eonet" })
        let usgs = try #require(events.first { $0.source == "usgs" })
        #expect(m.isStale(eonet))
        #expect(!m.isStale(usgs))
        #expect(!m.dataIsStale)

        clock = at.addingTimeInterval(181)
        let later = LiveMapModel(provider: p, debounce: .zero, now: { [clock] in clock })
        #expect(!later.dataIsStale) // nothing loaded yet is "loading", not stale
    }

    @Test func selectingAnEventLoadsItsAreaAndDeselectingClearsIt() async throws {
        let p = ScriptedProvider()
        let m = LiveMapModel(provider: p, debounce: .zero)
        let e = try #require(SampleData.events(now: .init(timeIntervalSince1970: 0)).first)
        m.selectedEventID = e.id
        await m.settleArea()
        guard case .polygons(let polygons)? = m.selectedGeometry else {
            Issue.record("expected a polygon, got \(String(describing: m.selectedGeometry))"); return
        }
        #expect(polygons.first?.first?.first == e.centroid)
        m.selectedEventID = nil
        #expect(m.selectedGeometry == nil)
    }

    @Test func aSlowAreaForAnOldSelectionIsDropped() async throws {
        let p = ScriptedProvider()
        let events = SampleData.events(now: .init(timeIntervalSince1970: 0))
        let (a, b) = (events[0], events[1])
        p.areaDelays[a.id] = .milliseconds(200)
        let m = LiveMapModel(provider: p, debounce: .zero)
        m.selectedEventID = a.id
        m.selectedEventID = b.id
        await m.settleArea()
        try? await Task.sleep(for: .milliseconds(250)) // a's answer would land now, if it could
        guard case .polygons(let polygons)? = m.selectedGeometry else {
            Issue.record("expected b's polygon"); return
        }
        #expect(polygons.first?.first?.first == b.centroid)
    }

    @Test func aFailedAreaDrawsNothingAndLeavesTheMapAlone() async throws {
        let p = ScriptedProvider()
        p.areaFailure = URLError(.badServerResponse)
        let m = LiveMapModel(provider: p, debounce: .zero)
        m.viewportDidChange(viewport(zoom: 2))
        await m.settle()
        m.selectedEventID = try #require(m.snapshot?.events.first?.id)
        await m.settleArea()
        #expect(m.selectedGeometry == nil)
        #expect(m.phase == .loaded)
    }

    @Test func aPointHasNoShapeToDraw() async throws {
        // The protocol default (sample data) answers with the centroid.
        let sample = SampleEventProvider()
        let e = try #require(try await sample.events(matching: EventQuery(window: nil)).first)
        let d = try #require(try await sample.detail(id: e.id))
        #expect(!d.geometry.hasShape)
    }

    @Test func describesFailuresInWordsForPeople() {
        #expect(LoadFailure(URLError(.notConnectedToInternet)) == .offline)
        #expect(LoadFailure(URLError(.cannotConnectToHost)).message == "Shield’s server isn’t responding.")
        struct Opaque: Error {}
        #expect(LoadFailure(Opaque()).message == "Something went wrong loading events.")
        // A library error with a debug-dump description stays generic.
        struct Dump: LocalizedError { var errorDescription: String? { "Unexpected response body, expected…" } }
        #expect(LoadFailure(Dump()).message == "Something went wrong loading events.")
        struct Worded: UserFacingError { var userMessage: String { "Shield’s server had a problem (503)." } }
        #expect(LoadFailure(Worded()).message == "Shield’s server had a problem (503).")
    }
}
