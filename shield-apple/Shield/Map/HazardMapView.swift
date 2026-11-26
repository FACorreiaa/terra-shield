import MapKit
import ShieldCore
import SwiftUI

/// The map canvas (DESIGN.md §9): MapKit, muted and dark, with events as severity markers and
/// clusters. The server clusters below zoom 6; MapKit clusters overlapping markers from zoom 6.
/// `MKMapView` in a representable, since SwiftUI `Map` has no clustering (ARCHITECTURE).
@MainActor
struct HazardMapView {
    let model: LiveMapModel
    /// Bump to fit the whole world (recentre control, ⌘0).
    var fitWorldRequest: Int = 0
    /// A place chosen in search; a new value moves the map there.
    var focus: MapFocus?
    /// Points of the map's bottom edge covered by other UI, on top of the safe area (iOS: the
    /// peeking sheet). The attribution must stay visible above it (DECISIONS #11).
    var bottomCover: CGFloat = 0
    /// Points of the trailing edge under the inspector (iPad), which framing leaves out.
    var trailingCover: CGFloat = 0

    func makeCoordinator() -> Coordinator { Coordinator(model: model) }

    private func makeMap(_ coordinator: Coordinator) -> MKMapView {
        let map = MKMapView()
        let config = MKStandardMapConfiguration(elevationStyle: .flat, emphasisStyle: .muted)
        config.pointOfInterestFilter = .excludingAll
        config.showsTraffic = false
        map.preferredConfiguration = config
        map.isPitchEnabled = false
        map.isRotateEnabled = false
        #if os(iOS)
        map.overrideUserInterfaceStyle = .dark
        // MapKit frames its region inside the layout margins. Only the sheet's cover should count
        // (`bottomCover`): with the safe area added too, a medium sheet left margins taller than
        // the map and a region with no height, which loads no events (2026-11-19).
        map.insetsLayoutMarginsFromSafeArea = false
        #else
        map.appearance = NSAppearance(named: .darkAqua)
        map.showsZoomControls = true
        #endif
        map.register(EventMarkerView.self, forAnnotationViewWithReuseIdentifier: EventMarkerView.reuseID)
        map.register(ServerClusterView.self, forAnnotationViewWithReuseIdentifier: ServerClusterView.reuseID)
        map.register(LocalClusterView.self, forAnnotationViewWithReuseIdentifier: LocalClusterView.reuseID)
        map.register(AircraftMarkerView.self, forAnnotationViewWithReuseIdentifier: AircraftMarkerView.reuseID)
        map.register(BaseMarkerView.self, forAnnotationViewWithReuseIdentifier: BaseMarkerView.reuseID)
        map.register(HistoricMarkerView.self, forAnnotationViewWithReuseIdentifier: HistoricMarkerView.reuseID)
        map.delegate = coordinator
        coordinator.map = map
        map.setRegion(Coordinator.startRegion, animated: false)
        return map
    }

    private func update(_ coordinator: Coordinator) {
        coordinator.trailingCover = trailingCover
        // Reading these here registers them with Observation: SwiftUI calls update again
        // when any of them changes.
        coordinator.apply(
            snapshot: model.snapshot,
            selectedID: model.selectedEventID,
            localClustering: model.viewport.zoom >= MapViewport.detailZoom,
            staleSources: Set(model.sources.values.filter { $0.isStale() }.map(\.id))
        )
        coordinator.apply(conflicts: model.conflicts)
        coordinator.apply(flights: model.flights, selectedHex: model.selectedAircraftHex)
        coordinator.apply(bases: model.bases, selectedID: model.selectedBaseID)
        coordinator.apply(historic: model.historic, chips: model.historicChips, selected: model.selectedHistoricSite)
        coordinator.apply(detections: model.detections)
        coordinator.apply(area: model.selectedGeometry, of: model.selectedEvent)
        if fitWorldRequest != coordinator.fitWorldRequest {
            coordinator.fitWorldRequest = fitWorldRequest
            coordinator.fitWorld()
        }
        if let focus, focus.id != coordinator.focusID {
            coordinator.focusID = focus.id
            coordinator.lastFocus = (focus.region, .now, bottomCover)
            coordinator.show(focus.region, bottomCover: bottomCover)
        } else if let pending = coordinator.pendingFocus {
            coordinator.show(pending.region, bottomCover: bottomCover)
        } else if let last = coordinator.lastFocus, last.cover != bottomCover,
                  Date.now.timeIntervalSince(last.at) < 1.5 {
            // The sheet the focus framed around is still moving (opening a country takes it to
            // medium): frame again as its height settles.
            coordinator.lastFocus?.cover = bottomCover
            coordinator.show(last.region, bottomCover: bottomCover)
        }
    }

