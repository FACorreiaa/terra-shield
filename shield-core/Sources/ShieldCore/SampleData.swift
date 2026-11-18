import Foundation

/// Sample events and sources for previews, the component gallery and tests. They match the
/// design mockup (design/mockups). Everything here is fake and must be shown with
/// `SampleDataBadge`. Times are relative to `now`, so freshness reads naturally.
public enum SampleData {
    public static func sources(now: Date = .now) -> [Source] {
        [
            Source(id: "usgs", name: "USGS Earthquake Hazards", url: URL(string: "https://earthquake.usgs.gov/")!,
                   license: "Public domain", attribution: "U.S. Geological Survey", pollIntervalSeconds: 60,
                   lastSuccessAt: now.addingTimeInterval(-40), stale: false),
            Source(id: "gdacs", name: "GDACS", url: URL(string: "https://www.gdacs.org/")!,
                   license: "Attribution required", attribution: "Global Disaster Alert and Coordination System",
                   pollIntervalSeconds: 600, lastSuccessAt: now.addingTimeInterval(-300), stale: false),
            Source(id: "eonet", name: "NASA EONET", url: URL(string: "https://eonet.gsfc.nasa.gov/")!,
                   license: "Public", attribution: "NASA Earth Observatory Natural Event Tracker",
                   pollIntervalSeconds: 900, lastSuccessAt: now.addingTimeInterval(-3 * 3600), stale: true),
            Source(id: "meteoalarm", name: "Meteoalarm", url: URL(string: "https://meteoalarm.org/")!,
                   license: "Attribution required", attribution: "EUMETNET Meteoalarm, national weather services",
                   pollIntervalSeconds: 600, lastSuccessAt: now.addingTimeInterval(-120), stale: false),
            Source(id: "firms", name: "NASA FIRMS", url: URL(string: "https://firms.modaps.eosdis.nasa.gov/")!,
                   license: "Attribution required", attribution: "NASA FIRMS VIIRS NRT", pollIntervalSeconds: 1800,
                   lastSuccessAt: now.addingTimeInterval(-900), stale: false),
        ]
    }

    public static func events(now: Date = .now) -> [HazardEvent] {
        func ago(_ minutes: Double) -> Date { now.addingTimeInterval(-minutes * 60) }
        func id(_ n: Int) -> UUID { UUID(uuidString: String(format: "00000000-0000-4000-8000-%012d", n))! }
        return [
            HazardEvent(id: id(1), source: "gdacs", type: .tropicalCyclone, kind: .observed, severity: .extreme,
                        sourceSeverity: "GDACS Red", title: "Tropical cyclone near Lesser Antilles",
                        centroid: .init(latitude: 16.5, longitude: -62), countries: [.init(iso2: "GP", relation: .nearby, distanceKm: 140)],
                        startedAt: ago(17 * 60), observedAt: ago(6), updatedAt: ago(6), fetchedAt: ago(5),
                        url: URL(string: "https://www.gdacs.org/"),
                        attrs: ["category": .number(4), "maxWindKmh": .number(215), "pressureHpa": .number(944)]),
            HazardEvent(id: id(2), source: "usgs", type: .earthquake, kind: .observed, severity: .severe,
                        sourceSeverity: "M6.1 mww, PAGER yellow", title: "M6.1 earthquake off Coquimbo",
                        centroid: .init(latitude: -30.2, longitude: -71.6), countries: [.init(iso2: "CL", relation: .nearby, distanceKm: 38)],
                        observedAt: ago(12), updatedAt: ago(5), fetchedAt: ago(4),
                        url: URL(string: "https://earthquake.usgs.gov/"),
                        attrs: ["mag": .number(6.1), "magType": .string("mww"), "depthKm": .number(33), "pagerAlert": .string("yellow"),
                                "tsunami": .bool(false), "feltReports": .number(412), "reviewStatus": .string("reviewed")]),
            HazardEvent(id: id(3), source: "meteoalarm", type: .flood, kind: .warning, severity: .severe,
                        sourceSeverity: "Level 3 · Orange", title: "Flood warning, Emilia-Romagna",
                        centroid: .init(latitude: 44.5, longitude: 11.3), countries: [.init(iso2: "IT", relation: .within)],
                        startedAt: now.addingTimeInterval(4 * 3600), observedAt: ago(30), updatedAt: ago(30), fetchedAt: ago(2),
                        url: URL(string: "https://meteoalarm.org/")),
            HazardEvent(id: id(4), source: "eonet", type: .wildfire, kind: .observed, severity: .moderate,
                        sourceSeverity: "1,240 ha burned", severityProvisional: true, title: "Wildfire near Leiria",
                        centroid: .init(latitude: 39.75, longitude: -8.8), countries: [.init(iso2: "PT", relation: .within)],
                        observedAt: ago(60), updatedAt: ago(40), fetchedAt: ago(15),
                        url: URL(string: "https://eonet.gsfc.nasa.gov/")),
            HazardEvent(id: id(5), source: "meteoalarm", type: .heavyRain, kind: .forecast, severity: .minor,
                        sourceSeverity: "Level 1 · Yellow", title: "Heavy rain forecast, Minho",
                        centroid: .init(latitude: 41.7, longitude: -8.5), countries: [.init(iso2: "PT", relation: .within)],
                        startedAt: now.addingTimeInterval(5 * 3600), observedAt: ago(90), updatedAt: ago(90), fetchedAt: ago(2)),
            HazardEvent(id: id(6), source: "eonet", type: .volcano, kind: .observed, severity: .minor,
                        sourceSeverity: "", severityProvisional: true, title: "Volcanic activity, Reykjanes",
                        centroid: .init(latitude: 63.9, longitude: -22.4), countries: [.init(iso2: "IS", relation: .within)],
                        observedAt: ago(180), updatedAt: ago(180), fetchedAt: ago(180)),
        ]
    }

    /// Display names for the sample ISO codes. The real list comes from `GET /v1/countries`.
    public static let countryNames: [String: String] = [
        "GP": "Guadeloupe", "CL": "Chile", "IT": "Italy", "PT": "Portugal", "IS": "Iceland",
    ]
}

/// A provider over `SampleData`. Always reports `isSampleData == true`.
public struct SampleEventProvider: HazardEventProvider {
    private let now: @Sendable () -> Date

    public init(now: @escaping @Sendable () -> Date = { .now }) {
        self.now = now
    }

    public var isSampleData: Bool { true }

    public func events(matching query: EventQuery) async throws -> [HazardEvent] {
        let t = now()
        return SampleData.events(now: t).filter { query.matches($0, now: t) }.sortedForDisplay()
    }

    /// Sample data never clusters: there are only a handful of events.
    public func snapshot(matching query: EventQuery, in viewport: MapViewport) async throws -> MapSnapshot {
        let t = now()
        let events = SampleData.events(now: t)
            .filter { query.matches($0, now: t) && viewport.box.contains($0.centroid) }
            .sortedForDisplay()
        return MapSnapshot(events: events, clusters: [], truncated: false, generatedAt: t)
    }

    public func event(id: UUID) async throws -> HazardEvent? {
        SampleData.events(now: now()).first { $0.id == id }
    }

    /// No sample detections: a fake heat map would read as real fires.
    public func detections(in viewport: MapViewport) async throws -> DetectionLayer {
        .empty
    }

    public func sources() async throws -> [Source] {
        SampleData.sources(now: now())
    }
}
