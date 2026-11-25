import Foundation
import HTTPTypes
import OpenAPIRuntime
import Testing
@testable import ShieldAPI
import ShieldCore

/// `GET /v1/bases` (ADR 29), shapes as the local API served them on 2026-11-25.
@Suite struct BasesAPITests {
    let base = URL(string: "http://shield.test")!

    let answer = """
    {"bases":[
      {"id":"Q161348","name":"Ramstein Air Base","description":"US Air Force base in Rhineland-Palatinate, Germany",
       "kind":"air","class":"air base","operator":"United States Air Force","country":"DE",
       "lon":7.6,"lat":49.436944444,"url":"https://www.wikidata.org/wiki/Q161348"},
      {"id":"Q9","name":"Somewhere at sea","kind":"spaceport","class":"military base",
       "lon":179.5,"lat":-17,"url":"https://www.wikidata.org/wiki/Q9"}
    ],
    "updatedAt":"2026-11-25T14:06:27.955241Z","attribution":"Wikidata","license":"CC0 1.0",
    "note":"Installations listed on Wikidata with coordinates and no recorded closure. Wikidata records closures unevenly, so a listing is not proof a base is in use."}
    """

    private func provider(_ status: HTTPResponse.Status, _ body: String) -> (APIEventProvider, StubTransport) {
        let t = StubTransport(["/v1/bases": (status, body)])
        return (APIEventProvider(client: .shield(baseURL: base, transport: t)), t)
    }

    @Test func asksForEveryBaseAndMapsEveryField() async throws {
        let (p, t) = provider(.ok, answer)
        let layer = try await p.bases()
        let path = try #require(t.requests.last?.path)
        // Worldwide: no bbox.
        #expect(URLComponents(string: path)?.queryItems?.contains { $0.name == "bbox" } != true)

        #expect(layer.attribution == "Wikidata" && layer.license == "CC0 1.0")
        #expect(layer.note.hasPrefix("Installations listed on Wikidata"))
        #expect(abs((layer.updatedAt?.timeIntervalSince1970 ?? 0) - 1_795_615_587.955) < 0.01) // 2026-11-25T14:06:27.955Z

        let r = try #require(layer.bases.first)
        #expect(r.id == "Q161348" && r.name == "Ramstein Air Base" && r.kind == .air && r.className == "air base")
        #expect(r.operatorName == "United States Air Force" && r.country == "DE")
        #expect(r.description == "US Air Force base in Rhineland-Palatinate, Germany")
        #expect(r.coordinate == Coordinate(latitude: 49.436944444, longitude: 7.6))
        #expect(r.url == URL(string: "https://www.wikidata.org/wiki/Q161348"))

        let sea = layer.bases[1]
        #expect(sea.kind == .base) // an unknown kind draws as a base
        #expect(sea.operatorName == nil && sea.country == nil && sea.description == nil)
    }

    @Test func theAPIOffersBases() {
        #expect(provider(.ok, answer).0.supportsBases)
    }

    @Test func aRateLimitIsReportedAsOne() async {
        let (p, _) = provider(.tooManyRequests, #"{"error":true,"code":"rate_limited","reason":"slow down"}"#)
        do {
            _ = try await p.bases()
            Issue.record("expected a rate-limit error")
        } catch APIError.rateLimited {
        } catch {
            Issue.record("expected rateLimited, got \(error)")
        }
    }

    @Test func unavailableIsAnError() async {
        let (p, _) = provider(.serviceUnavailable, #"{"error":true,"code":"unavailable","reason":"database"}"#)
        await #expect(throws: (any Error).self) { try await p.bases() }
    }
}