    @MainActor
    final class Coordinator: NSObject, MKMapViewDelegate {
        static let worldRegion = MKCoordinateRegion(
            center: CLLocationCoordinate2D(latitude: 20, longitude: 10),
            span: MKCoordinateSpan(latitudeDelta: 150, longitudeDelta: 360)
        )

        /// The whole world, or in DEBUG `SHIELD_REGION=lat,lon,latSpan,lonSpan` for screenshots.
        static var startRegion: MKCoordinateRegion {
            #if DEBUG
            let parts = ProcessInfo.processInfo.environment["SHIELD_REGION"]?
                .split(separator: ",").compactMap { Double($0) } ?? []
            if parts.count == 4 {
                return MKCoordinateRegion(center: CLLocationCoordinate2D(latitude: parts[0], longitude: parts[1]),
                                          span: MKCoordinateSpan(latitudeDelta: parts[2], longitudeDelta: parts[3]))
            }
            #endif
            return worldRegion
        }

        let model: LiveMapModel
        weak var map: MKMapView?
        var fitWorldRequest = 0

        private var hazards = AnnotationSet(isConflict: false)
        /// The conflicts layer (ADR 27): its own events and clusters, never mixed with hazards.
        private var conflictSet = AnnotationSet(isConflict: true)
        private var events: [UUID: EventAnnotation] { hazards.events.merging(conflictSet.events) { a, _ in a } }
        private var selectedID: UUID?
        private var localClustering = false
        private var staleSources: Set<String> = []
        private var detectionOverlay: DetectionOverlay?
        /// Live flights by address (ADR 28), moved in place on each refresh.
        private var aircraft: [String: AircraftAnnotation] = [:]
        private var selectedHex: String?
        private var appliedFlights: AircraftLayer?
        /// Military bases by Wikidata ID (ADR 29), added once per load of the list.
        private var bases: [String: BaseAnnotation] = [:]
        private var selectedBaseID: String?
        private var appliedBases: BaseLayer?
        /// Historic sites by Wikidata ID (ADR 30), kept across answers: a pan adds and removes
        /// only the sites that came into or left the answer.
        private var historicSites: [String: HistoricAnnotation] = [:]
        private var appliedChips: [HistoricSite] = []
        private var selectedSiteID: String?
        private var appliedHistoric: HistoricLayer?
        private var appliedSelectedSite: HistoricSite?
        private var historicDots: HistoricDotsOverlay?
        private var area: (geometry: EventGeometry, event: HazardEvent, isStale: Bool, overlay: any MKOverlay)?

        init(model: LiveMapModel) {
            self.model = model
        }

        // MARK: Model → map

