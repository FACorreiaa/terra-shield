import MapKit
import ShieldCore
import SwiftUI

// MARK: - Map

/// One installation listed on Wikidata (ADR 29). Immutable: the list changes once a day.
final class BaseAnnotation: NSObject, MKAnnotation {
    let base: MilitaryBase
    let coordinate: CLLocationCoordinate2D

    init(_ base: MilitaryBase) {
        self.base = base
        coordinate = CLLocationCoordinate2D(latitude: base.coordinate.latitude, longitude: base.coordinate.longitude)
    }

    var title: String? { base.name }
}

/// A base's chip. About 3,000 of them can be in view, so each draws one image rendered once per
/// kind, not a SwiftUI view of its own. Context, like flights: under every event, and hidden by
/// MapKit where they crowd each other.
final class BaseMarkerView: MarkerView {
    static let reuseID = "base"
    static let side: CGFloat = 24

    func configure(_ base: MilitaryBase, isSelected: Bool) {
        // Above aircraft (50), below resolved events (100) and everything active; never hidden
        // while it is the one in the sheet.
        displayPriority = isSelected ? .required : MKFeatureDisplayPriority(rawValue: 75)
        zPriority = isSelected ? .max : .min
        collisionMode = .circle
        clusteringIdentifier = nil
        image = BaseGlyphs.image(base.kind, isSelected: isSelected)
        speak(Self.spoken(base), isSelected: isSelected)
    }

    /// "Military base, Ramstein Air Base, air base, operated by United States Air Force".
    static func spoken(_ b: MilitaryBase) -> String {
        var parts = ["Military base", b.name]
        if !b.className.isEmpty { parts.append(b.className) }
        if let op = b.operatorName, !op.isEmpty { parts.append("operated by \(op)") }
        return parts.joined(separator: ", ")
    }
}

/// The chips as images, rendered on first use and kept: five kinds, selected or not.
@MainActor
enum BaseGlyphs {
    #if os(iOS)
    typealias PlatformImage = UIImage
    #else
    typealias PlatformImage = NSImage
    #endif

    private struct Key: Hashable { let kind: MilitaryBase.Kind; let isSelected: Bool }
    private static var cache: [Key: PlatformImage] = [:]

    static func image(_ kind: MilitaryBase.Kind, isSelected: Bool) -> PlatformImage? {
        let key = Key(kind: kind, isSelected: isSelected)
        if let image = cache[key] { return image }
        let renderer = ImageRenderer(content: BaseMarker(kind: kind, isSelected: isSelected)
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

extension MilitaryBase.Kind {
    /// Every name checked against SF Symbols. A boxed glyph, so an air base never reads as an
    /// aircraft in flight.
    var sfSymbol: String {
        switch self {
        case .air: "airplane"
        case .naval: "ferry.fill"
        case .missile: "location.north.line.fill" // not `scope`: that is the conflicts glyph
        case .radar: "antenna.radiowaves.left.and.right"
        case .base: "star.fill"
        }
    }

    var label: String {
        switch self {
        case .air: "Air base"
        case .naval: "Naval base"
        case .missile: "Missile site"
        case .radar: "Radar station"
        case .base: "Military base"
        }
    }
}

/// A small square chip: square, so it never reads as a hazard (circles) or an aircraft (a bare
/// glyph).
struct BaseMarker: View {
    let kind: MilitaryBase.Kind
    var isSelected = false

    var body: some View {
        ZStack {
            if isSelected {
                RoundedRectangle(cornerRadius: ShieldRadius.sm)
                    .fill(Color.accent.opacity(0.3))
                    .frame(width: 24, height: 24)
            }
            RoundedRectangle(cornerRadius: ShieldRadius.xs)
                .fill(Color.raised)
                .frame(width: 17, height: 17)
                .overlay(
                    RoundedRectangle(cornerRadius: ShieldRadius.xs)
                        .strokeBorder(isSelected ? Color.accent : Color.ink.opacity(ShieldAlpha.hairlineStrong * 3),
                                      lineWidth: isSelected ? 1.5 : 1)
                )
                .shadow(color: .black.opacity(0.6), radius: 1.5)
            Image(systemName: kind.sfSymbol)
                .font(.system(size: 8.5, weight: .bold))
                .foregroundStyle(Color.ink.opacity(0.85))
        }
        .frame(width: BaseMarkerView.side, height: BaseMarkerView.side)
    }
}

// MARK: - Legend

/// The layer's state over the map: how many in view, what a listing means, when the list was
/// refreshed and the CC0 attribution (ADR 29). Never "active": Wikidata records closures unevenly.
struct BasesLegend: View {
    let model: LiveMapModel

    var body: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s1) {
            HStack(spacing: ShieldSpace.s2) {
                Image(systemName: "star.square")
                    .font(.system(size: 11, weight: .semibold))
                    .accessibilityHidden(true)
                Text(headline)
                    .font(ShieldFont.meta.weight(.semibold))
            }
            .foregroundStyle(model.basesFailed ? Color.warning : Color.ink)
            if let layer = model.bases {
                Text(layer.note)
                    .font(ShieldFont.meta)
                    .foregroundStyle(Color.muted)
                    .fixedSize(horizontal: false, vertical: true)
                Text("\(layer.listedLine) · \(layer.license)")
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
        guard let layer = model.bases else {
            return model.basesFailed ? "Military bases didn’t load" : "Military bases · loading…"
        }
        let n = layer.count(in: model.viewport.box)
        let shown = n.formatted(.number.locale(Locale(identifier: "en_US")))
        // "Listed": the count takes every base in the box, those MapKit hides where they crowd
        // and those under the sheet included.
        return "Military bases · " + (n == 0 ? "none listed in this view" : "\(shown) listed in this view")
    }
}

// MARK: - Detail

/// The selected base in the sheet (iPhone) or inspector (Mac, iPad): what Wikidata lists, and a
/// link to the item. A listing, never a claim the base is in use (ADR 29).
struct BaseDetailView: View {
    let base: MilitaryBase
    let layer: BaseLayer?
    var close: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s4) {
            HStack(alignment: .firstTextBaseline) {
                VStack(alignment: .leading, spacing: ShieldSpace.s1) {
                    Text(base.kind.label.uppercased())
                        .font(ShieldFont.micro)
                        .tracking(ShieldTracking.micro)
                        .foregroundStyle(Color.muted)
                    Text(base.name)
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
            if let description = base.description, !description.isEmpty {
                Text(description.prefix(1).uppercased() + description.dropFirst())
                    .font(ShieldFont.body)
                    .foregroundStyle(Color.muted)
                    .fixedSize(horizontal: false, vertical: true)
            }
            Grid(alignment: .leading, horizontalSpacing: ShieldSpace.s4, verticalSpacing: ShieldSpace.s2) {
                ForEach(base.figures, id: \.self) { figure in
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
            Link(destination: base.url) {
                Label("Open on Wikidata", systemImage: "arrow.up.right.square")
                    .font(ShieldFont.body.weight(.semibold))
            }
            Rectangle().fill(Color.ink.opacity(ShieldAlpha.hairline)).frame(height: 1)
            VStack(alignment: .leading, spacing: ShieldSpace.s1) {
                if let layer {
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
