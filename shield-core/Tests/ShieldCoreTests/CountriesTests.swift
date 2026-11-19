import Foundation
import Testing
@testable import ShieldCore

private let now = Date(timeIntervalSince1970: 1_800_000_000)

private func country(_ iso2: String, _ name: String, iso3: String = "XXX", active: Int = 0,
                     box: GeoBox = GeoBox(minLongitude: 0, minLatitude: 0, maxLongitude: 1, maxLatitude: 1)) -> Country {
    Country(iso2: iso2, iso3: iso3, name: name, activeEvents: active, maxSeverity: nil, bbox: box)
}

private func event(_ type: HazardType = .flood, _ severity: Severity = .moderate, kind: EventKind = .warning,
                   at lon: Double = 0, lat: Double = 0) -> HazardEvent {
    HazardEvent(id: UUID(), source: "test", type: type, kind: kind, severity: severity, sourceSeverity: "", title: "e",
                centroid: .init(latitude: lat, longitude: lon), observedAt: now, updatedAt: now, fetchedAt: now)
}

@Suite struct CountryPickerTests {
    @Test func codesAreNormalizedOrRefused() {
        #expect(Country.normalizedCode(" pt ") == "PT")
        #expect(Country.normalizedCode("PRT") == nil)
        #expect(Country.normalizedCode("P1") == nil)
        #expect(Country.normalizedCode("") == nil)
    }

    @Test func searchPutsCodesThenPrefixesThenContains() {
        let all = [
            country("PT", "Portugal", iso3: "PRT"), country("PY", "Paraguay", iso3: "PRY"),
            country("CI", "Côte d'Ivoire", iso3: "CIV"), country("PG", "Papua New Guinea", iso3: "PNG"),
            country("GN", "Guinea", iso3: "GIN"),
        ]
        #expect(Country.matching("pt", in: all).first?.iso2 == "PT")
        #expect(Country.matching("prt", in: all).map(\.iso2) == ["PT"])
        #expect(Country.matching("cote", in: all).map(\.iso2) == ["CI"]) // accents ignored
        // "Guinea" starts the name before it is found inside "Papua New Guinea".
        #expect(Country.matching("guinea", in: all).map(\.iso2) == ["GN", "PG"])
        #expect(Country.matching("  ", in: all).isEmpty)
        #expect(Country.matching("a", in: all, limit: 2).count == 2)
    }
}

@Suite struct CountryMapBoxTests {
    @Test func anOrdinaryCountryIsItsBBox() {
        let pt = country("PT", "Portugal", box: GeoBox(minLongitude: -31.3, minLatitude: 32.6, maxLongitude: -6.2, maxLatitude: 42.1))
        #expect(pt.mapBox(for: []) == pt.bbox)
    }

    @Test func aCountryAcrossTheAntimeridianFramesItsEvents() {
        // /v1/countries gives the US [-178.2, 19.0, 179.8, 71.4]: the whole world.
        let us = country("US", "United States", box: GeoBox(minLongitude: -178.2, minLatitude: 19, maxLongitude: 179.8, maxLatitude: 71.4))
        let box = us.mapBox(for: [event(at: -120, lat: 37), event(at: -150, lat: 61), event(at: 179, lat: 52)])
        // From the Aleutians west of ±180° round to California: crosses the line, 61° wide.
        #expect(box.minLongitude == 179)
        #expect(box.maxLongitude == -120)
        #expect(box.crossesAntimeridian)
        #expect(box.minLatitude == 37 && box.maxLatitude == 61)
    }

    @Test func withoutEventsItIsTheBandOfLatitude() {
        let ru = country("RU", "Russia", box: GeoBox(minLongitude: -180, minLatitude: 41.2, maxLongitude: 180, maxLatitude: 81.9))
        #expect(ru.mapBox(for: []) == GeoBox(minLongitude: -180, minLatitude: 41.2, maxLongitude: 180, maxLatitude: 81.9))
    }

    @Test func theSmallestArcSkipsTheWidestGap() {
        #expect(Country.smallestArc([10, 20]) == (10, 20))
        #expect(Country.smallestArc([-170, 170]) == (170, -170))
        #expect(Country.smallestArc([5]) == (5, 5))
    }
}

@Suite struct CountrySummaryTests {
    private func summary(active: Int, events: [HazardEvent], counts: [SeverityCount]) -> CountrySummary {
        CountrySummary(country: country("CH", "Switzerland", active: active), counts: counts,
                       coverage: Coverage(officialWarnings: true, sources: ["meteoalarm"]),
                       events: events, recentChanges: [], generatedAt: now)
    }

    @Test func warningsAreALowerBoundWhenTheListIsCapped() {
        // Real shape: CH had 157 active, 100 listed.
        let listed = (0..<100).map { _ in event(kind: .warning) }
        let s = summary(active: 157, events: listed, counts: [.init(type: .wind, severity: .moderate, count: 157)])
        #expect(s.isListCapped)
        #expect(s.warningCount == (100, true))
        let all = summary(active: 2, events: [event(kind: .warning), event(kind: .observed)],
                          counts: [.init(type: .flood, severity: .moderate, count: 2)])
        #expect(all.warningCount == (1, false))
    }

