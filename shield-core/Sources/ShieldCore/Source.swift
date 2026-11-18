import Foundation

/// A feed Shield ingests. Mirrors `Source` in `backend/api/openapi.yaml` (`GET /v1/sources`).
public struct Source: Codable, Sendable, Hashable, Identifiable {
    public var id: String
    public var name: String
    public var url: URL
    public var license: String
    public var attribution: String
    public var pollIntervalSeconds: Int
    public var enabled: Bool
    public var lastSuccessAt: Date?
    public var lastErrorAt: Date?
    /// The server's verdict: last success older than three poll intervals.
    public var stale: Bool?

    public init(
        id: String,
        name: String,
        url: URL,
        license: String,
        attribution: String,
        pollIntervalSeconds: Int,
        enabled: Bool = true,
        lastSuccessAt: Date? = nil,
        lastErrorAt: Date? = nil,
        stale: Bool? = nil
    ) {
        self.id = id
        self.name = name
        self.url = url
        self.license = license
        self.attribution = attribution
        self.pollIntervalSeconds = pollIntervalSeconds
        self.enabled = enabled
        self.lastSuccessAt = lastSuccessAt
        self.lastErrorAt = lastErrorAt
        self.stale = stale
    }

    public var pollInterval: TimeInterval { TimeInterval(pollIntervalSeconds) }

    /// Uses the server's `stale` when present, else the same rule locally.
    public func isStale(now: Date = .now) -> Bool {
        if let stale { return stale }
        guard let lastSuccessAt else { return true }
        return Freshness.isStale(lastUpdate: lastSuccessAt, pollInterval: pollInterval, now: now)
    }
}
