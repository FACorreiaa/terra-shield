import MapKit
import ShieldCore

#if os(iOS)
import UIKit
private typealias PlatformColor = UIColor
#else
import AppKit
private typealias PlatformColor = NSColor
#endif

/// NASA FIRMS satellite heat detections as one world-sized overlay (DESIGN.md §5): a
/// `detection`-tinted density map below zoom 6, individual 3–6 px dots from zoom 6. Its own
/// layer, under every marker, never merged into hazard clusters (DECISIONS #10).
///
/// Immutable: new data means a new overlay, so the renderer never races a change while
/// MapKit draws tiles on background threads.
final class DetectionOverlay: NSObject, MKOverlay {
    let layer: DetectionLayer
    let coordinate = CLLocationCoordinate2D(latitude: 0, longitude: 0)
    let boundingMapRect = MKMapRect.world

    init(_ layer: DetectionLayer) {
        self.layer = layer
    }
}

/// Draws a `DetectionOverlay`. Everything it reads is precomputed and immutable, so `draw` is
/// safe on MapKit's tile threads.
final class DetectionRenderer: MKOverlayRenderer, @unchecked Sendable {
    // @unchecked: all stored state is `let` and value-typed after init.

    private struct Blob {
        let point: MKMapPoint
        /// Grid: radius in map points (scales with the map). Points: radius in screen points.
        let radius: Double
        let alpha: Double
    }

    private let blobs: [Blob]
    private let isGrid: Bool
    private let color: CGColor
    private let outline: CGColor

    init(_ overlay: DetectionOverlay) {
        color = (PlatformColor(named: "Detection") ?? .systemYellow).cgColor
        outline = (PlatformColor(named: "Canvas") ?? .black).withAlphaComponent(0.8).cgColor
        switch overlay.layer {
        case .grid(let cells, let cellDegrees):
            isGrid = true
            // A cell's width in map points; blobs overlap neighbours a little so dense
            // regions read as one patch of heat, not a lattice.
            let cellWidth = MKMapSize.world.width * cellDegrees / 360
            // Relative to the busiest cell in view, on a log scale: a lone detection (a gas
            // flare, a factory) is a faint trace, a burning region the full tint. An absolute
            // scale painted whole continents yellow.
            let peak = log1p(Double(cells.map(\.count).max() ?? 1))
            blobs = cells.map { cell in
                let t = peak > 0 ? log1p(Double(cell.count)) / peak : 0
                return Blob(point: MKMapPoint(CLLocationCoordinate2D(latitude: cell.center.latitude,
                                                                     longitude: cell.center.longitude)),
                            radius: cellWidth * (0.5 + 0.5 * t),
                            alpha: 0.06 + 0.74 * t * t)
            }
        case .points(let points, _):
            isGrid = false
            blobs = points.map { d in
                // 3 px for a weak or unknown reading up to 6 px at ≥ 100 MW.
                let frp = min(1, log10(max(d.frp ?? 1, 1)) / 2)
                return Blob(point: MKMapPoint(CLLocationCoordinate2D(latitude: d.coordinate.latitude,
                                                                     longitude: d.coordinate.longitude)),
                            radius: 1.5 + 1.5 * frp,
                            alpha: d.confidence == .low ? 0.55 : 0.95)
            }
        }
        super.init(overlay: overlay)
    }

    override func draw(_ mapRect: MKMapRect, zoomScale: MKZoomScale, in context: CGContext) {
        if isGrid {
            drawDensity(mapRect, in: context)
        } else {
            drawDots(mapRect, zoomScale: zoomScale, in: context)
        }
    }

    private func drawDensity(_ mapRect: MKMapRect, in context: CGContext) {
        guard let gradient = CGGradient(
            colorsSpace: nil,
            colors: [color, color.copy(alpha: 0)!] as CFArray,
            locations: [0, 1]
        ) else { return }
        for blob in blobs {
            let r = blob.radius
            guard mapRect.intersects(MKMapRect(x: blob.point.x - r, y: blob.point.y - r, width: 2 * r, height: 2 * r))
            else { continue }
            let center = self.point(for: blob.point)
            context.saveGState()
            context.setAlpha(blob.alpha)
            context.drawRadialGradient(gradient, startCenter: center, startRadius: 0,
                                       endCenter: center, endRadius: r, options: [])
            context.restoreGState()
        }
    }

    private func drawDots(_ mapRect: MKMapRect, zoomScale: MKZoomScale, in context: CGContext) {
        // Dots keep their screen size at every zoom: screen points / zoomScale = map points.
        let perPoint = 1 / Double(zoomScale)
        context.setStrokeColor(outline)
        context.setLineWidth(0.75 * perPoint)
        for blob in blobs {
            let r = blob.radius * perPoint
            let rect = MKMapRect(x: blob.point.x - r, y: blob.point.y - r, width: 2 * r, height: 2 * r)
            guard mapRect.intersects(rect) else { continue }
            let dot = self.rect(for: rect)
            context.setFillColor(color.copy(alpha: blob.alpha)!)
            context.fillEllipse(in: dot)
            context.strokeEllipse(in: dot)
        }
    }
}
