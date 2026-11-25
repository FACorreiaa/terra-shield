import Foundation
import Testing
@testable import ShieldCore

/// Military bases (ADR 29): a static layer from Wikidata, loaded worldwide once.
enum BaseFixture {
    static let ramstein = MilitaryBase(
        id: "Q161348", name: "Ramstein Air Base", description: "US Air Force base in Rhineland-Palatinate, Germany",
        kind: .air, className: "air base", operatorName: "United States Air Force", country: "DE",
        coordinate: Coordinate(latitude: 49.436944444, longitude: 7.6),
        url: URL(string: "https://www.wikidata.org/wiki/Q161348")!)
    static let rota = MilitaryBase(
        id: "Q729530", name: "Naval Station Rota", description: "United States Navy base in Spain",
        kind: .naval, className: "naval air base", operatorName: "United States Navy", country: "ES",
        coordinate: Coordinate(latitude: 36.620763888, longitude: -6.331558333),
        url: URL(string: "https://www.wikidata.org/wiki/Q729530")!)
    static let fiji = MilitaryBase(
        id: "Q1", name: "Somewhere", description: nil, kind: .base, className: "military base",
        operatorName: nil, country: nil, coordinate: Coordinate(latitude: -17, longitude: 179.5),
        url: URL(string: "https://www.wikidata.org/wiki/Q1")!)

    static let note = "Installations listed on Wikidata with coordinates and no recorded closure. Wikidata records closures unevenly, so a listing is not proof a base is in use."

    static func layer(_ bases: [MilitaryBase]) -> BaseLayer {
        BaseLayer(bases: bases, updatedAt: Date(timeIntervalSince1970: 1_795_000_000),
                  attribution: "Wikidata", license: "CC0 1.0", note: note)
    }
}

@MainActor @Suite struct BasesLayerTests {
    let small = MapViewport(box: GeoBox(minLongitude: -10, minLatitude: 36, maxLongitude: -5, maxLatitude: 41), zoom: 7)
    let other = MapViewport(box: GeoBox(minLongitude: 0, minLatitude: 45, maxLongitude: 10, maxLatitude: 52), zoom: 6)

    private final class Clock: @unchecked Sendable { var now = Date(timeIntervalSince1970: 1_795_000_000) }

    private func model(_ p: ScriptedProvider, clock: Clock = Clock(), filters: FilterStore? = nil) -> LiveMapModel {
        LiveMapModel(provider: p, filters: filters, debounce: .zero, now: { clock.now })
    }

    @Test func offByDefaultAndNeverRequested() async {
        let p = ScriptedProvider()
        let m = model(p)
        m.viewportDidChange(small)
        await m.settle()
        #expect(!m.showsBases)
        #expect(p.basesRequests == 0)
        #expect(m.bases == nil)
    }

    @Test func turningItOnLoadsEveryBaseOnceWhateverTheView() async {
        let p = ScriptedProvider()
        let m = model(p)
        m.viewportDidChange(small)
        await m.settle()
        m.showsBases = true
        await m.settle()
        #expect(m.bases?.bases.count == 2)
        m.viewportDidChange(other)
        await m.settle()
        m.reload()
        await m.settle()
        #expect(p.basesRequests == 1)
    }

    @Test func togglingBasesLeavesTheHazardsAlone() async {
        let p = ScriptedProvider()
        let m = model(p)
        m.viewportDidChange(small)
        await m.settle()
        let before = p.requests.count
        m.showsBases = true
        await m.settle()
        m.showsBases = false
        await m.settle()
        #expect(p.requests.count == before)
    }

    @Test func savedWithTheFiltersAndLoadedAtTheFirstMapLoad() async {
        let store = FilterStore(suiteName: "bases-\(UUID())")
        let first = model(ScriptedProvider(), filters: store)
        first.showsBases = true
        let p = ScriptedProvider()
        let next = model(p, filters: store)
        #expect(next.showsBases)
        // Restoring must not load before the map has a view.
        #expect(p.basesRequests == 0)
        next.viewportDidChange(small)
        await next.settle()
        #expect(p.basesRequests == 1)
        #expect(next.bases?.bases.count == 2)
    }

