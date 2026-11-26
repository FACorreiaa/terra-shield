import MapKit
import ShieldCore
import SwiftUI

#if os(iOS)
private typealias PlatformColor = UIColor
#else
private typealias PlatformColor = NSColor
#endif

// MARK: - Map

/// One historic site listed on Wikidata (ADR 30). Immutable: the list changes once a week.
final class HistoricAnnotation: NSObject, MKAnnotation {
    let site: HistoricSite
    let coordinate: CLLocationCoordinate2D

    init(_ site: HistoricSite) {
        self.site = site
        coordinate = CLLocationCoordinate2D(latitude: site.coordinate.latitude, longitude: site.coordinate.longitude)
    }

    var title: String? { site.name }
}

/// A site's chip. Up to `HistoricLayer.chipLimit` are drawn (beyond it, dots), so each draws one
/// image rendered once per kind. Context: under bases and every event, and
/// hidden by MapKit where they crowd each other.
final class HistoricMarkerView: MKAnnotationView {
    static let reuseID = "historic"
    static let width: CGFloat = 28
    static let height: CGFloat = 22

    func configure(_ site: HistoricSite, isSelected: Bool) {
        // Above aircraft (50), below bases (75), the better known first where chips collide
        // (58-72); never hidden while it is the one in the sheet.
        displayPriority = isSelected ? .required : MKFeatureDisplayPriority(rawValue: Float(58 + site.prominence))
        zPriority = isSelected ? .max : .min
        collisionMode = .circle
        // Never clustered, and never changed while on the map (see `AnnotationSet.renewEvents`).
        clusteringIdentifier = nil
        image = HistoricGlyphs.image(site.kind, isSelected: isSelected)
        let label = Self.spoken(site)
        #if os(iOS)
        isAccessibilityElement = true
        accessibilityLabel = label
        #else
        setAccessibilityElement(true)
        setAccessibilityLabel(label)
        #endif
    }

    /// "Historic site, Neuschwanstein Castle, castle".
    static func spoken(_ s: HistoricSite) -> String {
        var parts = ["Historic site", s.name]
        if !s.className.isEmpty { parts.append(s.className) }
        return parts.joined(separator: ", ")
    }
}

/// The chips as images, rendered on first use and kept: four looks, selected or not.
@MainActor
enum HistoricGlyphs {
    #if os(iOS)
    typealias PlatformImage = UIImage
    #else
    typealias PlatformImage = NSImage
    #endif

    private struct Key: Hashable { let kind: HistoricKind?; let isSelected: Bool }
    private static var cache: [Key: PlatformImage] = [:]

    static func image(_ kind: HistoricKind?, isSelected: Bool) -> PlatformImage? {
        let key = Key(kind: kind, isSelected: isSelected)
        if let image = cache[key] { return image }
        let renderer = ImageRenderer(content: HistoricMarker(kind: kind, isSelected: isSelected)
            .environment(\.colorScheme, .dark)) // the map is always dark
        #if os(iOS)
        renderer.scale = 3
        let image = renderer.uiImage
        #else
        renderer.scale = 2
        let image = renderer.nsImage
        #endif
        cache[key] = image
        return image
    }
}

extension HistoricKind {
    /// Every name checked against SF Symbols. None is a base's or a hazard's glyph.
    var sfSymbol: String {
        switch self {
        case .castle: "crown.fill"
        case .military: "shield.lefthalf.filled"
        case .memorial: "laurel.leading"
        case .general: "building.columns.fill"
        }
    }

}

/// A group this version doesn't know: a neutral pin, never a military glyph (ADR 30).
private let unknownKindSymbol = "mappin"

/// A small capsule chip, wider than tall: none of the hazards' severity shapes (circle, diamond,
/// triangle, octagon, DESIGN.md §4) nor a base's square.
struct HistoricMarker: View {
    let kind: HistoricKind?
    var isSelected = false

    var body: some View {
        ZStack {
            if isSelected {
                Capsule()
                    .fill(Color.accent.opacity(0.3))
                    .frame(width: 26, height: 19)
            }
            Capsule()
                .fill(Color.raised)
                .frame(width: 20, height: 13)
                .overlay(
                    Capsule()
                        .strokeBorder(isSelected ? Color.accent : Color.ink.opacity(ShieldAlpha.hairlineStrong * 3),
                                      lineWidth: isSelected ? 1.5 : 1)
                )
                .shadow(color: .black.opacity(0.6), radius: 1.5)
            Image(systemName: kind?.sfSymbol ?? unknownKindSymbol)
                .font(.system(size: 8, weight: .bold))
                .foregroundStyle(Color.ink.opacity(0.85))
        }
        .frame(width: HistoricMarkerView.width, height: HistoricMarkerView.height)
    }
}

