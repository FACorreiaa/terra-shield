import ShieldCore
import SwiftUI

/// The events in the viewport, most severe first, then newest (DESIGN.md §10.1). It is the
/// accessible alternative to the map (§14), so it is hosted by the iOS sheet and by the
/// sidebar, and choosing a row selects the same event the map would.
///
/// Events inside a cluster are not listed, because the server sends a cluster as a count. The
/// list says how many it leaves out instead of looking complete.
struct EventList: View {
    @Bindable var model: LiveMapModel
    var zoomOut: () -> Void

    var body: some View {
        if let snapshot = model.snapshot {
            if snapshot.totalCount == 0 {
                message {
                    // The list, not the map, so the Sentinel may stand here (DESIGN.md §11).
                    EmptyState {
                        EmptyViewWords(filtered: !model.query.isDefault)
                    } actions: {
                        if !model.query.isDefault {
                            Button("Reset filters") { model.query = EventQuery(country: model.query.country) }
                        }
                        Button("Zoom out", action: zoomOut)
                    }
                }
            } else {
                list(snapshot)
            }
        } else if case .failed(let failure) = model.phase {
            message {
                Text(failure.message)
                    .font(ShieldFont.body)
                    .foregroundStyle(Color.danger)
                Button("Retry", action: model.reload)
                    .font(ShieldFont.body.weight(.semibold))
                    .buttonStyle(.plain)
                    .foregroundStyle(Color.accent)
            }
        } else {
            message {
                Text("Loading events…")
                    .font(ShieldFont.body)
                    .foregroundStyle(Color.muted)
            }
        }
    }

    private func list(_ snapshot: MapSnapshot) -> some View {
        // Most severe, then newest; resolved events last.
        let events = snapshot.events.sortedForDisplay()
        let clustered = snapshot.totalCount - snapshot.events.count
        return rows(events)
            .safeAreaInset(edge: .bottom, spacing: 0) {
                if clustered > 0 || snapshot.truncated {
                    footnote(clustered: clustered, listed: events.count, truncated: snapshot.truncated)
                }
            }
    }

    @ViewBuilder
    private func rows(_ events: [HazardEvent]) -> some View {
        #if os(macOS)
        // A selection list, so ↑ / ↓ move through the events (DESIGN.md §10 shortcuts).
        List(events, selection: $model.selectedEventID) { event in
            row(event)
        }
        .listStyle(.sidebar)
        #else
        List(events) { event in
            Button { model.selectedEventID = event.id } label: { row(event) }
                .buttonStyle(.plain)
                .listRowBackground(model.selectedEventID == event.id ? Color.raised : Color.clear)
                .listRowSeparatorTint(Color.hairline)
                .listRowInsets(EdgeInsets(top: ShieldSpace.s2, leading: ShieldSpace.s5,
                                          bottom: ShieldSpace.s2, trailing: ShieldSpace.s5))
        }
        .listStyle(.plain)
        .scrollContentBackground(.hidden)
        #endif
    }

    private func row(_ event: HazardEvent) -> some View {
        EventRow(event: event, source: model.sources[event.source],
                 isStale: model.isStale(event), isSelected: model.selectedEventID == event.id)
    }

    private func footnote(clustered: Int, listed: Int, truncated: Bool) -> some View {
        var lines: [String] = []
        if clustered > 0 {
            let events = clustered == 1 ? "1 event is" : "\(clustered) events are"
            lines.append(listed == 0
                ? "\(events) grouped on the map. Zoom in to list them."
                : "\(events) also grouped on the map. Zoom in to list them.")
        }
        if truncated { lines.append("Showing the most severe events here. Zoom in to see them all.") }
        return Text(lines.joined(separator: "\n"))
            .font(ShieldFont.meta)
            .foregroundStyle(Color.muted)
            .fixedSize(horizontal: false, vertical: true)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, ShieldSpace.s5)
            .padding(.vertical, ShieldSpace.s3)
            // Opaque: it sits on the sheet's material, and translucent layers never stack
            // (DESIGN.md §8).
            .background(Color.panel)
    }

    private func message<Content: View>(@ViewBuilder _ content: () -> Content) -> some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s2) { content() }
            .padding(.horizontal, ShieldSpace.s5)
            .padding(.vertical, ShieldSpace.s4)
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
    }
}

/// What an empty view says. With filters on it says so first: the filters, not the sources, may
/// be what hides events here (DESIGN.md §15, data honesty).
struct EmptyViewWords: View {
    var filtered: Bool

    var body: some View {
        Text(filtered ? "No events in this view match your filters."
                      : "No active events in this view from covered sources.")
            .font(ShieldFont.body)
            .foregroundStyle(Color.ink)
        Text("Absence of data is not safety. Official warnings come from national services.")
            .font(ShieldFont.meta)
            .foregroundStyle(Color.muted)
    }
}
