import Foundation
import Testing
@testable import ShieldCore

// MARK: - Parser

@Suite struct SSEParserTests {
    private func messages(_ text: String, parser: inout SSEParser) -> [SSEMessage] {
        text.components(separatedBy: "\n").compactMap { parser.feed($0) }
    }

    @Test func parsesTheShapesTheServerSends() {
        var p = SSEParser()
        let out = messages("""
        retry: 5000

        id: 41
        event: event.updated
        data: {"id":"a"}

        id: 42

        id: 43
        event: ready
        data: {}

        """, parser: &p)
        #expect(out == [
            SSEMessage(id: "41", event: "event.updated", data: #"{"id":"a"}"#),
            SSEMessage(id: "43", event: "ready", data: "{}"),
        ])
        #expect(p.retryMilliseconds == 5000)
        #expect(p.lastEventID == "43")
    }

    @Test func theHeartbeatMovesTheCursorWithoutDispatching() {
        var p = SSEParser(lastEventID: "7")
        #expect(messages("id: 12\n\n", parser: &p).isEmpty)
        #expect(p.lastEventID == "12")
    }

    @Test func followsTheSpecForEdgeCases() {
        var p = SSEParser()
        let out = messages("""
        : a comment
        data:no space
        data:  two spaces keep one
        data

        event: custom
        data: x
        id: bad\0id

        """, parser: &p)
        #expect(out == [
            SSEMessage(id: nil, event: "message", data: "no space\n two spaces keep one\n"),
            SSEMessage(id: nil, event: "custom", data: "x"),
        ])
        #expect(p.lastEventID == nil) // an id with NUL is ignored
    }

    @Test func anEventWithoutDataIsNotDispatchedAndDoesNotLeak() {
        var p = SSEParser()
        let out = messages("event: orphan\n\ndata: y\n\n", parser: &p)
        #expect(out == [SSEMessage(id: nil, event: "message", data: "y")])
    }

    @Test func theByteDecoderSplitsEveryLineEndingAndReportsHeartbeats() {
        func run(_ text: String) -> [SSEStreamDecoder.Output] {
            var d = SSEStreamDecoder(lastEventID: "1")
            return Array(text.utf8).compactMap { d.feed($0) }
        }
        let message = SSEStreamDecoder.Output.message(SSEMessage(id: "3", event: "ready", data: "{}"))
        // LF, CRLF and bare CR all end lines; the empty line ends the message.
        #expect(run("id: 3\nevent: ready\ndata: {}\n\n") == [message])
        #expect(run("id: 3\r\nevent: ready\r\ndata: {}\r\n\r\n") == [message])
        #expect(run("id: 3\revent: ready\rdata: {}\r\r") == [message])
        // Every heartbeat is reported, a repeated id too: it proves the stream is alive
        // (the server repeats the cursor every 20 s while nothing changes).
        #expect(run("id: 7\n\nid: 7\n\n") == [.cursor("7"), .cursor("7")])
        // retry: alone (the connection's first block) is not a heartbeat.
        #expect(run("retry: 5000\n\n").isEmpty)
        // UTF-8 split across bytes survives.
        #expect(run("data: Évora\n\n") == [.message(SSEMessage(id: "1", event: "message", data: "Évora"))])
    }

    @Test func backsOffDoublingToAMinute() {
        #expect((1...6).map { LiveMapModel.streamBackoff(failures: $0) }
            == [.seconds(5), .seconds(10), .seconds(20), .seconds(40), .seconds(60), .seconds(60)])
    }
}

// MARK: - Model

/// Answers snapshots like `ScriptedProvider`, plus scripted stream connections.
private final class StreamingProvider: HazardEventProvider, @unchecked Sendable {
    // @unchecked: test-only; `lock` guards the mutable state.
    enum Connection {
        case updates([LiveUpdate], then: (any Error)?)
    }

    private let lock = NSLock()
    private var _connections: [Connection] = []
    private var _cursors: [String?] = []
    private var _requests = 0
    var clusters: [EventCluster] = []
    var truncated = false

