import Foundation

/// A longitude/latitude box. `minLongitude > maxLongitude` means it crosses the antimeridian,
/// the convention the API uses for `bbox` (ARCHITECTURE §4).
public struct GeoBox: Sendable, Hashable {
    public var minLongitude: Double
    public var minLatitude: Double
    public var maxLongitude: Double
    public var maxLatitude: Double

    public init(minLongitude: Double, minLatitude: Double, maxLongitude: Double, maxLatitude: Double) {
        self.minLongitude = minLongitude
        self.minLatitude = minLatitude
        self.maxLongitude = maxLongitude
        self.maxLatitude = maxLatitude
    }

    /// Web Mercator stops here; a box past it would ask for latitudes no map can show.
    public static let mercatorLatitudeLimit = 85.05112878
    public static let world = GeoBox(
        minLongitude: -180, minLatitude: -mercatorLatitudeLimit,
        maxLongitude: 180, maxLatitude: mercatorLatitudeLimit
    )

    public var crossesAntimeridian: Bool { minLongitude > maxLongitude }

    /// Width in degrees of longitude, across the antimeridian when the box crosses it.
    public var longitudeSpan: Double {
        crossesAntimeridian ? maxLongitude + 360 - minLongitude : maxLongitude - minLongitude
    }

    public func contains(_ c: Coordinate) -> Bool {
        guard c.latitude >= minLatitude, c.latitude <= maxLatitude else { return false }
        return crossesAntimeridian
            ? c.longitude >= minLongitude || c.longitude <= maxLongitude
            : c.longitude >= minLongitude && c.longitude <= maxLongitude
    }

    /// `minLon,minLat,maxLon,maxLat`, four decimals (about 11 m), the API's `bbox` parameter.
    public var parameter: String {
        [minLongitude, minLatitude, maxLongitude, maxLatitude]
            .map { String(format: "%.4f", $0) }
            .joined(separator: ",")
    }
}

/// What the map shows, as the API understands it: a box and a Web Mercator zoom level.
public struct MapViewport: Sendable, Hashable {
    public var box: GeoBox
    public var zoom: Int

    public init(box: GeoBox, zoom: Int) {
        self.box = box
        self.zoom = zoom
    }

    public static let world = MapViewport(box: .world, zoom: 1)

    /// Below this zoom the server clusters events and aggregates detections (ADR 9).
    public static let detailZoom = 6

    /// The visible region of a map, normalised for the API. MapKit hands back centre + span in
    /// degrees, which can run past ±180 or the poles when zoomed out.
    public init(
        centerLatitude: Double, centerLongitude: Double,
        latitudeSpan: Double, longitudeSpan: Double,
        widthPoints: Double
    ) {
        let limit = GeoBox.mercatorLatitudeLimit
        let minLat = max(-limit, centerLatitude - latitudeSpan / 2)
        let maxLat = min(limit, centerLatitude + latitudeSpan / 2)
        let box: GeoBox
        if longitudeSpan >= 360 {
            box = GeoBox(minLongitude: -180, minLatitude: minLat, maxLongitude: 180, maxLatitude: maxLat)
        } else {
            box = GeoBox(
                minLongitude: Self.wrap(centerLongitude - longitudeSpan / 2), minLatitude: minLat,
                maxLongitude: Self.wrap(centerLongitude + longitudeSpan / 2), maxLatitude: maxLat
            )
        }
        self.init(box: box, zoom: Self.zoom(longitudeSpan: longitudeSpan, widthPoints: widthPoints))
    }

    /// The zoom of a 256-point Web Mercator tile pyramid showing `longitudeSpan` degrees across
    /// `widthPoints`. It must match the server's pixel grid (backend `events/cluster.go`:
    /// 256 px tiles, 64 px cells). Rounded down, so a server cell never covers less screen than
    /// it assumes and clusters never overlap.
    public static func zoom(longitudeSpan: Double, widthPoints: Double) -> Int {
        guard longitudeSpan > 0, widthPoints > 0 else { return 0 }
        let z = log2(360 * widthPoints / (longitudeSpan * 256))
        return min(22, max(0, Int(z.rounded(.down))))
    }

    static func wrap(_ longitude: Double) -> Double {
        var l = longitude.truncatingRemainder(dividingBy: 360)
        if l > 180 { l -= 360 }
        if l < -180 { l += 360 }
        return l
    }
}

/// Events too close to draw apart at the requested zoom, merged by the server (ADR 9).
public struct EventCluster: Sendable, Hashable, Identifiable {
    public let centroid: Coordinate
    public let count: Int
    public let maxSeverity: Severity
    public let types: [HazardType: Int]
    /// Count per severity. Empty when the server did not send it; the marker then shows the
    /// maximum only, never a guessed split.
    public let severities: [Severity: Int]
    /// Zoom here to split the cluster.
    public let box: GeoBox

    public init(
        centroid: Coordinate, count: Int, maxSeverity: Severity,
        types: [HazardType: Int], severities: [Severity: Int] = [:], box: GeoBox
    ) {
        self.centroid = centroid
        self.count = count
        self.maxSeverity = maxSeverity
        self.types = types
        self.severities = severities
        self.box = box
    }

    public var id: String { "\(centroid.latitude),\(centroid.longitude),\(count)" }

    /// Share of each severity, most severe first, for the cluster's arc (DESIGN.md §9).
    public var severityShares: [(severity: Severity, share: Double)] {
        let total = severities.values.reduce(0, +)
        guard total > 0 else { return [] }
        return Severity.allCases.reversed().compactMap { s in
            severities[s].map { (s, Double($0) / Double(total)) }
        }
    }
}

/// One response of the events endpoint for a viewport: single events plus clusters.
public struct MapSnapshot: Sendable, Hashable {
    public let events: [HazardEvent]
    public let clusters: [EventCluster]
    /// More events matched than the server returned.
    public let truncated: Bool
    public let generatedAt: Date

    public init(events: [HazardEvent], clusters: [EventCluster], truncated: Bool, generatedAt: Date) {
        self.events = events
        self.clusters = clusters
        self.truncated = truncated
        self.generatedAt = generatedAt
    }

    /// Events plus every event inside a cluster.
    public var totalCount: Int { events.count + clusters.reduce(0) { $0 + $1.count } }
}
