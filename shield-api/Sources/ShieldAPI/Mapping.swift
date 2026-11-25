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
        coverage = s.coverage
        // A bad terms link only loses the link: the row falls back to the home page.
        termsUrl = s.termsUrl.flatMap(URL.init(string:))
        cadence = s.cadence
        onDemand = s.onDemand
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

extension Country {
    init(_ c: Components.Schemas.Country) {
        // [minLon, minLat, maxLon, maxLat]; anything else frames the world rather than failing
        // the whole list.
        let b = c.bbox.count == 4 ? c.bbox : [-180, -85, 180, 85]
        self.init(iso2: c.iso2, iso3: c.iso3, name: c.name, activeEvents: c.activeEvents,
                  maxSeverity: c.maxSeverity.map(Severity.init),
                  bbox: GeoBox(minLongitude: b[0], minLatitude: b[1], maxLongitude: b[2], maxLatitude: b[3]))
    }
}

extension CountrySummary {
    init(_ s: Components.Schemas.CountrySummary) throws {
        self.init(
            country: Country(s.country),
            counts: s.counts.map { SeverityCount(type: HazardType($0._type), severity: Severity($0.severity), count: $0.count) },
            coverage: Coverage(officialWarnings: s.coverage.officialWarnings, sources: s.coverage.sources),
            events: try s.events.map(HazardEvent.init),
            // A revision whose event id can't be read, or with nothing a person would notice, is
            // left out rather than failing the overview.
            recentChanges: s.recentChanges.compactMap { r in
                guard let id = UUID(uuidString: r.eventId) else { return nil }
                let changes = Change.parse((try? JSONValue.object(r.changes.additionalProperties)) ?? [:])
                guard !changes.isEmpty else { return nil }
                return RecentChange(eventID: id, title: r.title, at: r.at, changes: changes)
            },
            generatedAt: s.generatedAt
        )
    }
}

extension EventRevision {
    /// Nil for a revision with nothing a person would notice (only fields this build doesn't word).
    init?(_ r: Components.Schemas.Revision) {
        let changes = Change.parse((try? JSONValue.object(r.changes.additionalProperties)) ?? [:])
        guard !changes.isEmpty else { return nil }
        self.init(at: r.at, changes: changes)
    }
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

extension EventCluster {
    /// `nil` for a malformed bbox, which the spec rules out (four numbers).
    init?(_ c: Components.Schemas.Cluster) {
        guard c.bbox.count == 4 else { return nil }
        var types: [HazardType: Int] = [:]
        for (raw, n) in c.types.additionalProperties {
            types[HazardType(rawValue: raw) ?? .other, default: 0] += n
        }
        var severities: [Severity: Int] = [:]
        for (raw, n) in c.severities.additionalProperties {
            // A level this build doesn't know can't be drawn in its colour; leave it out of the
            // split rather than misfile it. The ring still shows maxSeverity.
            if let s = Severity(rawValue: raw) { severities[s, default: 0] += n }
        }
        self.init(
            centroid: Coordinate(latitude: c.centroid.latitude, longitude: c.centroid.longitude),
            count: c.count,
            maxSeverity: Severity(c.maxSeverity),
            types: types,
            severities: severities,
            box: GeoBox(minLongitude: c.bbox[0], minLatitude: c.bbox[1], maxLongitude: c.bbox[2], maxLatitude: c.bbox[3])
        )
    }
}

extension DetectionLayer {
    init(_ fc: Components.Schemas.FireFeatureCollection) {
        switch fc.aggregation {
        case .grid:
            let cells = fc.features.compactMap { f -> DetectionCell? in
                guard let cell = f.properties.value2, let center = f.geometry.point else { return nil }
                return DetectionCell(center: center, count: cell.count, maxFrp: cell.maxFrp.map(Double.init),
                                     latestAt: cell.latestAt)
            }
            self = .grid(cells, cellDegrees: fc.cellDegrees ?? 0)
        case .none:
            let points = fc.features.compactMap { f -> Detection? in
                guard let d = f.properties.value1, let at = f.geometry.point else { return nil }
                return Detection(coordinate: at, satellite: d.satellite, acquiredAt: d.acquiredAt,
                                 frp: d.frp.map(Double.init),
                                 confidence: d.confidence.flatMap { Detection.Confidence(rawValue: $0.rawValue) })
            }
            self = .points(points, truncated: fc.truncated)
        }
    }
}

extension Components.Schemas.Geometry {
    /// The coordinate of a GeoJSON Point (`[lon, lat]`); `nil` for any other geometry.
    var point: Coordinate? {
        guard _type == .point, let pair = coordinates.value as? [(any Sendable)?], pair.count >= 2,
              let lon = pair[0].flatMap(Self.number), let lat = pair[1].flatMap(Self.number) else { return nil }
        return Coordinate(latitude: lat, longitude: lon)
    }

