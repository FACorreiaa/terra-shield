import Foundation
import HTTPTypes
import OpenAPIRuntime

/// Answers each request by path with a fixed status and JSON body, and records the requests.
/// No network, no API key.
final class StubTransport: ClientTransport, @unchecked Sendable {
    // @unchecked: test-only; `lock` guards `requests`.
    private let lock = NSLock()
    private let routes: [String: (HTTPResponse.Status, String)]
    private var _requests: [HTTPRequest] = []

    init(_ routes: [String: (HTTPResponse.Status, String)]) {
        self.routes = routes
    }

    var requests: [HTTPRequest] { lock.withLock { _requests } }

    func send(
        _ request: HTTPRequest,
        body _: HTTPBody?,
        baseURL _: URL,
        operationID _: String
    ) async throws -> (HTTPResponse, HTTPBody?) {
        lock.withLock { _requests.append(request) }
        let path = request.path?.split(separator: "?").first.map(String.init) ?? ""
        guard let (status, json) = routes[path] else {
            return (HTTPResponse(status: .notFound, headerFields: [.contentType: "application/json"]),
                    HTTPBody(#"{"error":true,"code":"not_found","reason":"no route \#(path)"}"#))
        }
        return (HTTPResponse(status: status, headerFields: [.contentType: "application/json"]), HTTPBody(json))
    }
}
