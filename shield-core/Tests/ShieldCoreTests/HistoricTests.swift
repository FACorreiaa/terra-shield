import Foundation
import Testing
@testable import ShieldCore

/// Historic sites (ADR 30): a static layer from Wikidata, counted per grid cell below zoom 8 and
/// one point per site from zoom 8, filterable by kind.
enum HistoricFixture {
    static let neuschwanstein = HistoricSite(
        id: "Q4048", name: "Neuschwanstein Castle", description: "castle in Bavaria, Germany",
        kind: .castle, kindName: "castle", className: "castle", country: "DE",
        coordinate: Coordinate(latitude: 47.5575, longitude: 10.75),
        url: URL(string: "https://www.wikidata.org/wiki/Q4048")!)
    static let fortress = HistoricSite(
        id: "Q2", name: "Festung Ehrenberg", description: nil,
        kind: .military, kindName: "military", className: "fortress", country: "AT",
        coordinate: Coordinate(latitude: 47.47, longitude: 10.7),
        url: URL(string: "https://www.wikidata.org/wiki/Q2")!)
    static let tower = HistoricSite(
        id: "Q3", name: "Old tower", description: nil,
        kind: nil, kindName: "building", className: "tower", country: nil,
        coordinate: Coordinate(latitude: 47.5, longitude: 10.6),
        url: URL(string: "https://www.wikidata.org/wiki/Q3")!)

    static let note = "Historic sites listed on Wikidata: forts, castles, bunkers, barracks, war cemeteries and former bases."

    static func points(_ sites: [HistoricSite], truncated: Bool = false) -> HistoricLayer {
        HistoricLayer(content: .points(sites, truncated: truncated),
                      updatedAt: Date(timeIntervalSince1970: 1_795_000_000),
                      attribution: "Wikidata", license: "CC0 1.0", note: note)
    }

    static func grid(_ cells: [HistoricCell], cellDegrees: Double = 0.703125) -> HistoricLayer {
        HistoricLayer(content: .grid(cells, cellDegrees: cellDegrees), updatedAt: nil,
                      attribution: "Wikidata", license: "CC0 1.0", note: note)
    }

    static let cells = [
        HistoricCell(center: Coordinate(latitude: 50.98, longitude: 4.57), count: 574,
                     kinds: ["military": 522, "castle": 28, "memorial": 24]),
        HistoricCell(center: Coordinate(latitude: -33.9, longitude: 151.2), count: 3, kinds: ["memorial": 3]),
    ]
}

@Suite struct HistoricRequestTests {
    let alps = GeoBox(minLongitude: 10, minLatitude: 47, maxLongitude: 11, maxLatitude: 48)
    let all = Set(HistoricKind.allCases)

    @Test func theWholeWorldIsOneGridAnswerBelowZoom6() {
        let r = HistoricRequest(MapViewport(box: alps, zoom: 4), kinds: all)
        #expect(r.box == nil && r.zoom == 4 && !r.isPoints)
        // Every view at zoom 4 is in it; zoom 5 counts in other cells.
        #expect(r.serves(MapViewport(box: .world, zoom: 4), kinds: all, truncated: false))
        #expect(!r.serves(MapViewport(box: alps, zoom: 5), kinds: all, truncated: false))
    }

    @Test func zoom6And7AskForTheViewWithHalfAViewAroundIt() {
        let r = HistoricRequest(MapViewport(box: alps, zoom: 6), kinds: all)
        #expect(r.box == GeoBox(minLongitude: 9.5, minLatitude: 46.5, maxLongitude: 11.5, maxLatitude: 48.5))
        let nudged = GeoBox(minLongitude: 10.4, minLatitude: 47.3, maxLongitude: 11.4, maxLatitude: 48.3)
        #expect(r.serves(MapViewport(box: nudged, zoom: 6), kinds: all, truncated: false))
        let away = GeoBox(minLongitude: 12, minLatitude: 47, maxLongitude: 13, maxLatitude: 48)
        #expect(!r.serves(MapViewport(box: away, zoom: 6), kinds: all, truncated: false))
    }

