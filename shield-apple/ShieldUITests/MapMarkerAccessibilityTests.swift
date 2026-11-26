import XCTest

/// What VoiceOver gets from every kind of map marker on iOS, read through the accessibility tree:
/// one button per marker with a spoken label, a frame of at least 44 pt, and the selected state
/// after a selection. The 44 pt frame is VoiceOver's only: which marker a finger tap selects is
/// MapKit's own choice (it pads taps itself, so a tap up to about 24 pt from a 22 pt chip can select
/// it, depending on the markers around it), and this suite doesn't test MapKit.
///
/// Not covered here (an XCUITest tap is a touch, not VoiceOver): the double-tap activation, which
/// `ShieldTests/MarkerActivationTests` calls directly, and VoiceOver's touch exploration, which
/// only VoiceOver on an iPhone shows.
///
/// Needs the local API (`cd backend && make run`, or any API on `SHIELD_UITEST_API`, default
/// http://localhost:8081, the same URL the app is built with: `API_BASE_URL=...`). Skipped when it
/// doesn't answer. The aircraft test also needs the API on the flights replay
/// (`AIRCRAFT_FEED_URL`, ADR 28) and runs only with `TEST_RUNNER_SHIELD_UITEST_FLIGHTS=1`, so it
/// never spends the live feed's quota by accident.
@MainActor
final class MapMarkerAccessibilityTests: XCTestCase {
    override func setUpWithError() throws {
        continueAfterFailure = false
        try XCTSkipUnless(Self.apiAnswers(), "No local API at \(Self.api); see the class comment.")
    }

    func testHistoricSitesAreButtonsThatSelect() throws {
        let app = launch(["SHIELD_HISTORIC": "1", "SHIELD_REGION": "43.7234,10.3948,0.012,0.012"])
        let chip = try isolated(app, prefix: "Historic site,")
        try assertSelectsOnTap(app, chip)
    }

    func testAWorldHeritageSiteSaysSo() throws {
        let app = launch(["SHIELD_HISTORIC": "1", "SHIELD_REGION": "43.7234,10.3948,0.012,0.012"])
        let duomo = app.buttons["Historic site, Piazza del Duomo, Pisa, World Heritage Site"]
        XCTAssertTrue(duomo.waitForExistence(timeout: 30))
    }

    func testBasesAreButtonsThatSelect() throws {
        let app = launch(["SHIELD_BASES": "1", "SHIELD_REGION": "49.44,7.6,1.5,2.5"])
        let base = try isolated(app, prefix: "Military base,")
        try assertSelectsOnTap(app, base)
    }

    func testEventsAndClustersAreButtons() throws {
        let app = launch(["SHIELD_REGION": "50,10,30,40"])
        let cluster = app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Cluster,'")).firstMatch
        XCTAssertTrue(cluster.waitForExistence(timeout: 30), "no cluster over Europe")
        assertTarget(cluster)
        let event = app.buttons.matching(NSPredicate(format: "label CONTAINS ', updated '")).firstMatch
        if event.waitForExistence(timeout: 5) { assertTarget(event) }
    }

    func testAnEventSelectsOnTap() throws {
        let app = launch(["SHIELD_REGION": "50,10,30,40"])
        let onScreen = app.windows.firstMatch.frame.insetBy(dx: 30, dy: 120)
        func visible(_ q: XCUIElementQuery) -> XCUIElement? {
            q.allElementsBoundByIndex.first {
                $0.isHittable && $0.frame.width > 0 && onScreen.contains(CGPoint(x: $0.frame.midX, y: $0.frame.midY))
            }
        }
        let clusters = app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Cluster,'"))
        XCTAssertTrue(clusters.firstMatch.waitForExistence(timeout: 30))
        // Zoomed in through clusters until a single event shows (one drawn under a cluster is in
        // the tree but not hittable).
        var found = visible(singleEvents(app))
        for _ in 0..<5 where found == nil {
            guard let cluster = visible(clusters) else { break }
            cluster.tap()
            sleep(3)
            found = visible(singleEvents(app))
        }
        guard let found else { throw XCTSkip("no single event on screen") }
        // Dragged to the upper map: the layer legend floats mid-map and the sheet holds the
        // bottom, and a tap there lands on them (`isHittable` can't tell: they don't hide the
        // marker from the accessibility tree).
        let label = found.label
        // The drag starts on map beside the marker (a press on the marker selects it), and moves
        // the map by the vector that takes the marker to a quarter down the screen.
        let window = app.windows.firstMatch.frame
        let from = CGVector(dx: found.frame.midX, dy: found.frame.midY + 70)
        let to = CGVector(dx: window.midX, dy: window.height * 0.25 + 70)
        let origin = app.coordinate(withNormalizedOffset: .zero)
        origin.withOffset(from).press(forDuration: 0.05, thenDragTo: origin.withOffset(to))
        sleep(2)
        let event = app.buttons[label]
        XCTAssertTrue(event.exists, "the event left the view")
        assertTarget(event)
        // Events crowd each other; which one a tap picks is MapKit's choice. What matters here is
        // that the one it picks says it is selected and opens its detail.
        event.tap()
        XCTAssertTrue(app.buttons["Close"].waitForExistence(timeout: 10), "no event detail")
        let selected = app.buttons.matching(NSPredicate(format: "label CONTAINS ', updated ' AND selected == true")).firstMatch
        XCTAssertTrue(selected.waitForExistence(timeout: 10), "no event selected after a tap")
        XCTAssertGreaterThanOrEqual(selected.frame.width, 44)
    }

