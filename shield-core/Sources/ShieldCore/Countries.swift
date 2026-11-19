import Foundation

// MARK: - Country (GET /v1/countries)

/// A country as `GET /v1/countries` lists it.
public struct Country: Sendable, Hashable, Identifiable {
    public var iso2: String
    public var iso3: String
    public var name: String
    /// Active events in the country or near its coast: offshore events within 250 km count too
    /// (ADR 4), so a row for one of those must say "near", never "in".
    public var activeEvents: Int
    /// Highest severity among the active events; nil when there are none.
    public var maxSeverity: Severity?
    /// The bounds of its geometry. A country that crosses ±180° (Russia, the US, Fiji, New
    /// Zealand, Kiribati) comes back nearly 360° wide; `mapBox(for:)` handles that.
    public var bbox: GeoBox

    public var id: String { iso2 }

    public init(iso2: String, iso3: String, name: String, activeEvents: Int, maxSeverity: Severity?, bbox: GeoBox) {
        self.iso2 = iso2
        self.iso3 = iso3
        self.name = name
        self.activeEvents = activeEvents
        self.maxSeverity = maxSeverity
        self.bbox = bbox
    }

    /// "PT" for "pt" or " Pt ", nil for anything that isn't two letters A–Z (the API answers
    /// 400 to those).
    public static func normalizedCode(_ raw: String) -> String? {
        let code = raw.trimmingCharacters(in: .whitespaces).uppercased()
        guard code.count == 2, code.unicodeScalars.allSatisfy({ ("A"..."Z").contains($0) }) else { return nil }
        return code
    }

    /// Countries for the search field: an exact ISO code first (2 or 3 letters), then names that
    /// start with the text, then names that contain it. Case and accents are ignored, so "cote"
    /// finds "Côte d'Ivoire".
    public static func matching(_ text: String, in countries: [Country], limit: Int = 5) -> [Country] {
        let query = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !query.isEmpty else { return [] }
        let folded = query.folding(options: [.caseInsensitive, .diacriticInsensitive], locale: nil)
        func rank(_ c: Country) -> Int? {
            if folded.count <= 3 {
                let code = query.uppercased()
                if c.iso2 == code || c.iso3 == code { return 0 }
            }
            let name = c.name.folding(options: [.caseInsensitive, .diacriticInsensitive], locale: nil)
            if name.hasPrefix(folded) { return 1 }
            if name.range(of: folded) != nil { return 2 }
            return nil
        }
        var ranked: [(country: Country, rank: Int)] = []
        for c in countries {
            if let r = rank(c) { ranked.append((c, r)) }
        }
        ranked.sort { a, b in
            if a.rank != b.rank { return a.rank < b.rank }
            return a.country.name.localizedStandardCompare(b.country.name) == .orderedAscending
        }
        return ranked.prefix(limit).map(\.country)
    }

    /// Where the map goes for this country.
    ///
    /// Usually its bbox. A bbox nearly 360° wide (the country crosses ±180°) would frame the whole
    /// world, so then it is the smallest box around `events` that may cross ±180°, or, with no
    /// events, the country's band of latitude around the world.
    public func mapBox(for events: [HazardEvent]) -> GeoBox {
        guard bbox.longitudeSpan > 300 else { return bbox }
        let lons = events.map(\.centroid.longitude)
        let lats = events.map(\.centroid.latitude)
        guard let minLat = lats.min(), let maxLat = lats.max() else {
            return GeoBox(minLongitude: -180, minLatitude: bbox.minLatitude, maxLongitude: 180, maxLatitude: bbox.maxLatitude)
        }
        let (west, east) = Self.smallestArc(lons)
        return GeoBox(minLongitude: west, minLatitude: minLat, maxLongitude: east, maxLatitude: maxLat)
    }

    /// The shortest stretch of longitude that holds every value: the complement of the widest
    /// gap between neighbours on the circle. `west > east` when it crosses ±180°.
    static func smallestArc(_ longitudes: [Double]) -> (west: Double, east: Double) {
        let sorted = longitudes.sorted()
        guard let first = sorted.first, let last = sorted.last else { return (-180, 180) }
        // The gap that wraps from the last value round to the first.
        var widest = (gap: first + 360 - last, west: first, east: last)
        for (a, b) in zip(sorted, sorted.dropFirst()) where b - a > widest.gap {
            widest = (b - a, b, a)
        }
        return (widest.west, widest.east)
    }
}

// MARK: - Summary (GET /v1/countries/{iso2}/summary)

/// Active events of one type and severity in a country. Zero cells are omitted.
public struct SeverityCount: Sendable, Hashable {
    public var type: HazardType
    public var severity: Severity
    public var count: Int

    public init(type: HazardType, severity: Severity, count: Int) {
        self.type = type
        self.severity = severity
        self.count = count
    }
}