    /// The densest zoom-8 view of a large Mac window holds 9,630 sites; padding it passes the
    /// server's 10,000 cap, and a truncated answer can't stand for any view.
    @Test func pointsAskForTheExactViewAndServeAnyViewInsideIt() {
        let r = HistoricRequest(MapViewport(box: alps, zoom: 8), kinds: all)
        #expect(r.box == alps && r.isPoints)
        let inside = GeoBox(minLongitude: 10.2, minLatitude: 47.2, maxLongitude: 10.6, maxLatitude: 47.6)
        #expect(r.serves(MapViewport(box: inside, zoom: 10), kinds: all, truncated: false))
        #expect(!r.serves(MapViewport(box: inside, zoom: 10), kinds: all, truncated: true))
        // Zooming out to the grid needs the grid.
        #expect(!r.serves(MapViewport(box: inside, zoom: 7), kinds: all, truncated: false))
    }

    /// Above `chipLimit` the sites are drawn as dots, which can't be picked: zooming in asks again
    /// for the smaller view, so the chips come back once few enough are in it.
    @Test func anAnswerTooDenseForChipsIsAskedAgainOnZoomingIn() {
        let r = HistoricRequest(MapViewport(box: alps, zoom: 8), kinds: all)
        let inside = GeoBox(minLongitude: 10.2, minLatitude: 47.2, maxLongitude: 10.6, maxLatitude: 47.6)
        #expect(r.serves(MapViewport(box: inside, zoom: 10), kinds: all, truncated: false, sites: HistoricLayer.chipLimit))
        #expect(!r.serves(MapViewport(box: inside, zoom: 10), kinds: all, truncated: false, sites: HistoricLayer.chipLimit + 1))
        // The same view is still served: asking again would bring the same answer.
        #expect(r.serves(MapViewport(box: alps, zoom: 8), kinds: all, truncated: false, sites: HistoricLayer.chipLimit + 1))
    }

    @Test func anAnswerForMoreGroupsServesFewerUnlessCutShort() {
        let r = HistoricRequest(MapViewport(box: alps, zoom: 9), kinds: all)
        #expect(r.serves(MapViewport(box: alps, zoom: 9), kinds: [.castle], truncated: false))
        #expect(!r.serves(MapViewport(box: alps, zoom: 9), kinds: [.castle], truncated: true))
        let castles = HistoricRequest(MapViewport(box: alps, zoom: 9), kinds: [.castle, .memorial])
        #expect(castles.serves(MapViewport(box: alps, zoom: 9), kinds: [.castle], truncated: false))
        #expect(!castles.serves(MapViewport(box: alps, zoom: 9), kinds: [.castle, .military], truncated: false))
        #expect(!castles.serves(MapViewport(box: alps, zoom: 9), kinds: all, truncated: false))
        let grid = HistoricRequest(MapViewport(box: alps, zoom: 4), kinds: all)
        #expect(grid.serves(MapViewport(box: alps, zoom: 4), kinds: [.memorial], truncated: false))
    }

    @Test func filteringKeepsOnlyTheGroupsShown() {
        let layer = HistoricFixture.points([HistoricFixture.neuschwanstein, HistoricFixture.fortress, HistoricFixture.tower])
        #expect(layer.filtered(to: [.castle]).sites.map(\.id) == ["Q4048"])
        #expect(layer.filtered(to: Set(HistoricKind.allCases)).sites.count == 3) // the unknown group too
        let grid = HistoricFixture.grid(HistoricFixture.cells).filtered(to: [.castle])
        guard case .grid(let cells, _) = grid.content else { Issue.record("expected a grid"); return }
        #expect(cells.map(\.count) == [28]) // Sydney's memorials leave
    }

    @Test func kindsAreSentOnlyWhenSomeAreOff() {
        #expect(HistoricRequest(MapViewport(box: alps, zoom: 9), kinds: all).kinds == nil)
        let castles = HistoricRequest(MapViewport(box: alps, zoom: 9), kinds: [.castle])
        #expect(castles.kinds == [.castle])
        #expect(!castles.serves(MapViewport(box: alps, zoom: 9), kinds: all, truncated: false))
    }

