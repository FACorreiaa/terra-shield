import Foundation
import HTTPTypes
import Testing
@testable import ShieldAPI
import ShieldCore

/// A real `/v1/events/{id}` answer from the local API (2026-11-18), ring trimmed to 7 points.
private let cycloneDetail = #"""
{
 "id": "a230e8b1-0bd8-44b5-94d1-a55a9450efe1",
 "source": "gdacs",
 "type": "tropical_cyclone",
 "kind": "observed",
 "status": "active",
 "severity": "severe",
 "sourceSeverity": "Orange alert, Tropical Storm (maximum wind speed of 83 km/h)",
 "title": "Tropical cyclone AKVAN-26",
 "summary": "From 16/11/2026 to 18/11/2026, a Tropical Storm (maximum win",
 "centroid": {
  "latitude": 15.103,
  "longitude": 86.11828953815623
 },
 "countries": [
  {
   "iso2": "IN",
   "relation": "within"
  }
 ],
 "startedAt": "2026-11-16T18:00:00Z",
 "observedAt": "2026-11-18T06:00:00Z",
 "updatedAt": "2026-11-18T10:35:39Z",
 "fetchedAt": "2026-11-18T11:02:33.004549Z",
 "url": "https://www.gdacs.org/report.aspx?eventtype=TC&eventid=1001351",
 "attrs": {
  "alertLevel": "Orange",
  "alertScore": 2,
  "bbox": [
   73.2049523520329,
   1.1766733453532,
   99.2049523520329,
   27.1766733453532
  ],
  "capUrl": "https://www.gdacs.org/contentdata/resources/TC/1001351/cap_1001351.xml",
  "countries": [
   "India"
  ],
  "episodeId": "7",
  "eventId": "1001351",
  "eventName": "AKVAN-26",
  "footprint": true,
  "point": [
   86.9,
   13.1
  ],
  "populationText": "Population affected by Category 1 (120 km/h) wind speeds or higher is 0 (28 million in Tropical Storm)",
  "severityText": "Tropical Storm (maximum wind speed of 83 km/h)",
  "severityUnit": "km/h",
  "severityValue": 83.3328,
  "vulnerability": "High"
 },
 "geometry": {
  "type": "Polygon",
  "coordinates": [
   [
    [
     86.7,
     8.13
    ],
    [
     86.8128,
     8.1319
    ],
    [
     86.9255,
     8.1377
    ],
    [
     87.0379,
     8.1474
    ],
    [
     87.1499,
     8.1609
    ],
    [
     87.2613,
     8.1782
    ],
    [
     86.7,
     8.13
    ]
   ]
  ]
 },
 "sourceInfo": {
  "id": "gdacs",
  "name": "GDACS (Global Disaster Alert and Coordination System)",
  "url": "https://www.gdacs.org/",
  "license": "CC BY 4.0 (European Union)",
  "attribution": "GDACS, a cooperation framework of the United Nations and the European Commission",
  "pollIntervalSeconds": 300,
  "enabled": true,
  "lastSuccessAt": "2026-11-18T11:02:33.35052Z",
  "stale": true
 },
 "revisions": []
}
"""#


/// A real `/v1/events/{id}` answer with revisions (local API, 2026-11-19): XAVIER, lifted from
/// minor to severe by its wind (S1 severity floors). Geometry cut to its point.
private let xavierDetail = #"""
{
 "id": "78b0fc21-319f-4829-9cad-7781e8800f09",
 "source": "gdacs",
 "type": "tropical_cyclone",
 "kind": "observed",
 "status": "active",
 "severity": "severe",
 "sourceSeverity": "Green alert, Hurricane/Typhoon > 74 mph (maximum wind speed of 194 km/h)",
 "title": "Tropical cyclone XAVIER-26",
 "summary": "From 15/11/2026 to 19/11/2026, a Hurricane/Typhoon > 74 mph ",
 "centroid": {
  "latitude": -10.6,
  "longitude": -163.2
 },
 "countries": [],
 "startedAt": "2026-11-15T06:00:00Z",
 "observedAt": "2026-11-19T06:00:00Z",
 "updatedAt": "2026-11-19T14:35:04Z",
 "fetchedAt": "2026-11-19T15:06:02.299845Z",
 "url": "https://www.gdacs.org/report.aspx?eventtype=TC&eventid=1001350",
 "attrs": {
  "alertLevel": "Green",
  "alertScore": 1,
  "bbox": [
   -176.788227054715,
   -23.7429502872371,
   -150.788227054715,
   2.25704971276293
  ],
  "capUrl": "https://www.gdacs.org/contentdata/resources/TC/1001350/cap_1001350.xml",
  "countries": [
   "Cook Islands",
   "American Samoa"
  ],
  "episodeId": "9",
  "eventId": "1001350",
  "eventName": "XAVIER-26",
  "point": [
   -163.2,
   -10.6
  ],
  "populationText": "Population affected by Category 1 (120 km/h) wind speeds or higher is 0 (43522 in Tropical Storm)",
  "severityBasis": "wind",
  "severityText": "Hurricane/Typhoon > 74 mph (maximum wind speed of 194 km/h)",
  "severityUnit": "km/h",
  "severityValue": 240.7392,
  "vulnerability": "Medium"
 },
 "geometry": {
  "type": "Point",
  "coordinates": [
   -163.2,
   -10.6
  ]
 },
 "sourceInfo": {
  "id": "gdacs",
  "name": "GDACS (Global Disaster Alert and Coordination System)",
  "url": "https://www.gdacs.org/",
  "license": "CC BY 4.0 (European Union)",
  "attribution": "GDACS, a cooperation framework of the United Nations and the European Commission",
  "pollIntervalSeconds": 300,
  "enabled": true,
  "lastSuccessAt": "2026-11-19T15:36:07.796395Z",
  "stale": false
 },
 "revisions": [
  {
   "at": "2026-11-19T12:04:16.832124Z",
   "changes": {
    "severity": {
     "from": "minor",
     "to": "severe"
    }
   }
  },
  {
   "at": "2026-11-19T10:09:07.189239Z",
   "changes": {
    "geometry": {
     "changed": true
    },
    "sourceSeverity": {
     "from": "Green alert, Hurricane/Typhoon > 74 mph (maximum wind speed of 185 km/h)",
     "to": "Green alert, Hurricane/Typhoon > 74 mph (maximum wind speed of 194 km/h)"
    }
   }
  },
  {
   "at": "2026-11-19T08:07:59.09055Z",
   "changes": {
    "geometry": {
     "changed": true
    },
    "sourceSeverity": {
     "from": "Green alert, Hurricane/Typhoon > 74 mph (maximum wind speed of 213 km/h)",
     "to": "Green alert, Hurricane/Typhoon > 74 mph (maximum wind speed of 185 km/h)"
    }
   }
  }
 ]
}
"""#

private let xavierID = "78b0fc21-319f-4829-9cad-7781e8800f09"

private let base = URL(string: "http://shield.test")!

private let cycloneID = "a230e8b1-0bd8-44b5-94d1-a55a9450efe1"

private func detail(withGeometry geometry: String) -> String {
    var object = try! JSONSerialization.jsonObject(with: Data(cycloneDetail.utf8)) as! [String: Any]
    object["geometry"] = try! JSONSerialization.jsonObject(with: Data(geometry.utf8))
    return String(decoding: try! JSONSerialization.data(withJSONObject: object), as: UTF8.self)
}

@Suite struct EventDetailTests {
    func provider(_ body: String, status: HTTPResponse.Status = .ok) -> APIEventProvider {
        let t = StubTransport(["/v1/events/\(cycloneID)": (status, body)])
        return APIEventProvider(client: .shield(baseURL: base, transport: t))
    }

    var id: UUID { UUID(uuidString: cycloneID)! }

    @Test func readsAPolygonFromTheLiveShape() async throws {
        let d = try #require(try await provider(cycloneDetail).detail(id: id))
        #expect(d.event.title == "Tropical cyclone AKVAN-26")
        guard case .polygons(let polygons) = d.geometry else { Issue.record("\(d.geometry)"); return }
        #expect(polygons.count == 1 && polygons[0].count == 1)
        #expect(polygons[0][0].count == 7)
        #expect(polygons[0][0].first == polygons[0][0].last) // closed ring
        #expect(d.geometry.hasShape)
    }

    @Test func readsMultiPolygonsWithHolesAndLines() async throws {
        let multi = #"{"type":"MultiPolygon","coordinates":[[[[0,0],[4,0],[4,4],[0,0]],[[1,1],[2,1],[2,2],[1,1]]],[[[10,10],[11,10],[11,11],[10,10]]]]}"#
        let d = try #require(try await provider(detail(withGeometry: multi)).detail(id: id))
        guard case .polygons(let polygons) = d.geometry else { Issue.record("\(d.geometry)"); return }
        #expect(polygons.map(\.count) == [2, 1]) // outer + hole, then a plain polygon
        #expect(polygons[0][1][0] == Coordinate(latitude: 1, longitude: 1))

        let line = #"{"type":"LineString","coordinates":[[120.5,14.1],[121,15.25]]}"#
        let l = try #require(try await provider(detail(withGeometry: line)).detail(id: id))
        #expect(l.geometry == .lines([[Coordinate(latitude: 14.1, longitude: 120.5),
                                       Coordinate(latitude: 15.25, longitude: 121)]]))
    }

    @Test func malformedCoordinatesFallBackToTheCentroid() async throws {
        let broken = #"{"type":"Polygon","coordinates":[[[1,2],["x",3],[1,2]]]}"#
        let d = try #require(try await provider(detail(withGeometry: broken)).detail(id: id))
        #expect(d.geometry == .point(d.event.centroid))
        #expect(!d.geometry.hasShape)
    }

    @Test func aMissingEventIsNil() async throws {
        let notFound = #"{"error":true,"code":"not_found","reason":"no such event"}"#
        #expect(try await provider(notFound, status: .notFound).detail(id: id) == nil)
    }

    @Test func readsTheSourceAndRevisionsNewestFirst() async throws {
        let t = StubTransport(["/v1/events/\(xavierID)": (.ok, xavierDetail)])
        let p = APIEventProvider(client: .shield(baseURL: base, transport: t))
        let d = try #require(try await p.detail(id: UUID(uuidString: xavierID)!))
        #expect(d.source?.id == "gdacs")
        #expect(d.source?.license == "CC BY 4.0 (European Union)")
        #expect(d.revisions.count == 3)
        #expect(d.timeline.first?.words == "Severity Minor → Severe")
        #expect(abs((d.timeline.first?.at.timeIntervalSince1970 ?? 0) - 1_795_089_856.832) < 0.001)
        #expect(d.timeline[1].words.hasSuffix("· area updated"))
        #expect(d.event.severityRating == "Rated by wind speed · 241 km/h")
    }

    @Test func anUnreadableSourceOrRevisionDoesNotFailTheDetail() async throws {
        var object = try JSONSerialization.jsonObject(with: Data(xavierDetail.utf8)) as! [String: Any]
        var info = object["sourceInfo"] as! [String: Any]
        info["url"] = ""
        object["sourceInfo"] = info
        object["revisions"] = [["at": "2026-11-19T12:04:16Z", "changes": ["population": ["from": 1, "to": 2]]]]
        let body = String(decoding: try JSONSerialization.data(withJSONObject: object), as: UTF8.self)
        let t = StubTransport(["/v1/events/\(xavierID)": (.ok, body)])
        let p = APIEventProvider(client: .shield(baseURL: base, transport: t))
        let d = try #require(try await p.detail(id: UUID(uuidString: xavierID)!))
        #expect(d.source == nil)
        #expect(d.revisions.isEmpty)
    }
}
