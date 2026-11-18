import Foundation
import OpenAPIRuntime
import OpenAPIURLSession

extension Client {
    /// A client for the Shield API at `baseURL`, over URLSession unless a transport is given (tests).
    public static func shield(baseURL: URL, transport: (any ClientTransport)? = nil) -> Client {
        Client(
            serverURL: baseURL,
            configuration: Configuration(dateTranscoder: RFC3339DateTranscoder()),
            transport: transport ?? URLSessionTransport()
        )
    }
}

/// Go writes RFC 3339 with fractional seconds only when they are non-zero, so a payload mixes
/// `2026-11-16T13:26:00Z` and `2026-11-16T13:34:00.123456789Z`. The runtime's default
/// transcoder accepts only the first form. Encodes without fractions.
struct RFC3339DateTranscoder: DateTranscoder {
    func encode(_ date: Date) throws -> String {
        date.formatted(.iso8601)
    }

    func decode(_ string: String) throws -> Date {
        if let d = try? Date(string, strategy: .iso8601) { return d }
        let fractional = Date.ISO8601FormatStyle(includingFractionalSeconds: true)
        if let d = try? Date(string, strategy: fractional) { return d }
        // Nanosecond fractions (Go) can exceed what the format style parses; trim to milliseconds.
        if let dot = string.firstIndex(of: "."),
           let end = string[dot...].firstIndex(where: { !$0.isNumber && $0 != "." }) {
            let digits = string[string.index(after: dot)..<end].prefix(3)
            let trimmed = string[..<dot] + "." + digits + string[end...]
            if let d = try? Date(String(trimmed), strategy: fractional) { return d }
        }
        throw DecodingError.dataCorrupted(.init(codingPath: [], debugDescription: "Not an RFC 3339 date: \(string)"))
    }
}
