import Foundation

// What the event detail (DESIGN.md §10.3) says about an event beyond its badge: why it has the
// severity it has, its key figures and its times. Everything comes from fields the source sent;
// nothing is derived that the source didn't say (no cyclone category from wind, no pressure).

// MARK: - Why this severity

extension HazardEvent {
    /// What decided a GDACS severity (`attrs.severityBasis`, docs/severity.md).
    public enum SeverityBasis: String, Sendable {
        /// GDACS's own alert level; nothing to explain.
        case alert
        /// The wind speed, on the Saffir-Simpson bands.
        case wind
        /// The burned area, on the wildfire table every fire source shares.
        case area
    }

    public var severityBasis: SeverityBasis? {
        attrs["severityBasis"]?.stringValue.flatMap(SeverityBasis.init(rawValue:))
    }

    /// An EONET prescribed burn (`attrs.prescribed`): a planned fire, always minor.
    public var isPrescribed: Bool { attrs["prescribed"]?.boolValue == true }

    /// Whether the badge says "Provisional". A prescribed burn is minor by rule, not by estimate.
    public var showsProvisional: Bool { severityProvisional && !isPrescribed }

    /// The owner's labels (2026-11-19), with the number that decided: "Rated by wind speed ·
    /// 241 km/h", "Rated by burned area · 48,484 ha", "Prescribed burn". Nil when the source's
    /// own scale decided. The number is GDACS's structured value, which can disagree with its
    /// sentence; the sentence stays verbatim in `sourceSeverity`.
    public var severityRating: String? {
        if isPrescribed { return "Prescribed burn" }
        let label: String
        switch severityBasis {
        case .wind: label = "Rated by wind speed"
        case .area: label = "Rated by burned area"
        case .alert, nil: return nil
        }
        guard let value = gdacsValue else { return label }
        return "\(label) · \(value)"
    }

    /// One line on why the severity reads as it does, or nil when the badge says it all.
    public var severityNote: String? {
        if type == .conflict { return conflictSeverityNote }
        if isPrescribed { return "A planned, controlled fire, shown as minor whatever its size." }
        if attrs["guidance"]?.boolValue == true {
            // PTWC threat forecasts (#25): advice to governments, never a warning to the public.
            let guidance = "Guidance for national authorities, not a warning."
            guard severityProvisional, tsunamiBands.first?.band == "hazardous waves possible" else { return guidance }
            return "\(guidance) Rated \(severity.label.lowercased()) until wave heights are forecast."
        }
        switch severityBasis {
        case .wind, .area:
            guard let alert = attrs["alertLevel"]?.stringValue, !alert.isEmpty else { return nil }
            return "GDACS’s own alert is \(alert)."
        case .alert, nil:
            break
        }
        guard severityProvisional else { return nil }
        // DESIGN.md §5.
        if attrs["magnitudeValue"]?.doubleValue != nil {
            return "Estimated from burned area; the source publishes no severity."
        }
        return "The source publishes no scale; shown as minor until an official source rates it."
    }

    /// GDACS's structured value with its unit ("241 km/h"); nil without a positive value and a unit.
    private var gdacsValue: String? {
        guard let value = attrs["severityValue"]?.doubleValue, value > 0,
              let unit = attrs["severityUnit"]?.stringValue, !unit.isEmpty else { return nil }
        return "\(Figures.number(value)) \(unit)"
    }
}

// MARK: - Key figures

/// One type-specific fact: "Magnitude 6.1 mww", "Depth 10 km".
public struct KeyFigure: Sendable, Hashable, Identifiable {
    public var label: String
    public var value: String
    /// The figure that leads (shown large): an earthquake's magnitude, a cyclone's wind.
    public var isLead: Bool

    public var id: String { label }

    public init(label: String, value: String, isLead: Bool = false) {
        self.label = label
        self.value = value
        self.isLead = isLead
    }
}

