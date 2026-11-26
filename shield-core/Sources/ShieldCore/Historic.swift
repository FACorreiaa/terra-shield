import Foundation

/// A filter group of historic sites (ADR 30). The server may add groups; a site of a group this
/// version doesn't know has `kind == nil`.
public enum HistoricKind: String, Sendable, Hashable, CaseIterable, Comparable {
    case castle, military, memorial
    /// Landmarks: palaces, cathedrals, temples, shrines, manor houses, towers, windmills and
    /// cultural World Heritage Sites not in another group (since 2026-11-26).
    case general

    public static func < (a: Self, b: Self) -> Bool {
        allCases.firstIndex(of: a)! < allCases.firstIndex(of: b)!
    }

    /// The group's name in the filters and the legend.
    public var label: String {
        switch self {
        case .castle: "Castles"
        case .military: "Military heritage"
        case .memorial: "Memorials"
        case .general: "Landmarks"
        }
    }
}

/// One site listed on Wikidata (ADR 30): a static layer, never an event. Some sites are ruins or
/// memorials, some are still in use; none of it counts toward hazards.
public struct HistoricSite: Sendable, Hashable, Identifiable {
    /// The Wikidata item ID ("Q4048").
    public var id: String
    public var name: String
    /// Wikidata's English description.
    public var description: String?
    /// Nil for a group this version doesn't know: drawn in a neutral style, never as military.
    public var kind: HistoricKind?
    /// The group as the server named it, known or not.
    public var kindName: String
    /// The Wikidata class in English ("star fort").
    public var className: String
    /// ISO 3166-1 alpha-2; nil at sea or when unknown.
    public var country: String?
    public var coordinate: Coordinate
    /// The Wikidata item: every site links to it.
    public var url: URL
    /// A UNESCO World Heritage Site, or a listed part of one.
    public var worldHeritage: Bool
    /// How many Wikipedias (and other Wikimedia wikis) have a page on it: how well known it is.
    public var sitelinks: Int

    public init(id: String, name: String, description: String?, kind: HistoricKind?, kindName: String,
                className: String, country: String?, coordinate: Coordinate, url: URL,
                worldHeritage: Bool = false, sitelinks: Int = 0) {
        self.id = id
        self.name = name
        self.description = description
        self.kind = kind
        self.kindName = kindName
        self.className = className
        self.country = country
        self.coordinate = coordinate
        self.url = url
        self.worldHeritage = worldHeritage
        self.sitelinks = sitelinks
    }
}

extension HistoricSite {
    /// "Castle", or for a group this version doesn't know, the server's name for it.
    public var kindLabel: String {
        switch kind {
        case .castle: "Castle"
        case .military: "Military heritage"
        case .memorial: "Memorial"
        case .general: "Landmark"
        case nil: kindName.prefix(1).uppercased() + kindName.dropFirst()
        }
    }

    /// How well known the site is, 0 to 14, for which chip the map keeps when two collide: the
    /// number of Wikipedias covering it on a log scale (Neuschwanstein's ~90 rank 13), a World
    /// Heritage Site two steps up.
    public var prominence: Int {
        let known = Int((log2(Double(max(0, sitelinks)) + 1) * 2).rounded(.down))
        return min(14, known + (worldHeritage ? 2 : 0))
    }

    /// Lines of the site sheet; what Wikidata does not list is left out.
    public var figures: [AircraftFigure] {
        var out: [AircraftFigure] = []
        if !className.isEmpty { out.append(.init(label: "Type", value: className.prefix(1).uppercased() + className.dropFirst())) }
        if let country, let name = Locale(identifier: "en_US").localizedString(forRegionCode: country) {
            out.append(.init(label: "Country", value: name))
        }
        if worldHeritage, !className.localizedCaseInsensitiveContains("world heritage") {
            out.append(.init(label: "UNESCO", value: "World Heritage Site"))
        }
        out.append(.init(label: "Coordinates", value: coordinate.degreesText))
        out.append(.init(label: "Wikidata item", value: id))
        return out
    }
}