    @Test func highSeverityComesFromTheCounts() {
        let s = summary(active: 9, events: [], counts: [
            .init(type: .wind, severity: .extreme, count: 2), .init(type: .flood, severity: .severe, count: 3),
            .init(type: .flood, severity: .moderate, count: 4),
        ])
        #expect(s.highSeverityCount == 5)
    }

    @Test func theMatrixFoldsUnknownTypesIntoOtherAndSortsBySeverity() {
        // Two unknown wire types both decode as `.other`; they share one row.
        let s = summary(active: 10, events: [], counts: [
            .init(type: .wind, severity: .moderate, count: 6),
            .init(type: .other, severity: .minor, count: 1),
            .init(type: .other, severity: .minor, count: 2),
            .init(type: .flood, severity: .severe, count: 1),
        ])
        #expect(s.matrix.map(\.type) == [.flood, .wind, .other])
        #expect(s.matrix.last?.counts[.minor] == 3)
        #expect(s.count(in: MatrixCell(type: .other, severity: .minor)) == 3)
    }

    @Test func aCellFiltersTheList() {
        let a = event(.flood, .severe), b = event(.wind, .severe), c = event(.flood, .minor)
        let s = summary(active: 3, events: [a, b, c], counts: [])
        #expect(s.events(in: MatrixCell(type: .flood, severity: .severe)).map(\.id) == [a.id])
        #expect(s.events(in: nil).count == 3)
    }

    @Test func renamesAloneAreNotListed() {
        let renamed = RecentChange(eventID: UUID(), title: "Orange wind warning · Lička-senjska", at: now,
                                   changes: [.title(from: "Orange wind warning", to: "Orange wind warning · Lička-senjska")])
        let raised = RecentChange(eventID: UUID(), title: "Flood", at: now,
                                  changes: [.title(from: "a", to: "b"), .severity(from: .minor, to: .severe)])
        var s = summary(active: 0, events: [], counts: [])
        s.recentChanges = [renamed, raised]
        #expect(s.notableChanges.map(\.id) == [raised.id])
        #expect(raised.words == "Raised to Severe · renamed")
    }

    @Test func sampleSummariesAgreeWithTheirCountry() {
        for c in SampleData.countries(now: now) {
            let s = try! #require(SampleData.countrySummary(iso2: c.iso2, now: now))
            #expect(s.counts.reduce(0) { $0 + $1.count } == s.country.activeEvents, "\(c.iso2)")
        }
        #expect(SampleData.countrySummary(iso2: "ZZ", now: now) == nil)
    }
}

@Suite struct RecentChangeTests {
    private func parse(_ json: String) -> [Change] {
        let object = try! JSONDecoder().decode([String: JSONValue].self, from: Data(json.utf8))
        return Change.parse(object)
    }

    private func words(_ json: String) -> String {
        RecentChange(eventID: UUID(), title: "t", at: now, changes: parse(json)).words
    }

    @Test func aRealRevisionReadsAsOneLine() {
        // From the local API, HR, 2026-11-19: a flood that came back.
        let json = #"{"endedAt": {"from": "2026-11-14T01:00:00Z", "to": null}, "geometry": {"changed": true}, "status": {"from": "resolved", "to": "active"}}"#
        #expect(parse(json) == [.status(from: .resolved, to: .active), .geometry, .endedAt(to: nil)])
        // The end time going away is what "active again" means; it isn't said twice.
        #expect(words(json) == "Active again · area updated")
    }

    @Test func severityReadsAsRaisedOrLowered() {
        #expect(words(#"{"severity": {"from": "moderate", "to": "severe"}}"#) == "Raised to Severe")
        #expect(words(#"{"severity": {"from": "severe", "to": "minor"}}"#) == "Lowered to Minor")
        #expect(words(#"{"severity": {"from": null, "to": "minor"}}"#) == "Now Minor")
    }

    @Test func theSourceWordsAreQuotedVerbatim() {
        #expect(words(#"{"sourceSeverity": {"from": "Yellow warning", "to": "Orange warning"}}"#)
                == "Source now says “Orange warning”")
    }

    @Test func unknownOrUnreadableFieldsAreDropped() {
        #expect(parse(#"{"population": {"from": 1, "to": 2}, "severity": {"from": "minor", "to": "catastrophic"}}"#).isEmpty)
        #expect(words(#"{"population": {"from": 1, "to": 2}}"#) == "Updated")
        #expect(words(#"{"endedAt": {"from": null, "to": "2026-11-19T10:09:07.189239Z"}}"#) == "End time updated")
    }
}

/// Answers `countrySummary` from a script, each after its own delay.
private final class CountryProvider: HazardEventProvider, @unchecked Sendable {
    // @unchecked: test-only; `lock` guards the script.
    private let lock = NSLock()
    private var script: [(delay: Duration, result: Result<CountrySummary?, any Error>)]

