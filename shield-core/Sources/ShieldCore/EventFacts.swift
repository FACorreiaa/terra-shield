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
        if isPrescribed { return "A planned, controlled fire, shown as minor whatever its size." }
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
    public var id: String { label }

    public init(label: String, date: Date) {
        self.label = label
        self.date = date
    }
}

extension HazardEvent {
    /// Started (or "Starts", for a warning ahead of its onset), observed, updated, then ended or
    /// expires. A time equal to the one before it is left out (MeteoAlarm's onset is also its
    /// observation time).
    public func detailTimes(now: Date = .now) -> [DetailTime] {
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
            out.append(DetailTime(label: "Expires", date: expires))
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
