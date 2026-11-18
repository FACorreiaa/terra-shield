import Foundation
import HTTPTypes
import Testing
@testable import ShieldAPI
import ShieldCore

private let base = URL(string: "http://shield.test")!

/// Shaped like the backend's `EventList`: Go-style timestamps (with and without nanoseconds),
/// a nearby country, provisional severity at the top level, nested attrs.
private let eventList = """
{
  "events": [
    {
      "id": "6F1C2A7E-0000-4000-8000-000000000001",
      "source": "usgs", "type": "earthquake", "kind": "observed", "status": "active",
      "severity": "severe", "sourceSeverity": "M 6.1 mww, PAGER yellow",
      "title": "M 6.1 - 38 km W of Coquimbo, Chile",
      "centroid": {"latitude": -30.2, "longitude": -71.6},
      "countries": [{"iso2": "CL", "relation": "nearby", "distanceKm": 38.4}],
      "observedAt": "2026-11-16T13:26:00Z",
      "updatedAt": "2026-11-16T13:34:00.123456789Z",
      "fetchedAt": "2026-11-16T13:35:01.5Z",
      "url": "https://earthquake.usgs.gov/earthquakes/eventpage/us7000abcd",
      "attrs": {"mag": 6.1, "magType": "mww", "depthKm": 33, "pagerAlert": "yellow",
                "tsunami": false, "feltReports": 412, "reviewStatus": "reviewed",
                "nested": {"a": [1, "b", null]}}
    },
    {
      "id": "6f1c2a7e-0000-4000-8000-000000000002",
      "source": "eonet", "type": "volcano", "kind": "observed", "status": "active",
      "severity": "minor", "sourceSeverity": "", "severityProvisional": true,
      "title": "Reykjanes", "centroid": {"latitude": 63.9, "longitude": -22.4},
      "countries": [{"iso2": "IS", "relation": "within"}],
      "observedAt": "2026-11-16T10:00:00Z", "updatedAt": "2026-11-16T10:00:00Z",
      "fetchedAt": "2026-11-16T13:30:00Z", "attrs": {}
    }
  ],
  "clusters": [], "truncated": false, "generatedAt": "2026-11-16T13:35:02Z"
}
"""

private let sources = """
{"sources": [{"id": "usgs", "name": "USGS Earthquake Hazards Program", "url": "https://earthquake.usgs.gov/",
  "license": "Public domain", "attribution": "U.S. Geological Survey", "pollIntervalSeconds": 60,
  "enabled": true, "lastSuccessAt": "2026-11-16T13:35:00.987654321Z", "stale": false}]}
"""

/// "now" just after the sample payload, so the default 24 h window keeps everything.
private let now = Date(timeIntervalSince1970: 1_794_838_800) // 2026-11-16T14:20:00Z

@Suite struct APIEventProviderTests {
    func provider(_ routes: [String: (HTTPResponse.Status, String)]) -> (APIEventProvider, StubTransport) {
        let t = StubTransport(routes)
        return (APIEventProvider(client: .shield(baseURL: base, transport: t), now: { now }), t)
    }

    @Test func mapsEventsOntoDomainTypes() async throws {
        let (p, _) = provider(["/v1/events": (.ok, eventList)])
        let events = try await p.events(matching: EventQuery())
        #expect(events.count == 2)

        let quake = try #require(events.first)
        #expect(quake.id == UUID(uuidString: "6f1c2a7e-0000-4000-8000-000000000001"))
        #expect(quake.type == .earthquake && quake.severity == .severe && !quake.severityProvisional)
        #expect(quake.countries == [EventCountry(iso2: "CL", relation: .nearby, distanceKm: 38.4)])
        #expect(quake.earthquake?.magnitude == 6.1)
        #expect(quake.earthquake?.feltReports == 412)
        #expect(quake.attrs["nested"] == .object(["a": .array([.number(1), .string("b"), .null])]))
        #expect(quake.updatedAt.timeIntervalSince(quake.observedAt) > 480)

        let volcano = events[1]
        #expect(volcano.severityProvisional)
        #expect(volcano.sourceSeverity == "")
        #expect(volcano.url == nil)
    }

    @Test func unknownTypeBecomesOther() async throws {
        let list = eventList.replacingOccurrences(of: #""type": "volcano""#, with: #""type": "landslide""#)
        let (p, _) = provider(["/v1/events": (.ok, list)])
        let events = try await p.events(matching: EventQuery())
        #expect(events.count == 2)
        #expect(events[1].type == .other)
    }

    @Test func sendsTheQueryAsFilters() async throws {
        let (p, t) = provider(["/v1/events": (.ok, #"{"events":[],"clusters":[],"truncated":false,"generatedAt":"2026-11-16T13:35:02Z"}"#)])
        _ = try await p.events(matching: EventQuery(minimumSeverity: .severe, country: "PT"))
        let path = try #require(t.requests.first?.path)
        let items = URLComponents(string: path)?.queryItems ?? []
        let q = Dictionary(uniqueKeysWithValues: items.map { ($0.name, $0.value ?? "") })
        #expect(q["severity"] == "extreme,severe")
        #expect(q["kind"] == "forecast,observed,warning") // detections off by default
        #expect(q["country"] == "PT")
        #expect(q["type"] == nil) // every type: no filter sent
        #expect(q["since"] == "2026-11-15T14:20:00Z")
    }

    @Test func missingEventIsNil() async throws {
        let (p, _) = provider([:])
        #expect(try await p.event(id: UUID()) == nil)
    }

    @Test func serverErrorCarriesReason() async throws {
        let (p, _) = provider(["/v1/sources": (.internalServerError, #"{"error":true,"code":"internal","reason":"db down"}"#)])
        await #expect(throws: APIError.server(status: 500, "db down")) {
            try await p.sources()
        }
    }

    @Test func mapsSources() async throws {
        let (p, _) = provider(["/v1/sources": (.ok, sources)])
        let s = try #require(try await p.sources().first)
        #expect(s.id == "usgs" && s.pollInterval == 60 && s.stale == false)
        #expect(s.lastSuccessAt != nil)
        #expect(!p.isSampleData)
    }
}

@Suite struct DateTranscoderTests {
    let t = RFC3339DateTranscoder()

    @Test(arguments: [
        "2026-11-16T13:26:00Z",
        "2026-11-16T13:26:00.5Z",
        "2026-11-16T13:26:00.123456789Z",
        "2026-11-16T14:26:00.123456789+01:00",
    ])
    func decodesGoTimestamps(_ s: String) throws {
        let d = try t.decode(s)
        #expect(abs(d.timeIntervalSince1970 - 1_794_835_560) < 1)
    }

    @Test func rejectsGarbage() {
        #expect(throws: (any Error).self) { try t.decode("yesterday") }
    }
}
