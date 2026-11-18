import ShieldCore
import SwiftUI

/// What sort of record this is, in fixed words (DESIGN.md §2, §5). Neutral colours: colour
/// belongs to severity. Forecasts get a dashed border so they never pass for observations.
struct KindChip: View {
    var kind: EventKind

    var body: some View {
        Label {
            Text(kind.label)
        } icon: {
            Image(systemName: kind.sfSymbol)
                .imageScale(.small)
        }
        .labelStyle(ChipLabelStyle())
        .font(ShieldFont.meta)
        .foregroundStyle(Color.ink)
        .padding(.horizontal, ShieldSpace.s2)
        .padding(.vertical, 3)
        .background(Color.raised, in: chipShape)
        .overlay {
            if kind == .forecast {
                chipShape.strokeBorder(Color.muted, style: StrokeStyle(lineWidth: 1, dash: [4, 3]))
            } else {
                chipShape.strokeBorder(Color.ink.opacity(ShieldAlpha.hairline), lineWidth: 1)
            }
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(kind.label)
    }

    private var chipShape: RoundedRectangle {
        RoundedRectangle(cornerRadius: ShieldRadius.sm, style: .continuous)
    }
}

private struct ChipLabelStyle: LabelStyle {
    func makeBody(configuration: Configuration) -> some View {
        HStack(spacing: ShieldSpace.s1) {
            configuration.icon.foregroundStyle(Color.muted)
            configuration.title
        }
    }
}

#Preview("Kinds", traits: .sizeThatFitsLayout) {
    VStack(alignment: .leading, spacing: ShieldSpace.s2) {
        ForEach(EventKind.allCases, id: \.self) { KindChip(kind: $0) }
    }
    .padding(ShieldSpace.s5)
    .background(Color.panel)
}
