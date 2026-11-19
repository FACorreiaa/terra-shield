import Foundation
import HTTPTypes
import OpenAPIRuntime

/// Answers each request by path with a fixed status and JSON body, and records the requests.
/// No network, no API key.
final class StubTransport: ClientTransport, @unchecked Sendable {
    // @unchecked: test-only; `lock` guards `requests`.
    private let lock = NSLock()
    private let routes: [String: (HTTPResponse.Status, String)]
    private let contentTypes: [String: String]
    private let headers: [String: [HTTPField.Name: String]]
    private var _requests: [HTTPRequest] = []

    private let geoJSON: [String: String]

    /// `contentTypes` overrides `application/json` per path (e.g. GeoJSON for `/v1/fires`).
    /// `geoJSON` is a path's GeoJSON alternative, served when `Accept` asks for it.
    init(_ routes: [String: (HTTPResponse.Status, String)], contentTypes: [String: String] = [:],
         geoJSON: [String: String] = [:], headers: [String: [HTTPField.Name: String]] = [:]) {
        self.routes = routes
        self.contentTypes = contentTypes
        self.headers = headers
        self.geoJSON = geoJSON
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
        // Negotiates like the server (events.go `wantsGeoJSON`): any explicit
        // application/geo+json in Accept gets the GeoJSON alternative, if the route has one.
        if status == .ok, let geo = geoJSON[path], request.headerFields[.accept]?.contains("application/geo+json") == true {
            return (HTTPResponse(status: .ok, headerFields: [.contentType: "application/geo+json"]), HTTPBody(geo))
        }
        let type = status == .ok ? contentTypes[path] ?? "application/json" : "application/json"
        var fields: HTTPFields = [.contentType: type]
        for (name, value) in headers[path] ?? [:] { fields[name] = value }
        return (HTTPResponse(status: status, headerFields: fields), HTTPBody(json))
    }
}