/// Sites counted in one grid cell (below zoom 8).
public struct HistoricCell: Sendable, Hashable {
    public var center: Coordinate
    public var count: Int
    /// Sites per group, by the server's name, unknown groups included.
    public var kinds: [String: Int]

    public init(center: Coordinate, count: Int, kinds: [String: Int]) {
        self.center = center
        self.count = count
        self.kinds = kinds
    }

    /// Sites of the given groups. Every group shown means every site, unknown groups included.
    public func count(of shown: Set<HistoricKind>) -> Int {
        if shown.count == HistoricKind.allCases.count { return count }
        return shown.reduce(0) { $0 + (kinds[$1.rawValue] ?? 0) }
    }
}

/// `GET /v1/historic`: counts per grid cell below zoom 8, one point per site from zoom 8.
public struct HistoricLayer: Sendable, Hashable {
    public enum Content: Sendable, Hashable {
        case grid([HistoricCell], cellDegrees: Double)
        /// `truncated` when more sites matched than the server returns (10,000).
        case points([HistoricSite], truncated: Bool)
    }

    /// The first zoom the server sends sites rather than cells. The layer's own: events and
    /// detections switch at `MapViewport.detailZoom` (6).
    public static let pointsZoom = 8

    /// The most sites drawn as chips the map can pick. More cost MapKit too much on every pan
    /// (measured 2026-11-25 on a Mac, densest view: about 1,100 chips held the main thread's 95th
    /// percentile frame gap at 41 ms against 25 ms without the layer, 2,700 at 63-66 ms with
    /// hitches past 600 ms), so beyond it they are drawn as dots until the view holds fewer.
    public static let chipLimit = 1000

    public var content: Content
    /// When Shield last refreshed the sites returned (points only).
    public var updatedAt: Date?
    public var attribution: String
    public var license: String
    /// What the layer is; shown with it, verbatim.
    public var note: String

    public init(content: Content, updatedAt: Date?, attribution: String, license: String, note: String) {
        self.content = content
        self.updatedAt = updatedAt
        self.attribution = attribution
        self.license = license
        self.note = note
    }

    /// The sites, from zoom 8; none for a grid.
    public var sites: [HistoricSite] {
        if case .points(let sites, _) = content { return sites }
        return []
    }

    /// Whether the sites are drawn as chips that can be picked, or as dots (too many, or cut short).
    public var drawsChips: Bool {
        guard case .points(let sites, let truncated) = content else { return false }
        return !truncated && sites.count <= Self.chipLimit
    }

    public var isTruncated: Bool {
        if case .points(_, true) = content { return true }
        return false
    }

    /// "Listed on Wikidata · refreshed 18 Nov 2026"; sites only, a grid carries no date.
    public var listedLine: String { BaseLayer.listedLine(attribution: attribution, updatedAt: updatedAt) }

    /// The legend's line: how many sites are in `box`, "about" for a grid, whose cells on the
    /// view's edge are counted whole, and which groups when not all are shown.
    public func headline(in box: GeoBox, kinds: Set<HistoricKind>, worldHeritageOnly: Bool = false) -> String {
        var title = worldHeritageOnly ? "World Heritage sites" : "Historic sites"
        if kinds.count < HistoricKind.allCases.count {
            title += " (" + kinds.sorted { $0.label < $1.label }.map { $0.label.lowercased() }.joined(separator: ", ") + ")"
        }
        let n = count(in: box)
        guard n > 0 else { return title + " · none in this view" }
        let shown = n.formatted(.number.locale(Locale(identifier: "en_US")))
        switch content {
        case .grid: return title + " · about \(shown) in this view"
        case .points(_, true): return title + " · over \(shown) in this view"
        case .points: return title + " · \(shown) in this view"
        }
    }

    /// What to do to see more, when the sites can't be picked yet.
    public var hint: String? {
        switch content {
        case .grid: "Zoom in to see each site."
        case .points(_, true): "The best-known sites are shown. Zoom in for more."
        case .points: drawsChips ? nil : "Zoom in to pick a site."
        }
    }

