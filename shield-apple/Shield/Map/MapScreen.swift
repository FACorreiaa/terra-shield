import MapKit
import ShieldCore
import SwiftUI

/// The global live map (DESIGN.md §10.1): the map always on screen, a status capsule on top and
/// floating controls. Every state keeps the map: loading shows a progress line, failures keep
/// the last data and say how old it is.
///
/// Two shells (DESIGN.md §10). Compact width (iPhone) is the full-screen map under a sheet with
/// three detents. macOS and regular width (iPad) put the stat strip and the event list in a
/// sidebar and show the selected event as a card on the map.
struct MapScreen: View {
    @Bindable var model: LiveMapModel
    @State private var fitWorldRequest = 0
    @State private var search = MapSearch()
    @State private var showsFilters = false
    @State private var showsSources = false
    /// Where a chosen place moves the map.
    @State private var focus: MapFocus?
    /// The country whose overview is open (S3.4).
    @State private var country: CountryOverview?
    /// The sidebar holds the list and its notices; the map repeats them only when it's hidden.
    @State private var columnVisibility = NavigationSplitViewVisibility.automatic
    @FocusState private var sidebarSearchFocused: Bool
    /// The sidebar's section (DESIGN.md §10, macOS and iPad).
    @State private var section = SidebarSection.monitor
    /// The countries list failed to load (the Countries section offers Retry).
    @State private var countriesFailed = false
    /// The map layer's trailing safe-area inset. iPadOS gives the inspector as one, and the map,
    /// which ignores the safe area, runs on underneath it; framing has to leave that part out.
    @State private var mapTrailingInset: CGFloat = 0
    @Environment(\.accessibilityReduceMotion) private var reduceMotionForToast
    /// Flights poll only while the window can be seen: the feed's quota is shared server-wide.
    @Environment(\.scenePhase) private var scenePhase
    #if os(iOS)
    @Environment(\.horizontalSizeClass) private var sizeClass
    @State private var peekHeight = MapScreen.defaultPeekHeight
    @State private var detent = PresentationDetent.height(MapScreen.defaultPeekHeight)
    @State private var sheetHeight: CGFloat = 0
    /// Where the sheet was when search opened it to large, to go back on Cancel.
    @State private var detentBeforeSearch: PresentationDetent?
    #endif

    var body: some View {
        shell
            .background(Color.canvas)
            .task { await model.keepFresh() }
            // Flights refresh on their own minute while the layer is on (ADR 28).
            .task(id: model.showsFlights && scenePhase != .background) {
                if model.showsFlights, scenePhase != .background { await model.keepFlightsFresh() }
            }
            .task { await loadCountries() }
            // Opening a country loads it; one that crosses ±180° is framed by its events once
            // they arrive (`Country.mapBox(for:)`).
            // Keyed on the instance: choosing the open country again makes a new overview, which
            // must load too.
            .task(id: country.map(ObjectIdentifier.init)) {
                guard let country else { return }
                let opened = ContinuousClock.now
                await country.load()
                // Once the sheet has settled, so the map frames the country above it.
                try? await Task.sleep(until: opened + .milliseconds(450))
                guard !Task.isCancelled, let box = country.summary.map({ $0.country.mapBox(for: $0.events) })
                        ?? country.country?.mapBox(for: []) else { return }
                focus = MapFocus(region: MKCoordinateRegion(box))
            }
            // The map refreshed (poll or stream): an overview older than a minute follows it.
            .onChange(of: model.snapshot?.generatedAt) {
                Task { await country?.refresh(ifOlderThan: 60) }
            }
            // The list arrives late or failed: try again when someone starts typing.
            .onChange(of: search.isActive) { _, active in
                if active, search.countries.isEmpty { Task { await loadCountries() } }
            }
            #if DEBUG
            // Automation: `SHIELD_SHOW_SOURCES=1` opens the Sources screen.
            .task {
                if ProcessInfo.processInfo.environment["SHIELD_SHOW_SOURCES"] == "1" {
                    try? await Task.sleep(for: .seconds(1))
                    showsSources = true
                }
            }
            // Automation: `SHIELD_FAKE_ARRIVAL=1` sends a copy of an event in view, made severe
            // and new, through the stream path once it is live (the toast and the arrive fade).
            .task {
                guard ProcessInfo.processInfo.environment["SHIELD_FAKE_ARRIVAL"] == "1" else { return }
                // Once live, plus time for Show on map to settle when combined with it.
                for _ in 0..<120 where !model.isLive { try? await Task.sleep(for: .milliseconds(250)) }
                try? await Task.sleep(for: .seconds(4))
                guard var copy = model.snapshot?.events.first else { return }
                copy.id = UUID()
                copy.severity = .severe
                // Beside the original, so the two don't merge into one cluster.
                copy.centroid = Coordinate(latitude: copy.centroid.latitude - 0.6,
                                           longitude: copy.centroid.longitude + 0.6)
                model.simulate(.created(copy))
            }
            // Automation: `SHIELD_DETAIL_ACTION=map|country` runs the detail's Show on map, or its
            // first country link, once the selected event's detail loads (no simulator taps).
            .task {
                guard let action = ProcessInfo.processInfo.environment["SHIELD_DETAIL_ACTION"] else { return }
                for _ in 0..<40 {
                    if case .loaded = model.selectedDetail { break }
                    try? await Task.sleep(for: .milliseconds(250))
                }
                try? await Task.sleep(for: .seconds(1))
                guard let event = model.selectedEvent else { return }
                let detail = eventDetail(event)
                switch action {
                case "map": detail.showOnMap?()
                case "country": if let iso2 = event.countries.first?.iso2 { detail.openCountry?(iso2) }
                default: break
                }
            }
            // Screenshot automation: `SHIELD_SECTION=2` selects that sidebar section (⌘2, Countries).
            .task {
                guard let raw = ProcessInfo.processInfo.environment["SHIELD_SECTION"],
                      let target = Int(raw).flatMap(SidebarSection.init(rawValue:)) else { return }
                try? await Task.sleep(for: .seconds(1))
                go(target)
            }
            // Automation: `SHIELD_FLIGHTS=1` turns the flights layer on (only ever against a
            // replay: ADR 28); `SHIELD_AIRCRAFT=<hex>` then selects that aircraft.
            .task {
                let env = ProcessInfo.processInfo.environment
                guard env["SHIELD_FLIGHTS"] == "1" else { return }
                try? await Task.sleep(for: .seconds(1))
                model.showsFlights = true
                guard let hex = env["SHIELD_AIRCRAFT"] else { return }
                for _ in 0..<40 where model.flights == nil { try? await Task.sleep(for: .milliseconds(250)) }
                model.selectedAircraftHex = hex
            }
            // Automation: `SHIELD_BASE=<Wikidata ID>` selects that base once the list is in
            // (`SHIELD_BASES=1` turns the layer on at launch).
            .task {
                guard let id = ProcessInfo.processInfo.environment["SHIELD_BASE"] else { return }
                for _ in 0..<40 where model.bases == nil { try? await Task.sleep(for: .milliseconds(250)) }
                model.selectedBaseID = id
            }
            // Automation: `SHIELD_SITE=<Wikidata ID>` selects that historic site once an answer
            // holds it (`SHIELD_HISTORIC=1` turns the layer on at launch).
            .task {
                guard let id = ProcessInfo.processInfo.environment["SHIELD_SITE"] else { return }
                for _ in 0..<60 where model.historic?.sites.contains(where: { $0.id == id }) != true {
                    try? await Task.sleep(for: .milliseconds(250))
                }
                model.selectedHistoricID = id
            }
            // Screenshot automation: `SHIELD_COUNTRY=PT` opens Portugal's overview.
            .task {
                guard let code = ProcessInfo.processInfo.environment["SHIELD_COUNTRY"] else { return }
                let known = (try? await model.provider.countries())?.first { $0.iso2 == code.uppercased() }
                open(known, iso2: code)
                #if os(iOS)
                if ProcessInfo.processInfo.environment["SHIELD_SHEET"] == "large" { detent = .large }
                #endif
            }
            #endif
            #if os(macOS)
            .onKeyPress(.escape) {
                if search.isActive {
                    search.end()
                    sidebarSearchFocused = false
                    return .handled
                }
                if model.selectedEventID != nil || model.selectedAircraftHex != nil || model.selectedBaseID != nil
                    || model.selectedHistoricID != nil {
                    model.selectedEventID = nil
                    model.selectedAircraftHex = nil
                    model.selectedBaseID = nil
                    model.selectedHistoricID = nil
                    return .handled
                }
                guard country != nil else { return .ignored }
                closeCountry()
                return .handled
            }
            #endif
    }

