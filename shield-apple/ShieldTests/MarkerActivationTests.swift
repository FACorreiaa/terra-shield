import MapKit
import ShieldCore
import XCTest
@testable import Shield

/// What VoiceOver calls on a map marker, called directly on a real map in the app: the double-tap
/// (`accessibilityActivate`), the frame it focuses, the traits it reads, and that a marker MapKit
/// hides offers nothing. `MarkerView` is shared by every marker type; a historic chip stands in.
///
/// VoiceOver itself doesn't run in the simulator: these prove the marker's side, not that
/// VoiceOver on an iPhone reaches it (see `docs/BACKLOG.md`).
@MainActor
final class MarkerActivationTests: XCTestCase {
    private final class Delegate: NSObject, MKMapViewDelegate {
        func mapView(_ mapView: MKMapView, viewFor annotation: any MKAnnotation) -> MKAnnotationView? {
            guard let a = annotation as? HistoricAnnotation else { return nil }
            let view = HistoricMarkerView(annotation: a, reuseIdentifier: HistoricMarkerView.reuseID)
            view.configure(a.site, isSelected: false)
            return view
        }
    }

    private let site = HistoricSite(
        id: "Q4048", name: "Neuschwanstein Castle", description: nil,
        kind: .castle, kindName: "castle", className: "castle", country: "DE",
        coordinate: Coordinate(latitude: 47.5575, longitude: 10.75),
        url: URL(string: "https://www.wikidata.org/wiki/Q4048")!)
    private var window: UIWindow?
    private let delegate = Delegate()

    override func tearDown() {
        window?.isHidden = true
        window = nil
        super.tearDown()
    }

    /// A map on screen with the site's marker drawn.
    private func shown() throws -> (MKMapView, HistoricAnnotation, HistoricMarkerView) {
        let scene = try XCTUnwrap(UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.first)
        let window = UIWindow(windowScene: scene)
        window.frame = CGRect(x: 0, y: 0, width: 390, height: 844)
        let host = UIViewController()
        let map = MKMapView(frame: window.bounds)
        map.delegate = delegate
        host.view = map
        window.rootViewController = host
        window.makeKeyAndVisible()
        self.window = window
        let annotation = HistoricAnnotation(site)
        map.setRegion(MKCoordinateRegion(center: annotation.coordinate,
                                         span: MKCoordinateSpan(latitudeDelta: 0.05, longitudeDelta: 0.05)),
                      animated: false)
        map.addAnnotation(annotation)
        for _ in 0..<200 where map.view(for: annotation) == nil {
            RunLoop.main.run(until: Date().addingTimeInterval(0.05))
        }
        let view = try XCTUnwrap(map.view(for: annotation) as? HistoricMarkerView, "MapKit never drew the marker")
        // MapKit adds a marker hidden and shows it once placed (and then offers it to VoiceOver).
        for _ in 0..<200 where view.isHidden { RunLoop.main.run(until: Date().addingTimeInterval(0.05)) }
        XCTAssertFalse(view.isHidden, "MapKit never showed the marker")
        return (map, annotation, view)
    }

    func testADoubleTapSelectsTheMarkerOnTheMap() throws {
        let (map, annotation, view) = try shown()
        XCTAssertTrue(map.selectedAnnotations.isEmpty)
        XCTAssertTrue(view.accessibilityActivate())
        XCTAssertTrue(map.selectedAnnotations.contains { $0 === annotation })
    }

    func testAMarkerIsOneButtonThatSaysWhetherItIsSelected() throws {
        let (_, _, view) = try shown()
        XCTAssertTrue(view.isAccessibilityElement)
        XCTAssertEqual(view.accessibilityLabel, "Historic site, Neuschwanstein Castle, castle")
        XCTAssertTrue(view.accessibilityTraits.contains(.button))
        XCTAssertFalse(view.accessibilityTraits.contains(.selected))
        view.configure(site, isSelected: true)
        XCTAssertTrue(view.accessibilityTraits.contains(.selected))
        view.configure(site, isSelected: false) // a reused view says it again
        XCTAssertFalse(view.accessibilityTraits.contains(.selected))
    }

    /// VoiceOver's frame is at least 44 pt, around the drawn 28 × 22 pt chip.
    func testVoiceOversFrameIsAtLeast44PointsAroundTheChip() throws {
        let (_, _, view) = try shown()
        let frame = view.accessibilityFrame
        let drawn = UIAccessibility.convertToScreenCoordinates(view.bounds, in: view)
        XCTAssertEqual(view.bounds.size, CGSize(width: 28, height: 22))
        XCTAssertGreaterThanOrEqual(frame.width, 44)
        XCTAssertGreaterThanOrEqual(frame.height, 44)
        // Around the chip (contained, not compared to the point: in a parallel run on cloned
        // simulators the map can still be settling between the two reads).
        XCTAssertTrue(frame.insetBy(dx: -2, dy: -2).contains(drawn), "\(frame) doesn't hold \(drawn)")
    }

    /// The selected event never loses a collision, as a selected base or site doesn't: hidden, it
    /// would vanish from the map and from VoiceOver while its detail is open.
    func testTheSelectedEventIsNeverHidden() throws {
        let event = try XCTUnwrap(SampleData.events().first {
            !$0.isResolved && $0.type != .conflict && $0.severity.displayPriority != .required
        })
        let view = EventMarkerView(annotation: EventAnnotation(event), reuseIdentifier: nil)
        view.configure(event, isStale: false, isSelected: true, localClustering: true)
        XCTAssertEqual(view.displayPriority, .required)
        view.configure(event, isStale: false, isSelected: false, localClustering: true)
        XCTAssertEqual(view.displayPriority, event.severity.displayPriority)
    }

    /// MapKit hides a marker beaten in a collision (or a member of a cluster it draws) with
    /// `isHidden` and leaves it in its container, which still offers it to accessibility; seen
    /// in the app on 2026-11-26, an event under a cluster read as a button at the cluster.
    func testAMarkerTheMapHidesOffersNothingToVoiceOver() throws {
        let (_, _, view) = try shown()
        view.isHidden = true
        XCTAssertFalse(view.isAccessibilityElement)
        XCTAssertTrue(view.accessibilityElementsHidden)
        XCTAssertEqual(view.accessibilityFrame, .zero)
        view.isHidden = false
        XCTAssertTrue(view.isAccessibilityElement)
        XCTAssertFalse(view.accessibilityElementsHidden)
        XCTAssertGreaterThanOrEqual(view.accessibilityFrame.width, 44)
    }
}
