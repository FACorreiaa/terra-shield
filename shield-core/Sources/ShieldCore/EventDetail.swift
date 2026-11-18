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

/// One event with what only the detail endpoint returns. S3.5 adds revisions here.
public struct EventDetail: Sendable, Hashable {
    public let event: HazardEvent
    public let geometry: EventGeometry

    public init(event: HazardEvent, geometry: EventGeometry) {
        self.event = event
        self.geometry = geometry
    }
}

extension HazardEventProvider {
    /// Providers without real geometry (sample data, tests) answer with the event's centroid,
    /// so nothing is drawn beyond the marker.
    public func detail(id: UUID) async throws -> EventDetail? {
        try await event(id: id).map { EventDetail(event: $0, geometry: .point($0.centroid)) }
    }
}
