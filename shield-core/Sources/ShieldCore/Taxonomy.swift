import Foundation

/// The backend's `type` enum (`backend/db/migrations/00001_init.sql`). Raw values are the wire values.
public enum HazardType: String, Codable, Sendable, CaseIterable, Hashable {
    case earthquake
    case wildfire
    case flood
    case storm
    case tropicalCyclone = "tropical_cyclone"
    case volcano
    case drought
    case heat
    case cold
    case heavyRain = "heavy_rain"
    case wind
    case snowIce = "snow_ice"
    case tsunami
    case other
    /// An armed-conflict event (UCDP, ADR 27): its own layer, never a hazard. The hazard filters
    /// use `hazards`; only the conflicts layer asks for this type.
    case conflict

    /// An unknown future value decodes as `.other` instead of failing the whole payload.
    public init(from decoder: any Decoder) throws {
        let raw = try decoder.singleValueContainer().decode(String.self)
        self = HazardType(rawValue: raw) ?? .other
    }
}

/// Shield's derived, CAP-aligned severity (DECISIONS #2). The source's own value is
/// `HazardEvent.sourceSeverity` and is never merged into this.
public enum Severity: String, Codable, Sendable, CaseIterable, Hashable, Comparable {
    case minor
    case moderate
    case severe
    case extreme

    public var rank: Int {
        switch self {
        case .minor: 0
        case .moderate: 1
        case .severe: 2
        case .extreme: 3
        }
    }

    public var label: String {
        switch self {
        case .minor: "Minor"
        case .moderate: "Moderate"
        case .severe: "Severe"
        case .extreme: "Extreme"
        }
    }

    public static func < (lhs: Severity, rhs: Severity) -> Bool { lhs.rank < rhs.rank }
}

/// What sort of statement a record is (DECISIONS #3). The words are fixed by DESIGN.md §2.
public enum EventKind: String, Codable, Sendable, CaseIterable, Hashable {
    case observed
    case warning
    case forecast
    case detection

    public var label: String {
        switch self {
        case .observed: "Observed"
        case .warning: "Official warning"
        case .forecast: "Forecast"
        case .detection: "Satellite heat detection"
        }
    }
}

public enum EventStatus: String, Codable, Sendable, Hashable {
    case active
    case resolved
}

extension HazardType {
    public var label: String {
        switch self {
        case .earthquake: "Earthquake"
        case .wildfire: "Wildfire"
        case .flood: "Flood"
        case .storm: "Storm"
        case .tropicalCyclone: "Tropical cyclone"
        case .volcano: "Volcano"
        case .drought: "Drought"
        case .heat: "Extreme heat"
        case .cold: "Extreme cold"
        case .heavyRain: "Heavy rain"
        case .wind: "Wind"
        case .snowIce: "Snow and ice"
        case .tsunami: "Tsunami"
        case .other: "Other hazard"
        case .conflict: "Armed conflict"
        }
    }

    /// Every type but `conflict`: what the hazard filters offer and "all types" means.
    public static let hazards: [HazardType] = allCases.filter { $0 != .conflict }
}