    @Test func aPaddedBoxStaysOnTheMapAndWrapsTheAntimeridian() {
        let fiji = GeoBox(minLongitude: 178, minLatitude: 80, maxLongitude: -179, maxLatitude: 84)
        let p = fiji.padded(by: 0.5)
        #expect(p.minLongitude == 176.5 && p.maxLongitude == -177.5)
        #expect(p.maxLatitude == GeoBox.mercatorLatitudeLimit && p.minLatitude == 78)
        #expect(p.contains(fiji))
        let wide = GeoBox(minLongitude: -100, minLatitude: 0, maxLongitude: 100, maxLatitude: 10).padded(by: 0.5)
        #expect(wide.minLongitude == -180 && wide.maxLongitude == 180)
    }

    @Test func boxContainmentAcrossTheAntimeridian() {
        let pacific = GeoBox(minLongitude: 170, minLatitude: -20, maxLongitude: -170, maxLatitude: 0)
        #expect(pacific.contains(GeoBox(minLongitude: 175, minLatitude: -10, maxLongitude: -175, maxLatitude: -5)))
        #expect(pacific.contains(GeoBox(minLongitude: -179, minLatitude: -10, maxLongitude: -171, maxLatitude: -5)))
        #expect(!pacific.contains(GeoBox(minLongitude: 160, minLatitude: -10, maxLongitude: 175, maxLatitude: -5)))
        #expect(GeoBox.world.contains(pacific))
        #expect(!pacific.contains(.world))
    }
}

@Suite struct HistoricLayerTests {
    @Test func pointsCountTheSitesInTheBox() {
        let layer = HistoricFixture.points([HistoricFixture.neuschwanstein, HistoricFixture.fortress])
        #expect(layer.count(in: GeoBox(minLongitude: 10.72, minLatitude: 47, maxLongitude: 11, maxLatitude: 48)) == 1)
        #expect(layer.sites.count == 2)
    }

    @Test func sitesAreChipsUpToTheLimitAndDotsAbove() {
        let site = HistoricFixture.fortress
        #expect(HistoricFixture.points(Array(repeating: site, count: HistoricLayer.chipLimit)).drawsChips)
        #expect(!HistoricFixture.points(Array(repeating: site, count: HistoricLayer.chipLimit + 1)).drawsChips)
        #expect(!HistoricFixture.points([site], truncated: true).drawsChips)
        #expect(!HistoricFixture.grid(HistoricFixture.cells).drawsChips)
    }

    @Test func aGridCountsTheCellsWhoseCentreIsInTheBox() {
        let layer = HistoricFixture.grid(HistoricFixture.cells)
        #expect(layer.count(in: GeoBox(minLongitude: 0, minLatitude: 45, maxLongitude: 10, maxLatitude: 55)) == 574)
        #expect(layer.count(in: .world) == 577)
        #expect(layer.sites.isEmpty)
    }

    @Test func aGridCellCountsOnlyTheKindsShown() {
        let cell = HistoricFixture.cells[0]
        #expect(cell.count(of: [.castle, .memorial]) == 52)
        #expect(cell.count(of: Set(HistoricKind.allCases)) == 574)
    }
}

@MainActor @Suite struct HistoricModelTests {
    let alps = MapViewport(box: GeoBox(minLongitude: 10, minLatitude: 47, maxLongitude: 11, maxLatitude: 48), zoom: 9)
    let alpsWide = MapViewport(box: GeoBox(minLongitude: 8, minLatitude: 46, maxLongitude: 13, maxLatitude: 49), zoom: 4)

    private final class Clock: @unchecked Sendable { var now = Date(timeIntervalSince1970: 1_795_000_000) }

    private func model(_ p: ScriptedProvider, clock: Clock = Clock(), filters: FilterStore? = nil) -> LiveMapModel {
        LiveMapModel(provider: p, filters: filters, debounce: .zero, now: { clock.now })
    }

    @Test func offByDefaultAndNeverRequested() async {
        let p = ScriptedProvider()
        let m = model(p)
        m.viewportDidChange(alps)
        await m.settle()
        #expect(!m.showsHistoric && m.historic == nil)
        #expect(p.historicRequests.isEmpty)
    }