/// Who covers a country (ADR 7).
public struct Coverage: Sendable, Hashable {
    /// False when no official national warning feed covers it: the UI then says coverage is
    /// limited, because the absence of data is not safety.
    public var officialWarnings: Bool
    /// Ids of the sources that cover it, as in `/v1/sources`.
    public var sources: [String]

    public init(officialWarnings: Bool, sources: [String]) {
        self.officialWarnings = officialWarnings
        self.sources = sources
    }
}

/// What is happening in one country, how serious, and how well it is covered (DESIGN.md §10.2).
public struct CountrySummary: Sendable, Hashable {
    public var country: Country
    public var counts: [SeverityCount]
    public var coverage: Coverage
    /// Active events, most severe first, at most `eventLimit`.
    public var events: [HazardEvent]
    /// Revisions of its events in the last 24 h, newest first, at most 50.
    public var recentChanges: [RecentChange]
    public var generatedAt: Date

    /// The server lists at most this many events (`store/country_stats.go`).
    public static let eventLimit = 100

    public init(country: Country, counts: [SeverityCount], coverage: Coverage, events: [HazardEvent],
                recentChanges: [RecentChange], generatedAt: Date) {
        self.country = country
        self.counts = counts
        self.coverage = coverage
        self.events = events
        self.recentChanges = recentChanges
        self.generatedAt = generatedAt
    }

    /// The changes worth a line. A revision that only renamed its event is left out: MeteoAlarm
    /// titles gained their area in S1.5, and fifty "renamed" rows buried the one severity change
    /// (local API, 2026-11-19).
    public var notableChanges: [RecentChange] {
        recentChanges.filter { change in
            change.changes.contains { if case .title = $0 { false } else { true } }
        }
    }

    /// Fewer events listed than are active: the list holds the most severe ones.
    public var isListCapped: Bool { events.count < country.activeEvents }

    /// Severe and extreme, from the counts, so exact even when the list is capped.
    public var highSeverityCount: Int {
        counts.filter { $0.severity >= .severe }.reduce(0) { $0 + $1.count }
    }

    /// Official warnings. The counts carry no kind, so this comes from the listed events: at least
    /// this many when the list is capped.
    public var warningCount: (value: Int, isLowerBound: Bool) {
        (events.count { $0.kind == .warning }, isListCapped)
    }

    /// One row per hazard type present, most severe first, then the most events. Types this build
    /// doesn't know arrive as `.other` and are summed into its row.
    public var matrix: [MatrixRow] {
        var byType: [HazardType: [Severity: Int]] = [:]
        for c in counts { byType[c.type, default: [:]][c.severity, default: 0] += c.count }
        return byType.map { MatrixRow(type: $0.key, counts: $0.value) }.sorted {
            if $0.maxSeverity != $1.maxSeverity { return $0.maxSeverity > $1.maxSeverity }
            if $0.total != $1.total { return $0.total > $1.total }
            return $0.type.label < $1.type.label
        }
    }

    /// The listed events in one matrix cell, or all of them.
    public func events(in cell: MatrixCell?) -> [HazardEvent] {
        guard let cell else { return events }
        return events.filter { $0.type == cell.type && $0.severity == cell.severity }
    }

    /// The events a cell holds in all, from the counts (the list may hold fewer).
    public func count(in cell: MatrixCell) -> Int {
        matrix.first { $0.type == cell.type }?.counts[cell.severity] ?? 0
    }
}

/// A row of the type × severity matrix.
public struct MatrixRow: Sendable, Hashable, Identifiable {
    public var type: HazardType
    public var counts: [Severity: Int]

    public init(type: HazardType, counts: [Severity: Int]) {
        self.type = type
        self.counts = counts
    }

    public var id: HazardType { type }
    public var total: Int { counts.values.reduce(0, +) }
    public var maxSeverity: Severity { counts.filter { $0.value > 0 }.keys.max() ?? .minor }
}

/// One cell of the matrix: tapping it filters the country's list.
public struct MatrixCell: Sendable, Hashable {
    public var type: HazardType
    public var severity: Severity

    public init(type: HazardType, severity: Severity) {
        self.type = type
        self.severity = severity
    }
}

// MARK: - Recent changes

/// One revision of an event: what a person would notice changed (`store/revisions.go`).
public struct RecentChange: Sendable, Hashable, Identifiable {
    public var eventID: UUID
    /// The event's title now.
    public var title: String
    public var at: Date
    /// In the order they read best; never empty for a change worth listing.
    public var changes: [Change]

    public var id: String { "\(eventID.uuidString)@\(at.timeIntervalSince1970)" }

    public init(eventID: UUID, title: String, at: Date, changes: [Change]) {
        self.eventID = eventID
        self.title = title
        self.at = at
        self.changes = changes
    }

    /// "Raised to Severe · area updated".
    public var words: String { Change.sentence(changes, style: .summary) }
}

/// One field of a revision. Values the server may add later are dropped, never guessed at.
public enum Change: Sendable, Hashable {
    case severity(from: Severity?, to: Severity?)
    case status(from: EventStatus?, to: EventStatus?)
    /// The source's own words, verbatim.
    case sourceSeverity(from: String?, to: String?)
    case title(from: String?, to: String?)
    case endedAt(to: Date?)
    case geometry

