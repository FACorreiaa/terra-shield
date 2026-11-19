import Foundation

// MARK: - Active filters (DESIGN.md §10.6: "Active filters show as removable chips")

/// One dimension of `EventQuery` that differs from the default, as a removable chip.
public enum ActiveFilter: Sendable, Hashable, CaseIterable {
    case types
    case severity
    case kinds
    case window
}

extension EventQuery {
    /// The time windows the filter sheet offers (DESIGN.md §10.6): 1 h · 6 h · 24 h · 7 d.
    public static let windows: [TimeInterval] = [3600, 6 * 3600, 24 * 3600, 7 * 86400]

    /// The kinds a query can hold. Satellite heat detections are the map's FIRMS layer
    /// (`LiveMapModel.showsDetections`), not events, so they are never part of `kinds`.
    public static let eventKinds: [EventKind] = [.observed, .warning, .forecast]

    private static let standard = EventQuery()

    /// No filter set: every type, minor and up, observed + warnings + forecasts, last 24 h.
    /// The country (S3.4) is a place, not a filter, and doesn't count.
    public var isDefault: Bool { activeFilters.isEmpty }

    /// The dimensions that differ from the default, in chip order.
    public var activeFilters: [ActiveFilter] {
        ActiveFilter.allCases.filter { differs($0) }
    }

    private func differs(_ filter: ActiveFilter) -> Bool {
        switch filter {
        case .types: types != Self.standard.types
        case .severity: minimumSeverity != Self.standard.minimumSeverity
        case .kinds: kinds != Self.standard.kinds
        case .window: window != Self.standard.window
        }
    }

    /// This query with one dimension back at its default (the chip's ×).
    public func removing(_ filter: ActiveFilter) -> EventQuery {
        var q = self
        switch filter {
        case .types: q.types = Self.standard.types
        case .severity: q.minimumSeverity = Self.standard.minimumSeverity
        case .kinds: q.kinds = Self.standard.kinds
        case .window: q.window = Self.standard.window
        }
        return q
    }

    /// The chip's words: "Earthquake", "Flood, Storm", "5 types", "Severe+", "Warnings only",
    /// "Observed + forecasts", "7 d".
    public func label(for filter: ActiveFilter) -> String {
        switch filter {
        case .types:
            let chosen = HazardType.allCases.filter(types.contains)
            return chosen.count <= 2 ? chosen.map(\.label).joined(separator: ", ") : "\(chosen.count) types"
        case .severity:
            return "\(minimumSeverity.label)+"
        case .kinds:
            let chosen = Self.eventKinds.filter(kinds.contains)
            if chosen.count == 1 { return "\(chosen[0].pluralLabel.capitalizedFirst) only" }
            return chosen.map(\.pluralLabel).joined(separator: " + ").capitalizedFirst
        case .window:
            return window.map(Self.windowLabel) ?? "Any time"
        }
    }

    /// "1 h", "6 h", "24 h", "7 d".
    public static func windowLabel(_ window: TimeInterval) -> String {
        let hours = Int((window / 3600).rounded())
        return hours >= 48 && hours % 24 == 0 ? "\(hours / 24) d" : "\(hours) h"
    }
}

extension EventKind {
    /// For filter words: "Warnings only", "Observed + forecasts".
    var pluralLabel: String {
        switch self {
        case .observed: "observed"
        case .warning: "warnings"
        case .forecast: "forecasts"
        case .detection: "heat detections"
        }
    }
}

private extension String {
    var capitalizedFirst: String { prefix(1).uppercased() + dropFirst() }
}

// MARK: - Persistence (BACKLOG S3.3: "persisted locally")

/// The filters a person chose, kept across launches.
public struct SavedFilters: Sendable, Equatable {
    public var query: EventQuery
    public var showsDetections: Bool

    public init(query: EventQuery = EventQuery(), showsDetections: Bool = false) {
        self.query = query
        self.showsDetections = showsDetections
    }
}

/// Saves filters in `UserDefaults`. Stores what is *excluded*, so a hazard type or kind added in
/// a later version shows by default instead of being missing from every saved set. Reads
/// leniently: unknown values are dropped, and a blob that can't be read gives the defaults.
public struct FilterStore: Sendable {
    /// `nil` for the standard defaults; tests pass their own suite.
    public let suiteName: String?
    public let key: String

    public init(suiteName: String? = nil, key: String = "shield.filters") {
        self.suiteName = suiteName
        self.key = key
    }

    private var defaults: UserDefaults { suiteName.flatMap(UserDefaults.init(suiteName:)) ?? .standard }

    private struct Blob: Codable {
        var version = 1
        var excludedTypes: [String]
        var minimumSeverity: String
        var excludedKinds: [String]
        /// Seconds; nil for any time.
        var window: Double?
        var showsDetections: Bool
    }

    public func load() -> SavedFilters {
        guard let data = defaults.data(forKey: key),
              let blob = try? JSONDecoder().decode(Blob.self, from: data),
              blob.version == 1 else { return SavedFilters() }
        let standard = EventQuery()
        var query = standard
        let excludedTypes = Set(blob.excludedTypes.compactMap(HazardType.init(rawValue:)))
        query.types = Set(HazardType.allCases).subtracting(excludedTypes)
        query.minimumSeverity = Severity(rawValue: blob.minimumSeverity) ?? standard.minimumSeverity
        let excludedKinds = Set(blob.excludedKinds.compactMap(EventKind.init(rawValue:)))
        query.kinds = Set(EventQuery.eventKinds).subtracting(excludedKinds)
        // Only a window the sheet offers: anything else (or none, which would send no `since`
        // at all) can't be shown or changed back, so it falls back to the default.
        if let window = blob.window, EventQuery.windows.contains(window) { query.window = window }
        // Never restore a query that shows nothing: the UI can't produce one.
        if query.types.isEmpty { query.types = standard.types }
        if query.kinds.isEmpty { query.kinds = standard.kinds }
        return SavedFilters(query: query, showsDetections: blob.showsDetections)
    }

    public func save(_ filters: SavedFilters) {
        let q = filters.query
        let blob = Blob(
            excludedTypes: HazardType.allCases.filter { !q.types.contains($0) }.map(\.rawValue),
            minimumSeverity: q.minimumSeverity.rawValue,
            excludedKinds: EventQuery.eventKinds.filter { !q.kinds.contains($0) }.map(\.rawValue),
            window: q.window,
            showsDetections: filters.showsDetections
        )
        if let data = try? JSONEncoder().encode(blob) { defaults.set(data, forKey: key) }
    }

    /// Forgets the saved filters (DEBUG screenshot runs).
    public func clear() { defaults.removeObject(forKey: key) }
}

// MARK: - Quick filters (DESIGN.md §10.1: "Tapping a cell filters the map to that set")

/// A stat-strip cell that filters when tapped, and unfilters when tapped again.
public enum QuickFilter: Sendable, Hashable {
    /// Severe and extreme.
    case highSeverity
    /// Official warnings only.
    case warnings
}

extension EventQuery {
    public func isOn(_ quick: QuickFilter) -> Bool {
        switch quick {
        case .highSeverity: minimumSeverity == .severe
        case .warnings: kinds == [.warning]
        }
    }

    /// On if it was off; back to that dimension's default if it was on.
    public func toggling(_ quick: QuickFilter) -> EventQuery {
        var q = self
        switch quick {
        case .highSeverity:
            q.minimumSeverity = isOn(quick) ? EventQuery().minimumSeverity : .severe
        case .warnings:
            q.kinds = isOn(quick) ? EventQuery().kinds : [.warning]
        }
        return q
    }
}
