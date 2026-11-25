import Foundation

/// The macOS and iPad sidebar (DESIGN.md §10): Monitor · Countries · Saved regions · Alerts,
/// with Sources pinned at the bottom, on ⌘1–⌘5 in that order.
public enum SidebarSection: Int, CaseIterable, Sendable, Identifiable {
    case monitor = 1
    case countries
    case saved
    case alerts
    case sources

    public var id: Int { rawValue }

    public var title: String {
        switch self {
        case .monitor: "Monitor"
        case .countries: "Countries"
        case .saved: "Saved regions"
        case .alerts: "Alerts"
        case .sources: "Sources"
        }
    }

    /// The digit for ⌘1–⌘5. Saved regions and Alerts keep theirs while they wait.
    public var shortcut: Character { Character(String(rawValue)) }

    /// Saved regions and Alerts are Phase 3 and gated (VIEWS.md): nothing is built behind them.
    public var isAvailable: Bool { self != .saved && self != .alerts }

    /// The sections the sidebar lists, in order.
    public static let shown: [SidebarSection] = allCases.filter(\.isAvailable)
}

extension Source {
    /// The Sources row's dot: the worst health among the listed sources, or nil before the first
    /// list arrives.
    public static func overallHealth(_ sources: [Source], now: Date = .now) -> SourceHealth? {
        let listed = listed(sources)
        guard !listed.isEmpty else { return nil }
        let healths = listed.map { $0.health(now: now) }
        if healths.contains(.stale) { return .stale }
        if healths.contains(.failing) { return .failing }
        return .ok
    }
}

/// The Countries section (DESIGN.md §10): countries with active events first, most severe then
/// busiest, then the rest by name. A filter keeps every match, ranked like the search field's.
public struct CountryDirectory: Sendable, Hashable {
    public var active: [Country]
    public var quiet: [Country]

    public var isEmpty: Bool { active.isEmpty && quiet.isEmpty }

    public init(_ countries: [Country], filter: String) {
        let trimmed = filter.trimmingCharacters(in: .whitespacesAndNewlines)
        let shown = trimmed.isEmpty ? countries : Country.matching(trimmed, in: countries, limit: .max)
        active = shown.filter { $0.activeEvents > 0 }.sorted { a, b in
            let ra = a.maxSeverity?.rank ?? -1, rb = b.maxSeverity?.rank ?? -1
            if ra != rb { return ra > rb }
            if a.activeEvents != b.activeEvents { return a.activeEvents > b.activeEvents }
            return a.name.localizedStandardCompare(b.name) == .orderedAscending
        }
        quiet = shown.filter { $0.activeEvents == 0 }
            .sorted { $0.name.localizedStandardCompare($1.name) == .orderedAscending }
    }
}
