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
        /// The words in the compact strip, after the number: singular and plural.
        var unit: (one: String, many: String)? = nil
        /// Tapping the cell (full strip only) filters the map to its set (DESIGN.md §10.1).
        var filter: QuickFilter? = nil

        var id: String { label }

        static func == (a: Self, b: Self) -> Bool {
            a.value == b.value && a.isLowerBound == b.isLowerBound && a.label == b.label && a.filter == b.filter
        }

        func hash(into hasher: inout Hasher) {
            hasher.combine(value); hasher.combine(isLowerBound); hasher.combine(label)
        }

        /// "4 events", "1 warning", "– warnings".
        var compactWords: String {
            let unit = unit ?? (label.lowercased(), label.lowercased())
            return value == 1 && !isLowerBound ? unit.one : unit.many
        }

        var text: String {
            guard let value else { return "–" }
            return isLowerBound ? "\(value)+" : "\(value)"
        }

        var accessibilityText: String {
            guard let value else { return "\(label): not counted at this zoom" }
            return isLowerBound ? "\(label): at least \(value)" : "\(label): \(value)"
        }
    }

    enum Style {
        /// Big numbers, words below (DESIGN.md §10.1).
        case full
        /// One line, "4 events · 1 high severity · 1 warning": the iPhone peek, under the search
        /// field (owner, 2026-11-19).
        case compact
    }

    var cells: [Cell]
    var style: Style = .full
    /// The current filters, to show which cell's filter is on; with `tap`, cells with a
    /// `filter` become buttons.
    var query: EventQuery? = nil
    var tap: ((QuickFilter) -> Void)? = nil
    /// The numbers are not live, so they carry the badge (DESIGN.md §15).
    var isSample = false

    @Environment(\.dynamicTypeSize) private var typeSize

    var body: some View {
        switch style {
        case .full: full
        case .compact: compact
        }
    }

    private var compact: some View {
        Group {
            if typeSize.isAccessibilitySize {
                // One cell per line, so a number never wraps away from its words (§14).
                VStack(alignment: .leading, spacing: ShieldSpace.s1) {
                    ForEach(cells) { Text(compactText($0)) }
                }
            } else {
                Text(cells.enumerated().reduce(into: AttributedString()) { line, item in
                    if item.offset > 0 {
                        var dot = AttributedString("  ·  ")
                        dot.foregroundColor = Color.muted
                        line += dot
                    }
                    line += compactText(item.element)
                })
            }
        }
        .font(ShieldFont.body)
        .fixedSize(horizontal: false, vertical: true)
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(cells.map(\.accessibilityText).joined(separator: ", "))
        .animation(ShieldMotion.standard, value: cells)
    }

    /// "4 events": the number in ink and mono, the words muted.
    private func compactText(_ cell: Cell) -> AttributedString {
        var number = AttributedString(cell.text)
        number.font = ShieldFont.body.weight(.semibold).monospacedDigit()
        number.foregroundColor = cell.value == nil ? Color.muted : Color.ink
        var words = AttributedString("\u{00A0}" + cell.compactWords)
        words.foregroundColor = Color.muted
        return number + words
    }

    private var full: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s2) {
            if typeSize.isAccessibilitySize {
                // Three columns cannot hold the words at these sizes: one cell per line, number
                // first, and nothing is truncated (DESIGN.md §14).
                VStack(alignment: .leading, spacing: ShieldSpace.s1) {
                    ForEach(cells) { cell in
                        tappable(cell) {
                            HStack(alignment: .firstTextBaseline, spacing: ShieldSpace.s3) {
                                number(cell)
                                words(cell)
                            }
                        }
                    }
                }
            } else {
                HStack(alignment: .top, spacing: ShieldSpace.s3) {
                    ForEach(cells) { cell in
                        tappable(cell) {
                            VStack(alignment: .leading, spacing: 0) {
                                number(cell)
                                words(cell)
                            }
                        }
                        .frame(maxWidth: .infinity, alignment: .leading)
                    }
                }
            }
            if isSample { SampleDataBadge() }
        }
        .animation(ShieldMotion.standard, value: cells)
    }

    /// A cell with a filter is a button when the strip can filter; the filter that is on gets an
    /// accent underline (shape, not only colour) and the selected trait.
    @ViewBuilder
    private func tappable(_ cell: Cell, @ViewBuilder content: () -> some View) -> some View {
        if let filter = cell.filter, let tap {
            let on = query?.isOn(filter) ?? false
            Button { tap(filter) } label: {
                content()
                    .padding(.bottom, ShieldSpace.s1)
                    .overlay(alignment: .bottomLeading) {
                        Capsule().fill(Color.accent).frame(width: 24, height: 2).opacity(on ? 1 : 0)
                    }
                    .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityElement(children: .ignore)
            .accessibilityLabel(cell.accessibilityText)
            .accessibilityAddTraits(on ? [.isButton, .isSelected] : .isButton)
            .accessibilityHint(on ? "Shows every event again" : "Filters the map to these")
        } else {
            content()
                .accessibilityElement(children: .ignore)
                .accessibilityLabel(cell.accessibilityText)
        }
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
    ///
    /// The strip counts what is happening, so resolved events (shown when the filter includes
    /// them) never count. Clusters can't say how many of theirs are resolved, so while resolved
    /// events are included, the counts that need clusters show a dash.
    static func liveMap(_ snapshot: MapSnapshot?, includesResolved: Bool = false) -> [StatStrip.Cell] {
        guard let snapshot else {
            return [
                .init(value: nil, label: "Observed events", unit: units[0]),
                .init(value: nil, label: "High severity", unit: units[1]),
                .init(value: nil, label: "Official warnings", unit: units[2]),
            ]
        }

        let active = snapshot.events.filter { !$0.isResolved }
        let clustersMixStatuses = includesResolved && !snapshot.clusters.isEmpty
        var high = active.count { $0.severity >= .severe }
        var highIsLowerBound = false
        for cluster in snapshot.clusters where !clustersMixStatuses {
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

        let warnings = active.count { $0.kind == .warning }
        let clustered = !snapshot.clusters.isEmpty
        // More matched than the server sent, so every count is short.
        let short = snapshot.truncated

        return [
            .init(value: clustersMixStatuses ? nil : snapshot.totalCount - (snapshot.events.count - active.count),
                  isLowerBound: short, label: "Observed events", unit: units[0]),
            .init(value: clustersMixStatuses ? nil : high, isLowerBound: highIsLowerBound || short,
                  label: "High severity", unit: units[1],
                  filter: .highSeverity),
            .init(value: clustered && warnings == 0 ? nil : warnings,
                  isLowerBound: clustered || short, label: "Official warnings", unit: units[2],
                  filter: .warnings),
        ]
    }

    private static let units: [(one: String, many: String)] = [
        ("event", "events"), ("high severity", "high severity"), ("warning", "warnings"),
    ]
}

#Preview("StatStrip", traits: .sizeThatFitsLayout) {
    VStack(alignment: .leading, spacing: ShieldSpace.s5) {
        StatStrip(cells: [
            .init(value: 128, label: "Observed events"),
            .init(value: 14, label: "High severity"),
            .init(value: 3, isLowerBound: true, label: "Official warnings"),
        ], isSample: true)
        StatStrip(cells: StatStrip.Cell.liveMap(nil), isSample: true)
        StatStrip(cells: [
            .init(value: 128, label: "Observed events", unit: ("event", "events")),
            .init(value: 14, label: "High severity"),
            .init(value: 1, label: "Official warnings", unit: ("warning", "warnings")),
        ], style: .compact)
    }
    .padding(ShieldSpace.s5)
    .frame(width: 360)
    .background(Color.panel)
}
