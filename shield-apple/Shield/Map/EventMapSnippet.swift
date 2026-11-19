import MapKit
import ShieldCore
import SwiftUI

/// The event's geometry on a small, still map (DESIGN.md §10.3 item 5): its area drawn by kind
/// (§5) with the marker on top. Not interactive; tapping it shows the event on the main map.
struct EventMapSnippet: View {
    let event: HazardEvent
    /// The detail's geometry once loaded; the centroid until then.
    let geometry: EventGeometry?
    let isFaded: Bool
    var showOnMap: (() -> Void)?

    var body: some View {
        let shape = geometry ?? .point(event.centroid)
        let style = EventArea.style(for: event.kind)
        let color = isFaded ? Color.muted : event.severity.color
        Map(initialPosition: .region(MKCoordinateRegion(shape.frame(around: event.centroid))),
            interactionModes: []) {
            ForEach(Array(polygons(shape).enumerated()), id: \.offset) { _, polygon in
                MapPolygon(polygon)
                    .foregroundStyle(color.opacity(style.fill))
                    .stroke(color, style: StrokeStyle(lineWidth: style.width, lineJoin: .round,
                                                      dash: style.dashed ? [6, 4] : []))
            }
            ForEach(Array(polylines(shape).enumerated()), id: \.offset) { _, line in
                MapPolyline(line)
                    .stroke(color, style: StrokeStyle(lineWidth: style.width, lineJoin: .round,
                                                      dash: style.dashed ? [6, 4] : []))
            }
            Annotation("", coordinate: CLLocationCoordinate2D(latitude: event.centroid.latitude,
                                                              longitude: event.centroid.longitude)) {
                HazardMarker(type: event.type, severity: event.severity, kind: event.kind,
                             isStale: isFaded && !event.isResolved, isResolved: event.isResolved,
                             baseSize: 20, pulses: false)
            }
            .annotationTitles(.hidden)
        }
        .mapStyle(.standard(elevation: .flat, emphasis: .muted, pointsOfInterest: .excludingAll))
        .environment(\.colorScheme, .dark)
        // Rebuilt when the shape arrives, so the camera frames it.
        .id(shape)
        .frame(height: 160)
        .clipShape(RoundedRectangle(cornerRadius: ShieldRadius.md, style: .continuous))
        .overlay(RoundedRectangle(cornerRadius: ShieldRadius.md, style: .continuous)
            .strokeBorder(Color.ink.opacity(ShieldAlpha.hairline)))
        .contentShape(Rectangle())
        .onTapGesture { showOnMap?() }
        .accessibilityElement()
        .accessibilityLabel("Map of the event’s \(shape.hasShape ? "area" : "location")")
        .accessibilityAddTraits(showOnMap == nil ? [] : .isButton)
        .accessibilityHint(showOnMap == nil ? "" : "Shows it on the main map")
    }

    /// The main map's shapes (`EventArea`), built from map points so a ring across ±180° stays one.
    private func polygons(_ shape: EventGeometry) -> [MKPolygon] {
        (EventArea.overlay(for: shape) as? MKMultiPolygon)?.polygons ?? []
    }

    private func polylines(_ shape: EventGeometry) -> [MKPolyline] {
        (EventArea.overlay(for: shape) as? MKMultiPolyline)?.polylines ?? []
    }
}
