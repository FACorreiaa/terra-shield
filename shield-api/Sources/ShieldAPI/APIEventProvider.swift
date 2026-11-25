import Foundation
import OpenAPIRuntime
import ShieldCore

/// `HazardEventProvider` over the Shield API. Replaces `SampleEventProvider` once the backend
/// serves `/v1/events` (BACKLOG S2.2); `/v1/sources` is live already.
public struct APIEventProvider: HazardEventProvider {
    private let client: Client
    private let stream: LiveStreamConfig?
    private let now: @Sendable () -> Date

    /// `stream`: where to follow `/v1/stream` (S3.6); `nil` polls only.
    public init(client: Client, stream: LiveStreamConfig? = nil, now: @escaping @Sendable () -> Date = { .now }) {
        self.client = client
        self.stream = stream
        self.now = now
    }

    public var isSampleData: Bool { false }

    public var supportsLiveChanges: Bool { stream != nil }

    public func changes(after cursor: String?) -> AsyncThrowingStream<LiveUpdate, any Error> {
        guard let stream else { return AsyncThrowingStream { $0.finish() } }
        return Self.changes(after: cursor, using: stream)
    }

    public func events(matching query: EventQuery) async throws -> [HazardEvent] {
        let input = Operations.ListEvents.Input(query: .init(query, now: now()), headers: .jsonOnly)
        switch try await unwrapping { try await client.listEvents(input) } {
        case .ok(let ok):
            let events = try ok.body.json.events.map(HazardEvent.init)
            // The server filters by status (`includeResolved` asks for both); matching locally
            // also keeps the window exact.
            return events.filter { query.matches($0, now: now()) }
        case .notModified:
            throw APIError.unexpectedNotModified
        case .badRequest(let r):
            throw APIError.server(status: 400, try? r.body.json.reason)
        case .tooManyRequests(let r):
            throw APIError.rateLimited(retryAfter: r.headers.retryAfter)
        case .serviceUnavailable:
            throw APIError.unavailable
        case .default(let status, let r):
            throw APIError.server(status: status, try? r.body.json.reason)
        }
    }

    public func snapshot(matching query: EventQuery, in viewport: MapViewport) async throws -> MapSnapshot {
        var q = Operations.ListEvents.Input.Query(query, now: now())
        q.bbox = viewport.box.parameter
        q.zoom = viewport.zoom
        switch try await unwrapping { try await client.listEvents(query: q, headers: .jsonOnly) } {
        case .ok(let ok):
            let list = try ok.body.json
            let events = try list.events.map(HazardEvent.init)
            return MapSnapshot(
                // Matching locally keeps the time window exact; the server filters by status.
                // Clusters carry no status, so with resolved included their counts hold both.
                events: events.filter { query.matches($0, now: now()) },
                clusters: list.clusters.compactMap(EventCluster.init),
                truncated: list.truncated,
                generatedAt: list.generatedAt
            )
        case .notModified:
            throw APIError.unexpectedNotModified
        case .badRequest(let r):
            throw APIError.server(status: 400, try? r.body.json.reason)
        case .tooManyRequests(let r):
            throw APIError.rateLimited(retryAfter: r.headers.retryAfter)
        case .serviceUnavailable:
            throw APIError.unavailable
        case .default(let status, let r):
            throw APIError.server(status: status, try? r.body.json.reason)
        }
    }

    public func detections(in viewport: MapViewport) async throws -> DetectionLayer {
        // Points need a bbox (400 without); the grid takes one too and shrinks with it.
        let q = Operations.ListFires.Input.Query(bbox: viewport.box.parameter, zoom: viewport.zoom)
        switch try await unwrapping { try await client.listFires(query: q) } {
        case .ok(let ok):
            return DetectionLayer(try ok.body.applicationGeoJson)
        case .notModified:
            throw APIError.unexpectedNotModified
        case .badRequest(let r):
            throw APIError.server(status: 400, try? r.body.json.reason)
        case .tooManyRequests(let r):
            throw APIError.rateLimited(retryAfter: r.headers.retryAfter)
        case .serviceUnavailable:
            throw APIError.unavailable
        case .default(let status, let r):
            throw APIError.server(status: status, try? r.body.json.reason)
        }
    }

    public var supportsAircraft: Bool { true }

