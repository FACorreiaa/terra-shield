import Foundation
import Testing
@testable import ShieldCore

@Suite struct SeverityTests {
    @Test func ordersMinorToExtreme() {
        #expect(Severity.allCases.sorted() == [.minor, .moderate, .severe, .extreme])
        #expect(Severity.extreme > .severe)
    }

    @Test func displayOrderIsSeverityThenRecency() {
        let now = Date(timeIntervalSince1970: 1_800_000_000)
        let sorted = SampleData.events(now: now).sortedForDisplay()
        #expect(sorted.first?.severity == .extreme)
        let severe = sorted.filter { $0.severity == .severe }
        #expect(severe.map(\.title) == ["M6.1 earthquake off Coquimbo", "Flood warning, Emilia-Romagna"])
        #expect(sorted.last?.isResolved == true)
    }

    @Test func resolvedFollowsEveryActiveEventWhateverItsSeverity() {
        let now = Date(timeIntervalSince1970: 1_800_000_000)
        var resolvedExtreme = SampleData.events(now: now)[0]
        resolvedExtreme.status = .resolved
        let minor = SampleData.events(now: now)[5]
        #expect([resolvedExtreme, minor].sortedForDisplay().map(\.id) == [minor.id, resolvedExtreme.id])
    }

    @Test func resolvedWordsPreferAPassedEndTime() {
        let now = Date(timeIntervalSince1970: 1_800_000_000)
        let resolved = SampleData.events(now: now).last!
        #expect(resolved.resolvedWords(now: now) == "Resolved · ended 50 min ago")
        var noEnd = resolved
        noEnd.endedAt = nil
        #expect(noEnd.resolvedWords(now: now) == "Resolved · updated 50 min ago")
        var futureEnd = resolved
        futureEnd.endedAt = now.addingTimeInterval(3600)
        #expect(futureEnd.resolvedWords(now: now) == "Resolved · updated 50 min ago")
        #expect(SampleData.events(now: now)[0].resolvedWords(now: now) == nil)
    }

    @Test func includeResolvedShowsTheResolvedSample() async throws {
        let now = Date(timeIntervalSince1970: 1_800_000_000)
        let p = SampleEventProvider(now: { now })
        let events = try await p.events(matching: EventQuery(includeResolved: true))
        #expect(events.count == SampleData.events(now: now).count)
        #expect(events.last?.isResolved == true)
    }
}

@Suite struct CountryPhraseTests {
    @Test func withinIsTheName() {
        #expect(EventCountry(iso2: "PT", relation: .within).phrase(name: "Portugal") == "Portugal")
    }

    @Test func nearbyIsOffTheCoastWithDistance() {
        let c = EventCountry(iso2: "CL", relation: .nearby, distanceKm: 37.6)
        #expect(c.phrase(name: "Chile") == "Off the coast of Chile, 38 km")
    }

    @Test func nearbyWithoutDistance() {
        #expect(EventCountry(iso2: "CL", relation: .nearby).phrase(name: "Chile") == "Off the coast of Chile")
    }
}

@Suite struct FreshnessTests {
    let now = Date(timeIntervalSince1970: 1_800_000_000)

    @Test(arguments: [
        (5.0, "now"), (45.0, "45 s ago"), (12 * 60.0, "12 min ago"), (3 * 3600.0, "3 h ago"), (2 * 86_400.0, "2 d ago"),
    ])
    func relative(seconds: Double, expected: String) {
        #expect(Freshness.relative(now.addingTimeInterval(-seconds), now: now) == expected)
    }

    @Test func futureReadsIn() {
        #expect(Freshness.relative(now.addingTimeInterval(600), now: now) == "in 10 min")
    }

    @Test func staleAfterThreePollIntervals() {
        #expect(!Freshness.isStale(lastUpdate: now.addingTimeInterval(-179), pollInterval: 60, now: now))
        #expect(Freshness.isStale(lastUpdate: now.addingTimeInterval(-181), pollInterval: 60, now: now))
    }

    @Test func utcClock() {
        // 1_800_000_000 is 2027-01-15T08:00:00Z.
        #expect(Freshness.utcClock(now) == "08:00 UTC")
    }

    @Test func sourceUsesServerVerdictFirst() {
        let s = Source(id: "x", name: "X", url: URL(string: "https://x.test")!, license: "", attribution: "",
                       pollIntervalSeconds: 60, lastSuccessAt: now, stale: true)
        #expect(s.isStale(now: now))
    }
}

