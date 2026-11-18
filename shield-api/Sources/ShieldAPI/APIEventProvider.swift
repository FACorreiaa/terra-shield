import Foundation
import OpenAPIRuntime
import ShieldCore

/// `HazardEventProvider` over the Shield API. Replaces `SampleEventProvider` once the backend
/// serves `/v1/events` (BACKLOG S2.2); `/v1/sources` is live already.
public struct APIEventProvider: HazardEventProvider {
    private let client: Client
    private let now: @Sendable () -> Date

    public init(client: Client, now: @escaping @Sendable () -> Date = { .now }) {
        self.client = client
        self.now = now
    }

    public var isSampleData: Bool { false }

    public func events(matching query: EventQuery) async throws -> [HazardEvent] {
        let input = Operations.ListEvents.Input(query: .init(query, now: now()))
        switch try await client.listEvents(input) {
        case .ok(let ok):
            let events = try ok.body.json.events.map(HazardEvent.init)
            // The server filters by status (active only); resolved events need a second request,
            // which no screen makes yet. Matching locally also keeps the window exact.
            return events.filter { query.matches($0, now: now()) }
        case .notModified:
            throw APIError.unexpectedNotModified
        case .badRequest(let r):
            throw APIError.server(status: 400, try? r.body.json.reason)
        case .default(let status, let r):
            throw APIError.server(status: status, try? r.body.json.reason)
        }
    }

    public func event(id: UUID) async throws -> HazardEvent? {
        switch try await client.getEvent(path: .init(id: id.uuidString.lowercased())) {
        case .ok(let ok):
            return try HazardEvent(ok.body.json.value1)
        case .notFound:
            return nil
        case .badRequest(let r):
            // Only a malformed id; a UUID's string form never is.
            throw APIError.server(status: 400, try? r.body.json.reason)
        case .notModified:
            throw APIError.unexpectedNotModified
        case .default(let status, let r):
            throw APIError.server(status: status, try? r.body.json.reason)
        }
    }

    public func sources() async throws -> [Source] {
        switch try await client.listSources() {
        case .ok(let ok):
            return try ok.body.json.sources.map(Source.init)
        case .default(let status, let r):
            throw APIError.server(status: status, try? r.body.json.reason)
        }
    }
}

public enum APIError: Error, Equatable {
    /// The server's reason from the error envelope, when it sent one.
    case server(status: Int, String?)
    /// A 304 without our having sent If-None-Match. Caching arrives with the map (S3.2).
    case unexpectedNotModified
}

extension Operations.ListEvents.Input.Query {
    init(_ q: EventQuery, now: Date) {
        func csv<T: RawRepresentable<String>>(_ values: some Collection<T>, all: Int) -> String? {
            values.count == all ? nil : values.map(\.rawValue).sorted().joined(separator: ",")
        }
        let severities = Severity.allCases.filter { $0 >= q.minimumSeverity }
        self.init(
            _type: csv(q.types, all: HazardType.allCases.count),
            severity: csv(severities, all: Severity.allCases.count),
            kind: csv(q.kinds, all: EventKind.allCases.count),
            country: q.country,
            since: q.window.map { now.addingTimeInterval(-$0) }
        )
    }
}