    @Test func turningItOnAsksForTheViewAndLeavesTheHazardsAlone() async {
        let p = ScriptedProvider()
        let m = model(p)
        m.viewportDidChange(alps)
        await m.settle()
        let hazards = p.requests.count
        m.showsHistoric = true
        await m.settle()
        #expect(p.historicRequests == [HistoricRequest(alps, kinds: Set(HistoricKind.allCases))])
        #expect(m.historic?.sites.count == 3)
        #expect(p.requests.count == hazards)
    }

    @Test func aViewTheLastAnswerCoversIsNotAskedAgainNorAreTimerReloads() async {
        let p = ScriptedProvider()
        let m = model(p)
        m.showsHistoric = true
        m.viewportDidChange(alps)
        await m.settle()
        m.reload()
        await m.settle()
        let inside = MapViewport(box: GeoBox(minLongitude: 10.2, minLatitude: 47.2, maxLongitude: 10.8, maxLatitude: 47.8), zoom: 11)
        m.viewportDidChange(inside)
        await m.settle()
        #expect(p.historicRequests.count == 1)
        m.viewportDidChange(alpsWide)
        await m.settle()
        #expect(p.historicRequests.map(\.zoom) == [9, 4])
    }

    @Test func aTruncatedAnswerIsAskedAgainOnZoomingIn() async {
        let p = ScriptedProvider()
        p.historicAnswer = HistoricFixture.points([HistoricFixture.fortress], truncated: true)
        let m = model(p)
        m.showsHistoric = true
        m.viewportDidChange(alps)
        await m.settle()
        m.viewportDidChange(MapViewport(box: GeoBox(minLongitude: 10.2, minLatitude: 47.2, maxLongitude: 10.8, maxLatitude: 47.8), zoom: 11))
        await m.settle()
        #expect(p.historicRequests.count == 2)
    }

    /// A zoom-in while the wider view's request is in flight waits for it; when that answer turns
    /// out too dense for chips, the zoomed-in view is asked for at once, not at the next move.
    @Test func aZoomInWaitingOnATooDenseAnswerIsAskedForWhenItArrives() async {
        let p = ScriptedProvider()
        p.historicDelay = .milliseconds(80)
        p.historicAnswer = HistoricFixture.points(Array(repeating: HistoricFixture.fortress,
                                                        count: HistoricLayer.chipLimit + 1))
        let m = model(p)
        m.showsHistoric = true
        m.viewportDidChange(alps)
        for _ in 0..<200 where p.historicRequests.isEmpty { try? await Task.sleep(for: .milliseconds(2)) }
        let inside = MapViewport(box: GeoBox(minLongitude: 10.2, minLatitude: 47.2, maxLongitude: 10.8, maxLatitude: 47.8), zoom: 11)
        m.viewportDidChange(inside)
        await m.settleMap()
        #expect(p.historicRequests.count == 1) // the request in flight covers it
        for _ in 0..<200 where p.historicRequests.count < 2 { try? await Task.sleep(for: .milliseconds(5)) }
        #expect(p.historicRequests.map(\.box) == [alps.box, inside.box])
    }

    /// Back inside the answer on screen while another view's request is in flight: that request
    /// is dropped, so it can't replace what is shown with a view the map has left.
    @Test func movingBackInsideTheAnswerDropsTheRequestInFlight() async {
        let p = ScriptedProvider()
        let m = model(p)
        m.showsHistoric = true
        m.viewportDidChange(alps)
        await m.settle()
        p.historicDelay = .milliseconds(80)
        let away = MapViewport(box: GeoBox(minLongitude: 20, minLatitude: 47, maxLongitude: 21, maxLatitude: 48), zoom: 9)
        m.viewportDidChange(away)
        for _ in 0..<200 where p.historicRequests.count < 2 { try? await Task.sleep(for: .milliseconds(2)) }
        p.historicAnswer = HistoricFixture.points([])
        m.viewportDidChange(alps)
        await m.settleMap()
        try? await Task.sleep(for: .milliseconds(150))
        #expect(m.historic?.sites.count == 3)
        #expect(p.historicRequests.count == 2)
    }

