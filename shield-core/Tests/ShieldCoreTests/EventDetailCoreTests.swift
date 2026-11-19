import Foundation
import Testing
@testable import ShieldCore

private let at = Date(timeIntervalSince1970: 1_794_838_800) // 2026-11-16 14:20 UTC

private func changes(_ json: String) -> [Change] {
    let object = try! JSONDecoder().decode([String: JSONValue].self, from: Data(json.utf8))
    return Change.parse(object)
}

private func event(
    source: String = "gdacs", type: HazardType = .tropicalCyclone, kind: EventKind = .observed,
    severity: Severity = .severe, sourceSeverity: String? = nil, provisional: Bool = false,
    attrs: [String: JSONValue] = [:]
) -> HazardEvent {
    HazardEvent(id: UUID(), source: source, type: type, kind: kind, severity: severity,
                sourceSeverity: sourceSeverity, severityProvisional: provisional, title: "t",
                centroid: Coordinate(latitude: 0, longitude: 0), observedAt: at, updatedAt: at,
                fetchedAt: at, attrs: attrs)
}

// MARK: - Timeline

@Suite struct EventRevisionTests {
    @Test func aSeverityChangeNamesBothLevels() {
        let r = EventRevision(at: at, changes: changes(#"{"severity": {"from": "moderate", "to": "severe"}}"#))
        #expect(r.words == "Severity Moderate → Severe")
    }

    @Test func otherChangesFollowInReadingOrder() {
        let r = EventRevision(at: at, changes: changes(#"""
        {"geometry": {"changed": true}, "severity": {"from": "minor", "to": "severe"}}
        """#))
        #expect(r.words == "Severity Minor → Severe · area updated")
        let gdacs = EventRevision(at: at, changes: changes(#"""
        {"geometry": {"changed": true}, "sourceSeverity": {"from": "48470 ha", "to": "48484 ha"}}
        """#))
        #expect(gdacs.words == "Source now says “48484 ha” · area updated")
        let resolved = EventRevision(at: at, changes: changes(#"""
        {"status": {"from": "active", "to": "resolved"}, "endedAt": {"from": null, "to": "2026-11-19T10:09:07Z"}}
        """#))
        #expect(resolved.words == "Resolved")
    }

    @Test func aSeverityWithoutItsOldLevelSaysWhereItIsNow() {
        let r = EventRevision(at: at, changes: changes(#"{"severity": {"from": null, "to": "minor"}}"#))
        #expect(r.words == "Severity now Minor")
    }

    @Test func theTimelineLeavesOutRenamesAndEmptyRevisions() {
        let e = event()
        let detail = EventDetail(event: e, geometry: .point(e.centroid), revisions: [
            EventRevision(at: at, changes: changes(#"{"title": {"from": "a", "to": "b"}}"#)),
            EventRevision(at: at.addingTimeInterval(-60), changes: []),
            EventRevision(at: at.addingTimeInterval(-120), changes: changes(#"{"geometry": {"changed": true}}"#)),
        ])
        #expect(detail.timeline.map(\.words) == ["Area updated"])
    }
}

// MARK: - Severity labels (backend S1.x, owner 2026-11-19)

@Suite struct SeverityBasisTests {
    @Test func windSpeedNamesTheDecidingNumber() {
        let e = event(sourceSeverity: "Green alert, Hurricane/Typhoon > 74 mph (maximum wind speed of 194 km/h)",
                      attrs: ["severityBasis": "wind", "severityValue": 240.7392, "severityUnit": "km/h",
                              "alertLevel": "Green"])
        #expect(e.severityRating == "Rated by wind speed · 241 km/h")
        #expect(e.severityNote == "GDACS’s own alert is Green.")
    }

    @Test func burnedAreaNamesTheDecidingNumber() {
        let e = event(type: .wildfire, severity: .extreme,
                      attrs: ["severityBasis": "area", "severityValue": 48484, "severityUnit": "ha",
                              "alertLevel": "Green"])
        #expect(e.severityRating == "Rated by burned area · 48,484 ha")
        #expect(e.severityNote == "GDACS’s own alert is Green.")
    }

    @Test func theAlertDecidingNeedsNoLabel() {
        let e = event(attrs: ["severityBasis": "alert", "alertLevel": "Orange"])
        #expect(e.severityRating == nil)
        #expect(e.severityNote == nil)
    }

    @Test func aPrescribedBurnWinsOverTheProvisionalWords() {
        let e = event(source: "eonet", type: .wildfire, severity: .minor, sourceSeverity: "1,402 acres burned",
                      provisional: true,
                      attrs: ["prescribed": true, "magnitudeValue": 1401.8, "magnitudeUnit": "acres"])
        #expect(e.isPrescribed)
        #expect(e.severityRating == "Prescribed burn")
        #expect(e.severityNote == "A planned, controlled fire, shown as minor whatever its size.")
        #expect(!e.showsProvisional)
    }

    @Test func provisionalWordsFollowDesignFive() {
        let fire = event(source: "eonet", type: .wildfire, severity: .moderate, provisional: true,
                         attrs: ["magnitudeValue": 2378.77, "magnitudeUnit": "acres"])
        #expect(fire.severityRating == nil)
        #expect(fire.showsProvisional)
        #expect(fire.severityNote == "Estimated from burned area; the source publishes no severity.")
        let volcano = event(source: "eonet", type: .volcano, severity: .minor, provisional: true)
        #expect(volcano.severityNote == "The source publishes no scale; shown as minor until an official source rates it.")
    }
}

// MARK: - Key figures (DESIGN.md §10.3 item 4)

@Suite struct KeyFigureTests {
    @Test func earthquakeLeadsWithMagnitude() {
        let e = event(source: "usgs", type: .earthquake,
                      attrs: ["mag": 6.1, "magType": "mww", "depthKm": 10, "pagerAlert": "green",
                              "tsunami": true, "reviewStatus": "reviewed"])
        #expect(e.keyFigures.map(\.label) == ["Magnitude", "Depth", "PAGER", "Tsunami", "Review"])
        #expect(e.keyFigures.first?.value == "6.1 mww")
        #expect(e.keyFigures.first?.isLead == true)
        #expect(e.keyFigures[1].value == "10 km")
        #expect(e.keyFigures[2].value == "Green")
        #expect(e.keyFigures[3].value == "Possible (USGS flag)")
        #expect(e.keyFigures[4].value == "Reviewed")
    }

    @Test func cycloneShowsItsWindInTheSourcesUnit() {
        let e = event(attrs: ["severityValue": 83.3328, "severityUnit": "km/h", "vulnerability": "High",
                              "populationText": "28 million in Tropical Storm"])
        #expect(e.keyFigures.map(\.label) == ["Max wind", "Population", "Vulnerability"])
        #expect(e.keyFigures[0].value == "83 km/h")
        #expect(e.keyFigures[1].value == "28 million in Tropical Storm")
    }

    @Test func wildfiresShowTheirBurnedArea() {
        let eonet = event(source: "eonet", type: .wildfire,
                          attrs: ["magnitudeValue": 2378.77, "magnitudeUnit": "acres"])
        #expect(eonet.keyFigures.map(\.value) == ["2,379 acres"])
        let hectare = event(source: "eonet", type: .wildfire,
                            attrs: ["magnitudeValue": 5000, "magnitudeUnit": "hectare"])
        #expect(hectare.keyFigures.map(\.value) == ["5,000 ha"])
        let gdacs = event(type: .wildfire, attrs: ["severityValue": 48484, "severityUnit": "ha"])
        #expect(gdacs.keyFigures.first == KeyFigure(label: "Burned area", value: "48,484 ha", isLead: true))
    }

    @Test func aValueWithoutAKnownMeaningIsNotShown() {
        // GDACS floods send "Magnitude 0"; a flood's value has no label we can stand behind.
        let flood = event(type: .flood, attrs: ["severityValue": 0, "severityUnit": ""])
        #expect(flood.keyFigures.isEmpty)
        let noUnit = event(attrs: ["severityValue": 120])
        #expect(noUnit.keyFigures.isEmpty)
    }

    @Test func warningsShowTheCapAxesApart() {
        let e = event(source: "meteoalarm", type: .wind, kind: .warning,
                      attrs: ["urgency": "Immediate", "certainty": "Likely", "sender": "Deutscher Wetterdienst",
                              "areas": ["Kreis Harz", "Goslar"]])
        #expect(e.keyFigures.map(\.label) == ["Urgency", "Certainty", "Issued by", "Areas"])
        #expect(e.keyFigures.last?.value == "Kreis Harz, Goslar")
    }
}

// MARK: - Times

@Suite struct DetailTimesTests {
    @Test func olderThanADayCarriesTheDate() {
        let now = at
        #expect(Freshness.stamp(now.addingTimeInterval(-720), now: now) == "12 min ago · 14:08 UTC")
        #expect(Freshness.stamp(now.addingTimeInterval(-3 * 86_400), now: now) == "3 d ago · 13 Nov, 14:20 UTC")
        #expect(Freshness.stamp(now.addingTimeInterval(2 * 86_400), now: now) == "in 2 d · 18 Nov, 14:20 UTC")
    }

    @Test func warningsEndWhenTheyExpire() {
        var e = event(source: "meteoalarm", kind: .warning, attrs: ["expiresAt": "2026-11-16T18:00:00Z"])
        e.startedAt = at.addingTimeInterval(-3600)
        #expect(e.detailTimes(now: at).map(\.label) == ["Started", "Observed", "Updated", "Expires"])
        #expect(e.detailTimes(now: at).last?.date == Date(timeIntervalSince1970: 1_794_852_000))
        var ended = event()
        ended.endedAt = at
        #expect(ended.detailTimes(now: at).map(\.label) == ["Observed", "Updated", "Ended"])
    }

    @Test func aWarningThatHasNotBegunStarts() {
        // MeteoAlarm issues warnings ahead of their onset: "Starts in 7 h", never "Started in".
        var e = event(source: "meteoalarm", kind: .warning)
        e.startedAt = at.addingTimeInterval(7 * 3600)
        #expect(e.detailTimes(now: at).first?.label == "Starts")
        #expect(e.detailTimes(now: at.addingTimeInterval(8 * 3600)).first?.label == "Started")
    }
}

// MARK: - The model's detail

/// Answers detail requests from a table the test fills: an EventDetail, nil (404), or an error.
private final class DetailProvider: HazardEventProvider, @unchecked Sendable {
    private let lock = NSLock()
    private var _answers: [UUID: Result<EventDetail?, any Error>] = [:]
    let isSampleData = false
    let events: [HazardEvent]

    init(events: [HazardEvent]) { self.events = events }

    func answer(_ id: UUID, with result: Result<EventDetail?, any Error>) { lock.withLock { _answers[id] = result } }

    func events(matching query: EventQuery) async throws -> [HazardEvent] { [] }
    func event(id: UUID) async throws -> HazardEvent? { nil }
    func sources() async throws -> [Source] { [] }
    func detections(in viewport: MapViewport) async throws -> DetectionLayer { .empty }
    func snapshot(matching query: EventQuery, in viewport: MapViewport) async throws -> MapSnapshot {
        MapSnapshot(events: events, clusters: [], truncated: false, generatedAt: at)
    }
    func detail(id: UUID) async throws -> EventDetail? {
        guard let answer = lock.withLock({ _answers[id] }) else { return nil }
        return try answer.get()
    }
}

@MainActor @Suite struct SelectedDetailTests {
    let e = event()

    @Test func aLoadedDetailCarriesItsTimelineAndArea() async throws {
        let p = DetailProvider(events: [e])
        let ring = [Coordinate(latitude: 0, longitude: 0), Coordinate(latitude: 1, longitude: 0),
                    Coordinate(latitude: 1, longitude: 1), Coordinate(latitude: 0, longitude: 0)]
        let d = EventDetail(event: e, geometry: .polygons([[ring]]),
                            revisions: [EventRevision(at: at, changes: changes(#"{"geometry": {"changed": true}}"#))])
        p.answer(e.id, with: .success(d))
        let m = LiveMapModel(provider: p, debounce: .zero)
        m.selectedEventID = e.id
        #expect(m.selectedDetail == .loading)
        await m.settleArea()
        #expect(m.selectedDetail == .loaded(d))
        #expect(m.selectedGeometry == d.geometry)
        m.selectedEventID = nil
        #expect(m.selectedDetail == nil)
    }

    @Test func anEventOutOfViewStillShowsFromItsDetail() async throws {
        // Selected by id (a deep link, or panned away): nothing in the snapshot holds it.
        let p = DetailProvider(events: [])
        p.answer(e.id, with: .success(EventDetail(event: e, geometry: .point(e.centroid))))
        let m = LiveMapModel(provider: p, debounce: .zero)
        m.selectedEventID = e.id
        await m.settleArea()
        #expect(m.selectedEvent?.id == e.id)
    }

    @Test func aMissingEventIsGone() async throws {
        let p = DetailProvider(events: [e])
        p.answer(e.id, with: .success(nil))
        let m = LiveMapModel(provider: p, debounce: .zero)
        m.viewportDidChange(MapViewport(box: .world, zoom: 9))
        await m.settle()
        m.selectedEventID = e.id
        await m.settleArea()
        #expect(m.selectedDetail == .gone(e))
    }

    @Test func anIdNeverSeenThatIsMissingClearsTheSelection() async throws {
        // Nothing to say what it was, so nothing stays selected with nothing on screen.
        let p = DetailProvider(events: [])
        let m = LiveMapModel(provider: p, debounce: .zero)
        m.selectedEventID = UUID()
        await m.settleArea()
        #expect(m.selectedEventID == nil)
        #expect(m.selectedDetail == nil)
    }

    @Test func aFailedRefreshKeepsTheLastDetail() async throws {
        let p = DetailProvider(events: [e])
        let d = EventDetail(event: e, geometry: .point(e.centroid))
        p.answer(e.id, with: .success(d))
        let m = LiveMapModel(provider: p, debounce: .zero)
        m.selectedEventID = e.id
        await m.settleArea()
        p.answer(e.id, with: .failure(URLError(.timedOut)))
        m.apply(.upsert(e))
        await m.settleArea()
        #expect(m.selectedDetail == .loaded(d))
    }

    @Test func aFirstFailureSaysSo() async throws {
        let p = DetailProvider(events: [e])
        p.answer(e.id, with: .failure(URLError(.timedOut)))
        let m = LiveMapModel(provider: p, debounce: .zero)
        m.selectedEventID = e.id
        await m.settleArea()
        #expect(m.selectedDetail == .failed)
    }

    @Test func anEventTheSourceRetractsStaysOpenAsGone() async throws {
        let p = DetailProvider(events: [e])
        p.answer(e.id, with: .success(EventDetail(event: e, geometry: .point(e.centroid))))
        let m = LiveMapModel(provider: p, debounce: .zero)
        m.viewportDidChange(MapViewport(box: .world, zoom: 9))
        await m.settle()
        m.selectedEventID = e.id
        await m.settleArea()
        m.apply(.removed(e.id))
        #expect(m.selectedEventID == e.id)
        #expect(m.selectedDetail == .gone(e))
        #expect(m.selectedEvent?.id == e.id)
        #expect(m.snapshot?.events.isEmpty == true)
    }
}

extension JSONValue: ExpressibleByStringLiteral, ExpressibleByFloatLiteral,
    ExpressibleByIntegerLiteral, ExpressibleByBooleanLiteral,
    ExpressibleByArrayLiteral {
    public init(stringLiteral value: String) { self = .string(value) }
    public init(floatLiteral value: Double) { self = .number(value) }
    public init(integerLiteral value: Int) { self = .number(Double(value)) }
    public init(booleanLiteral value: Bool) { self = .bool(value) }
    public init(arrayLiteral elements: JSONValue...) { self = .array(elements) }
}

// MARK: - Show on map

@Suite struct EventFrameTests {
    @Test func aPointGetsAFixedSpan() {
        let box = EventGeometry.point(Coordinate(latitude: 10, longitude: 20))
            .frame(around: Coordinate(latitude: 10, longitude: 20))
        #expect(box == GeoBox(minLongitude: 19, minLatitude: 9, maxLongitude: 21, maxLatitude: 11))
    }

    @Test func anAreaIsPaddedByAFifthEachSide() {
        let ring = [(0.0, 0.0), (10, 0), (10, 5), (0, 5), (0, 0)].map { Coordinate(latitude: $0.1, longitude: $0.0) }
        let box = EventGeometry.polygons([[ring]]).frame(around: Coordinate(latitude: 2.5, longitude: 5))
        #expect(box == GeoBox(minLongitude: -2, minLatitude: -1, maxLongitude: 12, maxLatitude: 6))
    }

    @Test func anAreaAcrossTheAntimeridianStaysOneBox() {
        let ring = [(170.0, 0.0), (-170, 0), (-170, 10), (170, 10), (170, 0)].map { Coordinate(latitude: $0.1, longitude: $0.0) }
        let box = EventGeometry.polygons([[ring]]).frame(around: Coordinate(latitude: 5, longitude: 180))
        #expect(box.crossesAntimeridian)
        #expect(box.minLongitude == 166 && box.maxLongitude == -166)
    }
}
