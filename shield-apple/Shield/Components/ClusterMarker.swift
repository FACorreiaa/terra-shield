import ShieldCore
import SwiftUI

/// Several events drawn as one (VIEWS.md catalog; DESIGN.md §9): a `panel` disc, a 2 pt ring in
/// the highest severity inside, an arc per severity proportional to its share, and the count in
/// mono. With no split known (`shares` empty) the ring is the maximum only; no split is guessed.
struct ClusterMarker: View {
    var count: Int
    var maxSeverity: Severity
    /// Most severe first, summing to 1. From the server's counts or the members of a local cluster.
    var shares: [(severity: Severity, share: Double)] = []
    /// A cluster of armed conflicts (ADR 27): a dashed outer ring, so it never reads as hazards.
    var isConflict = false

    /// Grows with the count, slowly: 36 pt for 2 events, 56 pt from about a thousand.
    private var size: CGFloat {
        let growth = log10(CGFloat(max(count, 2)) / 2) / log10(500)
        return 36 + 20 * min(1, growth)
    }

    var body: some View {
        ZStack {
            Circle().fill(Color.panel)
            if isConflict {
                Circle().strokeBorder(maxSeverity.color, style: StrokeStyle(lineWidth: 2, dash: [4, 3]))
            } else {
                Circle().strokeBorder(maxSeverity.color, lineWidth: 2)
            }
            ForEach(Array(arcs.enumerated()), id: \.offset) { _, arc in
                Circle()
                    .trim(from: arc.start, to: arc.end)
                    .stroke(arc.severity.color, style: StrokeStyle(lineWidth: 4, lineCap: .butt))
                    .rotationEffect(.degrees(-90))
                    .padding(5)
            }
            Text(count, format: .number.notation(.compactName))
                .font(ShieldFont.meta.weight(.semibold))
                .foregroundStyle(Color.ink)
                .minimumScaleFactor(0.7)
                .lineLimit(1)
                .padding(.horizontal, 6)
        }
        .frame(width: size, height: size)
        .overlay(alignment: .topTrailing) {
            if isConflict {
                Image(systemName: HazardType.conflict.sfSymbol)
                    .font(.system(size: 9, weight: .bold))
                    .foregroundStyle(Color.canvas)
                    .frame(width: 16, height: 16)
                    .background(maxSeverity.color, in: Circle())
                    .offset(x: 2, y: -2)
            }
        }
        .shadow(color: .black.opacity(0.5), radius: 3, y: 1)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(accessibilityText)
        .accessibilityAddTraits(.isButton)
        .accessibilityHint("Zooms in to show the events")
    }

    private var arcs: [(severity: Severity, start: CGFloat, end: CGFloat)] {
        // A single-severity cluster is already the ring; an arc would repeat it.
        guard shares.count > 1 else { return [] }
        var start: CGFloat = 0
        return shares.map { s in
            defer { start += s.share }
            return (s.severity, start, start + s.share)
        }
    }

    private var accessibilityText: String {
        "\(count) \(isConflict ? "armed-conflict events" : "events"), highest severity \(maxSeverity.label.lowercased())"
    }
}

#Preview("Clusters", traits: .sizeThatFitsLayout) {
    HStack(spacing: ShieldSpace.s5) {
        ClusterMarker(count: 2, maxSeverity: .minor)
        ClusterMarker(count: 12, maxSeverity: .severe, shares: [(.severe, 0.25), (.moderate, 0.25), (.minor, 0.5)])
        ClusterMarker(count: 77, maxSeverity: .minor, shares: [(.minor, 1)])
        ClusterMarker(count: 1240, maxSeverity: .extreme, shares: [(.extreme, 0.05), (.severe, 0.2), (.minor, 0.75)])
    }
    .padding(ShieldSpace.s6)
    .background(Color.canvas)
}