        func apply(snapshot: MapSnapshot?, selectedID: UUID?, localClustering: Bool, staleSources: Set<String>) {
            guard let map else { return }
            let reclustering = localClustering != self.localClustering
            let restyle = selectedID != self.selectedID || staleSources != self.staleSources
            self.selectedID = selectedID
            self.localClustering = localClustering
            self.staleSources = staleSources

            // A marker's `clusteringIdentifier` must never change while it is on the map: MapKit
            // keeps clustered views under their identifier, and on macOS a stale entry later
            // asks for an empty cluster with no identifier, which throws inside MapKit and
            // crashed the app on "Show the whole world" (2026-11-25). Crossing zoom 6, every
            // event marker is replaced instead, so `viewFor` configures fresh views.
            if reclustering {
                hazards.renewEvents().apply(to: map)
                conflictSet.renewEvents().apply(to: map)
            }
            if let snapshot { hazards.diff(snapshot).apply(to: map) }
            if restyle { restyleVisibleMarkers(on: map) }
            syncSelection(on: map)
        }

        /// The conflicts layer's snapshot; nil (layer off) removes every conflict annotation.
        func apply(conflicts: MapSnapshot?) {
            guard let map else { return }
            conflictSet.diff(conflicts ?? MapSnapshot(events: [], clusters: [], truncated: false, generatedAt: .now))
                .apply(to: map)
            syncSelection(on: map)
        }

        /// Adds, moves in place and removes aircraft; re-renders a marker only when it looks
        /// different or its selection changed. A no-op when neither layer nor selection changed.
        func apply(flights: AircraftLayer?, selectedHex: String?) {
            guard let map, flights != appliedFlights || selectedHex != self.selectedHex else { return }
            let oldSelected = self.selectedHex
            // Before adding: `viewFor` reads it for the new annotations.
            self.selectedHex = selectedHex
            appliedFlights = flights
            let list = flights?.aircraft ?? []
            let wanted = Set(list.map(\.hex))
            let gone = aircraft.filter { !wanted.contains($0.key) }
            if !gone.isEmpty {
                map.removeAnnotations(Array(gone.values))
                for hex in gone.keys { aircraft[hex] = nil }
            }
            var added: [AircraftAnnotation] = []
            for a in list {
                if let existing = aircraft[a.hex] {
                    let changed = !AircraftMarkerView.looksSame(existing.aircraft, a)
                    existing.update(a)
                    if changed || (a.hex == selectedHex) != (a.hex == oldSelected),
                       let view = map.view(for: existing) as? AircraftMarkerView {
                        view.configure(a, isSelected: a.hex == selectedHex)
                    }
                } else {
                    let annotation = AircraftAnnotation(a)
                    aircraft[a.hex] = annotation
                    added.append(annotation)
                }
            }
            if !added.isEmpty { map.addAnnotations(added) }
            if let old = oldSelected, old != selectedHex, let annotation = aircraft[old] {
                map.deselectAnnotation(annotation, animated: false)
                (map.view(for: annotation) as? AircraftMarkerView)?.configure(annotation.aircraft, isSelected: false)
            }
        }

        /// Adds the whole list when it arrives and removes it when the layer goes off; between
        /// those only a selection changes, which re-renders two markers. The list's equality
        /// check is cheap when unchanged: the same array storage compares equal at once.
        func apply(bases layer: BaseLayer?, selectedID: String?) {
            guard let map else { return }
            let oldSelected = selectedBaseID
            // Before adding: `viewFor` reads it for the new annotations.
            selectedBaseID = selectedID
            if layer != appliedBases {
                appliedBases = layer
                // Emptied first: removing deselects, and `didDeselect` must not take that for
                // the user clearing the selection.
                let old = Array(bases.values)
                bases = [:]
                if !old.isEmpty { map.removeAnnotations(old) }
                let added = (layer?.bases ?? []).map(BaseAnnotation.init)
                for a in added { bases[a.base.id] = a }
                if !added.isEmpty { map.addAnnotations(added) }
                return
            }
            guard selectedID != oldSelected else { return }
            for id in [oldSelected, selectedID].compactMap({ $0 }) {
                guard let annotation = bases[id] else { continue }
                if id == oldSelected { map.deselectAnnotation(annotation, animated: false) }
                (map.view(for: annotation) as? BaseMarkerView)?.configure(annotation.base, isSelected: id == selectedID)
            }
        }