    init(_ script: [(Duration, Result<CountrySummary?, any Error>)]) { self.script = script }

    var isSampleData: Bool { false }
    func events(matching query: EventQuery) async throws -> [HazardEvent] { [] }
    func snapshot(matching query: EventQuery, in viewport: MapViewport) async throws -> MapSnapshot {
        MapSnapshot(events: [], clusters: [], truncated: false, generatedAt: now)
    }
    func event(id: UUID) async throws -> HazardEvent? { nil }
    func sources() async throws -> [Source] { [] }
    func detections(in viewport: MapViewport) async throws -> DetectionLayer { .empty }

    func countrySummary(iso2: String) async throws -> CountrySummary? {
        let next = lock.withLock { script.removeFirst() }
        try await Task.sleep(for: next.delay)
        return try next.result.get()
    }
}

@MainActor @Suite struct CountryOverviewTests {
    private func summary(_ name: String, generatedAt: Date = now, counts: [SeverityCount] = []) -> CountrySummary {
        CountrySummary(country: country("PT", name, active: counts.reduce(0) { $0 + $1.count }), counts: counts,
                       coverage: Coverage(officialWarnings: false, sources: []), events: [], recentChanges: [],
                       generatedAt: generatedAt)
    }

    @Test func aBadCodeMakesNoOverview() {
        #expect(CountryOverview(iso2: "Portugal", provider: CountryProvider([])) == nil)
        #expect(CountryOverview(iso2: "pt", provider: CountryProvider([]))?.iso2 == "PT")
    }

    @Test func aLateAnswerDoesNotOverwriteANewerOne() async {
        let p = CountryProvider([(.milliseconds(200), .success(summary("old"))), (.zero, .success(summary("new")))])
        let o = CountryOverview(iso2: "PT", provider: p)!
        async let first: Void = o.load()
        try? await Task.sleep(for: .milliseconds(20))
        await o.load()
        await first
        #expect(o.summary?.country.name == "new")
        #expect(o.phase == .loaded)
    }

    @Test func notFoundIsNotAFailure() async {
        let o = CountryOverview(iso2: "XK", provider: CountryProvider([(.zero, .success(nil))]))!
        await o.load()
        #expect(o.phase == .notFound)
    }

    @Test func aFailedReloadKeepsWhatWasShown() async {
        let p = CountryProvider([(.zero, .success(summary("Portugal"))), (.zero, .failure(URLError(.notConnectedToInternet)))])
        let o = CountryOverview(iso2: "PT", provider: p)!
        await o.load()
        await o.load()
        #expect(o.summary?.country.name == "Portugal")
        #expect(o.phase == .failed(.offline))
    }

    @Test func aCellThatEmptiedIsLetGo() async {
        let p = CountryProvider([
            (.zero, .success(summary("Portugal", counts: [.init(type: .flood, severity: .severe, count: 1)]))),
            (.zero, .success(summary("Portugal", counts: [.init(type: .wind, severity: .minor, count: 1)]))),
        ])
        let o = CountryOverview(iso2: "PT", provider: p)!
        await o.load()
        o.cell = MatrixCell(type: .flood, severity: .severe)
        await o.load()
        #expect(o.cell == nil)
    }

    @Test func refreshWaitsUntilTheSummaryIsOld() async {
        let p = CountryProvider([(.zero, .success(summary("one", generatedAt: now))),
                                 (.zero, .success(summary("two", generatedAt: now)))])
        let clock = Clock(now)
        let o = CountryOverview(iso2: "PT", provider: p, now: { clock.now })!
        await o.load()
        clock.now = now.addingTimeInterval(30)
        await o.refresh(ifOlderThan: 60)
        #expect(o.summary?.country.name == "one")
        clock.now = now.addingTimeInterval(90)
        await o.refresh(ifOlderThan: 60)
        #expect(o.summary?.country.name == "two")
    }
}

private final class Clock: @unchecked Sendable {
    var now: Date
    init(_ now: Date) { self.now = now }
}

@MainActor @Suite struct SelectOutsideTheMapTests {
    @Test func anEventTheMapDoesNotShowCanStillBeSelected() async {
        // The overview lists events regardless of the map's filters; this one is filtered out.
        let m = LiveMapModel(provider: CountryProvider([]), debounce: .zero, now: { now })
        let hidden = event(.flood, .minor)
        m.select(hidden)
        #expect(m.selectedEvent?.id == hidden.id)
        m.selectedEventID = nil
        #expect(m.selectedEvent == nil)
        m.selectedEventID = hidden.id // the same id again still finds it
        #expect(m.selectedEvent?.id == hidden.id)
    }
}

@Suite struct NearbyWordsTests {
    @Test func offshoreEventsSayNear() {
        let quake = SampleData.events(now: now)[1] // off Coquimbo, CL nearby 38 km
        #expect(quake.nearbyWords(iso2: "CL", countryName: "Chile") == "Near Chile, 38 km offshore")
        let flood = SampleData.events(now: now)[2] // within IT
        #expect(flood.nearbyWords(iso2: "IT", countryName: "Italy") == nil)
    }
}
