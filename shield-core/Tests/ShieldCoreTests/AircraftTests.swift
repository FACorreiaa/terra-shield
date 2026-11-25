import Foundation
import Testing
@testable import ShieldCore

enum AircraftFixture {
    static let at = Date(timeIntervalSince1970: 1_795_000_000) // 2026-11-18 11:06:40 UTC

    static func plane(_ hex: String, lat: Double = 38.7, lon: Double = -9.1, callsign: String? = "TAP215",
                      track: Double? = 275, altitude: Int? = 38_000, onGround: Bool = false,
                      military: Bool = false) -> Aircraft {
        Aircraft(hex: hex, callsign: callsign, registration: "CS-TXK", typeCode: "A21N",
                 coordinate: Coordinate(latitude: lat, longitude: lon), altitudeFeet: altitude,
                 onGround: onGround, groundSpeedKnots: 417.3, trackDegrees: track, verticalRateFpm: -128,
                 squawk: "2205", emergency: nil, military: military, positionAt: at)
    }

    static func layer(_ aircraft: [Aircraft], complete: Bool = true, dataAt: Date? = at) -> AircraftLayer {
        AircraftLayer(aircraft: aircraft, dataAt: dataAt, complete: complete,
                      attribution: "© adsb.lol contributors", license: "ODbL 1.0",
                      licenseURL: URL(string: "https://opendatacommons.org/licenses/odbl/1-0/"))
    }
}

/// ADR 28: the server answers a bbox only when it touches at most 4 cells of 5°.
@Suite struct AircraftAreaTests {
    private func box(_ minLon: Double, _ minLat: Double, _ maxLon: Double, _ maxLat: Double) -> GeoBox {
        GeoBox(minLongitude: minLon, minLatitude: minLat, maxLongitude: maxLon, maxLatitude: maxLat)
    }

    @Test func countsCellsLikeTheServer() {
        // The cases of backend `aircraft_test.go` TestCells.
        #expect(AircraftArea.cellCount(box(-9.3, 38.6, -9.0, 38.8)) == 1)
        #expect(AircraftArea.cellCount(box(-9.9, 36.1, -0.1, 44.9)) == 4) // Iberia
        #expect(AircraftArea.cellCount(box(178, -18, -178, -16)) == 2)   // across the antimeridian
        #expect(AircraftArea.cellCount(box(0, 88, 1, 90)) == 1)          // the pole clamps
        #expect(!AircraftArea.fits(box(-10, 35, 30, 60)))                // a continent
        #expect(!AircraftArea.fits(box(-9.9, 36, 4.9, 44)))              // six cells
    }

    @Test func anEdgeOnACellLineTouchesTheNextCell() {
        // Measured against the server on 2026-11-25: an aligned 10° × 10° box is 9 cells, a 400.
        #expect(AircraftArea.cellCount(box(-10, 35, 0, 45)) == 9)
        #expect(AircraftArea.fits(box(-9.9, 35.1, -0.1, 44.9)))
    }

    @Test func countsTheBoxAsSentRoundedToFourDecimals() {
        // 44.99996 goes out as "45.0000", which touches the cell above.
        #expect(AircraftArea.cellCount(box(-9.9, 35.1, -0.1, 44.99996)) == 6)
        // Across the antimeridian after rounding: 179.99996 goes out as 180, the last cell.
        #expect(AircraftArea.cellCount(box(179.99996, 1, -179.99996, 4)) == 2)
    }
}

/// ADR 28 clients: a toggle, a view small enough, no faster than once a minute.
@MainActor @Suite struct FlightsLayerTests {
    let small = MapViewport(box: GeoBox(minLongitude: -10, minLatitude: 36, maxLongitude: -5, maxLatitude: 41), zoom: 7)
    let large = MapViewport(box: GeoBox(minLongitude: -20, minLatitude: 30, maxLongitude: 10, maxLatitude: 60), zoom: 4)

    private final class Clock: @unchecked Sendable { var now = AircraftFixture.at }
    private final class Counter: @unchecked Sendable { var value = 0 }
    private final class Durations: @unchecked Sendable {
        var values: [Duration] = []
        func append(_ d: Duration) { values.append(d) }
    }

    /// Until the provider has been asked `count` times: a request is then in flight.
    private func waitForRequests(_ p: ScriptedProvider, _ count: Int) async {
        for _ in 0..<200 where p.aircraftRequests.count < count { await Task.yield(); try? await Task.sleep(for: .milliseconds(2)) }
    }

