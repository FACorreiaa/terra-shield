import Foundation

/// One dispatched Server-Sent Event.
public struct SSEMessage: Sendable, Hashable {
    public var id: String?
    public var event: String
    public var data: String

    public init(id: String?, event: String, data: String) {
        self.id = id
        self.event = event
        self.data = data
    }
}

/// The WHATWG event-stream parser, fed one line at a time (line terminators removed).
///
/// `lastEventID` updates on every `id:` field, including the id-only heartbeat that
/// `/v1/stream` sends every 20 s without dispatching anything; reconnecting with it skips the
/// changes the filters already passed over.
public struct SSEParser: Sendable {
    public private(set) var lastEventID: String?
    /// The server's reconnection delay (`retry:`), in milliseconds.
    public private(set) var retryMilliseconds: Int?

    private var event = ""
    private var data: [String] = []
    private var sawData = false

    public init(lastEventID: String? = nil) {
        self.lastEventID = lastEventID
    }

    /// Returns a message when `line` is the blank line that ends one with data.
    public mutating func feed(_ line: String) -> SSEMessage? {
        if line.isEmpty { return dispatch() }
        if line.hasPrefix(":") { return nil } // comment

        let field: Substring
        var value: Substring
        if let colon = line.firstIndex(of: ":") {
            field = line[..<colon]
            value = line[line.index(after: colon)...]
            if value.hasPrefix(" ") { value = value.dropFirst() }
        } else {
            field = Substring(line)
            value = ""
        }

        switch field {
        case "event":
            event = String(value)
        case "data":
            data.append(String(value))
            sawData = true
        case "id":
            // An id containing NUL is ignored (spec); an empty id resets it.
            if !value.contains("\0") { lastEventID = value.isEmpty ? nil : String(value) }
        case "retry":
            if let ms = Int(value), ms >= 0 { retryMilliseconds = ms }
        default:
            break
        }
        return nil
    }

    private mutating func dispatch() -> SSEMessage? {
        defer {
            event = ""
            data = []
            sawData = false
        }
        guard sawData else { return nil }
        return SSEMessage(id: lastEventID, event: event.isEmpty ? "message" : event, data: data.joined(separator: "\n"))
    }
}

/// Bytes in, messages and heartbeats out. Splits lines itself (CRLF, LF or CR, per the spec)
/// because `URLSession.AsyncBytes.lines` does not reliably yield the empty lines that end
/// a message.
public struct SSEStreamDecoder: Sendable {
    public enum Output: Sendable, Hashable {
        case message(SSEMessage)
        /// A message-less block with an `id`: the 20 s heartbeat. Reported even when the id
        /// repeats (nothing changed), because it is also the proof the stream is alive.
        case cursor(String)
    }

    private var parser: SSEParser
    private var line: [UInt8] = []
    private var afterCR = false
    /// The current block had an `id:` field.
    private var blockHasID = false

    public init(lastEventID: String? = nil) {
        parser = SSEParser(lastEventID: lastEventID)
    }

    public var lastEventID: String? { parser.lastEventID }

    public mutating func feed(_ byte: UInt8) -> Output? {
        if afterCR {
            afterCR = false
            if byte == 0x0A { return nil } // the LF of a CRLF
        }
        switch byte {
        case 0x0A:
            return endLine()
        case 0x0D:
            afterCR = true
            return endLine()
        default:
            line.append(byte)
            return nil
        }
    }

    private mutating func endLine() -> Output? {
        let text = String(decoding: line, as: UTF8.self)
        line.removeAll(keepingCapacity: true)
        if text == "id" || text.hasPrefix("id:") { blockHasID = true }
        if let message = parser.feed(text) {
            blockHasID = false
            return .message(message)
        }
        if text.isEmpty {
            defer { blockHasID = false }
            if blockHasID, let id = parser.lastEventID { return .cursor(id) }
        }
        return nil
    }
}

/// A change from `GET /v1/stream` (ADR 16), already decoded.
public enum LiveChange: Sendable, Hashable {
    /// `event.updated` or `event.resolved`: the event in its latest state.
    case upsert(HazardEvent)
    /// `event.created`: applied like an upsert (the server coalesces changes after a reconnect,
    /// so a created event may come again as updated). Once the stream is live it is also news:
    /// the map fades it in, and a severe one in view is toasted (DESIGN.md §10.1).
    case created(HazardEvent)
    /// `event.removed`: the source retracted it.
    case removed(UUID)
    /// The cursor is too far behind (or unknown): reload `/v1/events`.
    case reset
    /// Caught up; the stream continues live from here.
    case ready
}

/// One step of the stream: a change, or (for the 20 s heartbeat) only a new cursor.
public struct LiveUpdate: Sendable, Hashable {
    public let change: LiveChange?
    /// Where to resume after a reconnect (`Last-Event-ID`). Compare, never count: values have gaps.
    public let cursor: String?

    public init(change: LiveChange?, cursor: String?) {
        self.change = change
        self.cursor = cursor
    }
}

/// The stream refused the connection for now (`429` rate limit, `503` at capacity or shutting
/// down). Reconnect after `retryAfter`, without telling the user: polling still runs.
public struct StreamUnavailable: Error, Sendable, Hashable {
    public let status: Int
    public let retryAfter: Duration?

    public init(status: Int, retryAfter: Duration?) {
        self.status = status
        self.retryAfter = retryAfter
    }
}

extension HazardEventProvider {
    /// Providers without a live stream (sample data) never yield; the model keeps polling.
    public var supportsLiveChanges: Bool { false }

    public func changes(after cursor: String?) -> AsyncThrowingStream<LiveUpdate, any Error> {
        AsyncThrowingStream { $0.finish() }
    }
}
