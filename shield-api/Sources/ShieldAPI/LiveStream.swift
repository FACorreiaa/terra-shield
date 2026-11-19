import Foundation
import OSLog
import ShieldCore

/// Opens a byte stream for a request. URLSession in the app, canned bytes in tests.
public protocol StreamTransport: Sendable {
    func open(_ request: URLRequest) async throws -> (HTTPURLResponse, AsyncThrowingStream<UInt8, any Error>)
}

/// `GET /v1/stream` over URLSession (S3.6, ADR 16).
public struct URLSessionStreamTransport: StreamTransport {
    private let session: URLSession

    public init(session: URLSession = .shieldStream) {
        self.session = session
    }

    public func open(_ request: URLRequest) async throws -> (HTTPURLResponse, AsyncThrowingStream<UInt8, any Error>) {
        let (bytes, response) = try await session.bytes(for: request)
        guard let http = response as? HTTPURLResponse else { throw URLError(.badServerResponse) }
        let stream = AsyncThrowingStream<UInt8, any Error> { continuation in
            let task = Task {
                do {
                    for try await byte in bytes { continuation.yield(byte) }
                    continuation.finish()
                } catch {
                    continuation.finish(throwing: error)
                }
            }
            continuation.onTermination = { _ in task.cancel() }
        }
        return (http, stream)
    }
}

extension URLSession {
    /// For the endless stream: no cache, and an idle timeout of 45 s. The server sends a
    /// heartbeat every 20 s, so a silent connection is a dead one, and the timeout ends it so
    /// the model reconnects.
    public static let shieldStream: URLSession = {
        let config = URLSessionConfiguration.default
        config.urlCache = nil
        config.requestCachePolicy = .reloadIgnoringLocalCacheData
        config.timeoutIntervalForRequest = 45
        config.timeoutIntervalForResource = 7 * 24 * 3600
        return URLSession(configuration: config)
    }()
}

/// Where and how `APIEventProvider` streams.
public struct LiveStreamConfig: Sendable {
    public let baseURL: URL
    public let transport: any StreamTransport

    public init(baseURL: URL, transport: any StreamTransport = URLSessionStreamTransport()) {
        self.baseURL = baseURL
        self.transport = transport
    }
}

extension APIEventProvider {
    /// One connection to `/v1/stream`, unfiltered: the map pans constantly and there are
    /// a few hundred active events, so the model filters by viewport locally instead of
    /// reconnecting on every pan.
    static func changes(after cursor: String?, using config: LiveStreamConfig) -> AsyncThrowingStream<LiveUpdate, any Error> {
        AsyncThrowingStream { continuation in
            let task = Task {
                do {
                    var request = URLRequest(url: config.baseURL.appending(path: "v1/stream"))
                    request.setValue("text/event-stream", forHTTPHeaderField: "Accept")
                    request.setValue("no-cache", forHTTPHeaderField: "Cache-Control")
                    if let cursor { request.setValue(cursor, forHTTPHeaderField: "Last-Event-ID") }

                    let (response, bytes) = try await config.transport.open(request)
                    switch response.statusCode {
                    case 200:
                        break
                    case 429, 503:
                        let seconds = response.value(forHTTPHeaderField: "Retry-After").flatMap(Int.init)
                        throw StreamUnavailable(status: response.statusCode, retryAfter: seconds.map { .seconds($0) })
                    default:
                        throw APIError.server(status: response.statusCode, nil)
                    }

                    var decoder = SSEStreamDecoder(lastEventID: cursor)
                    for try await byte in bytes {
                        switch decoder.feed(byte) {
                        case .message(let message)?:
                            continuation.yield(LiveUpdate(change: liveChange(message), cursor: message.id))
                        case .cursor(let id)?:
                            continuation.yield(LiveUpdate(change: nil, cursor: id))
                        case nil:
                            break
                        }
                    }
                    continuation.finish()
                } catch {
                    continuation.finish(throwing: error)
                }
            }
            continuation.onTermination = { _ in task.cancel() }
        }
    }

    /// `nil` for an unknown event name or an undecodable payload. The cursor still moves past
    /// it: the 60 s poll that runs beside the stream picks the event up.
    static func liveChange(_ message: SSEMessage) -> LiveChange? {
        let change = decodeChange(message)
        if change == nil, message.event.hasPrefix("event.") {
            // Silent for the user (the poll covers it), loud for us: a contract drift.
            log.error("Undecodable \(message.event, privacy: .public) at \(message.id ?? "-", privacy: .public)")
        }
        return change
    }

    private static let log = Logger(subsystem: "com.fernandocorreia.shield", category: "stream")

    private static func decodeChange(_ message: SSEMessage) -> LiveChange? {
        let data = Data(message.data.utf8)
        switch message.event {
        case "event.created", "event.updated", "event.resolved":
            guard let event = try? decoder.decode(Components.Schemas.Event.self, from: data),
                  let mapped = try? HazardEvent(event) else { return nil }
            return message.event == "event.created" ? .created(mapped) : .upsert(mapped)
        case "event.removed":
            struct Removed: Decodable { let id: String }
            guard let removed = try? decoder.decode(Removed.self, from: data),
                  let id = UUID(uuidString: removed.id) else { return nil }
            return .removed(id)
        case "reset":
            return .reset
        case "ready":
            return .ready
        default:
            return nil
        }
    }

    /// Same date rules as the generated client (`RFC3339DateTranscoder`).
    private static let decoder: JSONDecoder = {
        let d = JSONDecoder()
        let transcoder = RFC3339DateTranscoder()
        d.dateDecodingStrategy = .custom { decoder in
            try transcoder.decode(try decoder.singleValueContainer().decode(String.self))
        }
        return d
    }()
}