extension HazardEvent {
    /// The figures `attrs` holds for this type and source, in reading order. Only what exists.
    public var keyFigures: [KeyFigure] {
        var out: [KeyFigure] = []
        func add(_ label: String, _ value: String?, lead: Bool = false) {
            guard let value, !value.isEmpty else { return }
            out.append(KeyFigure(label: label, value: value, isLead: lead && out.isEmpty))
        }
        func string(_ key: String) -> String? { attrs[key]?.stringValue }

        if type == .tsunami, attrs["center"] != nil {
            return tsunamiFigures
        }
        if type == .conflict {
            return conflictFigures
        }
        if let quake = earthquake {
            add("Magnitude", quake.magnitude.map { m in
                [String(format: "%.1f", m), quake.magnitudeType].compactMap { $0 }.joined(separator: " ")
            }, lead: true)
            add("Depth", quake.depthKm.map { "\(Figures.number($0)) km" })
            add("PAGER", quake.pagerAlert?.capitalized)
            if quake.tsunami { add("Tsunami", "Possible (USGS flag)") }
            add("Felt reports", quake.feltReports.map { Figures.number(Double($0)) })
            add("Review", quake.reviewStatus?.capitalized)
            return out
        }

        // GDACS's structured value: wind for cyclones, area for fires. Other types' values have
        // no meaning we can name (floods send "Magnitude 0"), so they are left out.
        if let value = attrs["severityValue"]?.doubleValue, value > 0,
           let unit = string("severityUnit"), !unit.isEmpty {
            switch type {
            case .tropicalCyclone: add("Max wind", "\(Figures.number(value)) \(unit)", lead: true)
            case .wildfire: add("Burned area", "\(Figures.number(value)) \(unit)", lead: true)
            default: break
            }
        }
        // EONET's latest magnitude: burned area for wildfires.
        if type == .wildfire, let value = attrs["magnitudeValue"]?.doubleValue, value > 0 {
            let unit = string("magnitudeUnit").map { $0 == "hectare" || $0 == "hectares" ? "ha" : $0 } ?? ""
            add("Burned area", [Figures.number(value), unit].filter { !$0.isEmpty }.joined(separator: " "), lead: true)
        }
        add("Population", string("populationText"))
        add("Vulnerability", string("vulnerability"))

        // CAP's axes, each its own field (DESIGN.md §10.5): never folded into the severity.
        add("Urgency", string("urgency"))
        add("Certainty", string("certainty"))
        add("Issued by", string("sender"))
        if case .array(let areas)? = attrs["areas"] {
            add("Areas", areas.compactMap(\.stringValue).joined(separator: ", "))
        }
        return out
    }
}

// MARK: - Times

/// One of an event's times, labelled: "Started", "Updated", "Expires".
public struct DetailTime: Sendable, Hashable, Identifiable {
    public var label: String
    public var date: Date
    /// Shown as a day ("10 Sep"), never as "N ago": a conflict's dates are months old estimates.
    public var dateOnly: Bool
    public var id: String { label }

    public init(label: String, date: Date, dateOnly: Bool = false) {
        self.label = label
        self.date = date
        self.dateOnly = dateOnly
    }
}

extension HazardEvent {
    /// Started (or "Starts", for a warning ahead of its onset), observed, updated, then ended or
    /// expires. A time equal to the one before it is left out (MeteoAlarm's onset is also its
    /// observation time).
    public func detailTimes(now: Date = .now) -> [DetailTime] {
        if type == .conflict { return conflictTimes }
        var out: [DetailTime] = []
        func add(_ label: String, _ date: Date?) {
            guard let date, out.last?.date != date else { return }
            out.append(DetailTime(label: label, date: date))
        }
        add(startedAt.map { $0 > now } == true ? "Starts" : "Started", startedAt)
        add("Observed", observedAt)
        out.append(DetailTime(label: "Updated", date: updatedAt))
        if let endedAt {
            out.append(DetailTime(label: "Ended", date: endedAt))
        } else if let raw = attrs["expiresAt"]?.stringValue, let expires = try? Date(raw, strategy: .iso8601) {
            // A PTWC forecast carries no expiry; its expiresAt is Shield's 12 h net (#25).
            let isNet = type == .tsunami && kind == .forecast
            out.append(DetailTime(label: isNet ? "Shown until" : "Expires", date: expires))
        }
        return out
    }
}

extension Freshness {
    /// "12 min ago · 14:08 UTC", and with the date once more than a day away:
    /// "3 d ago · 13 Nov, 14:20 UTC".
    public static func stamp(_ date: Date, now: Date = .now) -> String {
        guard abs(now.timeIntervalSince(date)) >= 86_400 else { return label(date, now: now) }
        let day = date.formatted(Date.FormatStyle(timeZone: TimeZone(identifier: "UTC")!)
            .locale(Locale(identifier: "en_GB")).day().month(.abbreviated))
        return "\(relative(date, now: now)) · \(day), \(utcClock(date))"
    }
}

// MARK: - Numbers

enum Figures {
    /// Whole numbers with grouping, in the app's one language: 48484 → "48,484".
    static func number(_ value: Double) -> String {
        value.formatted(.number.precision(.fractionLength(0)).locale(Locale(identifier: "en_US")))
    }
}

// MARK: - Tsunami (NOAA warning centres, #25)

extension HazardEvent {
    /// One band of a PTWC threat forecast, most severe first.
    struct TsunamiBand {
        var band: String
        var places: [String]
    }

