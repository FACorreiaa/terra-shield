import Foundation

/// A point on Earth, WGS 84.
public struct Coordinate: Codable, Sendable, Hashable {
    public var latitude: Double
    public var longitude: Double

    public init(latitude: Double, longitude: Double) {
        self.latitude = latitude
        self.longitude = longitude
    }
}

/// A country an event touches. `within` comes from the geometry intersecting the country;
/// `nearby` means offshore within 250 km, and the UI must not say "in" for it (DESIGN.md §15).
public struct EventCountry: Codable, Sendable, Hashable {
    public enum Relation: String, Codable, Sendable, Hashable {
        case within
        case nearby
    }

    public var iso2: String
    public var relation: Relation
    public var distanceKm: Double?

    public init(iso2: String, relation: Relation, distanceKm: Double? = nil) {
        self.iso2 = iso2
        self.relation = relation
        self.distanceKm = distanceKm
    }

    /// "Chile", or "Off the coast of Chile, 38 km". `name` is the display name for `iso2`.
    public func phrase(name: String) -> String {
        switch relation {
        case .within:
            return name
        case .nearby:
            guard let distanceKm else { return "Off the coast of \(name)" }
            return "Off the coast of \(name), \(Int(distanceKm.rounded())) km"
        }
    }
}

/// One normalized hazard record. The shape follows the backend's stored event model; field names
/// become final with the API spec (BACKLOG S2.1), when generated `shield-api` types replace this
/// through `HazardEventProvider`.
public struct HazardEvent: Codable, Sendable, Hashable, Identifiable {
    public var id: UUID
    /// Source id, e.g. "usgs". Resolve display data through `Source`.
    public var source: String
    public var type: HazardType
    public var kind: EventKind
    public var status: EventStatus
    public var severity: Severity
    /// The source's own severity, verbatim ("M5.4 mww, PAGER green", "Level 3 · Orange").
    public var sourceSeverity: String?
    /// True when Shield derived `severity` from a weak signal: every EONET event (burned area,
    /// or no scale at all and defaulted to minor).
    /// The UI then says "Provisional" (DESIGN.md §5).
    public var severityProvisional: Bool
    public var title: String
    public var summary: String?
    public var centroid: Coordinate
    public var countries: [EventCountry]
    public var startedAt: Date?
    public var observedAt: Date
    public var updatedAt: Date
    public var endedAt: Date?
    public var fetchedAt: Date
    public var url: URL?
    /// Per-type attributes, kept loose until the spec fixes them. Typed views below.
    public var attrs: [String: JSONValue]

    public init(
        id: UUID,
        source: String,
        type: HazardType,
        kind: EventKind,
        status: EventStatus = .active,
        severity: Severity,
        sourceSeverity: String?,
        severityProvisional: Bool = false,
        title: String,
        summary: String? = nil,
        centroid: Coordinate,
        countries: [EventCountry] = [],
        startedAt: Date? = nil,
        observedAt: Date,
        updatedAt: Date,
        endedAt: Date? = nil,
        fetchedAt: Date,
        url: URL? = nil,
        attrs: [String: JSONValue] = [:]
    ) {
        self.id = id
        self.source = source
        self.type = type
        self.kind = kind
        self.status = status
        self.severity = severity
        self.sourceSeverity = sourceSeverity
        self.severityProvisional = severityProvisional
        self.title = title
        self.summary = summary
        self.centroid = centroid
        self.countries = countries
        self.startedAt = startedAt
        self.observedAt = observedAt
        self.updatedAt = updatedAt
        self.endedAt = endedAt
        self.fetchedAt = fetchedAt
        self.url = url
        self.attrs = attrs
    }
}

/// Earthquake attributes (`attrs` when `type == .earthquake`).
public struct EarthquakeAttributes: Sendable, Hashable {
    public var magnitude: Double?
    public var magnitudeType: String?
    public var depthKm: Double?
    public var pagerAlert: String?
    public var tsunami: Bool
    public var feltReports: Int?
    public var reviewStatus: String?
}

extension HazardEvent {
    private enum CodingKeys: String, CodingKey {
        case id, source, type, kind, status, severity, sourceSeverity, severityProvisional, title, summary
        case centroid, countries, startedAt, observedAt, updatedAt, endedAt, fetchedAt, url, attrs
    }

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        let attrs = try c.decodeIfPresent([String: JSONValue].self, forKey: .attrs) ?? [:]
        // Until S2.1 the backend keeps the flag in attrs; the spec moves it to the top level.
        let provisional = try c.decodeIfPresent(Bool.self, forKey: .severityProvisional)
            ?? attrs["severityProvisional"]?.boolValue ?? false
        self.init(
            id: try c.decode(UUID.self, forKey: .id),
            source: try c.decode(String.self, forKey: .source),
            type: try c.decode(HazardType.self, forKey: .type),
            kind: try c.decode(EventKind.self, forKey: .kind),
            status: try c.decodeIfPresent(EventStatus.self, forKey: .status) ?? .active,
            severity: try c.decode(Severity.self, forKey: .severity),
            sourceSeverity: try c.decodeIfPresent(String.self, forKey: .sourceSeverity),
            severityProvisional: provisional,
            title: try c.decode(String.self, forKey: .title),
            summary: try c.decodeIfPresent(String.self, forKey: .summary),
            centroid: try c.decode(Coordinate.self, forKey: .centroid),
            countries: try c.decodeIfPresent([EventCountry].self, forKey: .countries) ?? [],
            startedAt: try c.decodeIfPresent(Date.self, forKey: .startedAt),
            observedAt: try c.decode(Date.self, forKey: .observedAt),
            updatedAt: try c.decode(Date.self, forKey: .updatedAt),
            endedAt: try c.decodeIfPresent(Date.self, forKey: .endedAt),
            fetchedAt: try c.decode(Date.self, forKey: .fetchedAt),
            url: try c.decodeIfPresent(URL.self, forKey: .url),
            attrs: attrs
        )
    }

    public var earthquake: EarthquakeAttributes? {
        guard type == .earthquake else { return nil }
        return EarthquakeAttributes(
            magnitude: attrs["mag"]?.doubleValue,
            magnitudeType: attrs["magType"]?.stringValue,
            depthKm: attrs["depthKm"]?.doubleValue,
            pagerAlert: attrs["pagerAlert"]?.stringValue,
            tsunami: attrs["tsunami"]?.boolValue ?? false,
            feltReports: attrs["feltReports"]?.doubleValue.map { Int($0) },
            reviewStatus: attrs["reviewStatus"]?.stringValue
        )
    }
}

extension Sequence where Element == HazardEvent {
    /// Display order everywhere (DESIGN.md §4): most severe first, then most recently updated.
    public func sortedForDisplay() -> [HazardEvent] {
        sorted { a, b in
            if a.severity != b.severity { return a.severity > b.severity }
            return a.updatedAt > b.updatedAt
        }
    }
}
