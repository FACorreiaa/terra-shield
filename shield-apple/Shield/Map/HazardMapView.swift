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
        map.delegate = coordinator
        coordinator.map = map
        map.setRegion(Coordinator.startRegion, animated: false)
        return map
    }

    private func update(_ coordinator: Coordinator) {
        // Reading these here registers them with Observation: SwiftUI calls update again
        // when any of them changes.
        coordinator.apply(
            snapshot: model.snapshot,
            selectedID: model.selectedEventID,
            localClustering: model.viewport.zoom >= MapViewport.detailZoom,
            staleSources: Set(model.sources.values.filter { $0.isStale() }.map(\.id))
        )
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

        private var events: [UUID: EventAnnotation] = [:]
        private var clusters: [String: ServerClusterAnnotation] = [:]
        private var selectedID: UUID?
        private var localClustering = false
        private var staleSources: Set<String> = []
        private var detectionOverlay: DetectionOverlay?
        private var area: (geometry: EventGeometry, event: HazardEvent, isStale: Bool, overlay: any MKOverlay)?

        init(model: LiveMapModel) {
            self.model = model
        }

        // MARK: Model → map

        func apply(snapshot: MapSnapshot?, selectedID: UUID?, localClustering: Bool, staleSources: Set<String>) {
            guard let map else { return }
            let restyle = selectedID != self.selectedID || localClustering != self.localClustering
                || staleSources != self.staleSources
            self.selectedID = selectedID
            self.localClustering = localClustering
            self.staleSources = staleSources

            if let snapshot { diff(snapshot, on: map) }
            if restyle { restyleVisibleMarkers(on: map) }
            syncSelection(on: map)
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
            // Index 0: below the detection layer (DESIGN.md §9, layer order).
            map.insertOverlay(overlay, at: 0, level: .aboveRoads)
        }

        /// Only changed annotations are touched, so a refresh doesn't flicker the map.
        private func diff(_ snapshot: MapSnapshot, on map: MKMapView) {
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
                let annotation = ServerClusterAnnotation(cluster)
                clusters[id] = annotation
                added.append(annotation)
            }

            if !removed.isEmpty { map.removeAnnotations(removed) }
            if !added.isEmpty { map.addAnnotations(added) }
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
            let padding = UIEdgeInsets(top: 104, left: 24, bottom: 24, right: 72)
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

        func mapView(_ mapView: MKMapView, viewFor annotation: any MKAnnotation) -> MKAnnotationView? {
            switch annotation {
            case let a as EventAnnotation:
                let view = mapView.dequeueReusableAnnotationView(withIdentifier: EventMarkerView.reuseID, for: a)
                if let view = view as? EventMarkerView { configure(view, for: a) }
                return view
            case let a as ServerClusterAnnotation:
                let view = mapView.dequeueReusableAnnotationView(withIdentifier: ServerClusterView.reuseID, for: a)
                (view as? ServerClusterView)?.configure(a.cluster)
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