    let isSampleData = false
    var supportsLiveChanges: Bool { true }

    var cursors: [String?] { lock.withLock { _cursors } }
    var requests: Int { lock.withLock { _requests } }
    func script(_ connections: [Connection]) { lock.withLock { _connections = connections } }

    func events(matching query: EventQuery) async throws -> [HazardEvent] { [] }
    func event(id: UUID) async throws -> HazardEvent? { nil }
    func sources() async throws -> [Source] { [] }
    func detections(in viewport: MapViewport) async throws -> DetectionLayer { .empty }

    func snapshot(matching query: EventQuery, in viewport: MapViewport) async throws -> MapSnapshot {
        lock.withLock { _requests += 1 }
        let events = SampleData.events(now: .init(timeIntervalSince1970: 0)).prefix(3)
        return MapSnapshot(events: Array(events).sortedForDisplay(), clusters: clusters, truncated: truncated,
                           generatedAt: .init(timeIntervalSince1970: 0))
    }

    func changes(after cursor: String?) -> AsyncThrowingStream<LiveUpdate, any Error> {
        let next: Connection? = lock.withLock {
            _cursors.append(cursor)
            return _connections.isEmpty ? nil : _connections.removeFirst()
        }
        return AsyncThrowingStream { continuation in
            guard case .updates(let updates, let error)? = next else {
                // Out of script: stay open (idle) until cancelled.
                continuation.onTermination = { _ in }
                return
            }
            for update in updates { continuation.yield(update) }
            continuation.finish(throwing: error)
        }
    }
}

/// Records each backoff and blocks from the `blockAt`-th one, so a test can inspect the loop.
private final class SleepRecorder: @unchecked Sendable {
    private let lock = NSLock()
    private var _slept: [Duration] = []
    let blockAt: Int

    init(blockAt: Int) { self.blockAt = blockAt }

    var slept: [Duration] { lock.withLock { _slept } }

    func sleep(_ d: Duration) async {
        let count = lock.withLock { _slept.append(d); return _slept.count }
        if count >= blockAt { try? await Task.sleep(for: .seconds(3600)) }
    }
}

private let epoch = Date(timeIntervalSince1970: 0)
private func sample(_ i: Int) -> HazardEvent { SampleData.events(now: epoch)[i] }

/// `e` moved to `lon,lat` and touched now, so it is inside the default 24 h window.
private func moved(_ e: HazardEvent, to c: Coordinate, title: String? = nil, now: Date) -> HazardEvent {
    var copy = e
    copy.centroid = c
    copy.updatedAt = now
    if let title { copy.title = title }
    return copy
}

/// A clock a test moves by hand.
private final class TestClock: @unchecked Sendable {
    private let lock = NSLock()
    private var _now: Date
    init(_ now: Date) { _now = now }
    var now: Date { lock.withLock { _now } }
    func advance(_ seconds: TimeInterval) { lock.withLock { _now += seconds } }
}

@MainActor @Suite struct LiveStreamModelTests {
    let now = Date(timeIntervalSince1970: 3600)

    private func loadedModel(_ p: StreamingProvider, zoom: Int = 9,
                             box: GeoBox = .world) async -> LiveMapModel {
        let m = LiveMapModel(provider: p, debounce: .zero, now: { [now] in now })
        m.viewportDidChange(MapViewport(box: box, zoom: zoom))
        await m.settle()
        return m
    }

    @Test func fromZoomSixChangesArePatchedInPlace() async throws {
        let p = StreamingProvider()
        let m = await loadedModel(p)
        let before = p.requests
        let shown = try #require(m.snapshot?.events.first)

        // Updated in view: replaced, not duplicated.
        m.apply(.upsert(moved(shown, to: shown.centroid, title: "Revised", now: now)))
        #expect(m.snapshot?.events.filter { $0.id == shown.id }.map(\.title) == ["Revised"])

        // New and in view: added.
        let fresh = moved(sample(3), to: Coordinate(latitude: 10, longitude: 10), now: now)
        m.apply(.upsert(fresh))
        #expect(m.snapshot?.events.contains { $0.id == fresh.id } == true)

        // Removed: gone.
        m.apply(.removed(fresh.id))
        #expect(m.snapshot?.events.contains { $0.id == fresh.id } == false)
        #expect(p.requests == before) // no reloads needed
    }

