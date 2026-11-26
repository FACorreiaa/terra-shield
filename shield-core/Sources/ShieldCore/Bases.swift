import Foundation

/// One installation listed on Wikidata (ADR 29): a static layer, never an event. Nothing about
/// it counts toward hazards, filters, the stat strip or the list.
public struct MilitaryBase: Sendable, Hashable, Identifiable {
    /// The glyph group. The server may add groups; an unknown one is drawn as `base`.
    public enum Kind: String, Sendable, CaseIterable {
        case air, naval, missile, radar, base

        public init(serverValue: String) {
            self = Kind(rawValue: serverValue) ?? .base
        }
    }

    /// The Wikidata item ID ("Q161348").
    public var id: String
    public var name: String
    /// Wikidata's English description.
    public var description: String?
    public var kind: Kind
    /// The Wikidata class in English ("air base").
    public var className: String
    /// Who runs it (Wikidata P137), when listed.
    public var operatorName: String?
    /// ISO 3166-1 alpha-2; nil at sea or when unknown.
    public var country: String?
    public var coordinate: Coordinate
    /// The Wikidata item: every base links to it.
    public var url: URL

    public init(id: String, name: String, description: String?, kind: Kind, className: String,
                operatorName: String?, country: String?, coordinate: Coordinate, url: URL) {
        self.id = id
        self.name = name
        self.description = description
        self.kind = kind
        self.className = className
        self.operatorName = operatorName
        self.country = country
        self.coordinate = coordinate
        self.url = url
    }
}

/// `GET /v1/bases` without a bbox: every base, a few thousand points, refreshed weekly.
public struct BaseLayer: Sendable, Hashable {
    public var bases: [MilitaryBase]
    /// When Shield last refreshed the layer from Wikidata; nil when empty.
    public var updatedAt: Date?
    public var attribution: String
    public var license: String
    /// What a listing means; shown with the layer, verbatim.
    public var note: String

    public init(bases: [MilitaryBase], updatedAt: Date?, attribution: String, license: String, note: String) {
        self.bases = bases
        self.updatedAt = updatedAt
        self.attribution = attribution
        self.license = license
        self.note = note
    }

    /// Bases inside the box, across the antimeridian too.
    public func count(in box: GeoBox) -> Int {
        bases.reduce(0) { $0 + (box.contains($1.coordinate) ? 1 : 0) }
    }

    /// "Listed on Wikidata · refreshed 18 Nov 2026": a listing, never "active" (ADR 29).
    public var listedLine: String { Self.listedLine(attribution: attribution, updatedAt: updatedAt) }

    /// "Listed on Wikidata · refreshed 18 Nov 2026", for the layers listed on Wikidata.
    static func listedLine(attribution: String, updatedAt: Date?) -> String {
        guard let updatedAt else { return "Listed on \(attribution)" }
        let f = DateFormatter()
        f.locale = Locale(identifier: "en_GB")
        f.timeZone = TimeZone(identifier: "UTC")
        f.dateFormat = "d MMM yyyy"
        return "Listed on \(attribution) · refreshed \(f.string(from: updatedAt))"
    }
}

extension MilitaryBase {
    /// Lines of the base sheet; what Wikidata does not list is left out.
    public var figures: [AircraftFigure] {
        var out: [AircraftFigure] = []
        if !className.isEmpty { out.append(.init(label: "Type", value: className.prefix(1).uppercased() + className.dropFirst())) }
        if let operatorName, !operatorName.isEmpty { out.append(.init(label: "Operated by", value: operatorName)) }
        if let country, let name = Locale(identifier: "en_US").localizedString(forRegionCode: country) {
            out.append(.init(label: "Country", value: name))
        }
        out.append(.init(label: "Coordinates", value: coordinate.degreesText))
        out.append(.init(label: "Wikidata item", value: id))
        return out
    }
}

extension Coordinate {
    /// "49.4369° N, 7.6000° E": four decimals, about 11 m.
    public var degreesText: String {
        let lat = String(format: "%.4f° %@", abs(latitude), latitude < 0 ? "S" : "N")
        let lon = String(format: "%.4f° %@", abs(longitude), longitude < 0 ? "W" : "E")
        return "\(lat), \(lon)"
    }
}
