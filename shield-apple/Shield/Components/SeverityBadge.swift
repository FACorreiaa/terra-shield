import ShieldCore
import SwiftUI

/// Shield's severity as colour + shape + word, then the source's own value verbatim in muted:
/// `▲ Severe · GDACS Orange` (DESIGN.md §4). The two severities are never merged.
struct SeverityBadge: View {
    enum Size { case regular, compact }

    var severity: Severity
    var sourceSeverity: String? = nil
    /// Derived from a weak signal (DESIGN.md §5): adds "Provisional" after the word.
    var isProvisional: Bool = false
    var size: Size = .regular

    @ScaledMetric(relativeTo: .footnote) private var glyph: CGFloat = 11

    var body: some View {
        HStack(spacing: ShieldSpace.s2) {
            HStack(spacing: ShieldSpace.s1 + 1) {
                Image(systemName: severity.sfSymbol)
                    .resizable()
                    .scaledToFit()
                    .foregroundStyle(Color.canvas)
                    .frame(width: glyphSize, height: glyphSize)
                Text(severity.label)
                    .font(size == .compact ? ShieldFont.micro : ShieldFont.meta)
                    .fontWeight(.semibold)
                    .foregroundStyle(Color.canvas)
            }
            .padding(.horizontal, size == .compact ? ShieldSpace.s1 + 2 : ShieldSpace.s2)
            .padding(.vertical, size == .compact ? 2 : 3)
            .background(severity.color, in: RoundedRectangle(cornerRadius: ShieldRadius.xs, style: .continuous))

            if isProvisional {
                Text("Provisional")
                    .font(size == .compact ? ShieldFont.micro : ShieldFont.meta)
                    .italic()
                    .foregroundStyle(Color.muted)
            }
            if let sourceSeverity, !sourceSeverity.isEmpty {
                Text(sourceSeverity)
                    .font(size == .compact ? ShieldFont.micro : ShieldFont.meta)
                    .foregroundStyle(Color.muted)
                    .lineLimit(1)
            }
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(accessibilityText)
    }

    private var glyphSize: CGFloat { size == .compact ? glyph * 0.8 : glyph }

    private var accessibilityText: String {
        var s = "Severity \(severity.label)"
        if isProvisional { s += ", provisional" }
        if let sourceSeverity, !sourceSeverity.isEmpty { s += ". Source says \(sourceSeverity)" }
        return s
    }
}

#Preview("Severity", traits: .sizeThatFitsLayout) {
    VStack(alignment: .leading, spacing: ShieldSpace.s3) {
        SeverityBadge(severity: .extreme, sourceSeverity: "GDACS Red")
        SeverityBadge(severity: .severe, sourceSeverity: "M6.1 mww, PAGER yellow")
        SeverityBadge(severity: .moderate, sourceSeverity: "1,240 ha burned", isProvisional: true)
        SeverityBadge(severity: .minor, sourceSeverity: "Level 1 · Yellow", size: .compact)
    }
    .padding(ShieldSpace.s5)
    .background(Color.panel)
}