    var tsunamiBands: [TsunamiBand] {
        guard case .array(let items)? = attrs["forecast"] else { return [] }
        return items.compactMap { item in
            guard case .object(let o) = item, let band = o["band"]?.stringValue else { return nil }
            var places: [String] = []
            if case .array(let p)? = o["places"] { places = p.compactMap(\.stringValue) }
            return TsunamiBand(band: band, places: places)
        }
    }

    /// Where the marker is not: a forecast sits on the earthquake, and its countries come from
    /// the forecast's list (`attrs.geometry == "epicenter"`). Nil when the map draws the coasts.
    /// A conflict geocoded only roughly says so (docs/attrs.md, `locationPrecision`).
    public var locationNote: String? {
        if type == .conflict { return conflictLocationNote }
        guard type == .tsunami, attrs["geometry"]?.stringValue == "epicenter" else { return nil }
        return "Shown at the earthquake’s epicentre. The waves reach the places listed below."
    }

    /// NWS marine forecast areas (UGC prefixes): coastal waters, the Great Lakes, St Lawrence.
    static let marineUGCPrefixes: Set<String> = [
        "AM", "AN", "GM", "LC", "LE", "LH", "LM", "LO", "LS", "PH", "PK", "PM", "PS", "PZ", "SL",
    ]

    fileprivate var tsunamiFigures: [KeyFigure] {
        var out: [KeyFigure] = []
        func strings(_ key: String) -> [String] {
            guard case .array(let a)? = attrs[key] else { return [] }
            return a.compactMap(\.stringValue)
        }
        let bands = tsunamiBands
        if let top = bands.first {
            out.append(KeyFigure(label: "Highest waves", value: top.band, isLead: true))
        }
        for band in bands where !band.places.isEmpty {
            out.append(KeyFigure(label: band.band, value: band.places.joined(separator: ", ")))
        }
        let unmatched = strings("unmatchedPlaces")
        if !unmatched.isEmpty { out.append(KeyFigure(label: "Also listed", value: unmatched.joined(separator: ", "))) }

        // US segments ("Chignik Bay, Alaska to Samalga Pass, Alaska") carry commas of their own.
        let areas = strings("areas")
        if !areas.isEmpty { out.append(KeyFigure(label: "Coasts", value: areas.joined(separator: "; "))) }
        // Marine zones (PZZ…, AMZ…) are never drawn, so only land zones count as missing.
        let unresolved = strings("zonesUnresolved").filter { !Self.marineUGCPrefixes.contains(String($0.prefix(2))) }
        if !unresolved.isEmpty {
            let noun = unresolved.count == 1 ? "zone" : "zones"
            out.append(KeyFigure(label: "Not on the map",
                                 value: "\(unresolved.count) \(noun) without a shape (\(unresolved.joined(separator: ", ")))"))
        }

        if let m = attrs["magnitude"]?.doubleValue {
            let type = attrs["magnitudeType"]?.stringValue.map { " \($0)" } ?? ""
            out.append(KeyFigure(label: "Earthquake", value: "M\(String(format: "%.1f", m))\(type)"))
        }
        switch attrs["center"]?.stringValue {
        case "PTWC": out.append(KeyFigure(label: "Issued by", value: "Pacific Tsunami Warning Center"))
        case "NTWC": out.append(KeyFigure(label: "Issued by", value: "National Tsunami Warning Center"))
        case let other?: out.append(KeyFigure(label: "Issued by", value: other))
        case nil: break
        }
        if let n = attrs["bulletin"]?.doubleValue {
            out.append(KeyFigure(label: "Bulletin", value: "No. \(Int(n))"))
        }
        return out
    }
}

// MARK: - Arrival toast

/// One line for a live arrival: "New · Severe earthquake · M6.1 · Chile" (DESIGN.md §10.1).
public struct EventToast: Sendable, Hashable, Identifiable {
    public let event: HazardEvent
    public var text: String { event.arrivalLine }
    public var id: UUID { event.id }

    public init(event: HazardEvent) { self.event = event }
}

extension HazardEvent {
    /// Severity and type, then the magnitude or lead figure, then the first country.
    public var arrivalLine: String {
        var parts = ["New", "\(severity.label) \(type.label.lowercased())"]
        if let m = earthquake?.magnitude {
            parts.append("M\(String(format: "%.1f", m))")
        } else if let lead = keyFigures.first(where: \.isLead) {
            parts.append(lead.value)
        }
        if let iso2 = countries.first?.iso2 {
            parts.append(Locale(identifier: "en").localizedString(forRegionCode: iso2) ?? iso2)
        }
        return parts.joined(separator: " · ")
    }
}

// MARK: - Armed conflicts (UCDP, ADR 27; docs/attrs.md)

extension HazardEvent {
    /// Anything but an exact geocode is drawn as approximate.
    public var isApproximateLocation: Bool {
        type == .conflict && attrs["locationPrecision"]?.stringValue.map { $0 != "exact" } == true
    }