    /// Fewer groups need no request: the answer on screen is filtered at once, cells by their
    /// count per group, and a site of a group the app doesn't know leaves with the filter.
    @Test func narrowingTheKindsFiltersWhatIsShownWithoutAsking() async {
        let p = ScriptedProvider()
        let m = model(p)
        m.showsHistoric = true
        m.viewportDidChange(alps)
        await m.settle()
        m.selectedHistoricID = "Q3" // the tower, of a group the app doesn't know
        m.historicKinds = [.castle]
        await m.settle()
        #expect(m.historic?.sites.map(\.id) == ["Q4048"])
        #expect(m.selectedHistoricID == nil)
        #expect(p.historicRequests.count == 1)

        m.historicKinds = Set(HistoricKind.allCases) // served: the answer held every group
        await m.settle()
        #expect(m.historic?.sites.count == 3)
        #expect(p.historicRequests.count == 1)

        m.viewportDidChange(alpsWide)
        await m.settle()
        m.historicKinds = [.memorial]
        await m.settle()
        guard case .grid(let cells, _)? = m.historic?.content else {
            Issue.record("expected a grid")
            return
        }
        #expect(cells.map(\.count) == [24, 3])
        #expect(cells.first?.kinds == ["memorial": 24])
        #expect(p.historicRequests.count == 2)
    }

    /// Sites cut short can't be filtered: the server would send more of the groups left.
    @Test func narrowingACutShortAnswerAsksAgain() async {
        let p = ScriptedProvider()
        p.historicAnswer = HistoricFixture.points([HistoricFixture.neuschwanstein, HistoricFixture.fortress], truncated: true)
        let m = model(p)
        m.showsHistoric = true
        m.viewportDidChange(alps)
        await m.settle()
        m.historicKinds = [.military]
        #expect(m.historic?.sites.map(\.id) == ["Q2"]) // never the castle meanwhile
        await m.settle()
        #expect(p.historicRequests.map(\.kinds) == [nil, [.military]])
    }

    private struct SlowDown: UserFacingError {
        var userMessage: String { "Too many requests." }
        var isRateLimit: Bool { true }
    }

    /// A rate limit is the server asking for quiet: every request waits the minute, not only the
    /// same one (from zoom 8 each pan is a new request).
    @Test func aRateLimitHoldsBackEveryViewForAMinute() async {
        let p = ScriptedProvider()
        let clock = Clock()
        p.historicFailure = SlowDown()
        let m = model(p, clock: clock)
        m.showsHistoric = true
        m.viewportDidChange(alps)
        await m.settle()
        p.historicFailure = nil
        m.viewportDidChange(alpsWide)
        await m.settle()
        #expect(p.historicRequests.count == 1)
        clock.now += 61
        m.reload()
        await m.settle()
        #expect(p.historicRequests.count == 2)
    }

    @Test func aFailureDoesNotHoldBackADifferentView() async {
        let p = ScriptedProvider()
        p.historicFailure = URLError(.badServerResponse)
        let m = model(p)
        m.showsHistoric = true
        m.viewportDidChange(alps)
        await m.settle()
        p.historicFailure = nil
        m.viewportDidChange(alpsWide)
        await m.settle()
        #expect(p.historicRequests.count == 2)
        #expect(!m.historicFailed && m.historic != nil)
    }

    @Test func moreKindsThanTheAnswerHeldAreAskedFor() async {
        let p = ScriptedProvider()
        let m = model(p)
        m.historicKinds = [.castle]
        m.showsHistoric = true
        m.viewportDidChange(alps)
        await m.settle()
        #expect(p.historicRequests.last?.kinds == [.castle])
        m.historicKinds = [.castle, .memorial]
        await m.settle()
        #expect(p.historicRequests.last?.kinds == [.castle, .memorial])
        m.historicKinds = Set(HistoricKind.allCases)
        await m.settle()
        #expect(p.historicRequests.last?.kinds == nil)
    }

    /// The API reads an empty `kind` as every kind, so turning the last one off is refused.
    @Test func theLastKindCannotBeTurnedOff() {
        let m = model(ScriptedProvider())
        m.historicKinds = [.castle]
        m.historicKinds = []
        #expect(m.historicKinds == [.castle])
    }

