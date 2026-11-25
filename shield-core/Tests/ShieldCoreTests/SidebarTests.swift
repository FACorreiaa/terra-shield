import Foundation
import Testing
@testable import ShieldCore

/// DESIGN.md §10 (macOS): the sidebar's sections and ⌘1–⌘5.
@Suite struct SidebarSectionTests {
    @Test func theShortcutsFollowTheDesignOrder() {
        #expect(SidebarSection.allCases.map(\.title) == ["Monitor", "Countries", "Saved regions", "Alerts", "Sources"])
        #expect(SidebarSection.allCases.map(\.shortcut) == ["1", "2", "3", "4", "5"])
    }

    @Test func savedAndAlertsWaitForPhase3() {
        // VIEWS.md: Saved regions and Alerts are gated; their numbers stay reserved.
        #expect(SidebarSection.shown == [.monitor, .countries, .sources])
        #expect(!SidebarSection.saved.isAvailable)
        #expect(!SidebarSection.alerts.isAvailable)
    }
}

/// DESIGN.md §10: the Sources row's overall health dot.
@Suite struct OverallSourceHealthTests {
    let now = Date(timeIntervalSince1970: 1_795_110_000) // 2026-11-19 17:40 UTC

    private func source(_ id: String, success: TimeInterval?, error: TimeInterval? = nil,
                        stale: Bool? = false, enabled: Bool = true, onDemand: Bool? = nil) -> Source {
        var s = Source(id: id, name: id, url: URL(string: "https://example.org/")!,
                       license: "Public domain", attribution: id,
                       pollIntervalSeconds: 600, enabled: enabled,
                       lastSuccessAt: success.map { now.addingTimeInterval($0) },
                       lastErrorAt: error.map { now.addingTimeInterval($0) }, stale: stale)
        s.onDemand = onDemand
        return s
    }

    @Test func noDotBeforeTheFirstList() {
        #expect(Source.overallHealth([], now: now) == nil)
    }

    @Test func allHealthyIsOK() {
        let list = [source("usgs", success: -60), source("eonet", success: -120, error: -2400)]
        #expect(Source.overallHealth(list, now: now) == .ok)
    }

    @Test func theWorstSourceSetsTheDot() {
        let failing = source("eonet", success: -3600, error: -180)
        let stale = source("gdacs", success: -19 * 3600, stale: true)
        #expect(Source.overallHealth([source("usgs", success: -60), failing], now: now) == .failing)
        #expect(Source.overallHealth([failing, stale, source("usgs", success: -60)], now: now) == .stale)
    }

    @Test func aDisabledSourceDoesNotCount() {
        let off = source("old", success: -19 * 3600, stale: true, enabled: false)
        #expect(Source.overallHealth([source("usgs", success: -60), off], now: now) == .ok)
    }

    @Test func anOnDemandSourceIsNeverStale() {
        // adsb.lol is fetched only while the flights layer is shown: unwatched is not failing.
        let flights = source("adsblol", success: nil, stale: nil, onDemand: true)
        #expect(flights.health(now: now) == .ok)
        #expect(Source.overallHealth([source("usgs", success: -60), flights], now: now) == .ok)
    }
}

/// DESIGN.md §10: the Countries section, a searchable list with active counts.
@Suite struct CountryDirectoryTests {
    private func country(_ iso2: String, _ name: String, active: Int = 0, max: Severity? = nil) -> Country {
        Country(iso2: iso2, iso3: iso2 + "X", name: name, activeEvents: active, maxSeverity: max,
                bbox: GeoBox(minLongitude: 0, minLatitude: 0, maxLongitude: 1, maxLatitude: 1))
    }

    var all: [Country] {
        [country("PT", "Portugal"),
         country("JP", "Japan", active: 2, max: .moderate),
         country("CL", "Chile", active: 1, max: .extreme),
         country("ID", "Indonesia", active: 5, max: .moderate),
         country("AF", "Afghanistan"),
         country("CI", "Côte d'Ivoire")]
    }

    @Test func activeCountriesComeFirstMostSevereThenBusiest() {
        let d = CountryDirectory(all, filter: "")
        #expect(d.active.map(\.iso2) == ["CL", "ID", "JP"])
        #expect(d.quiet.map(\.iso2) == ["AF", "CI", "PT"])
    }

    @Test func theFilterKeepsEveryMatchNotJustFive() {
        let d = CountryDirectory(all, filter: "a")
        // Every name with an "a", accents ignored, in the same two groups.
        #expect(d.active.map(\.iso2) == ["ID", "JP"])
        #expect(d.quiet.map(\.iso2) == ["AF", "PT"])
        #expect(CountryDirectory(all, filter: "cote").quiet.map(\.iso2) == ["CI"])
        #expect(CountryDirectory(all, filter: "JPX").active.map(\.iso2) == ["JP"])
    }

    @Test func nothingMatchedIsEmpty() {
        #expect(CountryDirectory(all, filter: "zz").isEmpty)
        #expect(!CountryDirectory(all, filter: "").isEmpty)
    }
}