        /// From zoom 8, adds and removes the chips that entered or left (every site, or past the
        /// chip limit the best known in the view, over the dots), keeping the rest; the dots and
        /// the grid are an overlay swapped with the answer. The selected site keeps its marker while
        /// the answer leaves it out (a grid, or a pan away), so the sheet's site stays on the map.
        func apply(historic layer: HistoricLayer?, chips: [HistoricSite], selected: HistoricSite?) {
            guard let map, layer != appliedHistoric || chips != appliedChips || selected != appliedSelectedSite
            else { return }
            appliedChips = chips
            let oldSelected = selectedSiteID
            // Before adding: `viewFor` reads it for the new annotations.
            selectedSiteID = selected?.id
            appliedSelectedSite = selected
            if layer != appliedHistoric {
                appliedHistoric = layer
                apply(historicDots: layer)
            }

            var wanted = chips
            if let selected, !wanted.contains(where: { $0.id == selected.id }) { wanted.append(selected) }
            let ids = Set(wanted.map(\.id))
            // Out of the dictionary before MapKit hears of it: removing the selected annotation
            // deselects it at once, and `didDeselect` must not take that for the user's tap.
            let gone = historicSites.filter { !ids.contains($0.key) }
            for id in gone.keys { historicSites[id] = nil }
            if !gone.isEmpty { map.removeAnnotations(Array(gone.values)) }
            var added: [HistoricAnnotation] = []
            for site in wanted where historicSites[site.id]?.site != site {
                if let old = historicSites[site.id] {
                    historicSites[site.id] = nil
                    map.removeAnnotation(old)
                }
                let annotation = HistoricAnnotation(site)
                historicSites[site.id] = annotation
                added.append(annotation)
            }
            if !added.isEmpty { map.addAnnotations(added) }

            if selectedSiteID != oldSelected {
                for id in [oldSelected, selectedSiteID].compactMap({ $0 }) {
                    guard let annotation = historicSites[id] else { continue }
                    if id == oldSelected { map.deselectAnnotation(annotation, animated: false) }
                    (map.view(for: annotation) as? HistoricMarkerView)?
                        .configure(annotation.site, isSelected: id == selectedSiteID)
                }
            }
            // MapKit's own selection follows the model's, as events' does: a site the app chose
            // (`SHIELD_SITE`, an annotation replaced by a changed site) is then cleared by a tap on
            // empty map like one the user tapped.
            if let selectedSiteID, let annotation = historicSites[selectedSiteID],
               !map.selectedAnnotations.contains(where: { $0 === annotation }) {
                map.selectAnnotation(annotation, animated: false)
            }
        }

        /// The grid, or sites too many for chips, as dots under everything else on the map: the
        /// selected event's footprint, the detections and every marker.
        private func apply(historicDots layer: HistoricLayer?) {
            guard let map else { return }
            if let old = historicDots { map.removeOverlay(old) }
            historicDots = nil
            guard let layer, !layer.drawsChips else { return }
            switch layer.content {
            case .grid(let cells, _) where cells.isEmpty: return
            case .points(let sites, _) where sites.isEmpty: return
            default: break
            }
            let overlay = HistoricDotsOverlay(layer.content)
            map.insertOverlay(overlay, at: 0, level: .aboveRoads)
            historicDots = overlay
        }

        /// Swaps the detection overlay when its data changes; an empty layer removes it.
        func apply(detections: DetectionLayer) {
            guard let map, detections != detectionOverlay?.layer ?? .empty else { return }
            if let old = detectionOverlay { map.removeOverlay(old) }
            detectionOverlay = nil
            guard !detections.isEmpty else { return }
            let overlay = DetectionOverlay(detections)
            map.addOverlay(overlay, level: .aboveRoads)
            detectionOverlay = overlay
        }