    /// The `changes` object of a revision: `{"severity": {"from": "moderate", "to": "severe"},
    /// "geometry": {"changed": true}, ...}`. Unknown keys and unreadable values are skipped.
    public static func parse(_ object: [String: JSONValue]) -> [Change] {
        func pair(_ key: String) -> (from: JSONValue?, to: JSONValue?)? {
            guard case .object(let o)? = object[key] else { return nil }
            return (o["from"], o["to"])
        }
        var out: [Change] = []
        if let p = pair("severity") {
            let to = p.to?.stringValue.flatMap(Severity.init(rawValue:))
            if to != nil { out.append(.severity(from: p.from?.stringValue.flatMap(Severity.init(rawValue:)), to: to)) }
        }
        if let p = pair("status") {
            let to = p.to?.stringValue.flatMap(EventStatus.init(rawValue:))
            if to != nil { out.append(.status(from: p.from?.stringValue.flatMap(EventStatus.init(rawValue:)), to: to)) }
        }
        if let p = pair("sourceSeverity") {
            out.append(.sourceSeverity(from: p.from?.stringValue, to: p.to?.stringValue))
        }
        if case .object(let o)? = object["geometry"], o["changed"]?.boolValue == true {
            out.append(.geometry)
        }
        if let p = pair("title") {
            out.append(.title(from: p.from?.stringValue, to: p.to?.stringValue))
        }
        if let p = pair("endedAt") {
            out.append(.endedAt(to: p.to?.stringValue.flatMap(Self.date)))
        }
        return out
    }

    /// How a change is worded: `summary` for a country's recent changes, `timeline` for one
    /// event's history, which names both severity levels.
    enum Style { case summary, timeline }

    /// The changes as one line, most important first, capitalised: "Raised to Severe · area updated".
    static func sentence(_ changes: [Change], style: Style) -> String {
        let parts = changes.sorted { $0.rank < $1.rank }.compactMap { $0.words(among: changes, style: style) }
        guard let first = parts.first else { return "Updated" }
        return ([first.capitalizedFirstLetter] + parts.dropFirst()).joined(separator: " · ")
    }

    private static func date(_ raw: String) -> Date? {
        (try? Date(raw, strategy: .iso8601))
            ?? (try? Date(raw, strategy: Date.ISO8601FormatStyle(includingFractionalSeconds: true)))
    }

    /// Reading order: what matters most first.
    var rank: Int {
        switch self {
        case .severity: 0
        case .status: 1
        case .sourceSeverity: 2
        case .geometry: 3
        case .title: 4
        case .endedAt: 5
        }
    }

    /// The words for this change, lower case; nil when another change in the same revision says it
    /// already (a status change covers its end time).
    func words(among all: [Change], style: Style = .summary) -> String? {
        switch self {
        case .severity(let from, let to?) where style == .timeline:
            guard let from, from != to else { return "severity now \(to.label)" }
            return "severity \(from.label) → \(to.label)"
        case .severity(let from, let to?):
            guard let from, from != to else { return "now \(to.label)" }
            return to > from ? "raised to \(to.label)" : "lowered to \(to.label)"
        case .severity(_, nil):
            return nil
        case .status(_, .resolved?):
            return "resolved"
        case .status(.resolved?, .active?):
            return "active again"
        case .status:
            return nil
        case .sourceSeverity(_, let to?) where !to.isEmpty:
            return "source now says “\(to)”"
        case .sourceSeverity:
            return "source severity cleared"
        case .geometry:
            return "area updated"
        case .title:
            return "renamed"
        case .endedAt(let to):
            if all.contains(where: { if case .status = $0 { true } else { false } }) { return nil }
            return to == nil ? "end time removed" : "end time updated"
        }
    }
}

private extension String {
    var capitalizedFirstLetter: String { prefix(1).uppercased() + dropFirst() }
}

// MARK: - In or near (ADR 4)

extension HazardEvent {
    /// "Near Chile, 38 km offshore" when the event is offshore of `iso2`; nil when it is within,
    /// which a country's own list needn't say. Never "in" for an offshore event (ADR 4).
    public func nearbyWords(iso2: String, countryName: String) -> String? {
        guard let link = countries.first(where: { $0.iso2 == iso2 }), link.relation == .nearby else { return nil }
        guard let km = link.distanceKm else { return "Near \(countryName), offshore" }
        return "Near \(countryName), \(Int(km.rounded())) km offshore"
    }
}

// MARK: - Provider

extension HazardEventProvider {
    /// Providers without countries (test stubs) list none. `APIEventProvider` and
    /// `SampleEventProvider` implement both.
    public func countries() async throws -> [Country] { [] }

    /// Nil for a country the server has no boundary for.
    public func countrySummary(iso2: String) async throws -> CountrySummary? { nil }
}