    @Test func savedWithTheFiltersAndLoadedAtTheFirstMapLoad() async {
        let store = FilterStore(suiteName: "historic-\(UUID())")
        let first = model(ScriptedProvider(), filters: store)
        first.showsHistoric = true
        first.historicKinds = [.memorial]
        let p = ScriptedProvider()
        let next = model(p, filters: store)
        #expect(next.showsHistoric && next.historicKinds == [.memorial])
        #expect(p.historicRequests.isEmpty)
        next.viewportDidChange(alps)
        await next.settle()
        #expect(p.historicRequests.map(\.kinds) == [[.memorial]])
    }

    @Test func aSavedLayerStaysOffWhereTheProviderHasNoHistoricSites() async {
        let store = FilterStore(suiteName: "historic-\(UUID())")
        model(ScriptedProvider(), filters: store).showsHistoric = true
        let p = ScriptedProvider()
        p.offersHistoric = false
        let next = model(p, filters: store)
        #expect(!next.showsHistoric)
        next.showsHistoric = true
        next.viewportDidChange(alps)
        await next.settle()
        #expect(p.historicRequests.isEmpty)
    }

    @Test func aLateAnswerAfterTurningOffIsDropped() async {
        let p = ScriptedProvider()
        p.historicDelay = .milliseconds(60)
        let m = model(p)
        m.viewportDidChange(alps)
        m.showsHistoric = true
        for _ in 0..<200 where p.historicRequests.isEmpty { try? await Task.sleep(for: .milliseconds(2)) }
        m.showsHistoric = false
        try? await Task.sleep(for: .milliseconds(150))
        #expect(m.historic == nil)
    }

    @Test func aLateAnswerForOtherKindsIsDropped() async {
        let p = ScriptedProvider()
        p.historicDelay = .milliseconds(60)
        let m = model(p)
        m.historicKinds = [.castle]
        m.viewportDidChange(alps)
        m.showsHistoric = true
        for _ in 0..<200 where p.historicRequests.isEmpty { try? await Task.sleep(for: .milliseconds(2)) }
        p.historicDelay = nil
        m.historicKinds = Set(HistoricKind.allCases)
        try? await Task.sleep(for: .milliseconds(150))
        #expect(p.historicRequests.map(\.kinds) == [[.castle], nil])
        #expect(m.historic?.sites.count == 3) // the castles-only answer, late, was dropped
    }

    @Test func aFailureKeepsTheLastLayerAndWaitsAMinuteFromTheRequest() async {
        let p = ScriptedProvider()
        let clock = Clock()
        let m = model(p, clock: clock)
        m.showsHistoric = true
        m.viewportDidChange(alps)
        await m.settle()
        p.historicFailure = URLError(.badServerResponse)
        m.viewportDidChange(alpsWide)
        await m.settle()
        #expect(m.historicFailed)
        #expect(m.historic?.sites.count == 3)
        clock.now += 59
        m.reload()
        await m.settle()
        #expect(p.historicRequests.count == 2)
        p.historicFailure = nil
        clock.now += 2
        m.reload()
        await m.settle()
        #expect(p.historicRequests.count == 3)
        #expect(!m.historicFailed)
        #expect(m.historic?.sites.isEmpty == true) // the grid answer
    }

    @Test func turningItOffAndOnAfterAFailureRetriesAtOnce() async {
        let p = ScriptedProvider()
        p.historicFailure = URLError(.badServerResponse)
        let m = model(p)
        m.showsHistoric = true
        m.viewportDidChange(alps)
        await m.settle()
        p.historicFailure = nil
        m.showsHistoric = false
        m.showsHistoric = true
        await m.settle()
        #expect(p.historicRequests.count == 2)
        #expect(m.historic != nil && !m.historicFailed)
    }

    @Test func aSelectedSiteStaysWhileTheMapShowsTheGrid() async {
        let p = ScriptedProvider()
        let m = model(p)
        m.showsHistoric = true
        m.viewportDidChange(alps)
        await m.settle()
        m.selectedHistoricID = "Q4048"
        #expect(m.selectedHistoricSite?.name == "Neuschwanstein Castle")
        m.viewportDidChange(alpsWide)
        await m.settle()
        #expect(m.historic?.sites.isEmpty == true)
        #expect(m.selectedHistoricSite?.name == "Neuschwanstein Castle")
    }

