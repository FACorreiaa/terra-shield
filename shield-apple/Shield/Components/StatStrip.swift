import ShieldCore
import SwiftUI

/// Numbers-first summary of what the map shows (VIEWS.md catalog; DESIGN.md §10.1): the number
/// in `statHero`, the words below.
///
/// Display-only for now. Tapping a cell to filter the map arrives with the filters in S3.3,
/// when a cell gains its `filter`.
struct StatStrip: View {
    struct Cell: Identifiable, Hashable {
        /// `nil` when the number is not known: nothing loaded yet, or the server's clusters do
        /// not carry it.
        var value: Int?
        /// The real number is at least `value`; shown as "12+".
        var isLowerBound = false
        var label: String

        var id: String { label }

        var text: String {
            guard let value else { return "–" }
            return isLowerBound ? "\(value)+" : "\(value)"
        }

        var accessibilityText: String {
            guard let value else { return "\(label): not counted at this zoom" }
            return isLowerBound ? "\(label): at least \(value)" : "\(label): \(value)"
        }
    }

    var cells: [Cell]
    /// The numbers are not live, so they carry the badge (DESIGN.md §15).
    var isSample = false

    @Environment(\.dynamicTypeSize) private var typeSize

    var body: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s2) {
            if typeSize.isAccessibilitySize {
                // Three columns cannot hold the words at these sizes: one cell per line, number
                // first, and nothing is truncated (DESIGN.md §14).
                VStack(alignment: .leading, spacing: ShieldSpace.s1) {
                    ForEach(cells) { cell in
                        HStack(alignment: .firstTextBaseline, spacing: ShieldSpace.s3) {
                            number(cell)
                            words(cell)
                        }
                        .accessibilityElement(children: .ignore)
                        .accessibilityLabel(cell.accessibilityText)
                    }
                }
            } else {
                HStack(alignment: .top, spacing: ShieldSpace.s3) {
                    ForEach(cells) { cell in
                        VStack(alignment: .leading, spacing: 0) {
                            number(cell)
                            words(cell)
                        }
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .accessibilityElement(children: .ignore)
                        .accessibilityLabel(cell.accessibilityText)
                    }
                }
            }
            if isSample { SampleDataBadge() }
        }
        .animation(ShieldMotion.standard, value: cells)
    }

    /// Never truncated or scaled down (DESIGN.md §14); the words wrap instead.
    private func number(_ cell: Cell) -> some View {
        Text(cell.text)
            .font(ShieldFont.statHero)
            .tracking(ShieldTracking.statHero)
            .foregroundStyle(cell.value == nil ? Color.muted : Color.ink)
            .contentTransition(.numericText())
            .fixedSize()
    }

    private func words(_ cell: Cell) -> some View {
        Text(cell.label.uppercased())
            .font(ShieldFont.micro)
            .tracking(ShieldTracking.micro)
            .foregroundStyle(Color.muted)
            .fixedSize(horizontal: false, vertical: true)
    }
}

extension StatStrip.Cell {
    /// The three cells of the live map (DESIGN.md §10.1): observed events, high severity
    /// (severe + extreme) and official warnings, for one response of the events endpoint.
    ///
    /// Counts include the events inside clusters, so the strip agrees with the map at every
    /// zoom. A cluster carries a total and, usually, a count per severity, but nothing about
    /// kind. Where a cluster cannot say, the number is a lower bound ("12+"), or a dash when
    /// nothing at all is known. It is never a guess.
    static func liveMap(_ snapshot: MapSnapshot?) -> [StatStrip.Cell] {
        guard let snapshot else {
            return [
                .init(value: nil, label: "Observed events"),
                .init(value: nil, label: "High severity"),
                .init(value: nil, label: "Official warnings"),
            ]
        }

        var high = snapshot.events.count { $0.severity >= .severe }
        var highIsLowerBound = false
        for cluster in snapshot.clusters {
            if cluster.severities.isEmpty {
                // No split from the server: all we know is whether its worst member is high.
                if cluster.maxSeverity >= .severe {
                    high += 1
                    highIsLowerBound = cluster.count > 1 || highIsLowerBound
                }
            } else {
                high += (cluster.severities[.severe] ?? 0) + (cluster.severities[.extreme] ?? 0)
            }
        }

        let warnings = snapshot.events.count { $0.kind == .warning }
        let clustered = !snapshot.clusters.isEmpty
        // More matched than the server sent, so every count is short.
        let short = snapshot.truncated

        return [
            .init(value: snapshot.totalCount, isLowerBound: short, label: "Observed events"),
            .init(value: high, isLowerBound: highIsLowerBound || short, label: "High severity"),
            .init(value: clustered && warnings == 0 ? nil : warnings,
                  isLowerBound: clustered || short, label: "Official warnings"),
        ]
    }
}

#Preview("StatStrip", traits: .sizeThatFitsLayout) {
    VStack(alignment: .leading, spacing: ShieldSpace.s5) {
        StatStrip(cells: [
            .init(value: 128, label: "Observed events"),
            .init(value: 14, label: "High severity"),
            .init(value: 3, isLowerBound: true, label: "Official warnings"),
        ], isSample: true)
        StatStrip(cells: StatStrip.Cell.liveMap(nil), isSample: true)
    }
    .padding(ShieldSpace.s5)
    .frame(width: 360)
    .background(Color.panel)
}