        /// Shows the selected event's footprint under everything else; replaces it when the
        /// selection or the event changes.
        func apply(area geometry: EventGeometry?, of event: HazardEvent?) {
            guard let map else { return }
            let stale = event.map { staleSources.contains($0.source) } ?? false
            if let area, area.geometry == geometry, area.event == event, area.isStale == stale { return }
            if let old = area?.overlay { map.removeOverlay(old) }
            area = nil
            guard let geometry, let event, let overlay = EventArea.overlay(for: geometry) else { return }
            // Recorded first: MapKit asks for the renderer during the insert, and caches it.
            area = (geometry, event, stale, overlay)
            // Below the detection layer (DESIGN.md §9, layer order), above the historic grid.
            if let historicDots {
                map.insertOverlay(overlay, above: historicDots)
            } else {
                map.insertOverlay(overlay, at: 0, level: .aboveRoads)
            }
        }

        private func restyleVisibleMarkers(on map: MKMapView) {
            for annotation in events.values {
                guard let view = map.view(for: annotation) as? EventMarkerView else { continue }
                configure(view, for: annotation)
            }
        }

        private func syncSelection(on map: MKMapView) {
            let current = map.selectedAnnotations.compactMap { $0 as? EventAnnotation }.first
            guard current?.event.id != selectedID else { return }
            if let selectedID, let annotation = events[selectedID] {
                map.selectAnnotation(annotation, animated: true)
            } else if let current {
                map.deselectAnnotation(current, animated: true)
            }
        }

        private func configure(_ view: EventMarkerView, for annotation: EventAnnotation) {
            let e = annotation.event
            view.configure(e, isStale: staleSources.contains(e.source), isSelected: e.id == selectedID,
                           localClustering: localClustering)
        }

        // MARK: Camera

        func fitWorld() {
            map?.setRegion(Self.worldRegion, animated: true)
        }

        var focusID: UUID?
        /// The last focus, when it came and the cover it was framed for.
        var lastFocus: (region: MKCoordinateRegion, at: Date, cover: CGFloat)?
        /// The inspector's cover of the trailing edge (iPad). MapKit leaves the bottom margin out
        /// of `setVisibleMapRect` by itself but not this one, so `show` pads for it.
        var trailingCover: CGFloat = 0
        /// A focus that came before the map had a size.
        var pendingFocus: (region: MKCoordinateRegion, cover: CGFloat)?

        /// A searched place or a country, framed in the part of the map nothing covers. The
        /// sheet is the map's bottom layout margin (`bottomCover`), which MapKit leaves out by
        /// itself; the padding keeps clear of the status capsule and the controls. A point-sized
        /// region (a small town) still shows its surroundings. Works across ±180°, where the
        /// rect simply runs past the world's edge.
        func show(_ region: MKCoordinateRegion, bottomCover: CGFloat) {
            guard let map else { return }
            // Before the map has a real size (launch), keep the focus for the next update.
            guard map.bounds.width > 120, map.bounds.height > 240 else {
                pendingFocus = (region, bottomCover)
                return
            }
            pendingFocus = nil
            let latSpan = max(region.span.latitudeDelta, 0.5)
            let lonSpan = max(region.span.longitudeDelta, 0.5)
            let north = MKMapPoint(CLLocationCoordinate2D(latitude: min(region.center.latitude + latSpan / 2, 85),
                                                          longitude: region.center.longitude))
            let south = MKMapPoint(CLLocationCoordinate2D(latitude: max(region.center.latitude - latSpan / 2, -85),
                                                          longitude: region.center.longitude))
            let width = MKMapSize.world.width * lonSpan / 360
            let center = MKMapPoint(region.center)
            let rect = MKMapRect(x: center.x - width / 2, y: north.y, width: width, height: south.y - north.y)
            #if os(iOS)
            let padding = UIEdgeInsets(top: 104, left: 24, bottom: 24, right: 72 + trailingCover)
            #else
            let padding = NSEdgeInsets(top: 72, left: 32, bottom: 56, right: 80)
            #endif
            map.setVisibleMapRect(rect, edgePadding: padding, animated: true)
        }