    @ViewBuilder
    private var shell: some View {
        #if os(macOS)
        splitShell
        #else
        if sizeClass == .regular { splitShell } else { sheetShell }
        #endif
    }

    // MARK: Shells

    private var splitShell: some View {
        NavigationSplitView(columnVisibility: $columnVisibility) {
            VStack(spacing: 0) {
                // DESIGN.md §10: Monitor and Countries on top, Sources pinned at the bottom.
                VStack(spacing: ShieldSpace.s1) {
                    ForEach(SidebarSection.shown.filter { $0 != .sources }) { item in
                        SidebarSectionRow(section: item, isSelected: section == item) { go(item) }
                    }
                }
                .padding(.horizontal, ShieldSpace.s2)
                .padding(.top, ShieldSpace.s3)
                .background {
                    // ⌘F: search places, countries and events (DESIGN.md §10 keyboard shortcuts);
                    // the field is Monitor's.
                    Button("Search") {
                        // Only a section change: in Monitor an open country stays open.
                        if section != .monitor { go(.monitor) }
                        Task { sidebarSearchFocused = true }
                    }
                    .keyboardShortcut("f", modifiers: .command)
                    .opacity(0)
                    .accessibilityHidden(true)
                }
                switch section {
                case .countries:
                    countriesSidebar
                default:
                    monitorSidebar
                }
                Spacer(minLength: 0)
                Rectangle().fill(Color.ink.opacity(ShieldAlpha.hairline)).frame(height: 1)
                SourcesSidebarRow(health: Source.overallHealth(model.sourceList)) { go(.sources) }
            }
            // The column takes the height it is given. Without this AppKit derives the window's
            // minimum from the sidebar measured at almost no width, where the vertically fixed
            // texts (stat strip, list footnote) wrap a word per line: the window opened 1121 pt
            // tall, over a 1084 pt screen, and could not be made shorter (S3.8).
            #if os(macOS)
            .frame(minHeight: 480, maxHeight: .infinity)
            #endif
            .navigationSplitViewColumnWidth(min: 220, ideal: 240, max: 260) // DESIGN.md §10
        } detail: {
            mapLayer(showsNotices: columnVisibility == .detailOnly, trailingCover: inspectorCover)
                #if os(iOS)
                .onGeometryChange(for: CGFloat.self) { $0.safeAreaInsets.trailing } action: { mapTrailingInset = $0 }
                #endif
                .toolbar(removing: .title)
                .focusedSceneValue(\.sidebarNavigation, SidebarNavigation(section: section, go: go))
                // DESIGN.md §10 (macOS): the selected event in the inspector, beside the map.
                .inspector(isPresented: inspectorShown) {
                    if let aircraft = model.selectedAircraft {
                        ScrollView {
                            AircraftDetailView(aircraft: aircraft, layer: model.flights,
                                               close: { model.selectedAircraftHex = nil })
                                .padding(ShieldSpace.s5)
                        }
                        .background(Color.panel)
                        .inspectorColumnWidth(min: 320, ideal: 350, max: 380)
                    } else if let base = model.selectedBase {
                        ScrollView {
                            BaseDetailView(base: base, layer: model.bases, close: { model.selectedBaseID = nil })
                                .padding(ShieldSpace.s5)
                        }
                        .background(Color.panel)
                        .inspectorColumnWidth(min: 320, ideal: 350, max: 380)
                    } else if let site = model.selectedHistoricSite {
                        ScrollView {
                            HistoricDetailView(site: site, layer: model.historic,
                                               close: { model.selectedHistoricID = nil })
                                .padding(ShieldSpace.s5)
                        }
                        .background(Color.panel)
                        .inspectorColumnWidth(min: 320, ideal: 350, max: 380)
                    } else if let event = model.selectedEvent {
                        ScrollView {
                            eventDetail(event)
                                .padding(ShieldSpace.s5)
                        }
                        .background(Color.panel)
                        .inspectorColumnWidth(min: 320, ideal: 350, max: 380)
                    }
                }
                .sheet(isPresented: $showsSources) { sourcesSheet }
                .sheet(isPresented: $showsFilters) {
                    NavigationStack {
                        FilterSheet(model: model, done: { showsFilters = false })
                    }
                    .frame(minWidth: 440, idealWidth: 480, minHeight: 600)
                }
                #if DEBUG
                .onAppear {
                    if ProcessInfo.processInfo.environment["SHIELD_SHOW_FILTERS"] == "1" { showsFilters = true }
                }
                #endif
        }
    }