    func testAircraftAreButtonsThatSelect() throws {
        try XCTSkipUnless(ProcessInfo.processInfo.environment["SHIELD_UITEST_FLIGHTS"] == "1",
                          "Flights only on the replay: set TEST_RUNNER_SHIELD_UITEST_FLIGHTS=1 (see the class comment).")
        // The replay places aircraft per 5° cell; hex 490031 lands near Lingen.
        let app = launch(["SHIELD_FLIGHTS": "1", "SHIELD_REGION": "52.42,7.31,0.8,0.5"])
        let aircraft = try isolated(app, prefix: "Aircraft ")
        try assertSelectsOnTap(app, aircraft)
    }

    // MARK: - Helpers

    private nonisolated static var api: String {
        ProcessInfo.processInfo.environment["SHIELD_UITEST_API"] ?? "http://localhost:8081"
    }

    private nonisolated static func apiAnswers() -> Bool {
        guard let url = URL(string: api + "/v1/sources") else { return false }
        var request = URLRequest(url: url)
        request.timeoutInterval = 3
        let done = DispatchSemaphore(value: 0)
        nonisolated(unsafe) var answered = false
        URLSession.shared.dataTask(with: request) { _, response, _ in
            answered = (response as? HTTPURLResponse) != nil
            done.signal()
        }.resume()
        _ = done.wait(timeout: .now() + 5)
        return answered
    }

    private func launch(_ env: [String: String]) -> XCUIApplication {
        let app = XCUIApplication()
        app.launchEnvironment = env.merging(["SHIELD_RESET_FILTERS": "1"]) { a, _ in a }
        app.launch()
        return app
    }

    private func singleEvents(_ app: XCUIApplication) -> XCUIElementQuery {
        app.buttons.matching(NSPredicate(format: "label CONTAINS ', updated ' AND NOT label BEGINSWITH 'Cluster,'"))
    }

    private func isolated(_ app: XCUIApplication, prefix: String) throws -> XCUIElement {
        try isolated(app, query: app.buttons.matching(NSPredicate(format: "label BEGINSWITH %@", prefix)))
    }

    /// A marker on screen with no other marker within 50 pt, so a tap can only mean it.
    private func isolated(_ app: XCUIElement, query: XCUIElementQuery) throws -> XCUIElement {
        XCTAssertTrue(query.firstMatch.waitForExistence(timeout: 30), "no marker found")
        sleep(2) // the rest of the answer's markers
        // The upper map only, clear of the controls on the right: the layer legend floats mid-map
        // and the sheet holds the bottom, and a tap there lands on them (they don't hide markers
        // from the accessibility tree, so `isHittable` can't tell).
        let window = (app as? XCUIApplication)?.windows.firstMatch.frame ?? .zero
        let upper = CGRect(x: 40, y: 140, width: window.width - 110, height: window.height * 0.45 - 140)
        let markers = app.buttons.allElementsBoundByIndex.filter { $0.frame.width <= 64 && $0.frame.height <= 64 }
        let centres = markers.map { CGPoint(x: $0.frame.midX, y: $0.frame.midY) }
        for candidate in query.allElementsBoundByIndex where candidate.isHittable {
            let c = CGPoint(x: candidate.frame.midX, y: candidate.frame.midY)
            guard upper.contains(c) else { continue }
            let neighbours = centres.filter { hypot($0.x - c.x, $0.y - c.y) < 50 }
            if neighbours.count == 1 { return candidate }
        }
        let found = query.allElementsBoundByIndex.prefix(6).map { "\($0.label.prefix(30)) \($0.frame.integral)" }
        throw XCTSkip("no marker stands alone on screen among \(found)")
    }

    private func assertTarget(_ marker: XCUIElement, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertEqual(marker.elementType, .button, file: file, line: line)
        XCTAssertGreaterThanOrEqual(marker.frame.width, 44, marker.label, file: file, line: line)
        XCTAssertGreaterThanOrEqual(marker.frame.height, 44, marker.label, file: file, line: line)
        XCTAssertFalse(marker.isSelected, marker.label, file: file, line: line)
    }

    private func assertSelectsOnTap(_ app: XCUIApplication, _ marker: XCUIElement,
                                    file: StaticString = #filePath, line: UInt = #line) throws {
        assertTarget(marker, file: file, line: line)
        let label = marker.label
        marker.tap()
        let selected = app.buttons.matching(NSPredicate(format: "label == %@ AND selected == true", label)).firstMatch
        XCTAssertTrue(selected.waitForExistence(timeout: 10), "\(label) not selected after a tap", file: file, line: line)
        XCTAssertTrue(app.buttons["Close"].waitForExistence(timeout: 10), "no detail sheet for \(label)", file: file, line: line)
    }
}