/// The sites as dots, under every marker: the grid's counts below zoom 8, and from zoom 8 the
/// sites themselves when too many are in view to draw as chips (`HistoricLayer.chipLimit`).
/// Drawn on MapKit's tile threads, so thousands of them cost the main thread nothing.
/// Immutable: new data means a new overlay, so the renderer never races a change while MapKit
/// draws tiles on background threads.
final class HistoricDotsOverlay: NSObject, MKOverlay {
    let content: HistoricLayer.Content
    let coordinate = CLLocationCoordinate2D(latitude: 0, longitude: 0)
    let boundingMapRect = MKMapRect.world

    init(_ content: HistoricLayer.Content) {
        self.content = content
    }
}

/// Draws a `HistoricDotsOverlay` in a neutral ink: a stipple, not a glow, so it never reads as the
/// detections' heat. Everything it reads is precomputed and immutable, so `draw` is safe on
/// MapKit's tile threads.
final class HistoricDotsRenderer: MKOverlayRenderer, @unchecked Sendable {
    // @unchecked: all stored state is `let` and value-typed after init.

    private struct Dot {
        let point: MKMapPoint
        /// Grid: in map points (scales with the map until the next answer). Sites: screen points.
        let radius: Double
        let alpha: Double
    }

    private let dots: [Dot]
    private let isGrid: Bool
    private let color: CGColor
    private let outline: CGColor

    init(_ overlay: HistoricDotsOverlay) {
        color = (PlatformColor(named: "Ink") ?? .white).cgColor
        outline = (PlatformColor(named: "Canvas") ?? .black).withAlphaComponent(0.8).cgColor
        switch overlay.content {
        case .grid(let cells, let cellDegrees):
            isGrid = true
            let cellWidth = MKMapSize.world.width * cellDegrees / 360
            // Relative to the busiest cell in the answer (the world's below zoom 6), on a log scale and squared: a lone fort is a
            // faint speck, Flanders' hundreds of bunkers and cemeteries the brightest dot, and a
            // sparse region doesn't read as an even lattice.
            let peak = log1p(Double(cells.map(\.count).max() ?? 1))
            dots = cells.map { cell in
                let t = peak > 0 ? log1p(Double(cell.count)) / peak : 0
                return Dot(point: MKMapPoint(CLLocationCoordinate2D(latitude: cell.center.latitude,
                                                                    longitude: cell.center.longitude)),
                           radius: cellWidth * (0.14 + 0.26 * t),
                           alpha: 0.08 + 0.6 * t * t)
            }
        case .points(let sites, _):
            isGrid = false
            dots = sites.map { site in
                Dot(point: MKMapPoint(CLLocationCoordinate2D(latitude: site.coordinate.latitude,
                                                             longitude: site.coordinate.longitude)),
                    radius: 2.5, alpha: 0.7)
            }
        }
        super.init(overlay: overlay)
    }

    override func draw(_ mapRect: MKMapRect, zoomScale: MKZoomScale, in context: CGContext) {
        // Sites keep their screen size at every zoom: screen points / zoomScale = map points.
        let scale = isGrid ? 1 : 1 / Double(zoomScale)
        // One fill colour; each dot's strength is the context's alpha, so nothing is allocated per dot.
        context.setFillColor(color)
        if !isGrid {
            context.setStrokeColor(outline)
            context.setLineWidth(0.75 * scale)
        }
        for dot in dots {
            let r = dot.radius * scale
            let rect = MKMapRect(x: dot.point.x - r, y: dot.point.y - r, width: 2 * r, height: 2 * r)
            guard mapRect.intersects(rect) else { continue }
            let shape = self.rect(for: rect)
            context.setAlpha(dot.alpha)
            context.fillEllipse(in: shape)
            if !isGrid {
                context.setAlpha(1) // the outline keeps its own strength
                context.strokeEllipse(in: shape)
            }
        }
    }
}

// MARK: - Legend

/// The layer's state over the map: how many sites are in view, whether to zoom in to pick one,
/// and what the layer is (the server's `note`) with the CC0 attribution (ADR 30). The groups are
/// chosen in the filter sheet, so this stays short beside the other legends. On a phone the note
/// (five lines there) is left to the filter sheet's section and the site's sheet, where the layer
/// is switched on and a site is read.
struct HistoricLegend: View {
    let model: LiveMapModel
    #if os(iOS)
    @Environment(\.horizontalSizeClass) private var sizeClass
    private var showsNote: Bool { sizeClass != .compact }
    #else
    private let showsNote = true
    #endif

