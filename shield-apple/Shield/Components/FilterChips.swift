import ShieldCore
import SwiftUI

/// The filters on the map (VIEWS.md catalog; DESIGN.md §10.6): a Filters button that opens the
/// sheet, then one removable chip per filter that differs from the default. Heat detections
/// have their own map control, so they are not a chip.
struct FilterChips: View {
    var query: EventQuery
    var openFilters: () -> Void
    var remove: (ActiveFilter) -> Void

    var body: some View {
        ScrollView(.horizontal) {
            HStack(spacing: ShieldSpace.s2) {
                Button(action: openFilters) {
                    HStack(spacing: ShieldSpace.s1) {
                        Image(systemName: "line.3.horizontal.decrease")
                            .accessibilityHidden(true)
                        Text("Filters")
                        if !query.activeFilters.isEmpty {
                            Text("\(query.activeFilters.count)")
                                .font(ShieldFont.micro.monospacedDigit())
                                .foregroundStyle(Color.canvas)
                                .padding(.horizontal, 5)
                                .background(Color.accent, in: Capsule())
                        }
                    }
                    .chip(isActive: !query.isDefault)
                }
                .buttonStyle(.plain)
                .accessibilityLabel(query.isDefault ? "Filters" : "Filters, \(query.activeFilters.count) on")
                .help("Filter the map")

                ForEach(query.activeFilters, id: \.self) { filter in
                    Button { remove(filter) } label: {
                        HStack(spacing: ShieldSpace.s1) {
                            Text(query.label(for: filter))
                            Image(systemName: "xmark")
                                .font(.system(size: 9, weight: .bold))
                                .accessibilityHidden(true)
                        }
                        .chip(isActive: true)
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel("Remove filter: \(query.label(for: filter))")
                    .transition(.scale(scale: 0.9).combined(with: .opacity))
                }
            }
            .padding(.vertical, 1) // keeps the hairline borders from clipping
        }
        .scrollIndicators(.hidden)
        // Clipped where the map controls begin; the last 16 pt fade so a cut chip reads as
        // "scroll for more", not as broken.
        .mask {
            HStack(spacing: 0) {
                Color.black
                LinearGradient(colors: [.black, .clear], startPoint: .leading, endPoint: .trailing)
                    .frame(width: 16)
            }
        }
        .animation(ShieldMotion.standard, value: query.activeFilters)
    }
}

private extension View {
    /// The map's floating material, one row tall; accent hairline when it carries a filter.
    func chip(isActive: Bool) -> some View {
        self
            .font(ShieldFont.meta.weight(.semibold))
            .foregroundStyle(Color.ink)
            .padding(.horizontal, ShieldSpace.s3)
            .frame(minHeight: 32)
            .background(.ultraThinMaterial, in: Capsule())
            .overlay(Capsule().strokeBorder(isActive ? Color.accent.opacity(0.6)
                                                     : Color.ink.opacity(ShieldAlpha.hairline)))
            .contentShape(Capsule())
    }
}

#Preview("FilterChips", traits: .sizeThatFitsLayout) {
    VStack(alignment: .leading, spacing: ShieldSpace.s4) {
        FilterChips(query: EventQuery(), openFilters: {}, remove: { _ in })
        FilterChips(query: {
            var q = EventQuery()
            q.minimumSeverity = .severe
            q.kinds = [.warning]
            q.window = 7 * 86400
            return q
        }(), openFilters: {}, remove: { _ in })
    }
    .padding(ShieldSpace.s5)
    .frame(width: 390)
    .background(Color.canvas)
}
