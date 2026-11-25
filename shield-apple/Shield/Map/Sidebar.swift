import ShieldCore
import SwiftUI

// MARK: - Navigation (⌘1–⌘5)

/// What the Go menu drives: the window in front's sidebar. Saved regions and Alerts (⌘3, ⌘4)
/// wait for Phase 3, so the menu leaves them out and their numbers stay free.
struct SidebarNavigation {
    var section: SidebarSection
    var go: (SidebarSection) -> Void
}

extension FocusedValues {
    @Entry var sidebarNavigation: SidebarNavigation?
}

/// DESIGN.md §10 shortcuts, in a menu so they are discoverable and work wherever focus is.
struct SidebarCommands: Commands {
    @FocusedValue(\.sidebarNavigation) private var navigation

    var body: some Commands {
        CommandMenu("Go") {
            ForEach(SidebarSection.shown) { section in
                Button(section.title) { navigation?.go(section) }
                    .keyboardShortcut(KeyEquivalent(section.shortcut), modifiers: .command)
                    .disabled(navigation == nil)
            }
        }
    }
}

// MARK: - Rows

/// Monitor and Countries at the top of the sidebar.
struct SidebarSectionRow: View {
    let section: SidebarSection
    let isSelected: Bool
    var action: () -> Void

    var body: some View {
        Button(action: action) {
            Label(section.title, systemImage: section.symbol)
                .font(ShieldFont.body.weight(.semibold))
                .foregroundStyle(isSelected ? Color.ink : Color.muted)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(.horizontal, ShieldSpace.s3)
                .frame(minHeight: 36)
                .background(isSelected ? Color.raised : Color.clear,
                            in: RoundedRectangle(cornerRadius: ShieldRadius.sm, style: .continuous))
                .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityAddTraits(isSelected ? .isSelected : [])
        .help("\(section.title) (⌘\(section.shortcut))")
    }
}

/// Sources, pinned at the bottom with the sources' overall health (DESIGN.md §10).
struct SourcesSidebarRow: View {
    let health: SourceHealth?
    var action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: ShieldSpace.s2) {
                Label(SidebarSection.sources.title, systemImage: SidebarSection.sources.symbol)
                    .font(ShieldFont.body.weight(.semibold))
                Spacer(minLength: ShieldSpace.s2)
                // No dot before the first list arrives: no health is not good health.
                if let health { HealthDot(health: health) }
            }
            .padding(.horizontal, ShieldSpace.s4)
            .frame(minHeight: 44)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .foregroundStyle(Color.ink)
        .help("Sources (⌘\(SidebarSection.sources.shortcut))")
    }
}

extension SidebarSection {
    var symbol: String {
        switch self {
        case .monitor: "map"
        case .countries: "globe"
        case .saved: "bookmark"
        case .alerts: "bell"
        case .sources: "antenna.radiowaves.left.and.right"
        }
    }
}

// MARK: - Countries

/// The Countries section: every country, active ones first, with a filter. Choosing one opens
/// its overview in the sidebar, as search does.
struct CountriesPane: View {
    let countries: [Country]
    /// The list failed to load, or hasn't yet.
    let failed: Bool
    var retry: () -> Void
    var open: (Country) -> Void

    @State private var filter = ""

    var body: some View {
        let directory = CountryDirectory(countries, filter: filter)
        VStack(spacing: 0) {
            // Styled like the Monitor's search field.
            HStack(spacing: ShieldSpace.s2) {
                Image(systemName: "line.3.horizontal.decrease")
                    .foregroundStyle(Color.muted)
                    .accessibilityHidden(true)
                TextField("Filter countries", text: $filter)
                    .textFieldStyle(.plain)
                    .autocorrectionDisabled()
                if !filter.isEmpty {
                    Button { filter = "" } label: {
                        Image(systemName: "xmark.circle.fill").foregroundStyle(Color.muted)
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel("Clear filter")
                }
            }
            .font(ShieldFont.body)
            .padding(.horizontal, ShieldSpace.s3)
            .frame(minHeight: 36)
            .background(Color.ink.opacity(ShieldAlpha.hairline), in: RoundedRectangle(cornerRadius: ShieldRadius.sm))
                .padding(.horizontal, ShieldSpace.s4)
                .padding(.vertical, ShieldSpace.s3)
            Rectangle().fill(Color.ink.opacity(ShieldAlpha.hairline)).frame(height: 1)
            if countries.isEmpty {
                note(failed ? "Couldn't load the countries." : "Loading countries…",
                     action: failed ? ("Retry", retry) : nil)
            } else if directory.isEmpty {
                note("No country matches “\(filter)”.", action: nil)
            } else {
                List {
                    if !directory.active.isEmpty {
                        Section("Active events") { ForEach(directory.active, content: row) }
                    }
                    if !directory.quiet.isEmpty {
                        Section("No active events") { ForEach(directory.quiet, content: row) }
                    }
                }
                .listStyle(.sidebar)
                .scrollContentBackground(.hidden)
            }
        }
    }

    private func row(_ country: Country) -> some View {
        Button { open(country) } label: {
            HStack(spacing: ShieldSpace.s2) {
                Text(country.name)
                    .font(ShieldFont.body)
                    .foregroundStyle(Color.ink)
                    .lineLimit(1)
                Spacer(minLength: ShieldSpace.s2)
                if let severity = country.maxSeverity, country.activeEvents > 0 {
                    Image(systemName: severity.sfSymbol)
                        .foregroundStyle(severity.color)
                        .accessibilityHidden(true)
                    Text("\(country.activeEvents)")
                        .font(ShieldFont.meta.monospacedDigit())
                        .foregroundStyle(Color.muted)
                }
            }
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel(label(country))
    }

    /// Offshore events within 250 km count too (ADR 4), so "in or near".
    private func label(_ country: Country) -> String {
        guard country.activeEvents > 0 else { return "\(country.name), no active events" }
        let events = country.activeEvents == 1 ? "1 active event" : "\(country.activeEvents) active events"
        let worst = country.maxSeverity.map { ", up to \($0.label.lowercased())" } ?? ""
        return "\(country.name), \(events) in or near it\(worst)"
    }

    private func note(_ text: String, action: (String, () -> Void)?) -> some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s2) {
            Text(text)
                .font(ShieldFont.body)
                .foregroundStyle(Color.muted)
            if let action {
                Button(action.0, action: action.1)
                    .font(ShieldFont.body.weight(.semibold))
                    .buttonStyle(.plain)
                    .foregroundStyle(Color.accent)
            }
        }
        .padding(.horizontal, ShieldSpace.s5)
        .padding(.vertical, ShieldSpace.s4)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
    }
}