    /// The answer with only the groups shown: sites of other groups, and of groups this version
    /// doesn't know, leave; a cell keeps the count of the groups shown, and leaves at none.
    public func filtered(to shown: Set<HistoricKind>, worldHeritageOnly: Bool = false) -> HistoricLayer {
        var out = self
        if worldHeritageOnly, case .points(let sites, let truncated) = content {
            // Sites carry the flag; grid cells don't, so a grid is the server's answer already.
            out.content = .points(sites.filter(\.worldHeritage), truncated: truncated)
        }
        guard shown.count < HistoricKind.allCases.count else { return out }
        switch out.content {
        case .grid(let cells, let cellDegrees):
            out.content = .grid(cells.compactMap { cell in
                let n = cell.count(of: shown)
                guard n > 0 else { return nil }
                return HistoricCell(center: cell.center, count: n,
                                    kinds: cell.kinds.filter { HistoricKind(rawValue: $0.key).map(shown.contains) ?? false })
            }, cellDegrees: cellDegrees)
        case .points(let sites, let truncated):
            out.content = .points(sites.filter { $0.kind.map(shown.contains) ?? false }, truncated: truncated)
        }
        return out
    }

    /// Sites in the box: those inside it, or for a grid the counts of the cells whose centre is.
    public func count(in box: GeoBox) -> Int {
        switch content {
        case .grid(let cells, _):
            cells.reduce(0) { $0 + (box.contains($1.center) ? $1.count : 0) }
        case .points(let sites, _):
            sites.reduce(0) { $0 + (box.contains($1.coordinate) ? 1 : 0) }
        }
    }
}

/// What the historic layer asks the server for, and which later views an answer still serves:
/// the layer changes weekly, so a pan the answer already covers is never asked again.
public struct HistoricRequest: Sendable, Hashable {
    /// Nil: the whole world (a grid below zoom 6, where the server needs no `bbox`).
    public var box: GeoBox?
    public var zoom: Int
    /// Nil: every group, those this version doesn't know included.
    public var kinds: Set<HistoricKind>?
    /// `worldHeritage=true`: only UNESCO World Heritage Sites.
    public var worldHeritage: Bool

    public var isPoints: Bool { zoom >= HistoricLayer.pointsZoom }

    public init(box: GeoBox?, zoom: Int, kinds: Set<HistoricKind>?, worldHeritage: Bool = false) {
        self.box = box
        self.zoom = zoom
        self.kinds = kinds
        self.worldHeritage = worldHeritage
    }

    /// The request for a view. Below zoom 6 the world's grid is one answer (17,448 cells at zoom
    /// 5, 129 KB gzipped, measured 2026-11-26 with 203,532 sites); at 6 and 7 half a view is added
    /// around it, so a pan stays inside. Points ask for the view alone: the densest view of a large
    /// window already passes the server's 10,000 cap (it comes back cut short, drawn as dots).
    public init(_ viewport: MapViewport, kinds: Set<HistoricKind>, worldHeritageOnly: Bool = false) {
        let box: GeoBox? = if viewport.zoom >= HistoricLayer.pointsZoom {
            Self.pointsBox(viewport.box)
        } else if viewport.zoom >= MapViewport.detailZoom {
            viewport.box.padded(by: 0.5)
        } else {
            nil
        }
        self.init(box: box, zoom: viewport.zoom, kinds: Self.sent(kinds), worldHeritage: worldHeritageOnly)
    }

