import Foundation
import Testing
@testable import ShieldCore

/// Armed conflicts (ADR 27, docs/attrs.md "UCDP"): a layer beside hazards, never one of them.
enum ConflictFixture {
    static let now = Date(timeIntervalSince1970: 1_795_110_000) // 2026-11-19 17:40 UTC

    /// The live Yemen row on 2026-11-19, the largest in the 90 days.
    static func yemen(precision: String = "area", datePrecision: String = "months",
                      codeStatus: String = "Check deaths") -> HazardEvent {
        HazardEvent(
            id: UUID(uuidString: "9127E03C-FCD5-4577-9B7D-698E63A5D447")!, source: "ucdp",
            type: .conflict, kind: .observed, severity: .extreme,
            sourceSeverity: "797 deaths (335–823)", severityProvisional: true,
            title: "Government of Yemen (North Yemen) – Forces of the Presidential Leadership Council",
            centroid: Coordinate(latitude: 13.58, longitude: 43.48),
            countries: [EventCountry(iso2: "YE", relation: .within)],
            startedAt: date("2026-08-09"), observedAt: date("2026-09-10"), updatedAt: date("2026-09-10"),
            fetchedAt: now, url: URL(string: "https://ucdp.uu.se/country/678"),
            attrs: [
                "violenceType": .string("state_based"),
                "conflictName": .string("Yemen (North Yemen): Government"),
                "sideA": .string("Government of Yemen (North Yemen)"),
                "sideB": .string("Forces of the Presidential Leadership Council"),
                "deathsBest": .number(797), "deathsLow": .number(335), "deathsHigh": .number(823),
                "deathsSideA": .number(456), "deathsSideB": .number(341),
                "deathsCivilians": .number(0), "deathsUnknown": .number(0),
                "locationPrecision": .string(precision), "datePrecision": .string(datePrecision),
                "codeStatus": .string(codeStatus), "sourceCount": .number(2),
                "expiresAt": .string("2026-12-09T00:00:00Z"),
            ])
    }

    static func oneSided() -> HazardEvent {
        var e = yemen(precision: "exact", datePrecision: "day", codeStatus: "Clear")
        e.attrs["violenceType"] = .string("one_sided")
        e.attrs["sideA"] = .string("IS")
        e.attrs["sideB"] = .string("Civilians")
        e.attrs["deathsSideA"] = .number(0); e.attrs["deathsSideB"] = .number(0)
        e.attrs["deathsCivilians"] = .number(12)
        e.startedAt = date("2026-09-21"); e.observedAt = date("2026-09-21")
        e.severityProvisional = false // Clear, and known to the day
        return e
    }

    static func date(_ day: String) -> Date { try! Date(day + "T00:00:00Z", strategy: .iso8601) }
}

@Suite struct ConflictTaxonomyTests {
    @Test func conflictIsAType_butNotAHazard() throws {
        let decoded = try JSONDecoder().decode(HazardType.self, from: Data(#""conflict""#.utf8))
        #expect(decoded == .conflict)
        #expect(HazardType.conflict.label == "Armed conflict")
        #expect(!HazardType.hazards.contains(.conflict))
        #expect(HazardType.hazards.count == HazardType.allCases.count - 1)
    }

    @Test func theHazardFiltersNeverPickUpConflicts() {
        #expect(!EventQuery().types.contains(.conflict))
        let store = FilterStore(suiteName: "conflict-tests", key: "f")
        store.clear()
        store.save(SavedFilters())
        #expect(!store.load().query.types.contains(.conflict))
        #expect(EventQuery().label(for: .types) == "\(HazardType.hazards.count) types")
    }

    @Test func theLayerQueryAsksForConflictsAtAnyAge() {
        // Shown for 90 days after their end date by the server; a 24 h window would hide them all.
        let q = EventQuery.conflicts
        #expect(q.types == [.conflict] && q.kinds == [.observed] && q.window == nil)
        #expect(q.minimumSeverity == .minor && !q.includeResolved)
    }
}

@Suite struct ConflictFactsTests {
    let now = ConflictFixture.now

    @Test func figuresSayWhoDiedAndHowSureItIs() {
        let f = ConflictFixture.yemen().keyFigures.map { "\($0.label): \($0.value)" }
        #expect(f == [
            "Deaths: 797",
            "Estimate range: 335–823",
            "Violence: State-based (a government is a side)",
            "Conflict: Yemen (North Yemen): Government",
            "Side A: Government of Yemen (North Yemen) · 456 deaths",
            "Side B: Forces of the Presidential Leadership Council · 341 deaths",
            "Location: Wider area (approximate)",
            "UCDP review: Check deaths",
            "Sources used: 2",
        ])
        #expect(ConflictFixture.yemen().keyFigures.first?.isLead == true)
    }

    @Test func oneSidedViolenceCountsTheCivilians() {
        let f = ConflictFixture.oneSided().keyFigures.map { "\($0.label): \($0.value)" }
        #expect(f.contains("Violence: One-sided (against civilians)"))
        #expect(f.contains("Side B: Civilians · 12 deaths"))
        let exactAndClear = !f.contains { $0.hasPrefix("Location") || $0.hasPrefix("UCDP review") }
        #expect(exactAndClear)
    }