    /// The toggle is hidden where the provider has no bases (sample data), so a layer saved as
    /// on elsewhere must not come back on there: its legend would stay with no way to close it.
    @Test func aSavedLayerStaysOffWhereTheProviderHasNoBases() async {
        let store = FilterStore(suiteName: "bases-\(UUID())")
        model(ScriptedProvider(), filters: store).showsBases = true
        let p = ScriptedProvider()
        p.offersBases = false
        let next = model(p, filters: store)
        #expect(!next.showsBases)
        next.viewportDidChange(small)
        await next.settle()
        #expect(p.basesRequests == 0)
    }

    @Test func turningItOffClearsTheLayerAndTheSelection() async {
        let p = ScriptedProvider()
        let m = model(p)
        m.viewportDidChange(small)
        m.showsBases = true
        await m.settle()
        m.selectedBaseID = "Q161348"
        #expect(m.selectedBase?.name == "Ramstein Air Base")
        m.showsBases = false
        #expect(m.bases == nil)
        #expect(m.selectedBaseID == nil)
    }

    @Test func aLateAnswerAfterTurningOffIsDropped() async {
        let p = ScriptedProvider()
        p.basesDelay = .milliseconds(60)
        let m = model(p)
        m.viewportDidChange(small)
        m.showsBases = true
        for _ in 0..<200 where p.basesRequests == 0 { try? await Task.sleep(for: .milliseconds(2)) }
        m.showsBases = false
        try? await Task.sleep(for: .milliseconds(150))
        #expect(m.bases == nil)
    }

    @Test func aFailureSaysSoAndIsRetriedNoSoonerThanAMinuteLater() async {
        let p = ScriptedProvider()
        p.basesFailure = URLError(.badServerResponse)
        let clock = Clock()
        let m = model(p, clock: clock)
        m.viewportDidChange(small)
        m.showsBases = true
        await m.settle()
        #expect(m.basesFailed)
        #expect(m.bases == nil)
        m.reload()
        await m.settle()
        #expect(p.basesRequests == 1)
        p.basesFailure = nil
        clock.now += 60
        m.reload()
        await m.settle()
        #expect(p.basesRequests == 2)
        #expect(!m.basesFailed)
        #expect(m.bases?.bases.count == 2)
    }

    /// The minute counts from when the failed request was asked, like flights: the map's own
    /// minute poll then retries it, rather than missing by the request's length and waiting two.
    @Test func theMinuteCountsFromWhenTheFailedRequestWasAsked() async throws {
        let p = ScriptedProvider()
        p.basesFailure = URLError(.badServerResponse)
        p.basesDelay = .milliseconds(80)
        let clock = Clock()
        let start = clock.now
        let m = model(p, clock: clock)
        m.viewportDidChange(small)
        m.showsBases = true
        try await Task.sleep(for: .milliseconds(20))
        clock.now = start + 0.5 // the answer arrives half a second after the request
        await m.settle()
        #expect(m.basesFailed)
        p.basesFailure = nil
        p.basesDelay = nil
        clock.now = start + 60
        m.reload()
        await m.settle()
        #expect(p.basesRequests == 2)
        #expect(m.bases != nil)
    }

    @Test func turningItOffAndOnAfterAFailureRetriesAtOnce() async {
        let p = ScriptedProvider()
        p.basesFailure = URLError(.badServerResponse)
        let m = model(p)
        m.viewportDidChange(small)
        m.showsBases = true
        await m.settle()
        #expect(m.basesFailed)
        p.basesFailure = nil
        m.showsBases = false
        m.showsBases = true
        await m.settle()
        #expect(p.basesRequests == 2)
        #expect(!m.basesFailed)
        #expect(m.bases?.bases.count == 2)
    }