@Suite struct DecodingTests {
    @Test func decodesEventWithEarthquakeAttrs() throws {
        let json = """
        {
          "id": "6f1c2a7e-0000-4000-8000-000000000001",
          "source": "usgs", "type": "earthquake", "kind": "observed", "status": "active",
          "severity": "moderate", "sourceSeverity": "M5.4 mww, PAGER green",
          "title": "M5.4 earthquake", "centroid": {"latitude": 38.3, "longitude": 38.4},
          "countries": [{"iso2": "TR", "relation": "within", "distanceKm": 0}],
          "observedAt": "2026-11-16T13:26:00Z", "updatedAt": "2026-11-16T13:34:00.123Z",
          "fetchedAt": "2026-11-16T13:35:00Z", "url": "https://earthquake.usgs.gov/",
          "attrs": {"mag": 5.4, "magType": "mww", "depthKm": 10, "pagerAlert": "green",
                    "tsunami": false, "feltReports": 1208, "reviewStatus": "reviewed"}
        }
        """
        let e = try JSONDecoder.shield.decode(HazardEvent.self, from: Data(json.utf8))
        #expect(e.type == .earthquake)
        #expect(e.earthquake?.magnitude == 5.4)
        #expect(e.earthquake?.feltReports == 1208)
        #expect(e.earthquake?.tsunami == false)
        #expect(e.countries.first?.relation == .within)
        #expect(e.severityProvisional == false)
    }

    @Test func decodesProvisionalSeverity() throws {
        let json = """
        {"id":"6f1c2a7e-0000-4000-8000-000000000002","source":"eonet","type":"wildfire","kind":"observed",
         "severity":"moderate","severityProvisional":true,"title":"Wildfire","centroid":{"latitude":39,"longitude":-8},
         "observedAt":"2026-11-16T13:26:00Z","updatedAt":"2026-11-16T13:26:00Z","fetchedAt":"2026-11-16T13:30:00Z"}
        """
        let e = try JSONDecoder.shield.decode(HazardEvent.self, from: Data(json.utf8))
        #expect(e.severityProvisional)
        #expect(e.status == .active)
        #expect(e.attrs.isEmpty)
    }

    @Test func readsProvisionalFromAttrsBeforeS21() throws {
        let json = """
        {"id":"6f1c2a7e-0000-4000-8000-000000000003","source":"eonet","type":"volcano","kind":"observed",
         "severity":"minor","sourceSeverity":"","title":"Volcano","centroid":{"latitude":63,"longitude":-22},
         "observedAt":"2026-11-16T13:26:00Z","updatedAt":"2026-11-16T13:26:00Z","fetchedAt":"2026-11-16T13:30:00Z",
         "attrs":{"severityProvisional":true,"category":"volcanoes"}}
        """
        let e = try JSONDecoder.shield.decode(HazardEvent.self, from: Data(json.utf8))
        #expect(e.severityProvisional)
        #expect(e.sourceSeverity == "")
    }

    @Test func unknownTypeFallsBackToOther() throws {
        let t = try JSONDecoder().decode([HazardType].self, from: Data(#"["tropical_cyclone","meteor"]"#.utf8))
        #expect(t == [.tropicalCyclone, .other])
    }

    @Test func decodesSourceFromSpecShape() throws {
        let json = """
        {"id":"usgs","name":"USGS","url":"https://earthquake.usgs.gov/","license":"Public domain",
         "attribution":"USGS","pollIntervalSeconds":60,"enabled":true,"lastSuccessAt":"2026-11-16T14:00:00Z","stale":false}
        """
        let s = try JSONDecoder.shield.decode(Source.self, from: Data(json.utf8))
        #expect(s.pollInterval == 60)
        #expect(s.stale == false)
    }
}

@Suite struct ProviderTests {
    let now = Date(timeIntervalSince1970: 1_800_000_000)

    @Test func defaultQueryHidesDetectionsAndStaysInWindow() async throws {
        let p = SampleEventProvider(now: { [now] in now })
        let events = try await p.events(matching: EventQuery())
        #expect(events.count == SampleData.events(now: now).filter { !$0.isResolved }.count)
        #expect(!events.contains { $0.isResolved })
        #expect(p.isSampleData)
    }

    @Test func filtersBySeverityAndCountry() async throws {
        let p = SampleEventProvider(now: { [now] in now })
        let severe = try await p.events(matching: EventQuery(minimumSeverity: .severe))
        #expect(severe.allSatisfy { $0.severity >= .severe })
        let pt = try await p.events(matching: EventQuery(country: "PT"))
        #expect(pt.map(\.title) == ["Wildfire near Leiria", "Heavy rain forecast, Minho"])
    }

    @Test func forecastKindCanBeExcluded() async throws {
        let p = SampleEventProvider(now: { [now] in now })
        let noForecast = try await p.events(matching: EventQuery(kinds: [.observed, .warning]))
        #expect(!noForecast.contains { $0.kind == .forecast })
    }
}