    /// Monitor: search, the stat strip and the events in view (or an open country).
    @ViewBuilder
    private var monitorSidebar: some View {
        MapSearchField(search: search, focused: $sidebarSearchFocused)
            .padding(.horizontal, ShieldSpace.s4)
            .padding(.top, ShieldSpace.s3)
        // The sample badge is the map's own (always in view), not repeated here. The
        // strip steps aside while searching, and for a country's overview.
        if let country, !search.isActive {
            Rectangle()
                .fill(Color.ink.opacity(ShieldAlpha.hairline))
                .frame(height: 1)
                .padding(.top, ShieldSpace.s3)
            CountryView(overview: country, model: model, close: closeCountry,
                        selectEvent: { model.select($0) })
        } else if !search.isActive {
            StatStrip(cells: StatStrip.Cell.liveMap(model.snapshot, includesResolved: model.query.includeResolved),
                      query: model.query, tap: { model.query = model.query.toggling($0) })
                .padding(.horizontal, ShieldSpace.s4)
                .padding(.vertical, ShieldSpace.s3)
        } else {
            Spacer().frame(height: ShieldSpace.s3)
        }
        if country == nil || search.isActive {
            Rectangle()
                .fill(Color.ink.opacity(ShieldAlpha.hairline))
                .frame(height: 1)
        }
        if search.isActive {
            MapSearchResults(search: search, model: model,
                             selectEvent: { event in
                                 sidebarSearchFocused = false
                                 search.end()
                                 model.selectedEventID = event.id
                             },
                             selectPlace: { place in
                                 sidebarSearchFocused = false
                                 Task { await go(to: place) }
                             },
                             selectCountry: { country in
                                 sidebarSearchFocused = false
                                 open(country)
                             })
        } else if country == nil {
            EventList(model: model, zoomOut: fitWorld)
        }
    }

    /// Countries: every country with its active count; one opens its overview here.
    @ViewBuilder
    private var countriesSidebar: some View {
        if let country {
            Rectangle().fill(Color.ink.opacity(ShieldAlpha.hairline)).frame(height: 1).padding(.top, ShieldSpace.s3)
            CountryView(overview: country, model: model, close: closeCountry,
                        selectEvent: { model.select($0) })
        } else {
            CountriesPane(countries: search.countries, failed: countriesFailed,
                          retry: { Task { await loadCountries(refresh: true) } },
                          open: { open($0) })
                .padding(.top, ShieldSpace.s2)
        }
    }

    /// ⌘1–⌘5 and the sidebar rows. Changing section closes an open country and any search.
    private func go(_ target: SidebarSection) {
        guard target.isAvailable else { return }
        if target == .sources {
            showsSources = true
            return
        }
        if columnVisibility == .detailOnly { columnVisibility = .all }
        // The same section again goes back to its top: the list, not the open country.
        guard target != section else {
            if country != nil { closeCountry() }
            return
        }
        search.end()
        sidebarSearchFocused = false
        if country != nil { closeCountry() }
        section = target
        if target == .countries { Task { await loadCountries(refresh: true) } }
    }

    #if os(iOS)
    /// The peek's height (search field + compact strip) before it has been measured.
    private static let defaultPeekHeight: CGFloat = 96

