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
    /// What it covers, in one line ("Earthquakes M2.5+ worldwide"); nil until the API sends it.
    public var coverage: String?
    /// The licence or terms page; the home page stands in without it.
    public var termsUrl: URL?
    /// How often the source itself publishes, when that differs from our poll ("monthly release").
    public var cadence: String?
    /// Fetched only while someone views its layer (adsb.lol, live flights), not on a schedule:
    /// `pollIntervalSeconds` is its fastest refresh, and it is never stale.
    public var onDemand: Bool?

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
        // Fetched only while its layer is shown: an unwatched layer is not a stale one (#43).
        if onDemand == true { return false }
        guard let lastSuccessAt else { return true }
        return Freshness.isStale(lastUpdate: lastSuccessAt, pollInterval: pollInterval, now: now)
    }
}

// MARK: - Health (DESIGN.md §10.7)

/// The dot on a source's row: `ok`, `warning` (the last check failed) or `danger` (stale).
public enum SourceHealth: Sendable, Hashable {
    case ok
    case failing
    case stale
}

extension Source {
    /// The sources the screen lists: every active one, in the API's order.
    public static func listed(_ sources: [Source]) -> [Source] { sources.filter(\.enabled) }

    /// An error counts only when it is newer than the last success (EONET failed at 17:03 and
    /// recovered at 17:41 on 2026-11-19: healthy). Stale, by the server's rule, outranks it.
    public func health(now: Date = .now) -> SourceHealth {
        if isStale(now: now) { return .stale }
        return lastCheckFailed ? .failing : .ok
    }

    private var lastCheckFailed: Bool {
        guard let lastErrorAt else { return false }
        guard let lastSuccessAt else { return true }
        return lastErrorAt > lastSuccessAt
    }

    /// "Checked 2 min ago · 17:38 UTC", "Last check failed 3 min ago · last good check 1 h ago",
    /// "Stale · last good check 19 h ago · 22:40 UTC".
    public func statusLine(now: Date = .now) -> String {
        if onDemand == true, !lastCheckFailed {
            guard let lastSuccessAt else { return "Not fetched yet" }
            return "Fetched \(Freshness.stamp(lastSuccessAt, now: now))"
        }
        if lastCheckFailed, let lastErrorAt {
            let good = lastSuccessAt.map { "last good check \(Freshness.relative($0, now: now))" } ?? "no good check yet"
            return "Last check failed \(Freshness.relative(lastErrorAt, now: now)) · \(good)"
        }
        guard let lastSuccessAt else { return "No good check yet" }
        if isStale(now: now) { return "Stale · last good check \(Freshness.stamp(lastSuccessAt, now: now))" }
        return "Checked \(Freshness.stamp(lastSuccessAt, now: now))"
    }

    /// Our poll, not the data's age: "Checked every 6 h", or with the source's own rhythm,
    /// "Monthly release · checked every 6 h".
    public var interval: String {
        if onDemand == true { return "Fetched only while the flights layer is shown, at most once a minute" }
        let n = pollIntervalSeconds
        let every = switch n {
        case ..<60: "\(n) s"
        case _ where n % 3600 == 0: "\(n / 3600) h"
        default: "\(Int((Double(n) / 60).rounded())) min"
        }
        guard let cadence, !cadence.isEmpty else { return "Checked every \(every)" }
        return "\(cadence.prefix(1).uppercased())\(cadence.dropFirst()) · checked every \(every)"
    }
}
