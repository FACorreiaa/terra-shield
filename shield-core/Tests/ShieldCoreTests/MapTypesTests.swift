import Foundation
import Testing
@testable import ShieldCore

@Suite struct MapViewportZoomTests {
    // The server's pyramid: zoom z shows 360° across 256 × 2^z points.
    @Test(arguments: [
        (360.0, 256.0, 0),
        (360.0, 1024.0, 2),
        (45.0, 1024.0, 5),
        (22.5, 1024.0, 6),
        (360.0, 100.0, 0), // narrower than one tile: clamped, never negative
    ])
    func matchesTheServerPyramid(span: Double, width: Double, expected: Int) {
        #expect(MapViewport.zoom(longitudeSpan: span, widthPoints: width) == expected)
    }

    @Test func roundsDownSoServerCellsNeverOverlapOnScreen() {
        // 5.9 on a continuous scale is still 5: the server's 64 px cells cover ≥ 64 pt here.
        let span = 360 * 1024 / (256 * pow(2, 5.9))
        #expect(MapViewport.zoom(longitudeSpan: span, widthPoints: 1024) == 5)
    }

    @Test func clampsToTheAPIRange() {
        #expect(MapViewport.zoom(longitudeSpan: 0.000001, widthPoints: 1024) == 22)
        #expect(MapViewport.zoom(longitudeSpan: 0, widthPoints: 1024) == 0)
    }
}

@Suite struct MapViewportRegionTests {
    @Test func zoomedOutPastTheWorldIsTheWholeWorld() {
        let v = MapViewport(centerLatitude: 0, centerLongitude: 30, latitudeSpan: 200, longitudeSpan: 400, widthPoints: 390)
        #expect(v.box.minLongitude == -180 && v.box.maxLongitude == 180)
        #expect(v.box.minLatitude == -GeoBox.mercatorLatitudeLimit)
        #expect(v.box.maxLatitude == GeoBox.mercatorLatitudeLimit)
    }

    @Test func aRegionOverThePacificCrossesTheAntimeridian() {
        let v = MapViewport(centerLatitude: -17, centerLongitude: 178, latitudeSpan: 10, longitudeSpan: 10, widthPoints: 390)
        #expect(v.box.crossesAntimeridian)
        #expect(v.box.minLongitude == 173)
        #expect(v.box.maxLongitude == -177)
        #expect(v.box.longitudeSpan == 10)
        #expect(v.box.contains(Coordinate(latitude: -18, longitude: 179.5)))  // Fiji, east of 180
        #expect(v.box.contains(Coordinate(latitude: -18, longitude: -178.5))) // and west of it
        #expect(!v.box.contains(Coordinate(latitude: -18, longitude: 0)))
    }

    @Test func formatsTheBBoxParameter() {
        let box = GeoBox(minLongitude: -9.5, minLatitude: 36.95, maxLongitude: -6.18901234, maxLatitude: 42.15)
        #expect(box.parameter == "-9.5000,36.9500,-6.1890,42.1500")
    }
}

@Suite struct EventClusterTests {
    let box = GeoBox(minLongitude: 0, minLatitude: 0, maxLongitude: 1, maxLatitude: 1)

    @Test func sharesAreMostSevereFirstAndSumToOne() {
        let c = EventCluster(
            centroid: Coordinate(latitude: 0, longitude: 0), count: 4, maxSeverity: .severe,
            types: [.wildfire: 4], severities: [.minor: 3, .severe: 1], box: box
        )
        #expect(c.severityShares.map(\.severity) == [.severe, .minor])
        #expect(c.severityShares.map(\.share) == [0.25, 0.75])
    }

    @Test func noSplitIsInventedWhenTheServerSentNone() {
        let c = EventCluster(
            centroid: Coordinate(latitude: 0, longitude: 0), count: 4, maxSeverity: .severe,
            types: [.wildfire: 4], box: box
        )
        #expect(c.severityShares.isEmpty)
    }
}

@Suite struct SampleProviderViewportTests {
    @Test func filtersToTheViewportAndNeverClusters() async throws {
        let p = SampleEventProvider()
        let world = try await p.snapshot(matching: EventQuery(), in: .world)
        let iberia = MapViewport(
            box: GeoBox(minLongitude: -10, minLatitude: 36, maxLongitude: 4, maxLatitude: 44), zoom: 6
        )
        let local = try await p.snapshot(matching: EventQuery(), in: iberia)
        #expect(world.clusters.isEmpty && local.clusters.isEmpty)
        #expect(local.events.count < world.events.count)
        #expect(local.events.allSatisfy { iberia.box.contains($0.centroid) })
        #expect(try await p.detections(in: .world).isEmpty)
    }
}

@Suite struct GeometryUnwrapTests {
    private func lons(_ ring: [Coordinate]) -> [Double] { ring.map(\.longitude) }
    private func ring(_ lons: [Double]) -> [Coordinate] { lons.map { Coordinate(latitude: 0, longitude: $0) } }

    @Test func aRingAcrossTheAntimeridianStaysContinuous() {
        #expect(lons(EventGeometry.unwrapped(ring([170, 179, -179, -170, 170]))) == [170, 179, 181, 190, 170])
        #expect(lons(EventGeometry.unwrapped(ring([-170, 175, -175]))) == [-170, -185, -175])
    }

    @Test func aRingAwayFromTheAntimeridianIsUnchanged() {
        let r = ring([10, 20, 30, 10])
        #expect(EventGeometry.unwrapped(r) == r)
        #expect(EventGeometry.unwrapped([]) == [])
    }

    @Test func aHoleStartsNearItsOuterRing() {
        // Outer ring unwrapped to 170…190; a hole written at -175 must sit at 185.
        #expect(lons(EventGeometry.unwrapped(ring([-175, -174, -175]), near: 170)) == [185, 186, 185])
    }
}
