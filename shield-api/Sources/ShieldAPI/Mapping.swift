import Foundation
import OpenAPIRuntime
import ShieldCore

/// Generated wire types → `ShieldCore` domain types. The UI only ever sees the latter.
enum MappingError: Error, Equatable {
    case invalidID(String)
    case invalidURL(String)
}

extension HazardEvent {
    init(_ e: Components.Schemas.Event) throws {
        guard let id = UUID(uuidString: e.id) else { throw MappingError.invalidID(e.id) }
        self.init(
            id: id,
            source: e.source,
            type: HazardType(e._type),
            kind: EventKind(e.kind),
            status: EventStatus(e.status),
            severity: Severity(e.severity),
            sourceSeverity: e.sourceSeverity,
            severityProvisional: e.severityProvisional ?? false,
            title: e.title,
            summary: e.summary,
            centroid: Coordinate(latitude: e.centroid.latitude, longitude: e.centroid.longitude),
            countries: e.countries.map(EventCountry.init),
            startedAt: e.startedAt,
            observedAt: e.observedAt,
            updatedAt: e.updatedAt,
            endedAt: e.endedAt,
            fetchedAt: e.fetchedAt,
            url: e.url.flatMap(URL.init(string:)),
            attrs: (try? JSONValue.object(e.attrs.additionalProperties)) ?? [:]
        )
    }
}

extension Source {
    init(_ s: Components.Schemas.Source) throws {
        guard let url = URL(string: s.url) else { throw MappingError.invalidURL(s.url) }
        self.init(
            id: s.id,
            name: s.name,
            url: url,
            license: s.license,
            attribution: s.attribution,
            pollIntervalSeconds: s.pollIntervalSeconds,
            enabled: s.enabled,
            lastSuccessAt: s.lastSuccessAt,
            lastErrorAt: s.lastErrorAt,
            stale: s.stale
        )
    }
}

extension EventCountry {
    init(_ c: Components.Schemas.EventCountry) {
        self.init(iso2: c.iso2, relation: c.relation == .nearby ? .nearby : .within, distanceKm: c.distanceKm)
    }
}

// The wire enums carry the same raw values as the domain enums (a backend Go test pins them).
extension HazardType {
    init(_ t: Components.Schemas.EventType) { self = HazardType(rawValue: t.rawValue) ?? .other }

    /// `type` is open on the wire (ADR 18): a value this build doesn't know becomes `.other`.
    init(_ t: Components.Schemas.OpenEventType) {
        if let known = t.value1 { self.init(known) }
        else { self = t.value2.flatMap(HazardType.init(rawValue:)) ?? .other }
    }
}

extension EventKind {
    // An unknown kind can't reach here (the wire enum is closed); `.observed` is unreachable.
    init(_ k: Components.Schemas.EventKind) { self = EventKind(rawValue: k.rawValue) ?? .observed }
}

extension EventStatus {
    init(_ s: Components.Schemas.EventStatus) { self = s == .resolved ? .resolved : .active }
}

extension Severity {
    init(_ s: Components.Schemas.Severity) { self = Severity(rawValue: s.rawValue) ?? .minor }
}

extension JSONValue {
    /// Free-form `attrs` → `[String: JSONValue]`, through JSON so every nested shape survives.
    static func object(_ container: OpenAPIObjectContainer) throws -> [String: JSONValue] {
        let data = try JSONEncoder().encode(container)
        return try JSONDecoder().decode([String: JSONValue].self, from: data)
    }
}
