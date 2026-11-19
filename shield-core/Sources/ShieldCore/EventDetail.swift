import Foundation

/// An event's footprint (GeoJSON geometry, `GET /v1/events/{id}`). Rings are
/// `[Coordinate]`, outer ring first, holes after.
public enum EventGeometry: Sendable, Hashable {
    case point(Coordinate)
    case lines([[Coordinate]])
    /// Each polygon is its rings: the outer boundary, then holes.
    case polygons([[[Coordinate]]])

    /// Whether there is an area or track to draw; a point is already the marker.
    public var hasShape: Bool {
        if case .point = self { return false }
        return true
    }
}

extension EventGeometry {
    /// `ring` with continuous longitudes (each step under 180°), so a shape crossing ±180°
    /// stays one shape: [170, -170] becomes [170, 190]. With `anchor`, the ring starts within
    /// 180° of it, which lines a polygon's holes up with its outer ring.
    public static func unwrapped(_ ring: [Coordinate], near anchor: Double? = nil) -> [Coordinate] {
        guard var previous = ring.first?.longitude else { return ring }
        if let anchor {
            while previous - anchor > 180 { previous -= 360 }
            while anchor - previous > 180 { previous += 360 }
        }
        var out = [Coordinate(latitude: ring[0].latitude, longitude: previous)]
        out.reserveCapacity(ring.count)
        for c in ring.dropFirst() {
            var lon = c.longitude
            while lon - previous > 180 { lon -= 360 }
            while previous - lon > 180 { lon += 360 }
            out.append(Coordinate(latitude: c.latitude, longitude: lon))
            previous = lon
        }
        return out
    }
}

/// One event with what only the detail endpoint returns (`GET /v1/events/{id}`).
public struct EventDetail: Sendable, Hashable {
    public let event: HazardEvent
    public let geometry: EventGeometry
    /// The source's record (`sourceInfo`); nil when it could not be read.
    public let source: Source?
    /// Newest first, at most 50 (the server's cap).
    public let revisions: [EventRevision]

    public init(event: HazardEvent, geometry: EventGeometry, source: Source? = nil, revisions: [EventRevision] = []) {
        self.event = event
        self.geometry = geometry
        self.source = source
        self.revisions = revisions
    }

    /// The revisions worth a line in the timeline (DESIGN.md §10.3 item 7). Rename-only ones are
    /// left out, as in a country's recent changes: S1.5's area titles renamed whole feeds.
    public var timeline: [EventRevision] {
        revisions.filter { r in r.changes.contains { if case .title = $0 { false } else { true } } }
    }
}

/// One change to an event, as the timeline words it.
public struct EventRevision: Sendable, Hashable, Identifiable {
    public var at: Date
    /// Never empty for a revision worth showing.
    public var changes: [Change]

    public var id: Date { at }

    public init(at: Date, changes: [Change]) {
        self.at = at
        self.changes = changes
    }

    /// "Severity Moderate → Severe · area updated". A timeline names both levels; a country's
    /// recent changes say "Raised to Severe" (`RecentChange.words`).
    public var words: String { Change.sentence(changes, style: .timeline) }
}

extension HazardEventProvider {
    /// Providers without real geometry (sample data, tests) answer with the event's centroid,
    /// so nothing is drawn beyond the marker.
    public func detail(id: UUID) async throws -> EventDetail? {
        try await event(id: id).map { EventDetail(event: $0, geometry: .point($0.centroid)) }
    }
}

extension EventGeometry {
    /// The box that frames this shape on the map ("Show on map"): its extent padded by a fifth
    /// on each side, at least 2° across, one box even across ±180°. A point gets the minimum.
    public func frame(around centroid: Coordinate, minimumSpan: Double = 2) -> GeoBox {
        let rings: [[Coordinate]]
        switch self {
        case .point(let c): rings = [[c]]
        case .lines(let lines): rings = lines
        case .polygons(let polygons): rings = polygons.flatMap { $0 }
        }
        let points = rings.flatMap { Self.unwrapped($0, near: centroid.longitude) }
        guard !points.isEmpty else { return Self.point(centroid).frame(around: centroid) }
        func padded(_ values: [Double]) -> (Double, Double) {
            let lo = values.min()!, hi = values.max()!
            let span = max(hi - lo, minimumSpan / 1.4)
            let mid = (lo + hi) / 2, half = span * 1.4 / 2
            return (mid - half, mid + half)
        }
        let (minLon, maxLon) = padded(points.map(\.longitude))
        let (minLat, maxLat) = padded(points.map(\.latitude))
        func wrap(_ lon: Double) -> Double {
            var l = lon
            while l > 180 { l -= 360 }
            while l < -180 { l += 360 }
            return l
        }
        let limit = GeoBox.mercatorLatitudeLimit
        return GeoBox(minLongitude: wrap(minLon), minLatitude: max(minLat, -limit),
                      maxLongitude: wrap(maxLon), maxLatitude: min(maxLat, limit))
    }
}