        /// Zooms to a server cluster's members. Co-located members have a zero-area box, so the
        /// fit never goes below a quarter of the current span: two zoom levels in, enough for the
        /// server to split or, from zoom 6, for MapKit to take over (backend, 2026-11-18).
        private func zoom(to box: GeoBox, on map: MKMapView) {
            let current = map.region.span
            let lonSpan = max(box.longitudeSpan * 1.4, current.longitudeDelta / 4)
            let latSpan = max((box.maxLatitude - box.minLatitude) * 1.4, current.latitudeDelta / 4)
            let center = CLLocationCoordinate2D(
                latitude: (box.minLatitude + box.maxLatitude) / 2,
                longitude: (box.minLongitude + box.maxLongitude) / 2
            )
            let region = MKCoordinateRegion(center: center, span: MKCoordinateSpan(latitudeDelta: latSpan, longitudeDelta: lonSpan))
            map.setRegion(map.regionThatFits(region), animated: true)
        }

        // MARK: MKMapViewDelegate

        /// DESIGN.md §13 `arrive`: a live arrival fades in and scales up from 0.6 (a plain fade
        /// under Reduce Motion). Only events the model names as arrivals; a reload's markers
        /// just appear.
        func mapView(_ mapView: MKMapView, didAdd views: [MKAnnotationView]) {
            for view in views {
                guard let a = view.annotation as? EventAnnotation, model.arrivals.contains(a.event.id) else { continue }
                model.arrived(a.event.id)
                Self.arrive(view)
            }
        }

        private static func arrive(_ view: MKAnnotationView) {
            #if os(iOS)
            let reduceMotion = UIAccessibility.isReduceMotionEnabled
            let layer = view.layer
            // UIKit layers scale about their centre.
            let from = CATransform3DMakeScale(0.6, 0.6, 1)
            #else
            let reduceMotion = NSWorkspace.shared.accessibilityDisplayShouldReduceMotion
            view.wantsLayer = true
            guard let layer = view.layer else { return }
            // AppKit layers anchor at the corner: scale about the centre explicitly.
            let mid = CGPoint(x: layer.bounds.midX, y: layer.bounds.midY)
            let from = CATransform3DConcat(
                CATransform3DConcat(CATransform3DMakeTranslation(-mid.x, -mid.y, 0), CATransform3DMakeScale(0.6, 0.6, 1)),
                CATransform3DMakeTranslation(mid.x, mid.y, 0))
            #endif
            let fade = CABasicAnimation(keyPath: "opacity")
            fade.fromValue = 0
            fade.toValue = 1
            var animations: [CAAnimation] = [fade]
            if !reduceMotion {
                let grow = CABasicAnimation(keyPath: "transform")
                grow.fromValue = NSValue(caTransform3D: from)
                grow.toValue = NSValue(caTransform3D: CATransform3DIdentity)
                animations.append(grow)
            }
            let group = CAAnimationGroup()
            group.animations = animations
            group.duration = 0.4 // tokens.json motion.arrive
            group.timingFunction = CAMediaTimingFunction(name: .easeOut)
            layer.add(group, forKey: "arrive")
        }

