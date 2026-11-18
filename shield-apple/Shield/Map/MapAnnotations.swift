import MapKit
import ShieldCore
import SwiftUI

/// One event on the map. Immutable: a changed event gets a new annotation, so MapKit redraws it.
final class EventAnnotation: NSObject, MKAnnotation {
    let event: HazardEvent
    let coordinate: CLLocationCoordinate2D

    init(_ event: HazardEvent) {
        self.event = event
        coordinate = CLLocationCoordinate2D(latitude: event.centroid.latitude, longitude: event.centroid.longitude)
    }

    var title: String? { event.title }
}

/// A cluster the server made below zoom 6 (ADR 9).
final class ServerClusterAnnotation: NSObject, MKAnnotation {
    let cluster: EventCluster
    let coordinate: CLLocationCoordinate2D

    init(_ cluster: EventCluster) {
        self.cluster = cluster
        coordinate = CLLocationCoordinate2D(latitude: cluster.centroid.latitude, longitude: cluster.centroid.longitude)
    }

    var title: String? { "\(cluster.count) events" }
}

/// An annotation view that draws a SwiftUI view, so the map uses the same `HazardMarker` and
/// `ClusterMarker` as the rest of the app (pulse, selection and stale styling included).
class SwiftUIAnnotationView: MKAnnotationView {
    #if os(iOS)
    private var host: UIHostingController<AnyView>?
    #else
    private var host: PassthroughHostingView?
    #endif

    /// One spoken label for the whole marker (VoiceOver, DESIGN.md §14).
    func speak(_ label: String) {
        #if os(iOS)
        isAccessibilityElement = true
        accessibilityLabel = label
        #else
        setAccessibilityElement(true)
        setAccessibilityLabel(label)
        #endif
    }

    func render(_ content: some View, size: CGSize) {
        let root = AnyView(content.frame(width: size.width, height: size.height))
        frame.size = size
        #if os(iOS)
        if let host {
            host.rootView = root
        } else {
            let h = UIHostingController(rootView: root)
            h.view.backgroundColor = .clear
            h.sizingOptions = []
            // The marker has nothing to tap inside it; MapKit does the selecting.
            h.view.isUserInteractionEnabled = false
            addSubview(h.view)
            host = h
        }
        host?.view.frame = bounds
        #else
        if let host {
            host.rootView = root
        } else {
            let h = PassthroughHostingView(rootView: root)
            addSubview(h)
            host = h
        }
        host?.frame = bounds
        #endif
    }
}

#if os(macOS)
/// A hosting view that never takes the click, so MapKit selects the annotation under it.
final class PassthroughHostingView: NSHostingView<AnyView> {
    override func hitTest(_ point: NSPoint) -> NSView? { nil }
}
#endif

final class EventMarkerView: SwiftUIAnnotationView {
    static let reuseID = "event"
    /// Members cluster locally from zoom 6, where the server stops clustering (ADR 9).
    static let clusteringID = "events"

    /// `localClustering` only from zoom 6; below it the server already clustered.
    func configure(_ event: HazardEvent, isStale: Bool, isSelected: Bool, localClustering: Bool) {
        let side = 28 * event.severity.markerScale
        // Room for the extreme pulse (1.9×) and the selection ring; others stay tight to the
        // marker so neighbouring hit areas don't overlap.
        let frameSide = event.severity.pulses && !isStale ? side * 2 : side + 10
        render(
            HazardMarker(type: event.type, severity: event.severity, kind: event.kind,
                         isStale: isStale, isSelected: isSelected),
            size: CGSize(width: frameSide, height: frameSide)
        )
        clusteringIdentifier = localClustering ? Self.clusteringID : nil
        collisionMode = .circle
        displayPriority = event.severity.displayPriority
        zPriority = isSelected ? .max : MKAnnotationViewZPriority(rawValue: Float(event.severity.rank) * 10)
        speak(Self.spoken(event, isStale: isStale))
    }

    /// "Severe earthquake, observed, M 6.1 - 38 km W of Coquimbo, updated 12 min ago" (DESIGN.md §14).
    static func spoken(_ e: HazardEvent, isStale: Bool) -> String {
        var parts = ["\(e.severity.label) \(e.type.label.lowercased())", e.kind.label.lowercased(), e.title,
                     "updated \(Freshness.relative(e.updatedAt))"]
        if isStale { parts.append("stale") }
        return parts.joined(separator: ", ")
    }
}

final class ServerClusterView: SwiftUIAnnotationView {
    static let reuseID = "server-cluster"

    func configure(_ cluster: EventCluster) {
        render(ClusterMarker(count: cluster.count, maxSeverity: cluster.maxSeverity, shares: cluster.severityShares),
               size: CGSize(width: 60, height: 60))
        collisionMode = .circle
        displayPriority = .required
        speak("\(cluster.count) events, highest severity \(cluster.maxSeverity.label.lowercased())")
    }
}

/// MapKit's own cluster of overlapping markers from zoom 6 up; same look as a server cluster.
final class LocalClusterView: SwiftUIAnnotationView {
    static let reuseID = "local-cluster"

    func configure(_ cluster: MKClusterAnnotation) {
        let severities = cluster.memberAnnotations
            .compactMap { ($0 as? EventAnnotation)?.event.severity }
        var counts: [Severity: Int] = [:]
        for s in severities { counts[s, default: 0] += 1 }
        let summary = EventCluster(
            centroid: Coordinate(latitude: cluster.coordinate.latitude, longitude: cluster.coordinate.longitude),
            count: severities.count, maxSeverity: severities.max() ?? .minor, types: [:], severities: counts,
            box: .world
        )
        render(ClusterMarker(count: summary.count, maxSeverity: summary.maxSeverity, shares: summary.severityShares),
               size: CGSize(width: 60, height: 60))
        collisionMode = .circle
        displayPriority = .required
        speak("\(summary.count) events, highest severity \(summary.maxSeverity.label.lowercased())")
    }
}

extension Severity {
    /// Which marker MapKit hides first when they collide: minor first, extreme never.
    var displayPriority: MKFeatureDisplayPriority {
        switch self {
        case .extreme: .required
        case .severe: .defaultHigh
        case .moderate: MKFeatureDisplayPriority(rawValue: 500)
        case .minor: .defaultLow
        }
    }
}