    private var sheetShell: some View {
        // The map's attribution sits above the sheet wherever the map is still usable: at peek
        // and at medium. The notices sit above the peek.
        // Above medium the sheet's list shows the empty and truncated notices itself.
        mapLayer(showsNotices: detent == .height(peekHeight),
                 bottomCover: detent == .medium ? max(sheetHeight, peekHeight) : peekHeight)
            .safeAreaInset(edge: .bottom, spacing: 0) { Color.clear.frame(height: peekHeight) }
            .sheet(isPresented: .constant(true)) {
                MapSheet(model: model, search: search, country: country, showsFilters: $showsFilters,
                         showsSources: $showsSources,
                         isPeek: detent == .height(peekHeight),
                         zoomOut: fitWorld,
                         selectEvent: { event in
                             model.select(event)
                             withAnimation(ShieldMotion.spring) { detent = .medium }
                         },
                         selectPlace: { place in Task { await go(to: place) } },
                         selectCountry: { open($0) },
                         closeCountry: closeCountry,
                         eventDetail: eventDetail,
                         peekHeightChanged: setPeekHeight,
                         heightChanged: { sheetHeight = $0.rounded(.up) })
                    .presentationDetents([.height(peekHeight), .medium, .large], selection: $detent)
                    // The map stays usable at peek and medium (DESIGN.md §10 shells).
                    .presentationBackgroundInteraction(.enabled(upThrough: .medium))
                    .presentationBackground { MapSheetBackground() }
                    .presentationDragIndicator(.visible)
                    .interactiveDismissDisabled()
                    .preferredColorScheme(.dark)
                    .tint(.accent)
            }
            // Typing needs the room: results fill the sheet, as in Maps.
            .onChange(of: search.isEditing) { _, editing in
                guard editing, detent != .large else { return }
                detentBeforeSearch = detent
                withAnimation(ShieldMotion.spring) { detent = .large }
            }
            // Search put away with nothing chosen (Cancel, or cleared and dismissed): back to
            // where the sheet was. Choosing an event or a place sets its own detent first.
            .onChange(of: search.isActive) { _, active in
                guard !active, let previous = detentBeforeSearch else { return }
                detentBeforeSearch = nil
                guard detent == .large, model.selectedEventID == nil, model.selectedAircraftHex == nil,
                      model.selectedBaseID == nil, model.selectedHistoricID == nil, country == nil else { return }
                withAnimation(ShieldMotion.spring) { detent = previous }
            }
            // A marker tapped while the sheet peeks: open far enough to show the event and
            // still leave the map in view.
            .onChange(of: model.selectedEventID, initial: true) { _, selected in
                guard selected != nil, detent == .height(peekHeight) else { return }
                withAnimation(ShieldMotion.spring) { detent = .medium }
            }
            .onChange(of: model.selectedAircraftHex) { _, selected in
                guard selected != nil, detent == .height(peekHeight) else { return }
                withAnimation(ShieldMotion.spring) { detent = .medium }
            }
            .onChange(of: model.selectedBaseID) { _, selected in
                guard selected != nil, detent == .height(peekHeight) else { return }
                withAnimation(ShieldMotion.spring) { detent = .medium }
            }
            .onChange(of: model.selectedHistoricID) { _, selected in
                guard selected != nil, detent == .height(peekHeight) else { return }
                withAnimation(ShieldMotion.spring) { detent = .medium }
            }
            #if DEBUG
            // Screenshot automation: `SHIELD_SHEET=peek|medium|large`.
            .onAppear {
                let env = ProcessInfo.processInfo.environment
                switch env["SHIELD_SHEET"] {
                case "medium": detent = .medium
                case "large": detent = .large
                default: break
                }
                // `SHIELD_SEARCH=<text>` types into search; `SHIELD_SEARCH_GO=1` also picks the
                // first place once it arrives.
                if env["SHIELD_SHOW_FILTERS"] == "1" { showsFilters = true }
                if let text = env["SHIELD_SEARCH"] {
                    search.text = text
                    detent = .large
                }
            }
            .onChange(of: search.places.first?.id) { _, first in
                guard ProcessInfo.processInfo.environment["SHIELD_SEARCH_GO"] == "1",
                      first != nil, let place = search.places.first else { return }
                // A matching country wins, as a tap on the first result would.
                if let match = search.countryMatches.first { open(match) } else { Task { await go(to: place) } }
            }
            // `SHIELD_CELL=wind:moderate` picks a matrix cell once the overview loads.
            .onChange(of: country?.summary?.generatedAt) {
                guard let raw = ProcessInfo.processInfo.environment["SHIELD_CELL"], country?.cell == nil else { return }
                let parts = raw.split(separator: ":").map(String.init)
                guard parts.count == 2, let type = HazardType(rawValue: parts[0]),
                      let severity = Severity(rawValue: parts[1]) else { return }
                country?.cell = MatrixCell(type: type, severity: severity)
            }
            #endif
    }

    /// Keeps the peek detent as tall as the stat strip, and the sheet on it if it was there.
    private func setPeekHeight(_ height: CGFloat) {
        let height = height.rounded(.up)
        guard height > 0, height != peekHeight else { return }
        let wasPeeking = detent == .height(peekHeight)
        peekHeight = height
        if wasPeeking { detent = .height(height) }
    }
    #endif

    private func fitWorld() { fitWorldRequest += 1 }

    private var filterChips: some View {
        FilterChips(query: model.query,
                    openFilters: { showsFilters = true },
                    remove: { model.query = model.query.removing($0) })
    }

    /// Loads the country list for search. A failure leaves search with places and events only.
    /// Once for search; again on each visit to the Countries section, whose counts should be current.
    private func loadCountries(refresh: Bool = false) async {
        guard refresh || search.countries.isEmpty else { return }
        do {
            search.countries = try await model.provider.countries()
            countriesFailed = false
        } catch {
            countriesFailed = true
        }
    }

    /// Opens a country's overview and frames it on the map (DESIGN.md §10.2). The map's filters
    /// and its own list stay as they were, ready for when the overview closes.
    private func open(_ known: Country?, iso2: String? = nil) {
        guard let overview = CountryOverview(iso2: iso2 ?? known?.iso2 ?? "", known: known,
                                             provider: model.provider) else { return }
        model.selectedEventID = nil
        search.end()
        country = overview
        #if os(iOS)
        // Medium, not large: the country stays in view above its overview (owner to confirm;
        // DESIGN.md §10.2 says large).
        withAnimation(ShieldMotion.spring) { detent = .medium }
        #endif
    }

    private func open(_ country: Country) { open(country, iso2: nil) }

    private func closeCountry() {
        model.selectedEventID = nil
        country = nil
    }

    // MARK: Event detail