        func mapView(_ mapView: MKMapView, viewFor annotation: any MKAnnotation) -> MKAnnotationView? {
            switch annotation {
            case let a as EventAnnotation:
                let view = mapView.dequeueReusableAnnotationView(withIdentifier: EventMarkerView.reuseID, for: a)
                if let view = view as? EventMarkerView { configure(view, for: a) }
                return view
            case let a as ServerClusterAnnotation:
                let view = mapView.dequeueReusableAnnotationView(withIdentifier: ServerClusterView.reuseID, for: a)
                (view as? ServerClusterView)?.configure(a.cluster, isConflict: a.isConflict)
                return view
            case let a as AircraftAnnotation:
                let view = mapView.dequeueReusableAnnotationView(withIdentifier: AircraftMarkerView.reuseID, for: a)
                (view as? AircraftMarkerView)?.configure(a.aircraft, isSelected: a.aircraft.hex == selectedHex)
                return view
            case let a as BaseAnnotation:
                let view = mapView.dequeueReusableAnnotationView(withIdentifier: BaseMarkerView.reuseID, for: a)
                (view as? BaseMarkerView)?.configure(a.base, isSelected: a.base.id == selectedBaseID)
                return view
            case let a as HistoricAnnotation:
                let view = mapView.dequeueReusableAnnotationView(withIdentifier: HistoricMarkerView.reuseID, for: a)
                (view as? HistoricMarkerView)?.configure(a.site, isSelected: a.site.id == selectedSiteID)
                return view
            case let a as MKClusterAnnotation:
                let view = mapView.dequeueReusableAnnotationView(withIdentifier: LocalClusterView.reuseID, for: a)
                (view as? LocalClusterView)?.configure(a)
                return view
            default:
                return nil
            }
        }

        func mapView(_ mapView: MKMapView, rendererFor overlay: any MKOverlay) -> MKOverlayRenderer {
            if let overlay = overlay as? DetectionOverlay { return DetectionRenderer(overlay) }
            if let overlay = overlay as? HistoricDotsOverlay { return HistoricDotsRenderer(overlay) }
            if let area, overlay === area.overlay { return EventArea.renderer(for: overlay, event: area.event, isStale: staleSources.contains(area.event.source)) }
            return MKOverlayRenderer(overlay: overlay)
        }

        func mapView(_ mapView: MKMapView, regionDidChangeAnimated animated: Bool) {
            let r = mapView.region
            model.viewportDidChange(MapViewport(
                centerLatitude: r.center.latitude, centerLongitude: r.center.longitude,
                latitudeSpan: r.span.latitudeDelta, longitudeSpan: r.span.longitudeDelta,
                widthPoints: mapView.bounds.width
            ))
        }

        func mapView(_ mapView: MKMapView, didSelect view: MKAnnotationView) {
            switch view.annotation {
            case let a as EventAnnotation:
                model.selectedEventID = a.event.id
            case let a as AircraftAnnotation:
                model.selectedAircraftHex = a.aircraft.hex
            case let a as BaseAnnotation:
                model.selectedBaseID = a.base.id
            case let a as HistoricAnnotation:
                model.selectedHistoricID = a.site.id
            case let a as ServerClusterAnnotation:
                mapView.deselectAnnotation(a, animated: false)
                zoom(to: a.cluster.box, on: mapView)
            case let a as MKClusterAnnotation:
                mapView.deselectAnnotation(a, animated: false)
                mapView.showAnnotations(a.memberAnnotations, animated: true)
            default:
                break
            }
        }

        func mapView(_ mapView: MKMapView, didDeselect view: MKAnnotationView) {
            guard let annotation = view.annotation else { return }
            if let a = annotation as? AircraftAnnotation {
                // A tap on empty map clears it; a refresh never replaces the annotation.
                if model.selectedAircraftHex == a.aircraft.hex, aircraft[a.aircraft.hex] === a {
                    model.selectedAircraftHex = nil
                }
                return
            }
            if let a = annotation as? BaseAnnotation {
                // A tap on empty map clears it; a reload of the list replaces every annotation.
                if model.selectedBaseID == a.base.id, bases[a.base.id] === a {
                    model.selectedBaseID = nil
                }
                return
            }
            if let a = annotation as? HistoricAnnotation {
                // A tap on empty map clears it; an answer that drops or replaces the annotation
                // has already taken it out of `historicSites`.
                if model.selectedHistoricID == a.site.id, historicSites[a.site.id] === a {
                    model.selectedHistoricID = nil
                }
                return
            }
            // A refresh replaces a changed event's annotation, which deselects the old one; only
            // a deselect of the live annotation means the user cleared the selection.
            guard let a = annotation as? EventAnnotation, events[a.event.id] === a,
                  model.selectedEventID == a.event.id else { return }
            model.selectedEventID = nil
        }
    }
}

