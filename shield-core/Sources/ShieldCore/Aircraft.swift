import Foundation

/// One aircraft position from adsb.lol (ADR 28): a separate layer, never an event. Nothing about
/// it counts toward hazards, filters, the stat strip or the list.
public struct Aircraft: Sendable, Hashable, Identifiable {
    /// The 24-bit ICAO address in lowercase hex; a leading "~" marks a non-ICAO address.
    public var hex: String
    public var callsign: String?
    public var registration: String?
    /// ICAO type designator ("A21N").
    public var typeCode: String?
    public var coordinate: Coordinate
    /// Barometric; nil on the ground or when not reported.
    public var altitudeFeet: Int?
    public var onGround: Bool
    public var groundSpeedKnots: Double?
    /// True track over ground, 0–360; nil when the feed has none.
    public var trackDegrees: Double?
    public var verticalRateFpm: Int?
    public var squawk: String?
    /// ADS-B emergency status (general, lifeguard, minfuel, nordo, unlawful, downed, reserved).
    public var emergency: String?
    /// The feed database's military flag, shown as the feed gives it.
    public var military: Bool
    public var positionAt: Date

    public var id: String { hex }

    public init(hex: String, callsign: String?, registration: String?, typeCode: String?,
                coordinate: Coordinate, altitudeFeet: Int?, onGround: Bool, groundSpeedKnots: Double?,
                trackDegrees: Double?, verticalRateFpm: Int?, squawk: String?, emergency: String?,
                military: Bool, positionAt: Date) {
        self.hex = hex
        self.callsign = callsign
        self.registration = registration
        self.typeCode = typeCode
        self.coordinate = coordinate
        self.altitudeFeet = altitudeFeet
        self.onGround = onGround
        self.groundSpeedKnots = groundSpeedKnots
        self.trackDegrees = trackDegrees
        self.verticalRateFpm = verticalRateFpm
        self.squawk = squawk
        self.emergency = emergency
        self.military = military
        self.positionAt = positionAt
    }
}

/// `GET /v1/aircraft`: the aircraft in a view, how old the oldest data is, and whether part of
/// the view had none young enough (`complete == false`).
public struct AircraftLayer: Sendable, Hashable {
    public var aircraft: [Aircraft]
    /// When the feed generated the oldest data here; nil when nothing was served.
    public var dataAt: Date?
    public var complete: Bool
    /// ODbL: shown on the map whenever aircraft are.
    public var attribution: String
    public var license: String
    public var licenseURL: URL?

    public init(aircraft: [Aircraft], dataAt: Date?, complete: Bool, attribution: String,
                license: String, licenseURL: URL?) {
        self.aircraft = aircraft
        self.dataAt = dataAt
        self.complete = complete
        self.attribution = attribution
        self.license = license
        self.licenseURL = licenseURL
    }

    public static let empty = AircraftLayer(aircraft: [], dataAt: nil, complete: true,
                                            attribution: "© adsb.lol contributors", license: "ODbL 1.0",
                                            licenseURL: URL(string: "https://opendatacommons.org/licenses/odbl/1-0/"))
}

/// The server's grid (ADR 28): fixed 5° cells, and a bbox may touch at most 4. A box touches
/// every cell its edges fall in, so an edge on a cell line touches the next cell too.
public enum AircraftArea {
    /// The server refused the view as larger than 4 cells (a 400): the layer asks to zoom in.
    public struct TooLarge: Error, Equatable {
        public init() {}
    }

    public static let cellDegrees = 5.0
    public static let maxCells = 4
    private static let latCells = 36, lonCells = 72

    /// Cells the box touches as the API receives it (`GeoBox.parameter`, four decimals), the
    /// same count as backend `aircraft.cells`.
    public static func cellCount(_ box: GeoBox) -> Int {
        let sent = box.asSent
        let lat = latIndex(sent.maxLatitude) - latIndex(sent.minLatitude) + 1
        if sent.crossesAntimeridian {
            let east = lonCells - 1 - lonIndex(sent.minLongitude) + 1
            let west = lonIndex(sent.maxLongitude) + 1
            return lat * (east + west)
        }
        return lat * (lonIndex(sent.maxLongitude) - lonIndex(sent.minLongitude) + 1)
    }

