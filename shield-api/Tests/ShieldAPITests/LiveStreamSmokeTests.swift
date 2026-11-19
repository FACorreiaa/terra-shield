import Foundation
import Testing
@testable import ShieldAPI
import ShieldCore

/// Against a running API only: `SHIELD_LIVE_URL=http://localhost:8080 swift test`.
/// Exercises the real URLSession transport, which the canned stream in
/// `LiveStreamAPITests` bypasses: buffering, framing, and the heartbeat. Skipped in CI.
@Suite(.enabled(if: ProcessInfo.processInfo.environment["SHIELD_LIVE_URL"] != nil))
struct LiveStreamSmokeTests {
    @Test(.timeLimit(.minutes(1)))
    func readyArrivesPromptlyAndAHeartbeatWithin25Seconds() async throws {
        let base = try #require(ProcessInfo.processInfo.environment["SHIELD_LIVE_URL"].flatMap(URL.init(string:)))
        let provider = APIEventProvider(client: .shield(baseURL: base), stream: LiveStreamConfig(baseURL: base))
        let opened = ContinuousClock.now
        var readyAfter: Duration?
        var heartbeatAfter: Duration?
        for try await update in provider.changes(after: nil) {
            if update.change == .ready, readyAfter == nil { readyAfter = ContinuousClock.now - opened }
            if update.change == nil, readyAfter != nil {
                heartbeatAfter = ContinuousClock.now - opened
                break
            }
            if ContinuousClock.now - opened > .seconds(30) { break }
        }
        let ready = try #require(readyAfter, "no ready")
        #expect(ready < .seconds(3), "ready took \(ready): something buffers")
        let heartbeat = try #require(heartbeatAfter, "no heartbeat within 30 s")
        #expect(heartbeat < .seconds(25))
    }

    /// Replays the server's recent changes and decodes every one: `liveChange` returns nil on
    /// a decode failure, and in the app the 60 s poll would hide it.
    @Test(.timeLimit(.minutes(1)))
    func everyReplayedChangeDecodes() async throws {
        let base = try #require(ProcessInfo.processInfo.environment["SHIELD_LIVE_URL"].flatMap(URL.init(string:)))
        var request = URLRequest(url: base.appending(path: "v1/stream"))
        request.setValue("text/event-stream", forHTTPHeaderField: "Accept")
        request.setValue("1", forHTTPHeaderField: "Last-Event-ID")
        let (response, bytes) = try await URLSessionStreamTransport().open(request)
        #expect(response.statusCode == 200)
        var decoder = SSEStreamDecoder(lastEventID: "1")
        var decoded = 0
        var failed: [String] = []
        loop: for try await byte in bytes {
            guard case .message(let m)? = decoder.feed(byte) else { continue }
            if m.event == "ready" { break loop }
            if APIEventProvider.liveChange(m) == nil { failed.append("\(m.id ?? "?") \(m.event)") } else { decoded += 1 }
        }
        #expect(failed.isEmpty, "undecodable: \(failed.prefix(5))")
        print("decoded \(decoded) replayed changes")
    }
}