    /// The whole geometry. `nil` when the coordinates don't have the shape the type promises.
    var eventGeometry: EventGeometry? {
        let raw = coordinates.value
        switch _type {
        case .point:
            return point.map(EventGeometry.point)
        case .multiPoint:
            // A point set has no area; the marker at the centroid stands for it.
            return nil
        case .lineString:
            return Self.line(raw).map { .lines([$0]) }
        case .multiLineString:
            return Self.list(raw, Self.line).map(EventGeometry.lines)
        case .polygon:
            return Self.list(raw, Self.line).map { .polygons([$0]) }
        case .multiPolygon:
            return Self.list(raw) { Self.list($0, Self.line) }.map(EventGeometry.polygons)
        }
    }

    /// `[lon, lat]` → coordinate.
    private static func position(_ v: (any Sendable)?) -> Coordinate? {
        guard let pair = v as? [(any Sendable)?], pair.count >= 2,
              let lon = pair[0].flatMap(number), let lat = pair[1].flatMap(number) else { return nil }
        return Coordinate(latitude: lat, longitude: lon)
    }

    /// A ring or line string: at least two positions.
    private static func line(_ v: (any Sendable)?) -> [Coordinate]? {
        guard let points = list(v, position), points.count >= 2 else { return nil }
        return points
    }

    /// Every element must map, or the whole list is rejected (no half-drawn shapes).
    private static func list<T>(_ v: (any Sendable)?, _ map: ((any Sendable)?) -> T?) -> [T]? {
        guard let items = v as? [(any Sendable)?], !items.isEmpty else { return nil }
        var out: [T] = []
        out.reserveCapacity(items.count)
        for item in items {
            guard let mapped = map(item) else { return nil }
            out.append(mapped)
        }
        return out
    }

    /// JSON numbers decode as Int or Double depending on whether they have a fraction.
    static func number(_ v: any Sendable) -> Double? {
        switch v {
        case let d as Double: d
        case let i as Int: Double(i)
        default: nil
        }
    }
}

// MARK: - Aircraft (ADR 28)

extension AircraftLayer {
    init(_ l: Components.Schemas.AircraftList) {
        self.init(aircraft: l.aircraft.map(Aircraft.init), dataAt: l.dataAt, complete: l.complete,
                  attribution: l.attribution, license: l.license, licenseURL: URL(string: l.licenseUrl))
    }
}

extension Aircraft {
    init(_ a: Components.Schemas.Aircraft) {
        self.init(hex: a.hex, callsign: a.callsign, registration: a.registration, typeCode: a.typeCode,
                  coordinate: Coordinate(latitude: a.lat, longitude: a.lon), altitudeFeet: a.altitudeFt,
                  onGround: a.onGround, groundSpeedKnots: a.groundSpeedKt, trackDegrees: a.trackDeg,
                  verticalRateFpm: a.verticalRateFpm, squawk: a.squawk, emergency: a.emergency,
                  military: a.military, positionAt: a.positionAt)
    }
}

// MARK: - Military bases (ADR 29)

extension BaseLayer {
    init(_ l: Components.Schemas.BaseList) {
        self.init(bases: l.bases.compactMap(MilitaryBase.init), updatedAt: l.updatedAt,
                  attribution: l.attribution, license: l.license, note: l.note)
    }
}

extension MilitaryBase {
    /// Nil only when the server sends a Wikidata link that is not a URL, which it never does.
    init?(_ b: Components.Schemas.MilitaryBase) {
        guard let url = URL(string: b.url) else { return nil }
        self.init(id: b.id, name: b.name, description: b.description,
                  kind: Kind(serverValue: b.kind.value1?.rawValue ?? b.kind.value2 ?? ""),
                  className: b._class, operatorName: b._operator, country: b.country,
                  coordinate: Coordinate(latitude: b.lat, longitude: b.lon), url: url)
    }
}

extension HistoricLayer {
    /// Cells or sites by the collection's `aggregation`, not by which shape a feature happens to
    /// decode as.
    init(_ fc: Components.Schemas.HistoricFeatureCollection) {
        let content: Content
        switch fc.aggregation {
        case .grid:
            let cells = fc.features.compactMap { f -> HistoricCell? in
                guard let cell = f.properties.value2, let center = f.geometry.point else { return nil }
                return HistoricCell(center: center, count: cell.count, kinds: cell.kinds.additionalProperties)
            }
            content = .grid(cells, cellDegrees: fc.cellDegrees ?? 0)
        case .none:
            content = .points(fc.features.compactMap(HistoricSite.init), truncated: fc.truncated)
        }
        self.init(content: content, updatedAt: fc.updatedAt, attribution: fc.attribution, license: fc.license,
                  note: fc.note)
    }
}

extension HistoricSite {
    /// Nil for a feature that is not a point, or whose Wikidata link is not a URL; the server sends
    /// neither.
    init?(_ f: Components.Schemas.HistoricFeature) {
        guard let s = f.properties.value1, let at = f.geometry.point, let url = URL(string: s.url) else { return nil }
        let kindName = s.kind.value1?.rawValue ?? s.kind.value2 ?? ""
        self.init(id: s.id, name: s.name, description: s.description, kind: HistoricKind(rawValue: kindName),
                  kindName: kindName, className: s._class, country: s.country, coordinate: at, url: url)
    }
}