#if os(iOS)
extension HazardMapView: UIViewRepresentable {
    func makeUIView(context: Context) -> MKMapView { makeMap(context.coordinator) }
    func updateUIView(_ uiView: MKMapView, context: Context) {
        // Lifts the attribution above whatever covers the map's bottom edge (the peeking sheet),
        // plus the home indicator's inset, which the sheet sits above. The window's inset, not
        // the map's: the map's own safe area grows strangely under a medium sheet.
        let cover = bottomCover + (uiView.window?.safeAreaInsets.bottom ?? 0)
        if uiView.directionalLayoutMargins.bottom != cover {
            uiView.directionalLayoutMargins.bottom = cover
        }
        update(context.coordinator)
    }
}
#else
extension HazardMapView: NSViewRepresentable {
    func makeNSView(context: Context) -> MKMapView { makeMap(context.coordinator) }
    func updateNSView(_ nsView: MKMapView, context: Context) { update(context.coordinator) }
}
#endif

/// One layer's annotations by id. Only changed ones are touched, so a refresh doesn't flicker
/// the map.
@MainActor
private struct AnnotationSet {
    let isConflict: Bool
    var events: [UUID: EventAnnotation] = [:]
    var clusters: [String: ServerClusterAnnotation] = [:]

    init(isConflict: Bool) { self.isConflict = isConflict }

    /// What to take off the map and put on it. Applied by the caller once the set's mutation has
    /// ended: removing the selected annotation calls `didDeselect` at once, which reads the
    /// sets, and a read during a `mutating` call is an exclusivity violation (a trap).
    @MainActor struct Changes {
        var removed: [any MKAnnotation] = []
        var added: [any MKAnnotation] = []

        func apply(to map: MKMapView) {
            if !removed.isEmpty { map.removeAnnotations(removed) }
            if !added.isEmpty { map.addAnnotations(added) }
        }
    }

    /// Replaces every event annotation with a new one for the same event, so MapKit drops the
    /// old views and asks `viewFor` again. Replaced in the set before removal: `didDeselect`
    /// then sees the old selected annotation is no longer the live one and keeps the selection.
    mutating func renewEvents() -> Changes {
        let old = Array(events.values)
        events = events.mapValues { EventAnnotation($0.event) }
        return Changes(removed: old, added: Array(events.values))
    }

    mutating func diff(_ snapshot: MapSnapshot) -> Changes {
        var removed: [any MKAnnotation] = []
        var added: [any MKAnnotation] = []

        let incoming = Dictionary(snapshot.events.map { ($0.id, $0) }, uniquingKeysWith: { a, _ in a })
        for (id, annotation) in events where incoming[id] != annotation.event {
            removed.append(annotation)
            events[id] = nil
        }
        for (id, event) in incoming where events[id] == nil {
            let annotation = EventAnnotation(event)
            events[id] = annotation
            added.append(annotation)
        }

        let incomingClusters = Dictionary(snapshot.clusters.map { ($0.id, $0) }, uniquingKeysWith: { a, _ in a })
        for (id, annotation) in clusters where incomingClusters[id] != annotation.cluster {
            removed.append(annotation)
            clusters[id] = nil
        }
        for (id, cluster) in incomingClusters where clusters[id] == nil {
            let annotation = ServerClusterAnnotation(cluster, isConflict: isConflict)
            clusters[id] = annotation
            added.append(annotation)
        }

        return Changes(removed: removed, added: added)
    }
}