    /// Only DEBUG automation can turn it on there (the toggle is hidden); it still asks nothing.
    @Test func neverAsksAProviderWithoutBases() async {
        let p = ScriptedProvider()
        p.offersBases = false
        let m = model(p)
        m.viewportDidChange(small)
        m.showsBases = true
        await m.settle()
        #expect(p.basesRequests == 0)
    }

    @Test func anOldSavedBlobReadsWithBasesOff() throws {
        let suite = "bases-old-\(UUID())"
        let blob = #"{"version":1,"excludedTypes":[],"minimumSeverity":"minor","excludedKinds":[],"showsDetections":true,"showsConflicts":true}"#
        UserDefaults(suiteName: suite)!.set(Data(blob.utf8), forKey: "shield.filters")
        let saved = FilterStore(suiteName: suite).load()
        #expect(saved.showsDetections)
        #expect(saved.showsConflicts)
        #expect(!saved.showsBases)
    }
}

@MainActor @Suite struct BaseSelectionTests {
    let small = MapViewport(box: GeoBox(minLongitude: -10, minLatitude: 36, maxLongitude: -5, maxLatitude: 41), zoom: 7)

    @Test func anEventAnAircraftOrABaseNeverTwo() async throws {
        let p = ScriptedProvider()
        let m = LiveMapModel(provider: p, debounce: .zero)
        m.viewportDidChange(small)
        m.showsBases = true
        m.showsFlights = true
        await m.settle()
        let event = try #require(m.snapshot?.events.first)

        m.selectedBaseID = "Q729530"
        m.selectedEventID = event.id
        #expect(m.selectedBaseID == nil)
        m.selectedBaseID = "Q729530"
        #expect(m.selectedEventID == nil)
        m.selectedAircraftHex = "a1"
        #expect(m.selectedBaseID == nil)
        m.selectedBaseID = "Q729530"
        #expect(m.selectedAircraftHex == nil)
        #expect(m.selectedBase?.name == "Naval Station Rota")
    }
}

@Suite struct BaseWordsTests {
    @Test func countsTheBasesInViewAcrossTheAntimeridian() {
        let layer = BaseFixture.layer([BaseFixture.ramstein, BaseFixture.rota, BaseFixture.fiji])
        #expect(layer.count(in: GeoBox(minLongitude: -10, minLatitude: 35, maxLongitude: 10, maxLatitude: 50)) == 2)
        #expect(layer.count(in: GeoBox(minLongitude: 179, minLatitude: -20, maxLongitude: -179, maxLatitude: -15)) == 1)
    }

    @Test func figuresSkipWhatWikidataDoesNotList() {
        #expect(BaseFixture.ramstein.figures == [
            .init(label: "Type", value: "Air base"),
            .init(label: "Operated by", value: "United States Air Force"),
            .init(label: "Country", value: "Germany"),
            .init(label: "Coordinates", value: "49.4369° N, 7.6000° E"),
            .init(label: "Wikidata item", value: "Q161348"),
        ])
        #expect(BaseFixture.fiji.figures == [
            .init(label: "Type", value: "Military base"),
            .init(label: "Coordinates", value: "17.0000° S, 179.5000° E"),
            .init(label: "Wikidata item", value: "Q1"),
        ])
    }

    @Test func theListedAsOfLineNeverSaysActive() {
        let line = BaseFixture.layer([]).listedLine
        #expect(line == "Listed on Wikidata · refreshed 18 Nov 2026")
        #expect(!line.lowercased().contains("active"))
        var empty = BaseFixture.layer([])
        empty.updatedAt = nil
        #expect(empty.listedLine == "Listed on Wikidata")
    }

    @Test func unknownKindsAreBases() {
        #expect(MilitaryBase.Kind(serverValue: "air") == .air)
        #expect(MilitaryBase.Kind(serverValue: "spaceport") == .base)
    }

    @Test func providersWithoutBasesDoNotOfferTheLayer() {
        #expect(!SampleEventProvider().supportsBases)
        let p = ScriptedProvider()
        p.offersBases = false
        #expect(!p.supportsBases)
    }
}