    /// The inspector is open while an event is selected and there is one to show.
    private var inspectorShown: Binding<Bool> {
        Binding(get: { (model.selectedEventID != nil && model.selectedEvent != nil) || model.selectedAircraft != nil
                    || model.selectedBase != nil || model.selectedHistoricSite != nil },
                set: {
                    if !$0 {
                        model.selectedEventID = nil
                        model.selectedAircraftHex = nil
                        model.selectedBaseID = nil
                        model.selectedHistoricID = nil
                    }
                })
    }

    /// The selected event in full (DESIGN.md §10.3), for the sheet and the inspector alike.
    private func eventDetail(_ event: HazardEvent) -> EventDetailView {
        var detailSource: Source?
        if case .loaded(let detail) = model.selectedDetail { detailSource = detail.source }
        return EventDetailView(
            event: event,
            state: model.selectedDetail,
            source: model.sources[event.source] ?? detailSource,
            isStale: model.isStale(event),
            countryName: countryName,
            openCountry: { iso2 in open(search.countries.first { $0.iso2 == iso2 }, iso2: iso2) },
            showOnMap: { showOnMap(event) },
            close: { model.selectedEventID = nil }
        )
    }

    /// Sources & attribution (DESIGN.md §10.7), as a sheet from either shell.
    private var sourcesSheet: some View {
        NavigationStack {
            SourcesView(model: model, done: { showsSources = false })
        }
        #if os(iOS)
        .presentationDetents([.large])
        #else
        .frame(minWidth: 460, idealWidth: 520, minHeight: 560, idealHeight: 720)
        #endif
        .presentationBackground(Color.panel)
        .preferredColorScheme(.dark)
    }

    /// The name the country list gives, else the system's English name.
    private func countryName(_ iso2: String) -> String {
        search.countries.first { $0.iso2 == iso2 }?.name
            ?? Locale(identifier: "en").localizedString(forRegionCode: iso2) ?? iso2
    }

    /// Frames the event's area (or its point) in the part of the map left in view.
    private func showOnMap(_ event: HazardEvent) {
        #if os(iOS)
        if sizeClass != .regular { withAnimation(ShieldMotion.spring) { detent = .medium } }
        #endif
        let region = MKCoordinateRegion((model.selectedGeometry ?? .point(event.centroid)).frame(around: event.centroid))
        Task {
            #if os(iOS)
            try? await Task.sleep(for: .milliseconds(400)) // after the sheet settles
            #endif
            focus = MapFocus(region: region)
        }
    }

    /// Moves the map to a chosen place and puts the search away, so the map is in view.
    private func go(to place: MapSearch.Place) async {
        guard let region = await search.region(for: place) else { return }
        search.end()
        #if os(iOS)
        // The sheet goes down first, so the place is framed in the map it leaves in view.
        withAnimation(ShieldMotion.spring) { detent = .height(peekHeight) }
        try? await Task.sleep(for: .milliseconds(400))
        #endif
        focus = MapFocus(region: region)
    }

    // MARK: Map

    /// How much of the map's trailing edge the inspector covers: zero beside it, its width over it.
    /// How much of the map's trailing edge sits under the inspector (iPad); zero on the Mac,
    /// whose inspector is a column beside the map.
    private var inspectorCover: CGFloat {
        #if os(iOS)
        mapTrailingInset
        #else
        0
        #endif
    }

    private func mapLayer(showsNotices: Bool, bottomCover: CGFloat = 0, trailingCover: CGFloat = 0) -> some View {
        ZStack {
            HazardMapView(model: model, fitWorldRequest: fitWorldRequest, focus: focus,
                          bottomCover: bottomCover, trailingCover: trailingCover)
                .ignoresSafeArea()

            VStack(spacing: ShieldSpace.s2) {
                HStack(alignment: .top) {
                    // iPhone: the chips sit under the capsule, left of the controls (DESIGN.md
                    // §10 shells); the Mac puts them top-centre.
                    VStack(alignment: .leading, spacing: ShieldSpace.s2) {
                        StatusCapsule(model: model)
                        #if os(iOS)
                        filterChips
                        #endif
                    }
                    Spacer(minLength: ShieldSpace.s2)
                    #if os(macOS)
                    filterChips.fixedSize()
                    Spacer(minLength: ShieldSpace.s2)
                    #endif
                    MapControls(
                        isRefreshing: model.phase == .loading,
                        showsDetections: $model.showsDetections,
                        showsConflicts: $model.showsConflicts,
                        showsFlights: model.provider.supportsAircraft ? $model.showsFlights : nil,
                        showsBases: model.provider.supportsBases ? $model.showsBases : nil,
                        showsHistoric: model.provider.supportsHistoric ? $model.showsHistoric : nil,
                        fitWorld: fitWorld,
                        refresh: { model.reload() }
                    )
                }
                if let toast = model.toast {
                    NewEventToast(toast: toast, open: {
                        model.selectedEventID = toast.event.id
                        model.dismissToast()
                    }, dismiss: model.dismissToast)
                    .id(toast.id)
                    .transition(reduceMotionForToast ? .opacity : .move(edge: .top).combined(with: .opacity))
                }
                if case .failed(.unreachable(let reason)) = model.phase {
                    ErrorBanner(message: reason, retry: model.reload)
                        .transition(.move(edge: .top).combined(with: .opacity))
                }
                if model.provider.isSampleData {
                    HStack { SampleDataBadge(); Spacer() }
                }
                if model.showsDetections {
                    HStack {
                        DetectionLegend(layer: model.detections, failed: model.detectionsFailed,
                                        isLoading: model.phase == .loading)
                        Spacer()
                    }
                    .transition(.opacity)
                }
                if model.showsFlights {
                    HStack {
                        FlightsLegend(model: model)
                        Spacer()
                    }
                    .transition(.opacity)
                }
                if model.showsBases {
                    HStack {
                        BasesLegend(model: model)
                        Spacer()
                    }
                    .transition(.opacity)
                }
                if model.showsHistoric {
                    HStack {
                        HistoricLegend(model: model)
                        Spacer()
                    }
                    .transition(.opacity)
                }
                if model.showsConflicts {
                    HStack {
                        ConflictLegend(count: model.conflicts?.totalCount, truncated: model.conflicts?.truncated ?? false,
                                       failed: model.conflictsFailed)
                        Spacer()
                    }
                    .transition(.opacity)
                }
                Spacer()
                bottom(showsNotices: showsNotices)
                    // Clear of the map's attribution ("Maps · Legal"), which must stay
                    // visible (DECISIONS #11).
                    .padding(.bottom, ShieldSpace.s6)
            }
            .padding(ShieldSpace.s3)
            #if os(macOS)
            // The legends must not size the window: AppKit takes the window's minimum from this
            // stack measured at almost no width, where their text wraps a word per line, so
            // turning a layer on made the window taller (S3.8 found the same in the sidebar).
            .frame(minHeight: 0, maxHeight: .infinity)
            #endif
            .animation(ShieldMotion.standard, value: model.phase)
            .animation(ShieldMotion.spring, value: model.selectedEventID)
            .animation(ShieldMotion.standard, value: model.showsDetections)
            .animation(ShieldMotion.standard, value: model.showsConflicts)
            .animation(ShieldMotion.standard, value: model.showsFlights)
            .animation(ShieldMotion.standard, value: model.showsBases)
            .animation(ShieldMotion.standard, value: model.showsHistoric)
            .animation(ShieldMotion.standard, value: model.toast)
        }
        // A toast is read out once and leaves after 6 s; a newer one restarts the clock.
        .task(id: model.toast?.id) {
            guard let toast = model.toast else { return }
            AccessibilityNotification.Announcement(toast.text).post()
            do { try await Task.sleep(for: .seconds(6)) } catch { return }
            if model.toast?.id == toast.id { model.dismissToast() }
        }
    }

