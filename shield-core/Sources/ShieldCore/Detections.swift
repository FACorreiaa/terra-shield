import Foundation

/// NASA FIRMS satellite thermal detections (`GET /v1/fires`). Never called "wildfires"
/// (ADR 10, DESIGN.md §5): a detection is heat seen from orbit, about 3 h after the pass.
public enum DetectionLayer: Sendable, Hashable {
    /// Below zoom 6: detections counted per grid cell of `cellDegrees`.
    case grid([DetectionCell], cellDegrees: Double)
    /// From zoom 6: individual detections, newest first. `truncated` when more matched.
    case points([Detection], truncated: Bool)

    public static let empty = DetectionLayer.grid([], cellDegrees: 0)

    public var isEmpty: Bool {
        switch self {
        case .grid(let cells, _): cells.isEmpty
        case .points(let points, _): points.isEmpty
        }
    }

    /// The newest acquisition in the layer, for "Detected N h ago".
    public var latest: Date? {
        switch self {
        case .grid(let cells, _): cells.map(\.latestAt).max()
        case .points(let points, _): points.map(\.acquiredAt).max()
        }
    }
}

public struct DetectionCell: Sendable, Hashable {
    public let center: Coordinate
    public let count: Int
    /// Highest fire radiative power in the cell, MW.
    public let maxFrp: Double?
    public let latestAt: Date

    public init(center: Coordinate, count: Int, maxFrp: Double?, latestAt: Date) {
        self.center = center
        self.count = count
        self.maxFrp = maxFrp
        self.latestAt = latestAt
    }
}

public struct Detection: Sendable, Hashable {
    public enum Confidence: String, Sendable, Hashable { case low, nominal, high }

    public let coordinate: Coordinate
    /// e.g. "NOAA-20".
    public let satellite: String
    public let acquiredAt: Date
    /// Fire radiative power, MW.
    public let frp: Double?
    public let confidence: Confidence?

    public init(coordinate: Coordinate, satellite: String, acquiredAt: Date, frp: Double?, confidence: Confidence?) {
        self.coordinate = coordinate
        self.satellite = satellite
        self.acquiredAt = acquiredAt
        self.frp = frp
        self.confidence = confidence
    }
}