    @Test func anEventThatResolvesWhileWatchedLeavesOrFadesAsTheFilterSays() async throws {
        let p = StreamingProvider()
        let m = await loadedModel(p)
        let shown = try #require(m.snapshot?.events.first)
        var resolved = moved(shown, to: shown.centroid, now: now)
        resolved.status = .resolved

        // Active only (the default): it leaves the map.
        m.apply(.upsert(resolved))
        #expect(m.snapshot?.events.contains { $0.id == shown.id } == false)

        // With resolved included: it stays, after every active event.
        m.query.includeResolved = true
        await m.settle()
        m.apply(.upsert(resolved))
        #expect(m.snapshot?.events.last?.id == shown.id)
        #expect(m.snapshot?.events.dropLast().allSatisfy { !$0.isResolved } == true)
    }

    @Test func aChangeOutOfViewOrFilteredOutLeavesTheMapAlone() async throws {
        let p = StreamingProvider()
        let europe = GeoBox(minLongitude: -25, minLatitude: 34, maxLongitude: 45, maxLatitude: 72)
        let m = await loadedModel(p, box: europe)
        let count = m.snapshot?.events.count

        m.apply(.upsert(moved(sample(4), to: Coordinate(latitude: -33, longitude: 151), now: now)))
        #expect(m.snapshot?.events.count == count)

        // In view but older than the 24 h window: filtered out, so not added.
        var old = moved(sample(5), to: Coordinate(latitude: 48, longitude: 2), now: now)
        old.updatedAt = now.addingTimeInterval(-3 * 86400)
        m.apply(.upsert(old))
        #expect(m.snapshot?.events.contains { $0.id == old.id } == false)
    }

    @Test func belowZoomSixAChangeInViewReloadsBecauseOfClusters() async throws {
        let p = StreamingProvider()
        p.clusters = [EventCluster(centroid: Coordinate(latitude: 0, longitude: 0), count: 12, maxSeverity: .moderate,
                                   types: [.wildfire: 12], severities: [.moderate: 12],
                                   box: GeoBox(minLongitude: -1, minLatitude: -1, maxLongitude: 1, maxLatitude: 1))]
        let m = await loadedModel(p, zoom: 3)
        let before = p.requests
        m.apply(.upsert(moved(sample(3), to: Coordinate(latitude: 0.2, longitude: 0.2), now: now)))
        await m.settle()
        #expect(p.requests == before + 1)
        #expect(m.snapshot?.events.contains { $0.id == sample(3).id } == false) // not patched in
    }

    @Test func belowZoomSixEvenWithoutClustersAChangeReloads() async throws {
        // The server may merge the new event with a neighbour; only it can say.
        let p = StreamingProvider()
        let m = await loadedModel(p, zoom: 3)
        let before = p.requests
        m.apply(.upsert(moved(sample(3), to: Coordinate(latitude: 10, longitude: 10), now: now)))
        await m.settle()
        #expect(p.requests == before + 1)
        #expect(m.snapshot?.events.contains { $0.id == sample(3).id } == false)
    }

    @Test func aTruncatedSnapshotReloadsInsteadOfAddingWhatTheServerCut() async throws {
        let p = StreamingProvider()
        p.truncated = true
        let m = await loadedModel(p)
        let before = p.requests
        m.apply(.upsert(moved(sample(3), to: Coordinate(latitude: 10, longitude: 10), now: now)))
        await m.settle()
        #expect(p.requests == before + 1)
        #expect(m.snapshot?.events.contains { $0.id == sample(3).id } == false)
    }

    @Test func removingTheSelectedEventKeepsItOpenAsGone() async throws {
        // DESIGN.md §10.3: the detail says the source no longer publishes it.
        let p = StreamingProvider()
        let m = await loadedModel(p)
        let shown = try #require(m.snapshot?.events.first)
        m.selectedEventID = shown.id
        m.apply(.removed(shown.id))
        #expect(m.selectedEventID == shown.id)
        #expect(m.selectedDetail == .gone(shown))
    }