    /// Whether the server answers this view rather than a 400 ("zoom in").
    public static func fits(_ box: GeoBox) -> Bool { cellCount(box) <= maxCells }

    private static func latIndex(_ lat: Double) -> Int { clamp(Int(((lat + 90) / cellDegrees).rounded(.down)), latCells - 1) }
    private static func lonIndex(_ lon: Double) -> Int { clamp(Int(((lon + 180) / cellDegrees).rounded(.down)), lonCells - 1) }
    private static func clamp(_ i: Int, _ hi: Int) -> Int { max(0, min(i, hi)) }
}

extension GeoBox {
    /// The box as the server reads it: `parameter` parsed back, so the two can't drift.
    var asSent: GeoBox {
        let v = parameter.split(separator: ",").compactMap { Double($0) }
        guard v.count == 4 else { return self }
        return GeoBox(minLongitude: v[0], minLatitude: v[1], maxLongitude: v[2], maxLatitude: v[3])
    }
}

// MARK: - Words (the aircraft sheet and the legend)

/// One line of the aircraft sheet.
public struct AircraftFigure: Sendable, Hashable {
    public var label: String
    public var value: String

    public init(label: String, value: String) {
        self.label = label
        self.value = value
    }
}

extension Aircraft {
    /// The callsign, else the registration, else the address.
    public var title: String {
        for name in [callsign, registration] {
            if let name = name?.trimmingCharacters(in: .whitespaces), !name.isEmpty { return name }
        }
        return hex.uppercased()
    }

    private var isICAOAddress: Bool { !hex.hasPrefix("~") }

    /// Aviation units first, metric after; lines the feed did not report are left out.
    public var figures: [AircraftFigure] {
        var out: [AircraftFigure] = []
        if onGround {
            out.append(.init(label: "Altitude", value: "On the ground"))
        } else if let altitudeFeet {
            let metres = Double(altitudeFeet) * 0.3048
            out.append(.init(label: "Altitude",
                             value: "\(Figures.number(Double(altitudeFeet))) ft (\(Figures.number(metres)) m)"))
        }
        if let groundSpeedKnots {
            out.append(.init(label: "Ground speed",
                             value: "\(Figures.number(groundSpeedKnots)) kt (\(Figures.number(groundSpeedKnots * 1.852)) km/h)"))
        }
        if let trackDegrees {
            // 359.6 rounds to 360, which is north: say 0°.
            let whole = Int(trackDegrees.rounded()) % 360
            out.append(.init(label: "Track", value: "\(whole)°"))
        }
        if let verticalRateFpm {
            // A true minus sign: it is read as a figure, not a hyphen.
            let sign = verticalRateFpm < 0 ? "−" : ""
            out.append(.init(label: "Vertical rate", value: "\(sign)\(Figures.number(Double(abs(verticalRateFpm)))) ft/min"))
        }
        if let typeCode, !typeCode.isEmpty { out.append(.init(label: "Type", value: typeCode)) }
        if let registration, !registration.isEmpty { out.append(.init(label: "Registration", value: registration)) }
        if let squawk, !squawk.isEmpty { out.append(.init(label: "Squawk", value: squawk)) }
        out.append(.init(label: isICAOAddress ? "ICAO address" : "Address (non-ICAO)", value: hex.uppercased()))
        return out
    }
}

extension Aircraft {
    /// Lines above the figures: an emergency first, the military flag, and how old this
    /// position is.
    public func notes(now: Date = .now) -> [String] {
        var out: [String] = []
        if let emergency, !emergency.isEmpty { out.append("Emergency: \(emergency)") }
        if military { out.append("Military (as the feed flags it)") }
        out.append("Position received \(AircraftLayer.age(positionAt, now: now))")
        return out
    }
}

extension AircraftLayer {
    /// "Positions from 3 min ago · 08:23 UTC": the oldest data in view, never "live".
    public func ageLine(now: Date = .now) -> String {
        guard let dataAt else { return "No recent positions here" }
        return "Positions from \(Self.age(dataAt, now: now))"
    }

    /// `Freshness.label`, except under ten seconds: "from now" reads as the future, so "just now".
    static func age(_ date: Date, now: Date) -> String {
        let relative = Freshness.relative(date, now: now)
        return "\(relative == "now" ? "just now" : relative) · \(Freshness.utcClock(date))"
    }
}