    /// Live flights (ADR 28). The caller keeps `box` within 4 cells of 5° (`AircraftArea.fits`);
    /// a larger one is a 400 carrying the server's "zoom in".
    public func aircraft(in box: GeoBox) async throws -> AircraftLayer {
        let q = Operations.ListAircraft.Input.Query(bbox: box.parameter)
        switch try await unwrapping { try await client.listAircraft(query: q) } {
        case .ok(let ok):
            return AircraftLayer(try ok.body.json)
        case .notModified:
            throw APIError.unexpectedNotModified
        case .badRequest:
            // The only 400 for a well-formed box: more than 4 cells, "zoom in".
            throw AircraftArea.TooLarge()
        case .tooManyRequests(let r):
            throw APIError.rateLimited(retryAfter: r.headers.retryAfter)
        case .serviceUnavailable:
            throw APIError.unavailable
        case .default(let status, let r):
            throw APIError.server(status: status, try? r.body.json.reason)
        }
    }

    public var supportsBases: Bool { true }

    /// Military bases (ADR 29): the whole list at once, worldwide; it changes once a day.
    public func bases() async throws -> BaseLayer {
        switch try await unwrapping { try await client.listBases(query: .init()) } {
        case .ok(let ok):
            return BaseLayer(try ok.body.json)
        case .notModified:
            throw APIError.unexpectedNotModified
        case .badRequest(let r):
            // Only a malformed bbox, and none is sent.
            throw APIError.server(status: 400, try? r.body.json.reason)
        case .tooManyRequests(let r):
            throw APIError.rateLimited(retryAfter: r.headers.retryAfter)
        case .serviceUnavailable:
            throw APIError.unavailable
        case .default(let status, let r):
            throw APIError.server(status: status, try? r.body.json.reason)
        }
    }

    public var supportsHistoric: Bool { true }

    /// Historic sites (ADR 30): the request says the box (none: the world's grid), the zoom and
    /// the groups (none: all, those added later included).
    public func historic(_ request: HistoricRequest) async throws -> HistoricLayer {
        let q = Operations.ListHistoricSites.Input.Query(
            bbox: request.box?.parameter, zoom: request.zoom,
            kind: request.kinds.map { $0.sorted().compactMap { .init(rawValue: $0.rawValue) } })
        switch try await unwrapping { try await client.listHistoricSites(query: q) } {
        case .ok(let ok):
            return HistoricLayer(try ok.body.applicationGeoJson)
        case .notModified:
            throw APIError.unexpectedNotModified
        case .badRequest(let r):
            throw APIError.server(status: 400, try? r.body.json.reason)
        case .tooManyRequests(let r):
            throw APIError.rateLimited(retryAfter: r.headers.retryAfter)
        case .serviceUnavailable:
            throw APIError.unavailable
        case .default(let status, let r):
            throw APIError.server(status: status, try? r.body.json.reason)
        }
    }

    public func event(id: UUID) async throws -> HazardEvent? {
        switch try await unwrapping { try await client.getEvent(path: .init(id: id.uuidString.lowercased())) } {
        case .ok(let ok):
            return try HazardEvent(ok.body.json.value1)
        case .notFound:
            return nil
        case .badRequest(let r):
            // Only a malformed id; a UUID's string form never is.
            throw APIError.server(status: 400, try? r.body.json.reason)
        case .notModified:
            throw APIError.unexpectedNotModified
        case .tooManyRequests(let r):
            throw APIError.rateLimited(retryAfter: r.headers.retryAfter)
        case .serviceUnavailable:
            throw APIError.unavailable
        case .default(let status, let r):
            throw APIError.server(status: status, try? r.body.json.reason)
        }
    }

    public func detail(id: UUID) async throws -> EventDetail? {
        switch try await unwrapping { try await client.getEvent(path: .init(id: id.uuidString.lowercased())) } {
        case .ok(let ok):
            let body = try ok.body.json
            let event = try HazardEvent(body.value1)
            let extra = body.value2
            // Unreadable geometry falls back to the centroid: the marker still shows. An
            // unreadable source or revision is left out rather than failing the detail.
            return EventDetail(
                event: event,
                geometry: extra.geometry.eventGeometry ?? .point(event.centroid),
                source: try? Source(extra.sourceInfo),
                revisions: extra.revisions.compactMap(EventRevision.init)
            )
        case .notFound:
            return nil
        case .badRequest(let r):
            throw APIError.server(status: 400, try? r.body.json.reason)
        case .notModified:
            throw APIError.unexpectedNotModified
        case .tooManyRequests(let r):
            throw APIError.rateLimited(retryAfter: r.headers.retryAfter)
        case .serviceUnavailable:
            throw APIError.unavailable
        case .default(let status, let r):
            throw APIError.server(status: status, try? r.body.json.reason)
        }
    }

