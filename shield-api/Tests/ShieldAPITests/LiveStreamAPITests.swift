import Foundation
import Testing
@testable import ShieldAPI
import ShieldCore

/// The live AKVAN-26 event in the stream's list shape (no geometry), 2026-11-18.
private let cyclone = #"""
{"id":"a230e8b1-0bd8-44b5-94d1-a55a9450efe1","source":"gdacs","type":"tropical_cyclone","kind":"observed","status":"active","severity":"severe","sourceSeverity":"Orange alert, Tropical Storm (maximum wind speed of 83 km/h)","title":"Tropical cyclone AKVAN-26","summary":"From 16/11/2026 to 18/11/2026, a Tropical Storm (maximum win","centroid":{"latitude":15.103,"longitude":86.11828953815623},"countries":[{"iso2":"IN","relation":"within"}],"startedAt":"2026-11-16T18:00:00Z","observedAt":"2026-11-18T06:00:00Z","updatedAt":"2026-11-18T10:35:39Z","fetchedAt":"2026-11-18T11:02:33.004549Z","url":"https://www.gdacs.org/report.aspx?eventtype=TC&eventid=1001351","attrs":{"alertLevel":"Orange","alertScore":2,"bbox":[73.2049523520329,1.1766733453532,99.2049523520329,27.1766733453532],"capUrl":"https://www.gdacs.org/contentdata/resources/TC/1001351/cap_1001351.xml","countries":["India"],"episodeId":"7","eventId":"1001351","eventName":"AKVAN-26","footprint":true,"point":[86.9,13.1],"populationText":"Population affected by Category 1 (120 km/h) wind speeds or higher is 0 (28 million in Tropical Storm)","severityText":"Tropical Storm (maximum wind speed of 83 km/h)","severityUnit":"km/h","severityValue":83.3328,"vulnerability":"High"}}
"""#

/// A real `event.updated` from the local API's stream (replayed from cursor 1000), 2026-11-19.
private let wireWildfire = #"""
{"id":"504bcc3b-8763-4c81-978f-3a5eebb205f9","source":"gdacs","type":"wildfire","kind":"observed","status":"active","severity":"minor","sourceSeverity":"Green alert, Green impact for forestfire in 5968 ha","title":"Wildfire in Sudan","summary":"On 08/11/2026, a forest fire started in Sudan,  until 15/11/2026.","centroid":{"latitude":12.543688111955925,"longitude":34.93195338230705},"countries":[{"iso2":"SD","relation":"within"}],"startedAt":"2026-11-08T00:00:00Z","observedAt":"2026-11-17T00:00:00Z","updatedAt":"2026-11-18T16:50:16Z","fetchedAt":"2026-11-18T19:15:37.445921Z","url":"https://www.gdacs.org/report.aspx?eventtype=WF\u0026eventid=1033867","attrs":{"alertLevel":"Green","alertScore":1,"bbox":[30.9319533823071,8.54368811195592,38.9319533823071,16.5436881119559],"capUrl":"https://www.gdacs.org/contentdata/resources/WF/1033867/cap_1033867.xml","countries":["Sudan"],"episodeId":"3","eventId":"1033867","point":[34.93195338230705,12.543688111955925],"populationText":"0 people affected in the area","severityText":"Green impact for forestfire in 5968 ha","severityUnit":"ha","severityValue":5968}}
"""#

/// Serves canned bytes with a status, and records the request.
private final class CannedStream: StreamTransport, @unchecked Sendable {
    // @unchecked: test-only; `lock` guards `requests`.
    private let lock = NSLock()
    private var _requests: [URLRequest] = []
    let status: Int
    let headers: [String: String]
    let body: String

    init(status: Int = 200, headers: [String: String] = [:], body: String) {
        self.status = status
        self.headers = headers
        self.body = body
    }

    var requests: [URLRequest] { lock.withLock { _requests } }

    func open(_ request: URLRequest) async throws -> (HTTPURLResponse, AsyncThrowingStream<UInt8, any Error>) {
        lock.withLock { _requests.append(request) }
        let response = HTTPURLResponse(url: request.url!, statusCode: status, httpVersion: "HTTP/1.1", headerFields: headers)!
        let bytes = Array(body.utf8)
        return (response, AsyncThrowingStream { c in
            for b in bytes { c.yield(b) }
            c.finish()
        })
    }
}

private let base = URL(string: "http://shield.test")!

private func provider(_ t: CannedStream) -> APIEventProvider {
    APIEventProvider(client: .shield(baseURL: base), stream: LiveStreamConfig(baseURL: base, transport: t))
}

private func collect(_ p: APIEventProvider, after cursor: String?) async throws -> [LiveUpdate] {
    var out: [LiveUpdate] = []
    for try await u in p.changes(after: cursor) { out.append(u) }
    return out
}

@Suite struct LiveStreamAPITests {
    @Test func decodesWhatTheServerSends() async throws {
        let removedID = "0b5f4a9e-2a0c-4c47-9a39-3b1f2b8a1c11"
        let t = CannedStream(body: """
        retry: 5000

        id: 101
        event: event.updated
        data: \(cyclone)

        id: 108

        id: 109
        event: event.removed
        data: {"id":"\(removedID)"}

        id: 110
        event: some.future.thing
        data: {}

        id: 110
        event: ready
        data: {}


        """)
        let updates = try await collect(provider(t), after: "100")

        #expect(updates.map(\.cursor) == ["101", "108", "109", "110", "110"])
        guard case .upsert(let e)? = updates[0].change else { Issue.record("\(updates[0])"); return }
        #expect(e.title == "Tropical cyclone AKVAN-26")
        #expect(updates[1].change == nil) // heartbeat
        #expect(updates[2].change == .removed(UUID(uuidString: removedID)!))
        #expect(updates[3].change == nil) // unknown names are skipped, the cursor still moves
        #expect(updates[4].change == .ready)

        let request = try #require(t.requests.first)
        #expect(request.url?.path == "/v1/stream")
        #expect(request.value(forHTTPHeaderField: "Accept") == "text/event-stream")
        #expect(request.value(forHTTPHeaderField: "Last-Event-ID") == "100")
    }

    @Test func decodesARealWireMessage() throws {
        let change = APIEventProvider.liveChange(SSEMessage(id: "1002", event: "event.updated", data: wireWildfire))
        guard case .upsert(let e)? = change else { Issue.record("not decoded: \(String(describing: change))"); return }
        #expect(e.id == UUID(uuidString: "504bcc3b-8763-4c81-978f-3a5eebb205f9"))
        #expect(e.title == "Wildfire in Sudan")
        #expect(e.source == "gdacs")
    }

    @Test func aCreatedEventIsTold() {
        // Only `event.created` is news (the map fades it in); updates and resolutions are not.
        let created = APIEventProvider.liveChange(SSEMessage(id: "1003", event: "event.created", data: wireWildfire))
        guard case .created(let e)? = created else { Issue.record("not created: \(String(describing: created))"); return }
        #expect(e.title == "Wildfire in Sudan")
        let resolved = APIEventProvider.liveChange(SSEMessage(id: "1004", event: "event.resolved", data: wireWildfire))
        guard case .upsert? = resolved else { Issue.record("not an upsert: \(String(describing: resolved))"); return }
    }

    @Test func aFirstConnectionSendsNoCursor() async throws {
        let t = CannedStream(body: "id: 1\nevent: ready\ndata: {}\n\n")
        _ = try await collect(provider(t), after: nil)
        #expect(t.requests.first?.value(forHTTPHeaderField: "Last-Event-ID") == nil)
    }

    @Test func rateLimitedAndBusyCarryRetryAfter() async {
        for status in [429, 503] {
            let t = CannedStream(status: status, headers: ["Retry-After": "3"], body: #"{"error":true}"#)
            await #expect(throws: StreamUnavailable(status: status, retryAfter: .seconds(3))) {
                _ = try await collect(provider(t), after: nil)
            }
        }
    }

    @Test func withoutAStreamThereAreNoLiveChanges() {
        #expect(!APIEventProvider(client: .shield(baseURL: base)).supportsLiveChanges)
        #expect(provider(CannedStream(body: "")).supportsLiveChanges)
    }
}
