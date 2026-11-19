import Foundation
import HTTPTypes
import OpenAPIRuntime
import Testing
@testable import ShieldAPI
import ShieldCore

private let base = URL(string: "http://shield.test")!
private let now = Date(timeIntervalSince1970: 1_794_838_800) // 2026-11-16T14:20:00Z

/// One event plus one server cluster, as `/v1/events?zoom=2` returns for a world view.
private let worldView = """
{
  "events": [{
    "id": "6f1c2a7e-0000-4000-8000-000000000001", "source": "usgs", "type": "earthquake",
    "kind": "observed", "status": "active", "severity": "severe", "sourceSeverity": "M 6.1",
    "title": "M 6.1 - 38 km W of Coquimbo, Chile", "centroid": {"latitude": -30.2, "longitude": -71.6},
    "countries": [{"iso2": "CL", "relation": "nearby", "distanceKm": 38}],
    "observedAt": "2026-11-16T13:26:00Z", "updatedAt": "2026-11-16T13:34:00Z",
    "fetchedAt": "2026-11-16T13:35:00Z", "attrs": {}
  }],
  "clusters": [{
    "centroid": {"latitude": -25.1, "longitude": 134.2}, "count": 77, "maxSeverity": "minor",
    "types": {"wildfire": 76, "landslide": 1}, "severities": {"minor": 75, "moderate": 2},
    "statuses": {"active": 77}, "activeSeverities": {"minor": 75, "moderate": 2},
    "bbox": [113.4, -43.1, 153.6, -10.9]
  }],
  "truncated": false, "generatedAt": "2026-11-16T14:19:58.5Z"
}
"""

private let fireGrid = """
{"type": "FeatureCollection", "aggregation": "grid", "cellDegrees": 2.8125, "truncated": false,
 "features": [
  {"type": "Feature", "geometry": {"type": "Point", "coordinates": [-8.4375, 40.78125]},
   "properties": {"count": 12, "maxFrp": 41.5, "latestAt": "2026-11-16T11:02:00Z"}},
  {"type": "Feature", "geometry": {"type": "Point", "coordinates": [146, -36]},
   "properties": {"count": 3, "latestAt": "2026-11-16T09:40:00Z"}}
 ]}
"""

private let firePoints = """
{"type": "FeatureCollection", "aggregation": "none", "truncated": true,
 "features": [
  {"type": "Feature", "geometry": {"type": "Point", "coordinates": [-8.41, 40.77]},
   "properties": {"satellite": "NOAA-20", "acquiredAt": "2026-11-16T11:02:00Z", "frp": 41.5,
                  "confidence": "nominal", "iso2": "PT"}}
 ]}
"""