    var body: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s1) {
            HStack(spacing: ShieldSpace.s2) {
                Image(systemName: "building.columns")
                    .font(.system(size: 11, weight: .semibold))
                    .accessibilityHidden(true)
                Text(headline)
                    .font(ShieldFont.meta.weight(.semibold))
            }
            .foregroundStyle(model.historicFailed ? Color.warning : Color.ink)
            if let layer = model.historic {
                if let text = [layer.hint, showsNote ? layer.note : nil].compactMap({ $0 }).nonEmptyJoined {
                    Text(text)
                        .font(ShieldFont.meta)
                        .foregroundStyle(Color.muted)
                        .fixedSize(horizontal: false, vertical: true)
                }
                Text("Listed on \(layer.attribution) · \(layer.license)")
                    .font(ShieldFont.micro)
                    .foregroundStyle(Color.muted)
            }
        }
        .padding(.horizontal, ShieldSpace.s3)
        .padding(.vertical, ShieldSpace.s2)
        .frame(maxWidth: 360, alignment: .leading)
        .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: ShieldRadius.md))
        .overlay(RoundedRectangle(cornerRadius: ShieldRadius.md).strokeBorder(Color.ink.opacity(ShieldAlpha.hairline)))
        .accessibilityElement(children: .combine)
    }

    private var headline: String {
        guard let layer = model.historic else {
            return model.historicFailed ? "Historic sites didn’t load" : "Historic sites · loading…"
        }
        let line = layer.headline(in: model.viewport.box, kinds: model.historicKinds)
        return model.historicFailed ? line + " · couldn’t refresh" : line
    }
}

// MARK: - Detail

/// The selected site in the sheet (iPhone) or inspector (Mac, iPad): what Wikidata lists, and a
/// link to the item. A listing: some sites are ruins, some still in use (ADR 30).
struct HistoricDetailView: View {
    let site: HistoricSite
    let layer: HistoricLayer?
    var close: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s4) {
            HStack(alignment: .firstTextBaseline) {
                VStack(alignment: .leading, spacing: ShieldSpace.s1) {
                    Text(site.kindLabel.uppercased())
                        .font(ShieldFont.micro)
                        .tracking(ShieldTracking.micro)
                        .foregroundStyle(Color.muted)
                    Text(site.name)
                        .font(ShieldFont.title)
                        .foregroundStyle(Color.ink)
                        .fixedSize(horizontal: false, vertical: true)
                }
                Spacer()
                Button(action: close) {
                    Image(systemName: "xmark.circle.fill")
                        .font(.system(size: 22))
                        .foregroundStyle(Color.muted)
                }
                .buttonStyle(.plain)
                .accessibilityLabel("Close")
            }
            if let description = site.description, !description.isEmpty {
                Text(description.prefix(1).uppercased() + description.dropFirst())
                    .font(ShieldFont.body)
                    .foregroundStyle(Color.muted)
                    .fixedSize(horizontal: false, vertical: true)
            }
            Grid(alignment: .leading, horizontalSpacing: ShieldSpace.s4, verticalSpacing: ShieldSpace.s2) {
                ForEach(site.figures, id: \.self) { figure in
                    GridRow {
                        Text(figure.label)
                            .font(ShieldFont.meta)
                            .foregroundStyle(Color.muted)
                        Text(figure.value)
                            .font(ShieldFont.body.monospacedDigit())
                            .foregroundStyle(Color.ink)
                            .textSelection(.enabled)
                    }
                }
            }
            Link(destination: site.url) {
                Label("Open on Wikidata", systemImage: "arrow.up.right.square")
                    .font(ShieldFont.body.weight(.semibold))
            }
            Rectangle().fill(Color.ink.opacity(ShieldAlpha.hairline)).frame(height: 1)
            if let layer {
                VStack(alignment: .leading, spacing: ShieldSpace.s1) {
                    Text(layer.note)
                        .font(ShieldFont.meta)
                        .foregroundStyle(Color.muted)
                        .fixedSize(horizontal: false, vertical: true)
                    Text("\(layer.listedLine) · \(layer.license)")
                        .font(ShieldFont.meta)
                        .foregroundStyle(Color.muted)
                }
            }
        }
    }
}

private extension Array where Element == String {
    /// The lines as one paragraph; nil when there are none.
    var nonEmptyJoined: String? { isEmpty ? nil : joined(separator: " ") }
}