    private func model(_ p: ScriptedProvider, clock: Clock = Clock()) -> LiveMapModel {
        LiveMapModel(provider: p, debounce: .zero, now: { clock.now })
    }

    @Test func offByDefaultAndNeverRequested() async {
        let p = ScriptedProvider()
        let m = model(p)
        m.viewportDidChange(small)
        await m.settle()
        #expect(!m.showsFlights)
        #expect(p.aircraftRequests.isEmpty)
        #expect(m.flights == nil)
    }

    @Test func onInASmallViewLoadsTheBoxAsSent() async {
        let p = ScriptedProvider()
        let m = model(p)
        let ragged = MapViewport(box: GeoBox(minLongitude: -9.876543, minLatitude: 36.123456,
                                             maxLongitude: -5.000049, maxLatitude: 40.99996), zoom: 7)
        m.viewportDidChange(ragged)
        await m.settle()
        m.showsFlights = true
        await m.settle()
        #expect(p.aircraftRequests == [GeoBox(minLongitude: -9.8765, minLatitude: 36.1235,
                                              maxLongitude: -5, maxLatitude: 41)])
        #expect(m.flights?.aircraft.map(\.hex) == ["a1", "a2"])
        #expect(!m.flightsNeedZoom)
    }

    @Test func togglingFlightsLeavesTheHazardsAlone() async {
        let p = ScriptedProvider()
        let m = model(p)
        m.viewportDidChange(small)
        await m.settle()
        let before = p.requests.count
        m.showsFlights = true
        await m.settle()
        m.showsFlights = false
        await m.settle()
        #expect(p.requests.count == before)
        #expect(p.aircraftRequests.count == 1)
    }

    @Test func aLargeViewAsksToZoomInAndClearsTheAircraft() async {
        let p = ScriptedProvider()
        let m = model(p)
        m.viewportDidChange(small)
        m.showsFlights = true
        await m.settle()
        #expect(m.flights != nil)
        m.viewportDidChange(large)
        await m.settle()
        #expect(m.flightsNeedZoom)
        #expect(m.flights == nil)
        #expect(p.aircraftRequests.count == 1)
    }

    @Test func reloadsOfTheSameViewDoNotAskAgain() async {
        // A filter change, a stream burst, Refresh: the map reloads, the flights wait for their
        // own minute.
        let p = ScriptedProvider()
        let m = model(p)
        m.viewportDidChange(small)
        m.showsFlights = true
        await m.settle()
        m.reload()
        await m.settle()
        m.showsConflicts = true
        await m.settle()
        #expect(p.aircraftRequests.count == 1)
        #expect(m.flights?.aircraft.count == 2)
    }

    @Test func theMinuteTimerAsksAgainForTheSameView() async {
        let p = ScriptedProvider()
        let clock = Clock()
        let m = model(p, clock: clock)
        m.viewportDidChange(small)
        m.showsFlights = true
        await m.settle()
        let slept = Counter()
        // Two ticks, then the layer goes off and the loop ends.
        await m.keepFlightsFresh(sleep: { duration in
            #expect(duration == .seconds(60))
            clock.now += 60
            slept.value += 1
            if slept.value == 3 { await MainActor.run { m.showsFlights = false } }
        })
        #expect(p.aircraftRequests.count == 3)
    }

    @Test func aNewViewRestartsTheMinute() async {
        // Pan at 59 s asks for the new view; the timer must not ask for it again a second later.
        let p = ScriptedProvider()
        let clock = Clock()
        let m = model(p, clock: clock)
        m.viewportDidChange(small)
        m.showsFlights = true
        await m.settle()
        let moved = MapViewport(box: GeoBox(minLongitude: -9, minLatitude: 37, maxLongitude: -4, maxLatitude: 42), zoom: 7)
        let slept = Counter()
        let durations = Durations()
        await m.keepFlightsFresh(sleep: { duration in
            durations.append(duration)
            slept.value += 1
            switch slept.value {
            case 1:
                clock.now += 59
                await m.viewportDidChange(moved)
                await m.settle()
                clock.now += 1
            case 2:
                #expect(p.aircraftRequests == [small.box, moved.box])
                clock.now += 59
            default:
                await MainActor.run { m.showsFlights = false }
            }
        })
        #expect(durations.values == [.seconds(60), .seconds(59), .seconds(60)])
        #expect(p.aircraftRequests == [small.box, moved.box, moved.box])
    }

    @Test func theTimerStillWaitsAMinuteWhenNothingWasAsked() async {
        // A view too large is never asked for; the loop must not spin.
        let p = ScriptedProvider()
        let clock = Clock()
        let m = model(p, clock: clock)
        m.viewportDidChange(large)
        m.showsFlights = true
        await m.settle()
        let durations = Durations()
        await m.keepFlightsFresh(sleep: { duration in
            durations.append(duration)
            clock.now += 60
            if durations.values.count == 2 { await MainActor.run { m.showsFlights = false } }
        })
        #expect(durations.values == [.seconds(60), .seconds(60)])
        #expect(p.aircraftRequests.isEmpty)
    }

    @Test func aSupersededMapLoadDoesNotLoseTheFirstAnswer() async {
        // The flights answer is applied on its own, so a map reload while it is in flight can't
        // leave the layer empty for a minute.
        let p = ScriptedProvider()
        p.aircraftDelay = .milliseconds(80)
        let m = model(p)
        m.viewportDidChange(small)
        await m.settle()
        m.showsFlights = true
        await waitForRequests(p, 1)
        m.reload()
        m.showsDetections = true
        await m.settle()
        #expect(m.flights?.aircraft.count == 2)
        #expect(p.aircraftRequests.count == 1)
    }

    @Test func aLateAnswerAfterTurningOffIsDropped() async {
        let p = ScriptedProvider()
        p.aircraftDelay = .milliseconds(60)
        let m = model(p)
        m.viewportDidChange(small)
        m.showsFlights = true
        await waitForRequests(p, 1)
        m.showsFlights = false
        try? await Task.sleep(for: .milliseconds(150))
        #expect(m.flights == nil)
    }

    @Test func offThenOnMidRequestAsksAgain() async {
        let p = ScriptedProvider()
        p.aircraftDelay = .milliseconds(60)
        let m = model(p)
        m.viewportDidChange(small)
        m.showsFlights = true
        await waitForRequests(p, 1)
        m.showsFlights = false
        m.showsFlights = true
        await m.settle()
        #expect(p.aircraftRequests.count == 2)
        #expect(m.flights?.aircraft.count == 2)
    }

    @Test func anOldViewsLateAnswerNeverReplacesTheNewOne() async {
        let p = ScriptedProvider()
        let moved = MapViewport(box: GeoBox(minLongitude: -9, minLatitude: 37, maxLongitude: -4, maxLatitude: 42), zoom: 7)
        p.aircraftDelays[small.box] = .milliseconds(100)
        p.aircraftAnswers[moved.box] = AircraftFixture.layer([AircraftFixture.plane("b1")])
        let m = model(p)
        m.viewportDidChange(small)
        m.showsFlights = true
        await waitForRequests(p, 1)
        m.viewportDidChange(moved)
        await m.settle()
        try? await Task.sleep(for: .milliseconds(150))
        #expect(m.flights?.aircraft.map(\.hex) == ["b1"])
    }

    @Test func hazardsDoNotWaitForTheFlightFeed() async {
        let p = ScriptedProvider()
        p.aircraftDelay = .seconds(5)
        let m = model(p)
        m.viewportDidChange(small)
        m.showsFlights = true
        await m.settleMap()
        #expect(m.snapshot != nil)
        #expect(m.flights == nil)
        m.showsFlights = false
    }

    @Test func aServerZoomInClearsTheLayerAndIsNotAskedAgainOnReload() async {
        let p = ScriptedProvider()
        let m = model(p)
        m.viewportDidChange(small)
        m.showsFlights = true
        await m.settle()
        m.selectedAircraftHex = "a1"
        p.aircraftFailure = AircraftArea.TooLarge()
        await m.refreshFlights()
        #expect(m.flightsNeedZoom)
        #expect(!m.flightsFailed)
        #expect(m.flights == nil)
        #expect(m.selectedAircraftHex == nil)
        // A reload of the refused view neither asks again nor flips back to "loading".
        m.reload()
        await m.settle()
        #expect(p.aircraftRequests.count == 2)
        #expect(m.flightsNeedZoom)
    }

    @Test func subPixelJitterIsNotANewView() async {
        let p = ScriptedProvider()
        let m = model(p)
        m.viewportDidChange(small)
        m.showsFlights = true
        await m.settle()
        var jittered = small
        jittered.box.minLongitude += 0.000_001
        m.viewportDidChange(jittered)
        await m.settle()
        #expect(p.aircraftRequests.count == 1)
    }

    @Test func aNewViewIsAskedAtOnce() async {
        let p = ScriptedProvider()
        let m = model(p)
        m.viewportDidChange(small)
        m.showsFlights = true
        await m.settle()
        let moved = MapViewport(box: GeoBox(minLongitude: -9, minLatitude: 37, maxLongitude: -4, maxLatitude: 42), zoom: 7)
        m.viewportDidChange(moved)
        await m.settle()
        #expect(p.aircraftRequests == [small.box, moved.box])
    }

    @Test func aFailureKeepsTheLastPositionsAndSaysSo() async {
        let p = ScriptedProvider()
        let m = model(p)
        m.viewportDidChange(small)
        m.showsFlights = true
        await m.settle()
        p.aircraftFailure = URLError(.badServerResponse)
        await m.refreshFlights()
        #expect(m.flightsFailed)
        #expect(m.flights?.aircraft.count == 2)
    }

    @Test func aFailureIsRetriedByTheTimerNotByEveryReload() async {
        // Hazard polls, Refresh and stream bursts reload the map; the feed's quota is shared, so
        // only the minute timer asks again.
        let p = ScriptedProvider()
        let m = model(p)
        m.viewportDidChange(small)
        m.showsFlights = true
        p.aircraftFailure = URLError(.badServerResponse)
        await m.settle()
        #expect(m.flightsFailed)
        m.reload()
        await m.settle()
        m.reload()
        await m.settle()
        #expect(p.aircraftRequests.count == 1)
        p.aircraftFailure = nil
        await m.refreshFlights()
        #expect(!m.flightsFailed)
        #expect(m.flights?.aircraft.count == 2)
    }

    @Test func aFailureForANewViewDropsTheOldViewsPositions() async {
        // "2 in this view" must not count the previous view's answer.
        let p = ScriptedProvider()
        let m = model(p)
        m.viewportDidChange(small)
        m.showsFlights = true
        await m.settle()
        p.aircraftFailure = URLError(.badServerResponse)
        m.viewportDidChange(MapViewport(box: GeoBox(minLongitude: -9, minLatitude: 37, maxLongitude: -4, maxLatitude: 42), zoom: 7))
        await m.settle()
        #expect(m.flightsFailed)
        #expect(m.flights == nil)
    }

    @Test func turningItOffClearsTheLayerAndTheSelection() async {
        let p = ScriptedProvider()
        let m = model(p)
        m.viewportDidChange(small)
        m.showsFlights = true
        await m.settle()
        m.selectedAircraftHex = "a1"
        m.showsFlights = false
        #expect(m.flights == nil)
        #expect(m.selectedAircraftHex == nil)
    }

    @Test func itIsNeverSavedWithTheFilters() async {
        // Off at every launch: a saved "on" would poll the feed's quota from the next run.
        let store = FilterStore(suiteName: "flights-\(UUID())")
        let m = LiveMapModel(provider: ScriptedProvider(), filters: store, debounce: .zero)
        m.showsFlights = true
        m.showsConflicts = true
        let next = LiveMapModel(provider: ScriptedProvider(), filters: store, debounce: .zero)
        #expect(next.showsConflicts)
        #expect(!next.showsFlights)
    }
}

