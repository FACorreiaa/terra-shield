import MapKit
import ShieldCore
import SwiftUI

#if os(iOS)
private typealias PlatformColor = UIColor
#else
private typealias PlatformColor = NSColor
#endif

/// The selected event's footprint (DESIGN.md §5, kind encoding), drawn under detections and
/// markers. Built from map points rather than coordinates, so a ring that crosses ±180° stays
/// one shape instead of a band across the whole world.
enum EventArea {
    /// The overlay for `geometry`, or `nil` when there is nothing beyond the marker to draw.
    static func overlay(for geometry: EventGeometry) -> (any MKOverlay)? {
        switch geometry {
        case .point:
            return nil
        case .lines(let lines):
            let polylines = lines.map { line -> MKPolyline in
                let points = mapPoints(EventGeometry.unwrapped(line))
                return MKPolyline(points: points, count: points.count)
            }
            return polylines.isEmpty ? nil : MKMultiPolyline(polylines)
        case .polygons(let polygons):
            let shapes = polygons.compactMap { rings -> MKPolygon? in
                guard let outer = rings.first else { return nil }
                let outerRing = EventGeometry.unwrapped(outer)
                let holes = rings.dropFirst().map { hole -> MKPolygon in
                    let points = mapPoints(EventGeometry.unwrapped(hole, near: outerRing.first?.longitude))
                    return MKPolygon(points: points, count: points.count)
                }
                let points = mapPoints(outerRing)
                return MKPolygon(points: points, count: points.count, interiorPolygons: holes)
            }
            return shapes.isEmpty ? nil : MKMultiPolygon(shapes)
        }
    }

    /// Styled by kind: observed is filled, an official warning has a stronger edge, a forecast
    /// is a dashed outline with no fill. Colour is the app severity.
    /// A stale or resolved event's area is desaturated like its marker (DECISIONS #8).
    static func renderer(for overlay: any MKOverlay, event: HazardEvent, isStale: Bool) -> MKOverlayRenderer {
        let faded = isStale || event.isResolved
        let color = faded ? (PlatformColor(named: "Muted") ?? .gray) : PlatformColor(event.severity.color)
        let style = Self.style(for: event.kind)
        let renderer: MKOverlayPathRenderer
        switch overlay {
        case let polygons as MKMultiPolygon:
            renderer = MKMultiPolygonRenderer(multiPolygon: polygons)
            renderer.fillColor = color.withAlphaComponent(style.fill)
        case let lines as MKMultiPolyline:
            renderer = MKMultiPolylineRenderer(multiPolyline: lines)
        default:
            return MKOverlayRenderer(overlay: overlay)
        }
        renderer.strokeColor = color
        renderer.lineWidth = style.width
        renderer.lineJoin = .round
        if style.dashed { renderer.lineDashPattern = [6, 4] }
        return renderer
    }

    /// Fill alpha, stroke width and dash by kind (DESIGN.md §5); the map and the detail's
    /// snippet share it.
    static func style(for kind: EventKind) -> (fill: CGFloat, width: CGFloat, dashed: Bool) {
        switch kind {
        case .observed: (0.18, 1.5, false)
        case .warning: (0.10, 2, false)
        case .forecast: (0, 1.5, true)
        // Detections have no area by design (§5); should one arrive with a shape, outline only.
        case .detection: (0, 1, false)
        }
    }

    /// MapKit's x is linear in longitude, so a longitude past ±180° maps just beyond the world's
    /// edge, which MapKit draws across the antimeridian.
    private static func mapPoints(_ ring: [Coordinate]) -> [MKMapPoint] {
        let width = MKMapSize.world.width
        return ring.map { c in
            let y = MKMapPoint(CLLocationCoordinate2D(latitude: c.latitude, longitude: 0)).y
            return MKMapPoint(x: (c.longitude + 180) / 360 * width, y: y)
        }
    }
}
