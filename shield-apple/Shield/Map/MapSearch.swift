import MapKit
import ShieldCore
import SwiftUI

/// Search on the live map (DESIGN.md §10 shells: peek search field, macOS ⌘F). No search
/// endpoint exists, so it covers what the app can answer itself (owner, 2026-11-19):
/// - places, through MapKit's completer (Apple's geocoder, no key), which move the map;
/// - countries, from `/v1/countries`, which open the country's overview (S3.4);
/// - events in the current view, by title.
@MainActor @Observable
final class MapSearch: NSObject {
    /// What the person typed.
    var text = "" {
        didSet { if text != oldValue { textDidChange() } }
    }
    /// The field has focus. Results show while editing or while there is text.
    var isEditing = false
    var isActive: Bool { isEditing || !text.isEmpty }

    private(set) var places: [Place] = []
    /// Every country the API has a boundary for; empty until loaded, and search works without
    /// them.
    var countries: [Country] = []
    private(set) var placesFailed = false
    /// A place is being resolved to a region after a tap.
    private(set) var resolvingPlace: Place.ID?

    struct Place: Identifiable {
        let id = UUID()
        let title: String
        let subtitle: String
        fileprivate let completion: MKLocalSearchCompletion
    }

    /// Places need this many characters, so a single letter doesn't fire a lookup.
    static let minimumPlaceQuery = 2

    @ObservationIgnored private let completer: MKLocalSearchCompleter = {
        let c = MKLocalSearchCompleter()
        // Cities, regions, countries. Not shops or landmarks: this is a hazard map.
        c.resultTypes = .address
        return c
    }()

    override init() {
        super.init()
        completer.delegate = self
    }

    private var trimmed: String { text.trimmingCharacters(in: .whitespacesAndNewlines) }

    private func textDidChange() {
        placesFailed = false
        if trimmed.count >= Self.minimumPlaceQuery {
            completer.queryFragment = trimmed
        } else {
            completer.cancel()
            places = []
        }
    }

    /// Events in `snapshot` whose title contains the text, ignoring case and accents, in the
    /// map's order (severity, then recency).
    func events(in snapshot: MapSnapshot?) -> [HazardEvent] {
        guard let snapshot, !trimmed.isEmpty else { return [] }
        return snapshot.events.filter { $0.title.localizedStandardContains(trimmed) }
    }

    /// Countries whose name or ISO code matches the text.
    var countryMatches: [Country] { Country.matching(text, in: countries) }

    /// The region that frames `place`, or nil if MapKit can't place it.
    func region(for place: Place) async -> MKCoordinateRegion? {
        resolvingPlace = place.id
        defer { resolvingPlace = nil }
        let request = MKLocalSearch.Request(completion: place.completion)
        request.resultTypes = .address
        guard let response = try? await MKLocalSearch(request: request).start() else { return nil }
        return response.boundingRegion
    }

    /// Clears the text and ends editing.
    func end() {
        text = ""
        isEditing = false
    }
}

extension MapSearch: MKLocalSearchCompleterDelegate {
    nonisolated func completerDidUpdateResults(_ completer: MKLocalSearchCompleter) {
        // Our one completer, which calls back on the main thread; read it through self so the
        // non-Sendable parameter never crosses into the main actor.
        MainActor.assumeIsolated {
            places = self.completer.results.prefix(6).map {
                Place(title: $0.title, subtitle: $0.subtitle, completion: $0)
            }
        }
    }

    nonisolated func completer(_ completer: MKLocalSearchCompleter, didFailWithError error: any Error) {
        MainActor.assumeIsolated {
            // MapKit reports a query with no matches as an error too; both read "no places".
            places = []
            placesFailed = (error as? MKError)?.code != .placemarkNotFound
        }
    }
}

extension MKCoordinateRegion {
    /// The region around `box`, also when it crosses ±180°. A box as wide as the world keeps
    /// just under 360° of longitude, which MapKit accepts.
    init(_ box: GeoBox) {
        let lonSpan = min(box.longitudeSpan, 359)
        var centerLon = box.minLongitude + box.longitudeSpan / 2
        if centerLon > 180 { centerLon -= 360 }
        self.init(center: CLLocationCoordinate2D(latitude: (box.minLatitude + box.maxLatitude) / 2, longitude: centerLon),
                  span: MKCoordinateSpan(latitudeDelta: min(box.maxLatitude - box.minLatitude, 170), longitudeDelta: lonSpan))
    }
}

/// Moves the map to a region; a new `id` each time, so the same place can be chosen twice.
struct MapFocus: Equatable {
    let id = UUID()
    let region: MKCoordinateRegion

    static func == (a: MapFocus, b: MapFocus) -> Bool { a.id == b.id }
}

// MARK: - Views

/// The search field: magnifier, text, clear button and, on iPhone while editing, Cancel.
struct MapSearchField: View {
    @Bindable var search: MapSearch
    var focused: FocusState<Bool>.Binding

