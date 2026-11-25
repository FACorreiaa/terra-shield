import Foundation
import HTTPTypes
import OpenAPIRuntime
import Testing
@testable import ShieldAPI
import ShieldCore

/// `GET /v1/historic` (ADR 30), shapes as the local API served them on 2026-11-25.
@Suite struct HistoricAPITests {
    let base = URL(string: "http://shield.test")!

    static let note = "Historic sites listed on Wikidata: forts, castles, bunkers, barracks, war cemeteries and former bases. Some are ruins or memorials and some are still in use; for military installations listed as in use, see the bases layer."

    let points = """
    {"type":"FeatureCollection","aggregation":"none","truncated":true,"updatedAt":"2026-11-25T17:22:55.982972Z",
     "attribution":"Wikidata","license":"CC0 1.0","note":"\(note)",
     "features":[
      {"type":"Feature","geometry":{"type":"Point","coordinates":[10.6761,47.5486]},
       "properties":{"id":"Q1013009","name":"Burg Loch","description":"castle ruin in Pinswang, Austria",
                     "kind":"castle","class":"castle ruin","country":"AT","url":"https://www.wikidata.org/wiki/Q1013009"}},
      {"type":"Feature","geometry":{"type":"Point","coordinates":[10.61,47.59]},
       "properties":{"id":"Q9","name":"Old granary","kind":"building","class":"granary",
                     "url":"https://www.wikidata.org/wiki/Q9"}}
     ]}
    """

    let grid = """
    {"type":"FeatureCollection","aggregation":"grid","cellDegrees":0.17578125,"truncated":false,
     "attribution":"Wikidata","license":"CC0 1.0","note":"\(note)",
     "features":[
      {"type":"Feature","geometry":{"type":"Point","coordinates":[4.48242,50.88867]},
       "properties":{"count":15,"kinds":{"castle":9,"memorial":6}}},
      {"type":"Feature","geometry":{"type":"Point","coordinates":[3.95508,50.18555]},
       "properties":{"count":14,"kinds":{"memorial":14}}}
     ]}
    """

    private func provider(_ status: HTTPResponse.Status, _ body: String) -> (APIEventProvider, StubTransport) {
        let t = StubTransport(["/v1/historic": (status, body)],
                              contentTypes: status == .ok ? ["/v1/historic": "application/geo+json"] : [:])
        return (APIEventProvider(client: .shield(baseURL: base, transport: t)), t)
    }

    private func query(_ t: StubTransport) throws -> [String: String] {
        let path = try #require(t.requests.last?.path)
        let items = URLComponents(string: path)?.queryItems ?? []
        return Dictionary(items.map { ($0.name, $0.value ?? "") }, uniquingKeysWith: { a, _ in a })
    }

    @Test func pointsMapEveryFieldAndKeepAnUnknownKindNeutral() async throws {
        let (p, t) = provider(.ok, points)
        let box = GeoBox(minLongitude: 10.6, minLatitude: 47.5, maxLongitude: 10.8, maxLatitude: 47.6)
        let layer = try await p.historic(HistoricRequest(box: box, zoom: 9, kinds: nil))
        let q = try query(t)
        #expect(q["bbox"] == "10.6000,47.5000,10.8000,47.6000" && q["zoom"] == "9" && q["kind"] == nil)

        #expect(layer.attribution == "Wikidata" && layer.license == "CC0 1.0" && layer.note == Self.note)
        #expect(layer.isTruncated)
        #expect(abs((layer.updatedAt?.timeIntervalSince1970 ?? 0) - 1_795_627_375.983) < 0.01) // 2026-11-25T17:22:55.983Z
        let loch = try #require(layer.sites.first)
        #expect(loch.id == "Q1013009" && loch.name == "Burg Loch" && loch.kind == .castle && loch.kindName == "castle")
        #expect(loch.className == "castle ruin" && loch.country == "AT")
        #expect(loch.description == "castle ruin in Pinswang, Austria")
        #expect(loch.coordinate == Coordinate(latitude: 47.5486, longitude: 10.6761))
        #expect(loch.url == URL(string: "https://www.wikidata.org/wiki/Q1013009"))

        let granary = layer.sites[1]
        #expect(granary.kind == nil && granary.kindName == "building") // never drawn as military
        #expect(granary.description == nil && granary.country == nil)
    }

    @Test func aGridIsReadAsCellsWithTheirKinds() async throws {
        let (p, t) = provider(.ok, grid)
        let box = GeoBox(minLongitude: 4, minLatitude: 50, maxLongitude: 5, maxLatitude: 51)
        let layer = try await p.historic(HistoricRequest(box: box, zoom: 6, kinds: [.memorial, .castle]))
        let q = try query(t)
        #expect(q["zoom"] == "6" && q["bbox"] == "4.0000,50.0000,5.0000,51.0000")
        #expect(q["kind"] == "castle,memorial")
        guard case .grid(let cells, let cellDegrees) = layer.content else {
            Issue.record("expected a grid")
            return
        }
        #expect(cellDegrees == 0.17578125)
        #expect(cells.first == HistoricCell(center: Coordinate(latitude: 50.88867, longitude: 4.48242), count: 15,
                                           kinds: ["castle": 9, "memorial": 6]))
        #expect(layer.count(in: .world) == 29)
    }

    @Test func theWholeWorldsGridSendsNoBox() async throws {
        let (p, t) = provider(.ok, grid)
        _ = try await p.historic(HistoricRequest(box: nil, zoom: 3, kinds: nil))
        let q = try query(t)
        #expect(q["bbox"] == nil && q["zoom"] == "3")
    }

    @Test func theAPIOffersHistoricSites() {
        #expect(provider(.ok, grid).0.supportsHistoric)
    }

    @Test func aRateLimitIsReportedAsOne() async {
        let (p, _) = provider(.tooManyRequests, #"{"error":true,"code":"rate_limited","reason":"slow down"}"#)
        do {
            _ = try await p.historic(HistoricRequest(box: nil, zoom: 3, kinds: nil))
            Issue.record("expected a rate-limit error")
        } catch APIError.rateLimited {
        } catch {
            Issue.record("expected rateLimited, got \(error)")
        }
    }
}