    /// The empty and truncated notices, when no list is in view to show them.
    @ViewBuilder
    private func bottom(showsNotices: Bool) -> some View {
        if !showsNotices {
            EmptyView()
        } else if let snapshot = model.snapshot, snapshot.totalCount == 0 {
            EmptyViewNotice(filtered: !model.query.isDefault, zoomOut: fitWorld,
                            resetFilters: { model.query = EventQuery(country: model.query.country) })
        } else if model.snapshot?.truncated == true {
            Notice(text: "Showing the most severe events here. Zoom in to see them all.")
        }
    }
}

// MARK: - Status capsule

/// `AEGIS TERRA` · live dot · freshness (DESIGN.md §10 shells). Re-reads the clock every 10 s.
private struct StatusCapsule: View {
    let model: LiveMapModel

    var body: some View {
        TimelineView(.periodic(from: .now, by: 10)) { context in
            HStack(spacing: ShieldSpace.s2) {
                    Text("AEGIS TERRA")
                        .font(ShieldFont.micro)
                        .tracking(ShieldTracking.micro)
                        .foregroundStyle(Color.muted)
                    // Filled while the live stream is connected, a ring while only polling:
                    // shape as well as colour (DESIGN.md §14).
                    Circle()
                        .fill(model.isLive ? dotColor : .clear)
                        .strokeBorder(dotColor, lineWidth: 1.5)
                        .frame(width: 6, height: 6)
                        .accessibilityHidden(true)
                    Text(status(now: context.date))
                        .font(ShieldFont.meta)
                        .foregroundStyle(textColor)
                        .contentTransition(.numericText())
                    if case .failed(.offline) = model.phase {
                        Button("Retry", action: model.reload)
                            .font(ShieldFont.meta.weight(.semibold))
                            .buttonStyle(.plain)
                            .foregroundStyle(Color.accent)
                    }
                }
            .padding(.horizontal, ShieldSpace.s3)
            .padding(.vertical, ShieldSpace.s2)
            // An overlay, so the line takes the capsule's width instead of widening it.
            .overlay(alignment: .bottom) { ProgressLine(isActive: model.phase == .loading) }
            .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: ShieldRadius.md))
            .overlay(RoundedRectangle(cornerRadius: ShieldRadius.md)
                .strokeBorder(Color.ink.opacity(ShieldAlpha.hairline)))
            .clipShape(RoundedRectangle(cornerRadius: ShieldRadius.md))
            .accessibilityElement(children: .combine)
            .accessibilityValue(model.isLive ? "Live" : "")
        }
    }

    private func status(now: Date) -> String {
        switch model.phase {
        case .loading where model.lastSuccess == nil:
            return "Loading events…"
        case .failed(.offline):
            return model.lastSuccess.map { "Offline · showing data from \(Freshness.utcClock($0))" } ?? "Offline"
        default:
            // While live, a heartbeat confirms the data too (S3.6); the dot says it's live.
            guard let last = model.lastUpdate else { return "No data yet" }
            let ago = Freshness.relative(last, now: now)
            return model.dataIsStale ? "Last update \(ago)" : "Updated \(ago)"
        }
    }

    private var dotColor: Color {
        switch model.phase {
        case .failed(.offline): Color.muted
        case .failed: Color.danger
        default: model.dataIsStale ? Color.warning : Color.ok
        }
    }

    private var textColor: Color {
        if case .failed(.offline) = model.phase { return Color.warning }
        return model.dataIsStale ? Color.warning : Color.ink
    }
}

/// A thin accent line under the capsule while loading (DESIGN.md §10.1 *Loading*).
private struct ProgressLine: View {
    var isActive: Bool
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        GeometryReader { proxy in
            if isActive {
                if reduceMotion {
                    Rectangle().fill(Color.accent.opacity(0.6))
                } else {
                    PhaseAnimator([false, true]) { forward in
                        Rectangle()
                            .fill(Color.accent)
                            .frame(width: proxy.size.width * 0.3)
                            .offset(x: forward ? proxy.size.width * 0.7 : 0)
                    } animation: { _ in .easeInOut(duration: 0.9) }
                }
            }
        }
        .frame(height: 2)
        .accessibilityHidden(true)
    }
}