    @Test func oneThingIsSelectedAtATime() async {
        let p = ScriptedProvider()
        let m = model(p)
        m.showsHistoric = true
        m.showsBases = true
        m.viewportDidChange(alps)
        await m.settle()
        m.selectedBaseID = "Q161348"
        m.selectedHistoricID = "Q4048"
        #expect(m.selectedBaseID == nil)
        m.selectedEventID = UUID()
        #expect(m.selectedHistoricID == nil && m.selectedHistoricSite == nil)
    }

    @Test func turningItOffClearsTheLayerAndTheSelection() async {
        let p = ScriptedProvider()
        let m = model(p)
        m.showsHistoric = true
        m.viewportDidChange(alps)
        await m.settle()
        m.selectedHistoricID = "Q4048"
        m.showsHistoric = false
        #expect(m.historic == nil && m.selectedHistoricID == nil)
    }

    @Test func turningOffTheSelectedSitesKindClosesIt() async {
        let p = ScriptedProvider()
        let m = model(p)
        m.showsHistoric = true
        m.viewportDidChange(alps)
        await m.settle()
        m.selectedHistoricID = "Q4048"
        m.historicKinds = [.military, .memorial]
        #expect(m.selectedHistoricID == nil)
    }
}

@Suite struct HistoricTextTests {
    let belgium = GeoBox(minLongitude: 0, minLatitude: 45, maxLongitude: 12, maxLatitude: 55)
    let all = Set(HistoricKind.allCases)

    @Test func figuresSkipWhatWikidataDoesNotList() {
        #expect(HistoricFixture.neuschwanstein.figures == [
            AircraftFigure(label: "Type", value: "Castle"),
            AircraftFigure(label: "Country", value: "Germany"),
            AircraftFigure(label: "Coordinates", value: "47.5575° N, 10.7500° E"),
            AircraftFigure(label: "Wikidata item", value: "Q4048"),
        ])
        #expect(HistoricFixture.tower.figures.map(\.label) == ["Type", "Coordinates", "Wikidata item"])
    }

    @Test func aSiteOfAnUnknownGroupIsNamedAsTheServerNamesIt() {
        #expect(HistoricFixture.tower.kindLabel == "Building")
        #expect(HistoricFixture.fortress.kindLabel == "Military heritage")
    }

    @Test func theListedLineSaysWhenItWasRefreshed() {
        #expect(HistoricFixture.points([]).listedLine == "Listed on Wikidata · refreshed 18 Nov 2026")
        #expect(HistoricFixture.grid([]).listedLine == "Listed on Wikidata")
    }

    @Test func theHeadlineSaysHowManyAndWhetherToZoomIn() {
        let grid = HistoricFixture.grid(HistoricFixture.cells)
        #expect(grid.headline(in: belgium, kinds: all) == "Historic sites · about 574 in this view")
        #expect(grid.hint == "Zoom in to see each site.")

        let chips = HistoricFixture.points([HistoricFixture.neuschwanstein, HistoricFixture.fortress])
        #expect(chips.headline(in: belgium, kinds: all) == "Historic sites · 2 in this view")
        #expect(chips.hint == nil)

        let dense = HistoricFixture.points(Array(repeating: HistoricFixture.fortress, count: 1_200))
        #expect(dense.headline(in: belgium, kinds: all) == "Historic sites · 1,200 in this view")
        #expect(dense.hint == "Zoom in to pick a site.")

        let cut = HistoricFixture.points([HistoricFixture.fortress], truncated: true)
        #expect(cut.headline(in: belgium, kinds: all) == "Historic sites · over 1 in this view")

        #expect(chips.headline(in: GeoBox(minLongitude: 100, minLatitude: 0, maxLongitude: 101, maxLatitude: 1),
                               kinds: all) == "Historic sites · none in this view")
        #expect(chips.headline(in: belgium, kinds: [.memorial, .castle])
                == "Historic sites (castles, memorials) · 2 in this view")
    }
}
