import ShieldCore
import SwiftUI

/// A country's active events by hazard type and severity (DESIGN.md §10.2): rows are the types
/// present, columns minor → extreme. A cell with events is tinted by its severity and filters the
/// list when tapped; the chosen cell gets an accent outline.
///
/// Columns carry the severity's shape and its name, never colour alone (§4, §14). Where a type's
/// name and four cells can't share a line (accessibility sizes, the Mac sidebar), each type takes
/// two lines, its name above its cells.
struct TypeSeverityMatrix: View {
    let rows: [MatrixRow]
    @Binding var selection: MatrixCell?

    @Environment(\.dynamicTypeSize) private var typeSize
    /// The width offered, measured: the grid only where a type's name and four cells fit.
    @State private var width: CGFloat = 0

    /// Below this, type names would squeeze the cells (the Mac sidebar is 220–260 pt).
    private static let gridMinimumWidth: CGFloat = 320

    var body: some View {
        Group {
            if typeSize.isAccessibilitySize || width < Self.gridMinimumWidth {
                stacked
            } else {
                grid
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .onGeometryChange(for: CGFloat.self) { $0.size.width } action: { width = $0 }
    }

    /// One line per type. The names keep their full width, so the grid only fits where they do.
    private var grid: some View {
        Grid(horizontalSpacing: ShieldSpace.s1, verticalSpacing: ShieldSpace.s1) {
            GridRow {
                Color.clear.gridCellUnsizedAxes([.horizontal, .vertical])
                ForEach(Severity.allCases, id: \.self) { header($0) }
            }
            ForEach(rows) { row in
                GridRow {
                    typeLabel(row.type).gridColumnAlignment(.leading)
                    ForEach(Severity.allCases, id: \.self) { cell(row, $0) }
                }
            }
        }
    }

    private var stacked: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s3) {
            if !typeSize.isAccessibilitySize {
                HStack(spacing: ShieldSpace.s1) {
                    ForEach(Severity.allCases, id: \.self) { header($0) }
                }
            }
            ForEach(rows) { row in
                VStack(alignment: .leading, spacing: ShieldSpace.s1) {
                    typeLabel(row.type)
                    HStack(spacing: ShieldSpace.s1) {
                        ForEach(Severity.allCases, id: \.self) { cell(row, $0) }
                    }
                }
            }
        }
    }

    private func header(_ severity: Severity) -> some View {
        VStack(spacing: 2) {
            Image(systemName: severity.sfSymbol)
                .font(.system(size: 11, weight: .bold))
                .foregroundStyle(severity.color)
            // The full word where it fits, its first three letters where it doesn't (Mac sidebar).
            ViewThatFits(in: .horizontal) {
                Text(severity.label)
                Text(severity.label.prefix(3))
            }
            .font(ShieldFont.micro)
            .foregroundStyle(Color.muted)
            .lineLimit(1)
        }
        .frame(maxWidth: .infinity)
        .accessibilityHidden(true) // each cell says its severity
    }

    private func typeLabel(_ type: HazardType) -> some View {
        Label {
            Text(type.label).lineLimit(2)
        } icon: {
            Image(systemName: type.sfSymbol).foregroundStyle(Color.muted)
        }
        .font(ShieldFont.meta)
        .foregroundStyle(Color.ink)
        .labelStyle(.titleAndIcon)
        .accessibilityHidden(true)
    }

    @ViewBuilder
    private func cell(_ row: MatrixRow, _ severity: Severity) -> some View {
        let count = row.counts[severity] ?? 0
        let target = MatrixCell(type: row.type, severity: severity)
        let chosen = selection == target
        let text = Text(count == 0 ? "–" : "\(count)")
            .font(ShieldFont.meta.weight(count == 0 ? .regular : .semibold))
            .foregroundStyle(count == 0 ? Color.muted : Color.ink)
            .frame(maxWidth: .infinity, minHeight: 32)
            .background {
                // `raised` is opaque, so the severity tint sits over it.
                RoundedRectangle(cornerRadius: ShieldRadius.sm).fill(Color.raised)
                    .overlay(RoundedRectangle(cornerRadius: ShieldRadius.sm)
                        .fill(count == 0 ? Color.clear : severity.color.opacity(ShieldAlpha.matrixTint)))
            }
            .overlay(RoundedRectangle(cornerRadius: ShieldRadius.sm)
                .strokeBorder(chosen ? Color.accent : Color.ink.opacity(ShieldAlpha.hairline), lineWidth: chosen ? 2 : 1))
        if count == 0 {
            text
                .accessibilityLabel("\(row.type.label), \(severity.label): none")
        } else {
            Button { selection = chosen ? nil : target } label: {
                text.contentShape(RoundedRectangle(cornerRadius: ShieldRadius.sm))
            }
            .buttonStyle(.plain)
            .accessibilityLabel("\(row.type.label), \(severity.label): \(count)")
            .accessibilityAddTraits(chosen ? [.isSelected] : [])
            .accessibilityHint(chosen ? "Shows every event again" : "Lists only these events")
        }
    }
}

#Preview("TypeSeverityMatrix", traits: .sizeThatFitsLayout) {
    @Previewable @State var selection: MatrixCell? = MatrixCell(type: .wind, severity: .severe)
    TypeSeverityMatrix(rows: [
        MatrixRow(type: .wind, counts: [.severe: 12, .moderate: 43]),
        MatrixRow(type: .storm, counts: [.moderate: 13]),
        MatrixRow(type: .flood, counts: [.minor: 1]),
    ], selection: $selection)
    .padding(ShieldSpace.s5)
    .frame(width: 360)
    .background(Color.panel)
}