// MARK: - Controls

/// Floating controls, top-right, 44 pt targets (DESIGN.md §9).
private struct MapControls: View {
    var isRefreshing: Bool
    @Binding var showsDetections: Bool
    @Binding var showsConflicts: Bool
    /// Nil when the provider has no flights (sample data).
    var showsFlights: Binding<Bool>?
    /// Nil when the provider has no bases (sample data).
    var showsBases: Binding<Bool>?
    /// Nil when the provider has no historic sites (sample data).
    var showsHistoric: Binding<Bool>?
    var fitWorld: () -> Void
    var refresh: () -> Void

    var body: some View {
        VStack(spacing: 0) {
            // Live flights (ADR 28): off at every launch, never saved.
            if let showsFlights {
                Button { showsFlights.wrappedValue.toggle() } label: {
                    Image(systemName: "airplane")
                        .font(.system(size: 16, weight: .semibold))
                        .foregroundStyle(showsFlights.wrappedValue ? Color.accent : Color.ink)
                        .frame(width: 44, height: 44)
                        .background(showsFlights.wrappedValue ? Color.accent.opacity(0.14) : .clear)
                        .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .accessibilityLabel("Live flights")
                .accessibilityValue(showsFlights.wrappedValue ? "Shown" : "Hidden")
                .accessibilityAddTraits(.isToggle)
                .help(showsFlights.wrappedValue ? "Hide live flights" : "Show live flights")
                .keyboardShortcut("a", modifiers: [.command, .option])
                Divider().overlay(Color.ink.opacity(ShieldAlpha.hairline))
            }
            // Military bases (ADR 29): off by default, saved with the filters.
            if let showsBases {
                Button { showsBases.wrappedValue.toggle() } label: {
                    Image(systemName: "star.square")
                        .font(.system(size: 16, weight: .semibold))
                        .foregroundStyle(showsBases.wrappedValue ? Color.accent : Color.ink)
                        .frame(width: 44, height: 44)
                        .background(showsBases.wrappedValue ? Color.accent.opacity(0.14) : .clear)
                        .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .accessibilityLabel("Military bases")
                .accessibilityValue(showsBases.wrappedValue ? "Shown" : "Hidden")
                .accessibilityAddTraits(.isToggle)
                .help(showsBases.wrappedValue ? "Hide military bases" : "Show military bases")
                .keyboardShortcut("b", modifiers: [.command, .option])
                Divider().overlay(Color.ink.opacity(ShieldAlpha.hairline))
            }
            // Historic sites (ADR 30): off by default, saved with the filters.
            if let showsHistoric {
                Button { showsHistoric.wrappedValue.toggle() } label: {
                    Image(systemName: "building.columns")
                        .font(.system(size: 16, weight: .semibold))
                        .foregroundStyle(showsHistoric.wrappedValue ? Color.accent : Color.ink)
                        .frame(width: 44, height: 44)
                        .background(showsHistoric.wrappedValue ? Color.accent.opacity(0.14) : .clear)
                        .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .accessibilityLabel("Historic sites")
                .accessibilityValue(showsHistoric.wrappedValue ? "Shown" : "Hidden")
                .accessibilityAddTraits(.isToggle)
                .help(showsHistoric.wrappedValue ? "Hide historic sites" : "Show historic sites")
                .keyboardShortcut("l", modifiers: [.command, .option])
                Divider().overlay(Color.ink.opacity(ShieldAlpha.hairline))
            }
            // Armed conflicts (ADR 27): a layer beside hazards, off by default.
            Button { showsConflicts.toggle() } label: {
                Image(systemName: HazardType.conflict.sfSymbol)
                    .font(.system(size: 16, weight: .semibold))
                    .foregroundStyle(showsConflicts ? Color.accent : Color.ink)
                    .frame(width: 44, height: 44)
                    .background(showsConflicts ? Color.accent.opacity(0.14) : .clear)
                    .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityLabel("Armed conflicts")
            .accessibilityValue(showsConflicts ? "Shown" : "Hidden")
            .accessibilityAddTraits(.isToggle)
            .help(showsConflicts ? "Hide armed conflicts" : "Show armed conflicts")
            .keyboardShortcut("c", modifiers: [.command, .option])
            Divider().overlay(Color.ink.opacity(ShieldAlpha.hairline))
            // Off by default (DESIGN.md §5). The word is "heat detections", never "fires".
            Button { showsDetections.toggle() } label: {
                Image(systemName: "thermometer.high")
                    .font(.system(size: 16, weight: .semibold))
                    .foregroundStyle(showsDetections ? Color.detection : Color.ink)
                    .frame(width: 44, height: 44)
                    .background(showsDetections ? Color.detection.opacity(0.14) : .clear)
                    .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityLabel("Satellite heat detections")
            .accessibilityValue(showsDetections ? "Shown" : "Hidden")
            .accessibilityAddTraits(.isToggle)
            .help(showsDetections ? "Hide satellite heat detections" : "Show satellite heat detections")
            .keyboardShortcut("h", modifiers: [.command, .option])
            Divider().overlay(Color.ink.opacity(ShieldAlpha.hairline))
            button("globe.europe.africa", label: "Show the whole world", action: fitWorld)
                .keyboardShortcut("0", modifiers: .command)
            Divider().overlay(Color.ink.opacity(ShieldAlpha.hairline))
            button("arrow.clockwise", label: "Refresh", action: refresh)
                .keyboardShortcut("r", modifiers: .command)
                .disabled(isRefreshing)
        }
        .frame(width: 44)
        .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: ShieldRadius.md))
        .overlay(RoundedRectangle(cornerRadius: ShieldRadius.md).strokeBorder(Color.ink.opacity(ShieldAlpha.hairline)))
    }

    private func button(_ symbol: String, label: String, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Image(systemName: symbol)
                .font(.system(size: 16, weight: .semibold))
                .foregroundStyle(Color.ink)
                .frame(width: 44, height: 44)
                .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityLabel(label)
        .help(label)
    }
}

// MARK: - Banners and notices

/// What the heat layer is and how old it is: "Detected 3 h ago", never "Updated" (FIRMS NRT
/// lags the satellite pass by ~3 h, DESIGN.md §5).
private struct DetectionLegend: View {
    var layer: DetectionLayer
    var failed: Bool
    var isLoading: Bool

    var body: some View {
        TimelineView(.periodic(from: .now, by: 60)) { context in
            HStack(spacing: ShieldSpace.s2) {
                Circle().fill(Color.detection).frame(width: 6, height: 6).accessibilityHidden(true)
                Text(text(now: context.date))
                    .font(ShieldFont.meta)
                    .foregroundStyle(failed ? Color.warning : Color.ink)
            }
            .padding(.horizontal, ShieldSpace.s3)
            .padding(.vertical, ShieldSpace.s2)
            .background(.ultraThinMaterial, in: Capsule())
            .overlay(Capsule().strokeBorder(Color.ink.opacity(ShieldAlpha.hairline)))
            .accessibilityElement(children: .combine)
        }
    }

    private func text(now: Date) -> String {
        if failed { return "Heat detections didn’t load" }
        var parts = ["Satellite heat detections"]
        if let latest = layer.latest {
            parts.append("Detected \(Freshness.relative(latest, now: now))")
        } else if isLoading {
            parts.append("Loading…")
        } else {
            parts.append("None in this view")
        }
        if case .points(_, truncated: true) = layer { parts.append("Zoom in for all") }
        return parts.joined(separator: " · ")
    }
}

/// The conflicts layer's key: what is shown, and that it is not live (BACKLOG L2 copy).
private struct ConflictLegend: View {
    var count: Int?
    var truncated = false
    var failed: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s1) {
            HStack(spacing: ShieldSpace.s2) {
                Image(systemName: HazardType.conflict.sfSymbol)
                    .font(.system(size: 11, weight: .semibold))
                    .accessibilityHidden(true)
                Text(headline)
                    .font(ShieldFont.meta.weight(.semibold))
            }
            .foregroundStyle(failed ? Color.warning : Color.ink)
            Text("Conflict data are monthly research estimates (UCDP), not live reports.")
                .font(ShieldFont.meta)
                .foregroundStyle(Color.muted)
                .fixedSize(horizontal: false, vertical: true)
        }
        .padding(.horizontal, ShieldSpace.s3)
        .padding(.vertical, ShieldSpace.s2)
        .frame(maxWidth: 360, alignment: .leading)
        .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: ShieldRadius.md))
        .overlay(RoundedRectangle(cornerRadius: ShieldRadius.md).strokeBorder(Color.ink.opacity(ShieldAlpha.hairline)))
        .accessibilityElement(children: .combine)
    }

    private var headline: String {
        if failed { return "Armed conflicts didn’t load" }
        guard let count else { return "Armed conflicts · loading…" }
        let n = count == 0 ? "none in this view" : count == 1 ? "1 in this view" : "\(count) in this view"
        return "Armed conflicts, past 90 days · \(n)" + (truncated ? " · zoom in for all" : "")
    }
}