    /// docs/severity.md "Armed conflicts": the level is the best death estimate; provisional
    /// while UCDP checks the row, or when one row sums a month or more of incidents.
    fileprivate var conflictSeverityNote: String {
        let basis = "Rated from UCDP’s best estimate of deaths."
        guard severityProvisional else { return basis }
        if let status = attrs["codeStatus"]?.stringValue, status != "Clear" {
            return "\(basis) Provisional: UCDP is still checking this event (\(status))."
        }
        return "\(basis) Provisional: one total for incidents over a month or more."
    }

    fileprivate var conflictLocationNote: String? {
        switch attrs["locationPrecision"]?.stringValue {
        case "within_25km": "Approximate location: within 25 km of the point shown."
        case "admin2": "Approximate location: UCDP places this at the district’s centre."
        case "admin1": "Approximate location: UCDP places this at the province’s centre."
        case "area": "Approximate location: UCDP records a wider area or a line, shown at its centre."
        default: nil
        }
    }

    fileprivate var conflictFigures: [KeyFigure] {
        var out: [KeyFigure] = []
        func add(_ label: String, _ value: String?, lead: Bool = false) {
            guard let value, !value.isEmpty else { return }
            out.append(KeyFigure(label: label, value: value, isLead: lead && out.isEmpty))
        }
        func int(_ key: String) -> Int? { attrs[key]?.doubleValue.map { Int($0) } }
        func string(_ key: String) -> String? { attrs[key]?.stringValue }
        func deaths(_ n: Int) -> String { n == 1 ? "1 death" : "\(Figures.number(Double(n))) deaths" }

        add("Deaths", int("deathsBest").map { Figures.number(Double($0)) }, lead: true)
        if let low = int("deathsLow"), let high = int("deathsHigh"), low != high {
            add("Estimate range", "\(Figures.number(Double(low)))–\(Figures.number(Double(high)))")
        }
        let violence: String? = switch string("violenceType") {
            case "state_based": "State-based (a government is a side)"
            case "non_state": "Non-state (organized groups)"
            case "one_sided": "One-sided (against civilians)"
            default: nil
        }
        add("Violence", violence)
        add("Conflict", string("conflictName"))
        let civilians = int("deathsCivilians") ?? 0
        let sideB = string("sideB")
        if let a = string("sideA") { add("Side A", "\(a) · \(deaths(int("deathsSideA") ?? 0))") }
        if let sideB {
            let n = sideB == "Civilians" ? civilians : (int("deathsSideB") ?? 0)
            add("Side B", "\(sideB) · \(deaths(n))")
        }
        if sideB != "Civilians", civilians > 0 { add("Civilians", deaths(civilians)) }
        if let unknown = int("deathsUnknown"), unknown > 0 { add("Unknown", deaths(unknown)) }
        let location: String? = switch string("locationPrecision") {
            case "within_25km": "Within 25 km"
            case "admin2": "District (approximate)"
            case "admin1": "Province (approximate)"
            case "area": "Wider area (approximate)"
            default: nil
        }
        add("Location", location)
        if let status = string("codeStatus"), status != "Clear" { add("UCDP review", status) }
        add("Sources used", int("sourceCount").map(String.init))
        return out
    }

    /// From and To (or one Date), then when the app stops showing it: days, not times.
    fileprivate var conflictTimes: [DetailTime] {
        var out: [DetailTime] = []
        let from = startedAt ?? observedAt
        if Self.utcDay(from) == Self.utcDay(observedAt) {
            out.append(DetailTime(label: "Date", date: observedAt, dateOnly: true))
        } else {
            out.append(DetailTime(label: "From", date: from, dateOnly: true))
            out.append(DetailTime(label: "To", date: observedAt, dateOnly: true))
        }
        if let raw = attrs["expiresAt"]?.stringValue, let until = try? Date(raw, strategy: .iso8601) {
            out.append(DetailTime(label: "Shown until", date: until, dateOnly: true))
        }
        return out
    }

    /// The row's line, in place of "updated N ago": "9 Aug – 10 Sep · monthly estimate (UCDP)".
    public var conflictDateLine: String {
        let from = startedAt ?? observedAt
        let days = Self.utcDay(from) == Self.utcDay(observedAt)
            ? Self.utcDay(observedAt) : "\(Self.utcDay(from)) – \(Self.utcDay(observedAt))"
        return "\(days) · monthly estimate (UCDP)"
    }

    /// "10 Sep", in UTC: UCDP dates are days without a time zone.
    public static func utcDay(_ date: Date) -> String {
        date.formatted(Date.FormatStyle(timeZone: TimeZone(identifier: "UTC")!)
            .locale(Locale(identifier: "en_GB")).day().month(.abbreviated))
    }
}