@Suite struct MapProviderTests {
    func provider(_ routes: [String: (HTTPResponse.Status, String)]) -> (APIEventProvider, StubTransport) {
        // /v1/events has a GeoJSON alternative, as on the server: a client that offers
        // geo+json gets it and fails to read the list (found against the live API).
        let t = StubTransport(routes, contentTypes: ["/v1/fires": "application/geo+json"],
                              geoJSON: ["/v1/events": #"{"type":"FeatureCollection","features":[]}"#])
        return (APIEventProvider(client: .shield(baseURL: base, transport: t), now: { now }), t)
    }

    func query(_ t: StubTransport) throws -> [String: String] {
        let path = try #require(t.requests.last?.path)
        let items = URLComponents(string: path)?.queryItems ?? []
        return Dictionary(uniqueKeysWithValues: items.map { ($0.name, $0.value ?? "") })
    }

    @Test func snapshotSendsViewportAndKeepsClusters() async throws {
        let (p, t) = provider(["/v1/events": (.ok, worldView)])
        let viewport = MapViewport(box: .world, zoom: 2)
        let s = try await p.snapshot(matching: EventQuery(), in: viewport)

        let q = try query(t)
        #expect(t.requests.last?.headerFields[.accept] == "application/json")
        #expect(q["zoom"] == "2")
        #expect(q["bbox"] == "-180.0000,-85.0511,180.0000,85.0511")
        #expect(q["kind"] == "forecast,observed,warning")

        #expect(s.events.map(\.title) == ["M 6.1 - 38 km W of Coquimbo, Chile"])
        let c = try #require(s.clusters.first)
        #expect(c.count == 77 && c.maxSeverity == .minor)
        #expect(c.types == [.wildfire: 76, .other: 1]) // an unknown type counts as other
        #expect(c.severities == [.minor: 75, .moderate: 2])
        #expect(c.severityShares.map(\.severity) == [.moderate, .minor])
        #expect(c.box == GeoBox(minLongitude: 113.4, minLatitude: -43.1, maxLongitude: 153.6, maxLatitude: -10.9))
        #expect(s.totalCount == 78)
        #expect(!s.truncated)
    }

    @Test func gridDetectionsBelowZoomSix() async throws {
        let (p, t) = provider(["/v1/fires": (.ok, fireGrid)])
        let layer = try await p.detections(in: MapViewport(box: .world, zoom: 3))
        #expect(try query(t)["zoom"] == "3")
        guard case .grid(let cells, let degrees) = layer else {
            Issue.record("expected a grid, got \(layer)")
            return
        }
        #expect(degrees == 2.8125)
        #expect(cells.map(\.count) == [12, 3])
        #expect(cells[0].center == Coordinate(latitude: 40.78125, longitude: -8.4375))
        #expect(cells[1].center == Coordinate(latitude: -36, longitude: 146)) // integer coordinates
        #expect(cells[1].maxFrp == nil)
        #expect(layer.latest == cells[0].latestAt)
    }

    @Test func pointDetectionsFromZoomSixCarryTruncation() async throws {
        let (p, _) = provider(["/v1/fires": (.ok, firePoints)])
        let iberia = MapViewport(box: GeoBox(minLongitude: -10, minLatitude: 36, maxLongitude: 4, maxLatitude: 44), zoom: 7)
        guard case .points(let points, let truncated) = try await p.detections(in: iberia) else {
            Issue.record("expected points")
            return
        }
        #expect(truncated)
        let d = try #require(points.first)
        #expect(d.satellite == "NOAA-20" && d.confidence == .nominal && d.frp == 41.5)
    }

    @Test func badRequestCarriesTheReason() async throws {
        let (p, _) = provider(["/v1/fires": (.badRequest, #"{"error":true,"code":"bad_request","reason":"bbox is required from zoom 6"}"#)])
        await #expect(throws: APIError.server(status: 400, "bbox is required from zoom 6")) {
            try await p.detections(in: MapViewport(box: .world, zoom: 6))
        }
    }
}

/// A transport that fails like URLSession does with nothing listening on the port.
private struct RefusingTransport: ClientTransport {
    func send(_ request: HTTPRequest, body: HTTPBody?, baseURL: URL, operationID: String)
        async throws -> (HTTPResponse, HTTPBody?) {
        throw URLError(.cannotConnectToHost)
    }
}

@Suite struct TransportErrorTests {
    @Test func surfacesTheURLErrorNotTheClientWrapper() async {
        let p = APIEventProvider(client: .shield(baseURL: base, transport: RefusingTransport()))
        await #expect(throws: URLError(.cannotConnectToHost)) {
            try await p.snapshot(matching: EventQuery(), in: .world)
        }
    }
}

@Suite struct RateLimitTests {
    @Test func a429SaysWhenToRetryAndA503SaysBusy() async throws {
        let envelope = #"{"error":true,"code":"rate_limited","reason":"slow down"}"#
        let t = StubTransport(["/v1/events": (.tooManyRequests, envelope)],
                              headers: ["/v1/events": [HTTPField.Name("Retry-After")!: "30"]])
        let p = APIEventProvider(client: .shield(baseURL: base, transport: t))
        await #expect(throws: APIError.rateLimited(retryAfter: 30)) {
            try await p.snapshot(matching: EventQuery(), in: .world)
        }
        #expect(APIError.rateLimited(retryAfter: 30).userMessage
            == "Shield is getting too many requests from this network. Try again in 30 s.")

        let busy = StubTransport(["/v1/sources": (.serviceUnavailable, #"{"error":true,"code":"unavailable","reason":"timeout"}"#)])
        await #expect(throws: APIError.unavailable) {
            try await APIEventProvider(client: .shield(baseURL: base, transport: busy)).sources()
        }
    }
}
