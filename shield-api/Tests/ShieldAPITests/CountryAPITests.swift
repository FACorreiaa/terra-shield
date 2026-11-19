import Foundation
import HTTPTypes
import Testing
@testable import ShieldAPI
import ShieldCore

private let base = URL(string: "http://shield.test")!

/// A real `/v1/countries/HR/summary` from the local API (2026-11-19), trimmed to two events, plus
/// two unknown hazard types and two revisions the app can't use.
private let croatia = #"""
{
 "country": {
  "iso2": "HR",
  "iso3": "HRV",
  "name": "Croatia",
  "activeEvents": 81,
  "maxSeverity": "severe",
  "bbox": [
   13.517188,
   42.43291,
   19.400977,
   46.534619
  ]
 },
 "counts": [
  {
   "type": "wind",
   "severity": "severe",
   "count": 12
  },
  {
   "type": "wind",
   "severity": "moderate",
   "count": 43
  },
  {
   "type": "storm",
   "severity": "moderate",
   "count": 13
  },
  {
   "type": "heavy_rain",
   "severity": "moderate",
   "count": 9
  },
  {
   "type": "flood",
   "severity": "minor",
   "count": 1
  },
  {
   "type": "landslide",
   "severity": "minor",
   "count": 2
  },
  {
   "type": "sandstorm",
   "severity": "minor",
   "count": 1
  }
 ],
 "coverage": {
  "officialWarnings": true,
  "sources": [
   "eonet",
   "firms",
   "gdacs",
   "meteoalarm",
   "usgs"
  ]
 },
 "events": [
  {
   "id": "2106776e-afd3-489b-ad19-3085c33fdc07",
   "source": "meteoalarm",
   "type": "wind",
   "kind": "warning",
   "status": "active",
   "severity": "severe",
   "sourceSeverity": "Orange warning",
   "title": "Orange wind warning",
   "centroid": {
    "latitude": 45.381195,
    "longitude": 17.7656216276972
   },
   "countries": [
    {
     "iso2": "HR",
     "relation": "within"
    }
   ],
   "startedAt": "2026-11-19T17:00:00Z",
   "observedAt": "2026-11-19T08:57:54Z",
   "updatedAt": "2026-11-19T08:57:54Z",
   "fetchedAt": "2026-11-19T10:39:22.972588Z",
   "url": "https://meteoalarm.org/",
   "attrs": {
    "areas": [
     "Požeško-slavonska"
    ],
    "awarenessLevel": 3
   }
  },
  {
   "id": "261b4b20-e980-4c6c-8343-0cbfcc1c26d8",
   "source": "meteoalarm",
   "type": "wind",
   "kind": "warning",
   "status": "active",
   "severity": "severe",
   "sourceSeverity": "Orange warning",
   "title": "Orange wind warning",
   "centroid": {
    "latitude": 45.73354,
    "longitude": 17.644556770258518
   },
   "countries": [
    {
     "iso2": "HR",
     "relation": "within"
    }
   ],
   "startedAt": "2026-11-19T17:00:00Z",
   "observedAt": "2026-11-19T08:57:54Z",
   "updatedAt": "2026-11-19T08:57:54Z",
   "fetchedAt": "2026-11-19T10:39:22.972588Z",
   "url": "https://meteoalarm.org/",
   "attrs": {
    "areas": [
     "Virovitičko-podravska"
    ],
    "awarenessLevel": 3
   }
  }
 ],
 "recentChanges": [
  {
   "eventId": "9e70aa50-7e92-4057-8c56-4d4ef50a98e6",
   "title": "Flood in Croatia",
   "at": "2026-11-19T10:09:07.189239Z",
   "changes": {
    "endedAt": {
     "from": "2026-11-14T01:00:00Z",
     "to": null
    },
    "geometry": {
     "changed": true
    },
    "status": {
     "from": "resolved",
     "to": "active"
    }
   }
  },
  {
   "eventId": "not-a-uuid",
   "title": "x",
   "at": "2026-11-19T09:00:00Z",
   "changes": {
    "severity": {
     "from": "minor",
     "to": "severe"
    }
   }
  },
  {
   "eventId": "9e70aa50-7e92-4057-8c56-4d4ef50a98e6",
   "title": "y",
   "at": "2026-11-19T08:00:00Z",
   "changes": {
    "population": {
     "from": 1,
     "to": 2
    }
   }
  }
 ],
 "generatedAt": "2026-11-19T11:33:12.655564Z"
}
"""#

/// Shaped like `/v1/countries`: an ordinary bbox and one across ±180° (real values).
private let countryList = #"""
{"countries": [
 {"iso2": "FJ", "iso3": "FJI", "name": "Fiji", "activeEvents": 0, "bbox": [-180, -21.705859, 180, -12.476953]},
 {"iso2": "PT", "iso3": "PRT", "name": "Portugal", "activeEvents": 22, "maxSeverity": "severe",
  "bbox": [-31.282959, 32.648291, -6.2125, 42.137402]}
]}
"""#

private let error = #"{"error": true, "code": "x", "reason": "nope"}"#

@Suite struct CountryAPITests {
    private func provider(_ routes: [String: (HTTPResponse.Status, String)],
                          headers: [String: [HTTPField.Name: String]] = [:]) -> (APIEventProvider, StubTransport) {
        let t = StubTransport(routes, headers: headers)
        return (APIEventProvider(client: .shield(baseURL: base, transport: t)), t)
    }

    @Test func listsCountries() async throws {
        let (p, _) = provider(["/v1/countries": (.ok, countryList)])
        let list = try await p.countries()
        #expect(list.map(\.iso2) == ["FJ", "PT"])
        #expect(list[0].maxSeverity == nil)
        #expect(list[0].bbox.longitudeSpan == 360)
        #expect(list[1].maxSeverity == .severe)
        #expect(list[1].bbox == GeoBox(minLongitude: -31.282959, minLatitude: 32.648291, maxLongitude: -6.2125, maxLatitude: 42.137402))
    }

    @Test func mapsARealSummary() async throws {
        let (p, t) = provider(["/v1/countries/HR/summary": (.ok, croatia)])
        let s = try #require(try await p.countrySummary(iso2: "hr"))
        #expect(t.requests.first?.path == "/v1/countries/HR/summary") // upper-cased before asking
        #expect(s.country.name == "Croatia")
        #expect(s.coverage.officialWarnings)
        #expect(s.coverage.sources.contains("meteoalarm"))
        #expect(s.events.count == 2)
        #expect(s.isListCapped)
        // "landslide" and "sandstorm" are unknown here: one Other row holding both.
        #expect(s.matrix.filter { $0.type == .other }.first?.counts[.minor] == 3)
        #expect(s.counts.reduce(0) { $0 + $1.count } == s.country.activeEvents)
        // Fractional-second dates; the unreadable revisions are dropped.
        #expect(s.recentChanges.count == 1)
        #expect(s.recentChanges[0].words == "Active again · area updated")
        #expect(s.generatedAt > Date(timeIntervalSince1970: 1_790_000_000))
    }

    @Test func anUnknownCountryIsNil() async throws {
        let (p, _) = provider(["/v1/countries/ZZ/summary": (.notFound, error)])
        #expect(try await p.countrySummary(iso2: "ZZ") == nil)
    }

    @Test func aCodeThatCantBeOneIsNotAsked() async throws {
        let (p, t) = provider([:])
        #expect(try await p.countrySummary(iso2: "Portugal") == nil)
        #expect(t.requests.isEmpty)
    }

    @Test func busyAndDownAreErrorsNotMissingCountries() async {
        let (busy, _) = provider(["/v1/countries/PT/summary": (.tooManyRequests, error)],
                                 headers: ["/v1/countries/PT/summary": [.retryAfter: "7"]])
        await #expect(throws: APIError.rateLimited(retryAfter: 7)) { try await busy.countrySummary(iso2: "PT") }
        let (down, _) = provider(["/v1/countries/PT/summary": (.serviceUnavailable, error),
                                  "/v1/countries": (.serviceUnavailable, error)])
        await #expect(throws: APIError.unavailable) { try await down.countrySummary(iso2: "PT") }
        await #expect(throws: APIError.unavailable) { try await down.countries() }
    }
}