    public func sources() async throws -> [Source] {
        switch try await unwrapping { try await client.listSources() } {
        case .ok(let ok):
            return try ok.body.json.sources.map(Source.init)
        case .tooManyRequests(let r):
            throw APIError.rateLimited(retryAfter: r.headers.retryAfter)
        case .serviceUnavailable:
            throw APIError.unavailable
        case .default(let status, let r):
            throw APIError.server(status: status, try? r.body.json.reason)
        }
    }
}

extension APIEventProvider {
    public func countries() async throws -> [Country] {
        switch try await unwrapping { try await client.listCountries() } {
        case .ok(let ok):
            return try ok.body.json.countries.map(Country.init)
        case .notModified:
            throw APIError.unexpectedNotModified
        case .tooManyRequests(let r):
            throw APIError.rateLimited(retryAfter: r.headers.retryAfter)
        case .serviceUnavailable:
            throw APIError.unavailable
        case .default(let status, let r):
            throw APIError.server(status: status, try? r.body.json.reason)
        }
    }

    public func countrySummary(iso2: String) async throws -> CountrySummary? {
        // The API answers 400 to anything but two capitals; a code that can't be one has no
        // boundary either.
        guard let code = Country.normalizedCode(iso2) else { return nil }
        switch try await unwrapping({ try await client.getCountrySummary(path: .init(iso2: code)) }) {
        case .ok(let ok):
            return try CountrySummary(ok.body.json)
        case .notFound:
            return nil
        case .badRequest(let r):
            throw APIError.server(status: 400, try? r.body.json.reason)
        case .notModified:
            throw APIError.unexpectedNotModified
        case .tooManyRequests(let r):
            throw APIError.rateLimited(retryAfter: r.headers.retryAfter)
        case .serviceUnavailable:
            throw APIError.unavailable
        case .default(let status, let r):
            throw APIError.server(status: status, try? r.body.json.reason)
        }
    }
}

/// The generated client wraps transport and decoding failures in `ClientError`, whose
/// description is a debug dump. The UI words failures from the underlying error (`URLError`
/// for "offline" vs "server not responding"), so unwrap it here.
private func unwrapping<T>(_ call: () async throws -> T) async throws -> T {
    do {
        return try await call()
    } catch let error as ClientError {
        throw error.underlyingError
    }
}

public enum APIError: Error, Equatable {
    /// The server's reason from the error envelope, when it sent one.
    case server(status: Int, String?)
    /// A 304 that reached the client. URLCache answers revalidations itself, so this means a
    /// cache was bypassed somewhere.
    case unexpectedNotModified
    /// 429: over the per-IP limit (20 req/s). `retryAfter` in seconds, when sent.
    case rateLimited(retryAfter: Int?)
    /// 503: the request timed out on the server, or the pod is shutting down.
    case unavailable
}

extension APIError: UserFacingError {
    public var userMessage: String {
        switch self {
        case .server(let status, _) where status >= 500:
            "Shield’s server had a problem (\(status)). Try again shortly."
        case .server(let status, let reason):
            reason.map { "Shield couldn’t answer that request: \($0)" } ?? "Shield couldn’t answer that request (\(status))."
        case .unexpectedNotModified:
            "Shield answered from a cache that wasn’t there. Try again."
        case .rateLimited(let seconds?):
            "Shield is getting too many requests from this network. Try again in \(seconds) s."
        case .rateLimited(nil):
            "Shield is getting too many requests from this network. Try again shortly."
        case .unavailable:
            "Shield’s server is busy. Try again shortly."
        }
    }
}

extension Operations.ListEvents.Input.Headers {
    /// `/v1/events` answers GeoJSON whenever `Accept` names it, and the generated default
    /// offers both types; the map reads the list shape, so ask for JSON only.
    static let jsonOnly = Self(accept: [.init(contentType: .json)])
}

extension Operations.ListEvents.Input.Query {
    init(_ q: EventQuery, now: Date) {
        func csv<T: RawRepresentable<String>>(_ values: some Collection<T>, all: Int) -> String? {
            values.count == all ? nil : values.map(\.rawValue).sorted().joined(separator: ",")
        }
        let severities = Severity.allCases.filter { $0 >= q.minimumSeverity }
        self.init(
            // Every hazard is the server's default; conflicts only when named (ADR 27).
            _type: q.types == Set(HazardType.hazards) ? nil : q.types.map(\.rawValue).sorted().joined(separator: ","),
            severity: csv(severities, all: Severity.allCases.count),
            kind: csv(q.kinds, all: EventKind.allCases.count),
            // Omitted for active only, the server's default.
            status: q.includeResolved ? "active,resolved" : nil,
            country: q.country,
            since: q.window.map { now.addingTimeInterval(-$0) }
        )
    }
}