@MainActor @Suite struct AircraftSelectionTests {
    let small = MapViewport(box: GeoBox(minLongitude: -10, minLatitude: 36, maxLongitude: -5, maxLatitude: 41), zoom: 7)

    @Test func anAircraftOrAnEventNeverBoth() async throws {
        let p = ScriptedProvider()
        let m = LiveMapModel(provider: p, debounce: .zero)
        m.viewportDidChange(small)
        m.showsFlights = true
        await m.settle()
        m.selectedAircraftHex = "a2"
        #expect(m.selectedAircraft?.hex == "a2")
        let event = try #require(m.snapshot?.events.first)
        m.selectedEventID = event.id
        #expect(m.selectedAircraftHex == nil)
        m.selectedAircraftHex = "a1"
        #expect(m.selectedEventID == nil)
    }

    @Test func aSelectedAircraftStaysUntilClosed() async {
        // On iPhone the sheet rises over the aircraft just tapped, the view shrinks, and the
        // next answer may not hold it: the sheet keeps its last position until closed.
        let p = ScriptedProvider()
        let m = LiveMapModel(provider: p, debounce: .zero)
        m.viewportDidChange(small)
        m.showsFlights = true
        await m.settle()
        m.selectedAircraftHex = "a2"
        p.aircraftAnswer = AircraftFixture.layer([AircraftFixture.plane("a1")])
        let moved = MapViewport(box: GeoBox(minLongitude: -9, minLatitude: 37, maxLongitude: -4, maxLatitude: 42), zoom: 7)
        m.viewportDidChange(moved)
        await m.settle()
        #expect(m.selectedAircraftHex == "a2")
        #expect(m.selectedAircraft?.hex == "a2")
        m.selectedAircraftHex = nil
        #expect(m.selectedAircraft == nil)
    }