    var body: some View {
        HStack(spacing: ShieldSpace.s2) {
            HStack(spacing: ShieldSpace.s2) {
                Image(systemName: "magnifyingglass")
                    .foregroundStyle(Color.muted)
                    .accessibilityHidden(true)
                TextField("Search places or events", text: $search.text)
                    .textFieldStyle(.plain)
                    .focused(focused)
                    .submitLabel(.search)
                    .autocorrectionDisabled()
                    #if os(iOS)
                    .textInputAutocapitalization(.never)
                    #endif
                if !search.text.isEmpty {
                    Button {
                        search.text = ""
                    } label: {
                        Image(systemName: "xmark.circle.fill").foregroundStyle(Color.muted)
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel("Clear search")
                }
            }
            .font(ShieldFont.body)
            .foregroundStyle(Color.ink)
            .padding(.horizontal, ShieldSpace.s3)
            .frame(minHeight: 36)
            .background(Color.ink.opacity(ShieldAlpha.hairline), in: RoundedRectangle(cornerRadius: ShieldRadius.sm))

            #if os(iOS)
            if search.isEditing {
                Button("Cancel") {
                    search.end()
                    focused.wrappedValue = false
                }
                .font(ShieldFont.body)
                .foregroundStyle(Color.accent)
                .transition(.move(edge: .trailing).combined(with: .opacity))
            }
            #endif
        }
        .animation(ShieldMotion.standard, value: search.isEditing)
        #if os(iOS)
        // On iPhone, focusing the field opens the results (and the sheet), as in Maps. On the Mac
        // the field can hold focus at launch; results wait for text there, so the list stays.
        .onChange(of: focused.wrappedValue, initial: true) { _, isFocused in
            search.isEditing = isFocused
        }
        #endif
    }
}

/// What the search finds: events in view first (the map's own data), then places.
struct MapSearchResults: View {
    let search: MapSearch
    let model: LiveMapModel
    var selectEvent: (HazardEvent) -> Void
    var selectPlace: (MapSearch.Place) -> Void
    var selectCountry: (Country) -> Void

    var body: some View {
        let events = search.events(in: model.snapshot)
        let countries = search.countryMatches
        let clustered = model.snapshot?.clusters.reduce(0) { $0 + $1.count } ?? 0
        ScrollView {
            LazyVStack(alignment: .leading, spacing: 0) {
                if search.text.trimmingCharacters(in: .whitespaces).isEmpty {
                    hint("Type a country, a place to move the map, or part of an event’s title.")
                } else {
                    // Only when something matches: a country list that failed to load mustn't
                    // read as "no such country".
                    if !countries.isEmpty {
                        section("Countries") {
                            ForEach(countries) { country in
                                Button { selectCountry(country) } label: { countryRow(country) }
                                    .buttonStyle(.plain)
                            }
                        }
                    }
                    section("Events in view") {
                        if events.isEmpty {
                            hint("No event in view matches.")
                        }
                        ForEach(events) { event in
                            Button { selectEvent(event) } label: {
                                EventRow(event: event, source: model.sources[event.source],
                                         isStale: model.isStale(event))
                                    .contentShape(Rectangle())
                            }
                            .buttonStyle(.plain)
                            .padding(.horizontal, ShieldSpace.s5)
                            .padding(.vertical, ShieldSpace.s2)
                        }
                        if clustered > 0 {
                            hint("Events grouped on the map aren’t searched. Zoom in to include them.")
                        }
                    }
                    section("Places") {
                        if search.placesFailed {
                            hint("Place search isn’t available right now.")
                        } else if search.places.isEmpty {
                            hint(search.text.count < MapSearch.minimumPlaceQuery
                                 ? "Keep typing to search places." : "No places found.")
                        }
                        ForEach(search.places) { place in
                            Button { selectPlace(place) } label: { placeRow(place) }
                                .buttonStyle(.plain)
                        }
                    }
                }
            }
            .padding(.vertical, ShieldSpace.s3)
        }
        .scrollDismissesKeyboard(.immediately)
    }

    private func section(_ title: String, @ViewBuilder content: () -> some View) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            Text(title.uppercased())
                .font(ShieldFont.micro)
                .tracking(ShieldTracking.micro)
                .foregroundStyle(Color.muted)
                .padding(.horizontal, ShieldSpace.s5)
                .padding(.top, ShieldSpace.s3)
                .padding(.bottom, ShieldSpace.s1)
                .accessibilityAddTraits(.isHeader)
            content()
        }
    }

    private func countryRow(_ country: Country) -> some View {
        HStack(spacing: ShieldSpace.s3) {
            Image(systemName: "flag")
                .foregroundStyle(Color.muted)
                .frame(width: 32)
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 2) {
                Text(country.name).font(ShieldFont.body).foregroundStyle(Color.ink)
                Text(country.activeEvents == 0 ? "No active events"
                     : country.activeEvents == 1 ? "1 active event" : "\(country.activeEvents) active events")
                    .font(ShieldFont.meta).foregroundStyle(Color.muted)
            }
            Spacer(minLength: 0)
            if let worst = country.maxSeverity {
                SeverityBadge(severity: worst)
            }
        }
        .padding(.horizontal, ShieldSpace.s5)
        .padding(.vertical, ShieldSpace.s2)
        .contentShape(Rectangle())
        .accessibilityElement(children: .combine)
        .accessibilityHint("Opens the country’s overview")
    }

    private func placeRow(_ place: MapSearch.Place) -> some View {
        HStack(spacing: ShieldSpace.s3) {
            Image(systemName: "mappin.and.ellipse")
                .foregroundStyle(Color.muted)
                .frame(width: 32)
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 2) {
                Text(place.title).font(ShieldFont.body).foregroundStyle(Color.ink)
                if !place.subtitle.isEmpty {
                    Text(place.subtitle).font(ShieldFont.meta).foregroundStyle(Color.muted)
                }
            }
            Spacer(minLength: 0)
            if search.resolvingPlace == place.id { ProgressView().controlSize(.small) }
        }
        .padding(.horizontal, ShieldSpace.s5)
        .padding(.vertical, ShieldSpace.s2)
        .contentShape(Rectangle())
        .accessibilityElement(children: .combine)
        .accessibilityHint("Moves the map there")
    }

    private func hint(_ text: String) -> some View {
        Text(text)
            .font(ShieldFont.meta)
            .foregroundStyle(Color.muted)
            .fixedSize(horizontal: false, vertical: true)
            .padding(.horizontal, ShieldSpace.s5)
            .padding(.vertical, ShieldSpace.s2)
    }
}
