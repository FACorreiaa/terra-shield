import Foundation
import HTTPTypes
import OpenAPIRuntime
import Testing
@testable import ShieldAPI
import ShieldCore

/// `GET /v1/aircraft` (ADR 28), shapes as the server sends them (recorded from the local replay,
/// 2026-11-25).
@Suite struct AircraftAPITests {
    let base = URL(string: "http://shield.test")!
    let lisbon = GeoBox(minLongitude: -10, minLatitude: 36, maxLongitude: -5, maxLatitude: 41)

    let answer = """
    {"aircraft":[
      {"hex":"407179","callsign":"EXS94K","registration":"G-JZBL","typeCode":"B738","lon":-9.420649,"lat":38.681931,
       "altitudeFt":38000,"onGround":false,"groundSpeedKt":417.3,"trackDeg":27.24,"verticalRateFpm":-128,
       "squawk":"2205","military":false,"positionAt":"2026-11-25T13:22:22Z"},
      {"hex":"~2a0001","lon":-7.685913,"lat":37.424136,"onGround":true,"military":true,
       "emergency":"general","positionAt":"2026-11-25T13:22:20Z"}
    ],
    "dataAt":"2026-11-25T13:22:22.057Z","complete":true,"attribution":"© adsb.lol contributors",
    "license":"ODbL 1.0","licenseUrl":"https://opendatacommons.org/licenses/odbl/1-0/"}
    """

    private func provider(_ status: HTTPResponse.Status, _ body: String,
                          headers: [HTTPField.Name: String] = [:]) -> (APIEventProvider, StubTransport) {
        let t = StubTransport(["/v1/aircraft": (status, body)], headers: ["/v1/aircraft": headers])
        return (APIEventProvider(client: .shield(baseURL: base, transport: t)), t)
    }

    @Test func sendsTheBoxAndMapsEveryField() async throws {
        let (p, t) = provider(.ok, answer)
        let layer = try await p.aircraft(in: lisbon)
        let path = try #require(t.requests.last?.path)
        let bbox = URLComponents(string: path)?.queryItems?.first { $0.name == "bbox" }?.value
        #expect(bbox == lisbon.parameter)

        #expect(layer.complete)
        #expect(layer.attribution == "© adsb.lol contributors")
        #expect(layer.license == "ODbL 1.0")
        #expect(layer.licenseURL == URL(string: "https://opendatacommons.org/licenses/odbl/1-0/"))
        #expect(abs((layer.dataAt?.timeIntervalSince1970 ?? 0) - 1795612942.057) < 0.001) // 2026-11-25T13:22:22.057Z

        let a = try #require(layer.aircraft.first)
        #expect(a.hex == "407179" && a.callsign == "EXS94K" && a.registration == "G-JZBL" && a.typeCode == "B738")
        #expect(a.coordinate == Coordinate(latitude: 38.681931, longitude: -9.420649))
        #expect(a.altitudeFeet == 38_000 && !a.onGround && a.groundSpeedKnots == 417.3)
        #expect(a.trackDegrees == 27.24 && a.verticalRateFpm == -128 && a.squawk == "2205" && !a.military)
        #expect(a.positionAt.timeIntervalSince1970 == 1_795_612_942) // 2026-11-25T13:22:22Z

        let ground = layer.aircraft[1]
        #expect(ground.onGround && ground.military && ground.emergency == "general")
        #expect(ground.callsign == nil && ground.trackDegrees == nil && ground.altitudeFeet == nil)
    }

    @Test func anIncompleteAnswerWithNothingServedHasNoDataTime() async throws {
        let (p, _) = provider(.ok, #"{"aircraft":[],"complete":false,"attribution":"© adsb.lol contributors","license":"ODbL 1.0","licenseUrl":"https://opendatacommons.org/licenses/odbl/1-0/"}"#)
        let layer = try await p.aircraft(in: lisbon)
        #expect(!layer.complete && layer.aircraft.isEmpty && layer.dataAt == nil)
    }

    @Test func tooLargeAsksToZoomIn() async throws {
        let (p, _) = provider(.badRequest, #"{"error":true,"code":"bad_request","reason":"bbox spans more than 4 cells of 5°: zoom in"}"#)
        await #expect(throws: AircraftArea.TooLarge()) {
            try await p.aircraft(in: lisbon)
        }
    }

    @Test func theAPIOffersFlights() {
        #expect(provider(.ok, answer).0.supportsAircraft)
    }

    @Test func rateLimitedCarriesRetryAfter() async throws {
        let (p, _) = provider(.tooManyRequests, #"{"error":true,"code":"rate_limited","reason":"slow down"}"#,
                              headers: [.retryAfter: "30"])
        await #expect(throws: APIError.rateLimited(retryAfter: 30)) {
            try await p.aircraft(in: lisbon)
        }
    }
}