    @Test func aSelectedAircraftFollowsItsNewPosition() async {
        let p = ScriptedProvider()
        let m = LiveMapModel(provider: p, debounce: .zero)
        m.viewportDidChange(small)
        m.showsFlights = true
        await m.settle()
        m.selectedAircraftHex = "a2"
        p.aircraftAnswer = AircraftFixture.layer([AircraftFixture.plane("a2", lat: 39.5)])
        await m.refreshFlights()
        #expect(m.selectedAircraft?.coordinate.latitude == 39.5)
    }
}

@Suite struct AircraftWordsTests {
    let now = AircraftFixture.at

    @Test func titleIsTheCallsignElseRegistrationElseAddress() {
        var a = AircraftFixture.plane("49530a")
        #expect(a.title == "TAP215")
        a.callsign = "  "
        #expect(a.title == "CS-TXK")
        a.registration = nil
        #expect(a.title == "49530A")
    }

    @Test func figuresInAviationUnitsWithMetric() {
        let a = AircraftFixture.plane("49530a")
        #expect(a.figures == [
            AircraftFigure(label: "Altitude", value: "38,000 ft (11,582 m)"),
            AircraftFigure(label: "Ground speed", value: "417 kt (773 km/h)"),
            AircraftFigure(label: "Track", value: "275°"),
            AircraftFigure(label: "Vertical rate", value: "−128 ft/min"),
            AircraftFigure(label: "Type", value: "A21N"),
            AircraftFigure(label: "Registration", value: "CS-TXK"),
            AircraftFigure(label: "Squawk", value: "2205"),
            AircraftFigure(label: "ICAO address", value: "49530A"),
        ])
    }

    @Test func onTheGroundSaysSoAndSkipsWhatIsMissing() {
        var a = AircraftFixture.plane("~2a0001", callsign: nil, track: nil, altitude: nil, onGround: true)
        a.groundSpeedKnots = nil
        a.verticalRateFpm = nil
        a.squawk = nil
        a.typeCode = nil
        #expect(a.figures.map(\.label) == ["Altitude", "Registration", "Address (non-ICAO)"])
        #expect(a.figures.first?.value == "On the ground")
        // A "~" address is not an ICAO one.
        #expect(a.figures.last == AircraftFigure(label: "Address (non-ICAO)", value: "~2A0001"))
    }

    @Test func flagsAndTimesGoFirstAndTheTrackWrapsAt360() {
        var a = AircraftFixture.plane("49530a", track: 359.6, military: true)
        a.emergency = "minfuel"
        a.positionAt = now.addingTimeInterval(-40)
        let lines = a.notes(now: now)
        #expect(lines == ["Emergency: minfuel", "Military (as the feed flags it)", "Position received 40 s ago · 11:06 UTC"])
        #expect(a.figures.first { $0.label == "Track" }?.value == "0°")
        #expect(AircraftFixture.plane("x").notes(now: now) == ["Position received just now · 11:06 UTC"])
    }

    @Test func theLegendGivesTheAgeOfTheOldestData() {
        let layer = AircraftFixture.layer([AircraftFixture.plane("a1")], dataAt: now.addingTimeInterval(-180))
        #expect(layer.ageLine(now: now) == "Positions from 3 min ago · 11:03 UTC")
        #expect(AircraftFixture.layer([], dataAt: nil).ageLine(now: now) == "No recent positions here")
        // Under ten seconds: "just now", never "from now".
        #expect(AircraftFixture.layer([], dataAt: now.addingTimeInterval(-5)).ageLine(now: now)
                == "Positions from just now · 11:06 UTC")
    }
}

@Suite struct AircraftSupportTests {
    @Test func providersWithoutFlightsDoNotOfferTheLayer() {
        // The API provider says yes (shield-api AircraftAPITests).
        #expect(!SampleEventProvider().supportsAircraft)
        #expect(!ScriptedProvider().supportsAircraft)
    }

    @Test func theBoxAsSentIsParsedBackFromTheParameter() {
        let box = GeoBox(minLongitude: -9.99996, minLatitude: 35.12344, maxLongitude: -0.1, maxLatitude: 44.99996)
        #expect(box.asSent == GeoBox(minLongitude: -10, minLatitude: 35.1234, maxLongitude: -0.1, maxLatitude: 45))
    }
}