    @Test func readyMeansLiveAndReloadsSoNothingFallsBetween() async throws {
        let p = StreamingProvider()
        let m = await loadedModel(p)
        let before = p.requests
        m.apply(.ready)
        await m.settle()
        #expect(m.isLive)
        #expect(!m.dataIsStale)
        #expect(p.requests == before + 1)
    }

    @Test func silenceEndsLiveWithoutWaitingForTheSocketToFail() async throws {
        let clock = TestClock(now)
        let p = StreamingProvider()
        let m = LiveMapModel(provider: p, debounce: .zero, now: { clock.now })
        m.viewportDidChange(MapViewport(box: .world, zoom: 9))
        await m.settle()

        m.apply(.ready)
        #expect(m.isLive)
        clock.advance(30)
        #expect(m.isLive) // one heartbeat late is fine
        #expect(m.lastUpdate == now) // no newer word from the stream
        clock.advance(20)
        #expect(!m.isLive) // 50 s of silence: a dead connection
        #expect(m.lastUpdate == m.lastSuccess)
    }

    @Test func aShownEventThatStopsMatchingIsRemoved() async throws {
        let p = StreamingProvider()
        let m = await loadedModel(p)
        let shown = try #require(m.snapshot?.events.first)

        var resolved = moved(shown, to: shown.centroid, now: now)
        resolved.status = .resolved
        m.apply(.upsert(resolved))
        #expect(m.snapshot?.events.contains { $0.id == shown.id } == false)

        let second = try #require(m.snapshot?.events.first)
        var aged = moved(second, to: second.centroid, now: now)
        aged.updatedAt = now.addingTimeInterval(-3 * 86400)
        m.apply(.upsert(aged))
        #expect(m.snapshot?.events.contains { $0.id == second.id } == false)
    }

    @Test func aChangeBelowTheSeverityFilterIsNotPatchedIn() async throws {
        let p = StreamingProvider()
        let m = await loadedModel(p)
        m.query.minimumSeverity = .severe
        await m.settle()
        var minor = moved(sample(3), to: Coordinate(latitude: 10, longitude: 10), now: now)
        minor.severity = .minor
        m.apply(.upsert(minor))
        #expect(m.snapshot?.events.contains { $0.id == minor.id } == false)
    }

    @Test func aShownEventThatMovesOutOfViewIsRemoved() async throws {
        let p = StreamingProvider()
        let europe = GeoBox(minLongitude: -25, minLatitude: 34, maxLongitude: 45, maxLatitude: 72)
        let m = await loadedModel(p, box: europe)
        let shown = try #require(m.snapshot?.events.first)
        m.apply(.upsert(moved(shown, to: Coordinate(latitude: -33, longitude: 151), now: now)))
        #expect(m.snapshot?.events.contains { $0.id == shown.id } == false)
    }

    @Test func reconnectsWithTheLastCursorAndHonoursRetryAfter() async throws {
        let p = StreamingProvider()
        p.script([
            .updates([LiveUpdate(change: .ready, cursor: "5")], then: URLError(.networkConnectionLost)),
            .updates([], then: StreamUnavailable(status: 429, retryAfter: .seconds(7))),
            .updates([LiveUpdate(change: nil, cursor: "9")], then: nil), // a heartbeat, then the server closes
        ])
        let m = await loadedModel(p)
        let sleeps = SleepRecorder(blockAt: 3)
        let follow = Task { await m.followChanges(sleep: sleeps.sleep, jitter: { 1 }) }
        for _ in 0..<200 where sleeps.slept.count < 3 { try await Task.sleep(for: .milliseconds(10)) }
        follow.cancel()
        await follow.value

        #expect(p.cursors == [nil, "5", "5"])
        #expect(sleeps.slept == [.seconds(5), .seconds(7), .seconds(5)]) // heartbeat reset the count
        #expect(m.streamCursor == "9")
        #expect(!m.isLive)
    }
}
