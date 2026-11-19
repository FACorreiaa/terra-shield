import ShieldCore
import SwiftUI

/// A hazard on the map (VIEWS.md catalog; DESIGN.md §4–5). Severity sets colour, shape and
/// size; kind sets the form (filled, ringed, outline); the type glyph sits inside. Satellite
/// heat detections are small dots with no glyph. Stale markers fade, desaturate and get a clock;
/// resolved ones (shown only when the filter includes them) do the same with a checkmark.
struct HazardMarker: View {
    var type: HazardType
    var severity: Severity
    var kind: EventKind
    var isStale: Bool = false
    /// Resolved wins over stale: a finished event isn't waiting on its source.
    var isResolved: Bool = false
    var isSelected: Bool = false
    /// Point size of a `moderate` marker; other severities scale from it.
    var baseSize: CGFloat = 28
    /// The extreme-severity halo. On the map only (DESIGN.md §8, §13); lists and cards hold still.
    var pulses = true

    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        Group {
            if kind == .detection {
                Circle()
                    .fill(Color.detection)
                    .frame(width: 6, height: 6)
            } else {
                marker
            }
        }
        .opacity(isFaded ? ShieldAlpha.stale : 1)
        .saturation(isFaded ? 0 : 1)
        .overlay(alignment: .topTrailing) {
            // A 6 pt detection dot would vanish under the badge; it only fades.
            if kind != .detection {
                if isResolved { badge("checkmark") } else if isStale { badge("clock.fill") }
            }
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(accessibilityText)
        .accessibilityAddTraits(isSelected ? [.isButton, .isSelected] : .isButton)
    }

    private var isFaded: Bool { isStale || isResolved }
    private var size: CGFloat { baseSize * severity.markerScale }
    private var shape: MarkerShape { MarkerShape(kind: severity.markerShape) }

    private var marker: some View {
        ZStack {
            if pulses && severity.pulses && !reduceMotion && !isFaded {
                PulseHalo(shape: shape, color: severity.color)
            }
            body(for: kind)
            Image(systemName: type.sfSymbol)
                .resizable()
                .scaledToFit()
                .fontWeight(.semibold)
                .foregroundStyle(kind == .forecast ? severity.color : Color.canvas)
                .frame(width: size * shape.glyphScale, height: size * shape.glyphScale)
                .offset(y: size * shape.glyphOffsetY)
        }
        .frame(width: size, height: size)
        .scaleEffect(isSelected ? 1.15 : 1)
        .shadow(color: .black.opacity(0.45), radius: 3, y: 1)
        .animation(ShieldMotion.spring, value: isSelected)
    }

    @ViewBuilder
    private func body(for kind: EventKind) -> some View {
        switch kind {
        case .forecast:
            shape.fill(Color.canvas.opacity(0.85))
                .overlay(shape.stroke(severity.color, lineWidth: 2))
        case .warning:
            shape.fill(severity.color)
                .overlay(shape.inset(by: 3).stroke(Color.canvas, lineWidth: 2))
                .overlay(selectionRing)
        default:
            shape.fill(severity.color)
                .overlay(selectionRing)
        }
    }

    @ViewBuilder
    private var selectionRing: some View {
        if isSelected {
            shape.stroke(Color.ink, lineWidth: 2).padding(-3)
        }
    }

    /// `muted`, never `ok`: green stays off the map (DESIGN.md §3).
    private func badge(_ symbol: String) -> some View {
        Image(systemName: symbol)
            .font(.system(size: 9, weight: .bold))
            .foregroundStyle(Color.canvas)
            .padding(2)
            .background(Circle().fill(Color.muted))
            .offset(x: 4, y: -4)
    }

    private var accessibilityText: String {
        if kind == .detection {
            return "Satellite heat detection" + (isStale ? ", stale" : "")
        }
        var parts = ["\(severity.label) \(type.label.lowercased())", kind.label.lowercased()]
        if isResolved { parts.append("resolved") } else if isStale { parts.append("stale") }
        return parts.joined(separator: ", ")
    }
}

extension MarkerShape: InsettableShape {
    func inset(by amount: CGFloat) -> InsetMarkerShape { InsetMarkerShape(base: self, amount: amount) }
}

struct InsetMarkerShape: InsettableShape {
    var base: MarkerShape
    var amount: CGFloat

    func path(in rect: CGRect) -> Path { base.path(in: rect.insetBy(dx: amount, dy: amount)) }
    func inset(by more: CGFloat) -> InsetMarkerShape { InsetMarkerShape(base: base, amount: amount + more) }
}

/// One slow halo for extreme severity (DESIGN.md §13). Not shown under Reduce Motion.
private struct PulseHalo: View {
    var shape: MarkerShape
    var color: Color

    var body: some View {
        PhaseAnimator([false, true]) { expanded in
            shape.fill(color)
                .scaleEffect(expanded ? 1.9 : 1)
                .opacity(expanded ? 0 : 0.45)
        } animation: { expanded in
            expanded ? .easeOut(duration: ShieldMotion.extremePulseDuration) : nil
        }
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}

#Preview("Markers", traits: .sizeThatFitsLayout) {
    VStack(spacing: ShieldSpace.s4) {
        ForEach(EventKind.allCases, id: \.self) { kind in
            HStack(spacing: ShieldSpace.s5) {
                ForEach(Severity.allCases, id: \.self) { severity in
                    HazardMarker(type: .earthquake, severity: severity, kind: kind)
                }
                HazardMarker(type: .flood, severity: .severe, kind: kind, isStale: true)
                HazardMarker(type: .wildfire, severity: .moderate, kind: kind, isSelected: true)
            }
        }
    }
    .padding(ShieldSpace.s6)
    .background(Color.canvas)
}