/// *Error:* `danger` text on `panel`, with Retry. The map stays (DESIGN.md §10.1).
private struct ErrorBanner: View {
    var message: String
    var retry: () -> Void

    var body: some View {
        HStack(spacing: ShieldSpace.s3) {
            Image(systemName: "exclamationmark.triangle.fill")
                .foregroundStyle(Color.danger)
                .accessibilityHidden(true)
            Text(message)
                .font(ShieldFont.body)
                .foregroundStyle(Color.danger)
                .frame(maxWidth: .infinity, alignment: .leading)
            Button("Retry", action: retry)
                .font(ShieldFont.body.weight(.semibold))
                .buttonStyle(.plain)
                .foregroundStyle(Color.accent)
        }
        .padding(ShieldSpace.s3)
        .background(Color.panel, in: RoundedRectangle(cornerRadius: ShieldRadius.md))
        .overlay(RoundedRectangle(cornerRadius: ShieldRadius.md).strokeBorder(Color.danger.opacity(0.35)))
        .frame(maxWidth: 560)
        .accessibilityElement(children: .combine)
    }
}

/// *Empty view:* says what isn't covered instead of implying safety (DECISIONS #7).
private struct EmptyViewNotice: View {
    /// Filters are on: they, not the sources, may be what hides the events (DESIGN.md §15).
    var filtered: Bool
    var zoomOut: () -> Void
    var resetFilters: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s2) {
            EmptyViewWords(filtered: filtered)
            HStack(spacing: ShieldSpace.s5) {
                if filtered {
                    Button("Reset filters", action: resetFilters)
                }
                Button("Zoom out", action: zoomOut)
            }
            .font(ShieldFont.body.weight(.semibold))
            .buttonStyle(.plain)
            .foregroundStyle(Color.accent)
        }
        .padding(ShieldSpace.s4)
        .frame(maxWidth: 560, alignment: .leading)
        .background(Color.panel, in: RoundedRectangle(cornerRadius: ShieldRadius.lg))
        .overlay(RoundedRectangle(cornerRadius: ShieldRadius.lg).strokeBorder(Color.ink.opacity(ShieldAlpha.hairline)))
    }
}

private struct Notice: View {
    var text: String

    var body: some View {
        Text(text)
            .font(ShieldFont.meta)
            .foregroundStyle(Color.muted)
            .padding(.horizontal, ShieldSpace.s3)
            .padding(.vertical, ShieldSpace.s2)
            .background(.ultraThinMaterial, in: Capsule())
    }
}