    @Test func anApproximateLocationIsSaidOnTheDetail() {
        #expect(ConflictFixture.yemen(precision: "admin2").locationNote
                == "Approximate location: UCDP places this at the district’s centre.")
        #expect(ConflictFixture.yemen(precision: "admin1").locationNote
                == "Approximate location: UCDP places this at the province’s centre.")
        #expect(ConflictFixture.yemen(precision: "within_25km").locationNote
                == "Approximate location: within 25 km of the point shown.")
        #expect(ConflictFixture.yemen(precision: "exact").locationNote == nil)
        #expect(ConflictFixture.yemen().isApproximateLocation)
        #expect(!ConflictFixture.oneSided().isApproximateLocation)
    }

    @Test func itsTimesAreDatesNotUpdates() {
        // UCDP gives no edit time: the event's own dates, never "updated N ago".
        let t = ConflictFixture.yemen().detailTimes(now: now)
        #expect(t.map(\.label) == ["From", "To", "Shown until"])
        let allDays = t.allSatisfy(\.dateOnly)
        #expect(allDays)
        let single = ConflictFixture.oneSided().detailTimes(now: now)
        #expect(single.map(\.label) == ["Date", "Shown until"])
    }

    @Test func theSeverityNoteExplainsTheDeathCount() {
        // docs/severity.md: the level comes from UCDP's best estimate of deaths.
        #expect(ConflictFixture.oneSided().severityNote == "Rated from UCDP’s best estimate of deaths.")
        var checking = ConflictFixture.yemen(datePrecision: "day")
        checking.severityProvisional = true
        #expect(checking.severityNote == "Rated from UCDP’s best estimate of deaths. Provisional: UCDP is still checking this event (Check deaths).")
        var monthly = ConflictFixture.yemen(codeStatus: "Clear")
        monthly.severityProvisional = true
        #expect(monthly.severityNote == "Rated from UCDP’s best estimate of deaths. Provisional: one total for incidents over a month or more.")
    }

    @Test func theRowLineIsAMonthlyEstimate() {
        #expect(ConflictFixture.yemen().conflictDateLine == "9 Aug – 10 Sep · monthly estimate (UCDP)")
        #expect(ConflictFixture.oneSided().conflictDateLine == "21 Sep · monthly estimate (UCDP)")
    }
}

/// The layer: off by default, its own request, never in the hazard counts, saved with the filters.
@MainActor @Suite struct ConflictLayerTests {
    private func viewport() -> MapViewport {
        MapViewport(box: GeoBox(minLongitude: -180, minLatitude: -60, maxLongitude: 180, maxLatitude: 80), zoom: 2)
    }

    @Test func offByDefaultAndAskedForOnlyWhenShown() async {
        let p = ScriptedProvider()
        let m = LiveMapModel(provider: p, debounce: .zero)
        m.viewportDidChange(viewport())
        await m.settle()
        #expect(p.conflictRequests.isEmpty && m.conflicts == nil)

        m.showsConflicts = true
        await m.settle()
        #expect(p.conflictRequests == [EventQuery.conflicts])
        #expect(m.conflicts?.events.map(\.type) == [.conflict])
        // The hazard snapshot and its counts stay hazards only.
        let hazardTypes = m.snapshot?.events.map(\.type) ?? []
        #expect(!hazardTypes.contains(.conflict))

        m.showsConflicts = false
        #expect(m.conflicts == nil)
    }

    @Test func aConflictCanBeSelected() async {
        let m = LiveMapModel(provider: ScriptedProvider(), debounce: .zero)
        m.viewportDidChange(viewport())
        m.showsConflicts = true
        await m.settle()
        m.selectedEventID = ConflictFixture.yemen().id
        #expect(m.selectedEvent?.type == .conflict)
    }

    @Test func itsFailureLeavesTheHazardsAlone() async {
        let p = ScriptedProvider()
        p.conflictFailure = URLError(.badServerResponse)
        let m = LiveMapModel(provider: p, debounce: .zero)
        m.viewportDidChange(viewport())
        m.showsConflicts = true
        await m.settle()
        #expect(m.phase == .loaded)
        #expect(m.conflictsFailed)
        p.conflictFailure = nil
        m.reload()
        await m.settle()
        #expect(!m.conflictsFailed && m.conflicts != nil)
    }

    @Test func theToggleIsSavedWithTheFilters() async {
        let store = FilterStore(suiteName: "conflict-layer-tests", key: "f")
        store.clear()
        let m = LiveMapModel(provider: ScriptedProvider(), filters: store, debounce: .zero)
        m.showsConflicts = true
        #expect(store.load().showsConflicts)
        let again = LiveMapModel(provider: ScriptedProvider(), filters: store, debounce: .zero)
        #expect(again.showsConflicts)
    }
}
