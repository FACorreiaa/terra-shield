#if os(iOS)
import ShieldCore
import SwiftUI

/// What the iOS bottom sheet shows (DESIGN.md §10 shells). Peek: the search field over a
/// one-line stat strip (owner, 2026-11-19). Medium: the full strip and the event list, or the
/// selected event's detail. Large: the same with room for all of it (S3.5). While searching, the
/// results replace the list. A country's overview (S3.4) replaces the strip and list until
/// closed; an event opened from it goes back to it.
struct MapSheet: View {
    @Bindable var model: LiveMapModel
    @Bindable var search: MapSearch
    /// The open country, if any.
    var country: CountryOverview?
    /// The filter sheet, presented from here: a second sheet from the map screen would fail
    /// while this one is up.
    @Binding var showsFilters: Bool
    /// The sheet sits on its peek detent.
    var isPeek: Bool
    var zoomOut: () -> Void
    /// A search result was chosen: an event (select it) or a place (move the map there).
    var selectEvent: (HazardEvent) -> Void
    var selectPlace: (MapSearch.Place) -> Void
    var selectCountry: (Country) -> Void
    var closeCountry: () -> Void
    /// The selected event in full (DESIGN.md §10.3), built by the map screen.
    var eventDetail: (HazardEvent) -> EventDetailView
    /// The height the peek needs for the search field and compact strip; grows with Dynamic Type.
    var peekHeightChanged: (CGFloat) -> Void
    /// The height of the sheet's content as it is dragged or settles.
    var heightChanged: (CGFloat) -> Void

    @FocusState private var searchFocused: Bool

    var body: some View {
        VStack(spacing: 0) {
            if let event = model.selectedEvent, !search.isActive {
                ScrollView {
                    // At medium its top (title, both severities, kind and times) is in view; the
                    // rest scrolls, and fills the sheet at large.
                    eventDetail(event)
                    .padding(.horizontal, ShieldSpace.s5)
                    .padding(.top, ShieldSpace.s5)
                    .padding(.bottom, ShieldSpace.s4)
                }
                .transition(.opacity)
            } else if let country, !search.isActive {
                CountryView(overview: country, model: model, close: closeCountry, selectEvent: selectEvent)
                    .transition(.opacity)
            } else {
                header
                Rectangle()
                    .fill(Color.ink.opacity(ShieldAlpha.hairline))
                    .frame(height: 1)
                // An overlay, so at peek the content is cut off below the header instead of
                // pushing the header out of the sheet.
                Color.clear
                    .overlay(alignment: .top) {
                        if search.isActive {
                            MapSearchResults(search: search, model: model,
                                             selectEvent: choose, selectPlace: choose,
                                             selectCountry: choose)
                        } else {
                            EventList(model: model, zoomOut: zoomOut)
                        }
                    }
                    .clipped()
                    .transition(.opacity)
            }
        }
        .frame(maxHeight: .infinity, alignment: .top)
        .onGeometryChange(for: CGFloat.self) { $0.size.height } action: { heightChanged($0) }
        .animation(ShieldMotion.spring, value: model.selectedEventID)
        .animation(ShieldMotion.standard, value: country?.iso2)
        .animation(ShieldMotion.standard, value: search.isActive)
        .sheet(isPresented: $showsFilters) {
            NavigationStack {
                FilterSheet(model: model, done: { showsFilters = false })
            }
            .presentationDetents([.large])
            .presentationBackground(Color.panel)
            .preferredColorScheme(.dark)
        }
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s3) {
            MapSearchField(search: search, focused: $searchFocused)
            // Compact at peek, where the map needs the room; the big numbers above it. Gone
            // while searching: the results need the room.
            if !search.isActive || isPeek {
                StatStrip(cells: StatStrip.Cell.liveMap(model.snapshot, includesResolved: model.query.includeResolved), style: isPeek ? .compact : .full,
                          query: model.query, tap: { model.query = model.query.toggling($0) })
            }
        }
        .padding(.horizontal, ShieldSpace.s5)
        .padding(.top, ShieldSpace.s4)
        .padding(.bottom, ShieldSpace.s3)
        .fixedSize(horizontal: false, vertical: true)
        .onGeometryChange(for: CGFloat.self) { $0.size.height } action: { height in
            // Only the peek layout sets the peek detent; the full strip is taller.
            if isPeek { peekHeightChanged(height) }
        }
    }

    private func choose(_ event: HazardEvent) {
        searchFocused = false
        search.end()
        selectEvent(event)
    }

    private func choose(_ country: Country) {
        searchFocused = false
        selectCountry(country)
    }

    private func choose(_ place: MapSearch.Place) {
        searchFocused = false
        selectPlace(place)
    }
}

/// The sheet's surface: the one place a panel floats over the map, so it is material with a
/// canvas tint (DESIGN.md §8), and opaque `panel` when transparency is reduced (§14).
struct MapSheetBackground: View {
    @Environment(\.accessibilityReduceTransparency) private var reduceTransparency
    @Environment(\.colorSchemeContrast) private var contrast

    var body: some View {
        if reduceTransparency || contrast == .increased {
            Color.panel
        } else {
            Rectangle().fill(.ultraThinMaterial).overlay(Color.canvas.opacity(0.4))
        }
    }
}
#endif
