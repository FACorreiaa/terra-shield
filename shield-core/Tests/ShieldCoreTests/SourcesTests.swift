import Foundation
import Testing
@testable import ShieldCore

/// DESIGN.md §10.7: each source's health dot and lines.
@Suite struct SourceHealthTests {
    let now = Date(timeIntervalSince1970: 1_795_110_000) // 2026-11-19 17:40 UTC

    private func source(poll: Int = 600, success: TimeInterval?, error: TimeInterval? = nil,
                        stale: Bool? = false, enabled: Bool = true, cadence: String? = nil) -> Source {
        var s = Source(id: "eonet", name: "NASA EONET", url: URL(string: "https://eonet.gsfc.nasa.gov/")!,
                       license: "Public domain (NASA)", attribution: "NASA EONET",
                       pollIntervalSeconds: poll, enabled: enabled,
                       lastSuccessAt: success.map { now.addingTimeInterval($0) },
                       lastErrorAt: error.map { now.addingTimeInterval($0) }, stale: stale)
        s.cadence = cadence
        return s
    }

    @Test func anOlderErrorDoesNotMatter() {
        // Live on 2026-11-19: EONET failed at 17:03 and succeeded since at 17:41.
        let s = source(success: -120, error: -2400)
        #expect(s.health(now: now) == .ok)
        #expect(s.statusLine(now: now) == "Checked 2 min ago · 17:38 UTC")
    }

    @Test func aFailedLastCheckIsAWarning() {
        let s = source(success: -3600, error: -180)
        #expect(s.health(now: now) == .failing)
        #expect(s.statusLine(now: now) == "Last check failed 3 min ago · last good check 1 h ago")
    }

    @Test func neverSucceededSaysSo() {
        let s = source(success: nil, error: -180, stale: nil)
        #expect(s.health(now: now) == .stale)
        #expect(s.statusLine(now: now) == "Last check failed 3 min ago · no good check yet")
    }

    @Test func theServersStaleVerdictWins() {
        let s = source(success: -19 * 3600, stale: true)
        #expect(s.health(now: now) == .stale)
        #expect(s.statusLine(now: now) == "Stale · last good check 19 h ago · 22:40 UTC")
    }

    @Test func theCheckIntervalIsOurPollNotTheData() {
        #expect(source(poll: 60, success: 0).interval == "Checked every 1 min")
        #expect(source(poll: 300, success: 0).interval == "Checked every 5 min")
        #expect(source(poll: 21_600, success: 0).interval == "Checked every 6 h")
        #expect(source(poll: 30, success: 0).interval == "Checked every 30 s")
        #expect(source(poll: 21_600, success: 0, cadence: "monthly release").interval
                == "Monthly release · checked every 6 h")
    }

    @Test func onlyActiveSourcesAreListed() {
        let list = [source(success: 0), source(success: 0, enabled: false)]
        #expect(Source.listed(list).count == 1)
    }
}

/// The Sources screen refreshes on open and keeps what it had when that fails.
@MainActor @Suite struct SourcesRefreshTests {
    @Test func aRefreshListsSourcesInTheAPIsOrder() async {
        let m = LiveMapModel(provider: ScriptedProvider(), debounce: .zero)
        #expect(m.sourceList.isEmpty)
        await m.refreshSources()
        #expect(m.sourceList.map(\.id) == SampleData.sources().map(\.id))
        #expect(m.sources.count == m.sourceList.count)
        #expect(!m.sourcesRefreshFailed)
    }

    @Test func aFailedRefreshKeepsTheLastList() async {
        let p = ScriptedProvider()
        let m = LiveMapModel(provider: p, debounce: .zero)
        await m.refreshSources()
        p.sourcesFailure = URLError(.notConnectedToInternet)
        await m.refreshSources()
        #expect(!m.sourceList.isEmpty)
        #expect(m.sourcesRefreshFailed)
        p.sourcesFailure = nil
        await m.refreshSources()
        #expect(!m.sourcesRefreshFailed)
    }
}