    /// Whether this request's answer, `truncated` or not and holding `sites` sites, also serves
    /// `viewport` with `kinds` shown. Grid cells change size with the zoom; points don't, so any
    /// zoom from 8 inside the box is served, unless the answer was cut short or held too many
    /// sites to draw as chips: a smaller view may then hold few enough. An answer for more groups
    /// than shown serves fewer, filtered on the device (`HistoricLayer.filtered(to:)`), unless its
    /// sites were cut short: the server would send more of the groups left.
    public func serves(_ viewport: MapViewport, kinds shown: Set<HistoricKind>, worldHeritageOnly: Bool = false,
                       truncated: Bool, sites: Int = 0) -> Bool {
        let wanted = Self.sent(shown)
        var narrower: Bool
        if kinds == wanted {
            narrower = false
        } else if let wanted, kinds.map({ wanted.isSubset(of: $0) }) ?? true {
            narrower = true
        } else {
            return false
        }
        if worldHeritage != worldHeritageOnly {
            // Every site answered serves World Heritage only (sites carry the flag); the reverse,
            // or a grid (cells don't), needs the server.
            guard worldHeritageOnly, viewport.zoom >= HistoricLayer.pointsZoom else { return false }
            narrower = true
        }
        if viewport.zoom >= HistoricLayer.pointsZoom {
            guard isPoints, let box else { return false }
            if narrower && truncated { return false }
            // Asking again for the same view would bring the same answer.
            if box == Self.pointsBox(viewport.box) { return true }
            return !truncated && sites <= HistoricLayer.chipLimit && box.contains(viewport.box)
        }
        guard zoom == viewport.zoom else { return false }
        return box?.contains(viewport.box) ?? true
    }

    /// The most a points request spans: the server's cap (30° by 20°, a 400 beyond) less a hair,
    /// so the `bbox` rounded to 4 decimals never passes it.
    public static let maxPointsSpan = (longitude: 29.99, latitude: 19.99)

    /// The view, or for one wider than the server takes (a big display, a pitched camera) as much
    /// as it takes around the view's centre.
    static func pointsBox(_ box: GeoBox) -> GeoBox {
        let lonSpan = box.longitudeSpan, latSpan = box.maxLatitude - box.minLatitude
        var out = box
        if lonSpan > maxPointsSpan.longitude {
            let center = box.minLongitude + lonSpan / 2
            out.minLongitude = MapViewport.wrap(center - maxPointsSpan.longitude / 2)
            out.maxLongitude = MapViewport.wrap(center + maxPointsSpan.longitude / 2)
        }
        if latSpan > maxPointsSpan.latitude {
            let center = box.minLatitude + latSpan / 2
            out.minLatitude = center - maxPointsSpan.latitude / 2
            out.maxLatitude = center + maxPointsSpan.latitude / 2
        }
        return out
    }

    /// The `kind` filter sent for the groups shown: none when every group is, so groups added
    /// later are included.
    private static func sent(_ shown: Set<HistoricKind>) -> Set<HistoricKind>? {
        shown.count < HistoricKind.allCases.count ? shown : nil
    }
}

extension GeoBox {
    /// The box grown by `fraction` of its size on every side, kept within the map: latitudes stop
    /// at Web Mercator's limit, and a box that would wrap the world becomes the world's width.
    public func padded(by fraction: Double) -> GeoBox {
        let lonPad = longitudeSpan * fraction
        let latPad = (maxLatitude - minLatitude) * fraction
        let minLat = max(-Self.mercatorLatitudeLimit, minLatitude - latPad)
        let maxLat = min(Self.mercatorLatitudeLimit, maxLatitude + latPad)
        guard longitudeSpan + 2 * lonPad < 360 else {
            return GeoBox(minLongitude: -180, minLatitude: minLat, maxLongitude: 180, maxLatitude: maxLat)
        }
        return GeoBox(minLongitude: MapViewport.wrap(minLongitude - lonPad), minLatitude: minLat,
                      maxLongitude: MapViewport.wrap(maxLongitude + lonPad), maxLatitude: maxLat)
    }

    /// Whether `other` lies wholly inside, across the antimeridian too.
    public func contains(_ other: GeoBox) -> Bool {
        guard other.minLatitude >= minLatitude, other.maxLatitude <= maxLatitude else { return false }
        if longitudeSpan >= 360 { return true }
        guard other.longitudeSpan <= longitudeSpan else { return false }
        // Measure from this box's west edge, eastward, in [0, 360).
        var offset = (other.minLongitude - minLongitude).truncatingRemainder(dividingBy: 360)
        if offset < 0 { offset += 360 }
        return offset + other.longitudeSpan <= longitudeSpan
    }
}
