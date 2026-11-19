#if os(iOS)
import ShieldCore
import SwiftUI

/// What the iOS bottom sheet shows (DESIGN.md §10 shells). Peek: the `StatStrip`. Medium: the
/// event list, or the selected event. Large: the selected event with room for the full detail
/// (S3.5), or the whole list.
struct MapSheet: View {
    @Bindable var model: LiveMapModel
    var zoomOut: () -> Void
    /// The height the peek detent needs to show the stat strip, which grows with Dynamic Type.
    var peekHeightChanged: (CGFloat) -> Void
    /// The height of the sheet's content as it is dragged or settles.
    var heightChanged: (CGFloat) -> Void

    var body: some View {
        VStack(spacing: 0) {
            if let event = model.selectedEvent {
                ScrollView {
                    SelectedEventSummary(
                        event: event,
                        source: model.sources[event.source],
                        isStale: model.isStale(event),
                        showsSummary: true,
                        close: { model.selectedEventID = nil }
                    )
                    .padding(.horizontal, ShieldSpace.s5)
                    .padding(.top, ShieldSpace.s5)
                    .padding(.bottom, ShieldSpace.s4)
                }
                .transition(.opacity)
            } else {
                StatStrip(cells: StatStrip.Cell.liveMap(model.snapshot),
                          isSample: model.provider.isSampleData)
                    .padding(.horizontal, ShieldSpace.s5)
                    .padding(.top, ShieldSpace.s5)
                    .padding(.bottom, ShieldSpace.s3)
                    .fixedSize(horizontal: false, vertical: true)
                    .onGeometryChange(for: CGFloat.self) { $0.size.height } action: { peekHeightChanged($0) }
                Rectangle()
                    .fill(Color.ink.opacity(ShieldAlpha.hairline))
                    .frame(height: 1)
                // An overlay, so at peek the list is cut off below the strip instead of pushing
                // the strip out of the sheet.
                Color.clear
                    .overlay(alignment: .top) { EventList(model: model, zoomOut: zoomOut) }
                    .clipped()
                    .transition(.opacity)
            }
        }
        .frame(maxHeight: .infinity, alignment: .top)
        .onGeometryChange(for: CGFloat.self) { $0.size.height } action: { heightChanged($0) }
        .animation(ShieldMotion.spring, value: model.selectedEventID)
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
