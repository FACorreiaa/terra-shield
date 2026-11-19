import ShieldCore
import SwiftUI

/// A country's overview (DESIGN.md §10.2): header, numbers, type × severity matrix, coverage,
/// recent changes, then its events. Hosted by the iPhone sheet and the Mac sidebar.
///
/// Its events ignore the map's filters: this is what is happening in the country, all of it.
struct CountryView: View {
    let overview: CountryOverview
    let model: LiveMapModel
    var close: () -> Void
    var selectEvent: (HazardEvent) -> Void

    /// Recent changes beyond this many wait behind "Show all".
    private static let changesShown = 4
    @State private var showsAllChanges = false

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: ShieldSpace.s5) {
                header
                content
            }
            .padding(.horizontal, ShieldSpace.s5)
            .padding(.vertical, ShieldSpace.s4)
        }
        .scrollDismissesKeyboard(.immediately)
    }

    // MARK: Header

    private var header: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s1) {
            HStack(alignment: .firstTextBaseline, spacing: ShieldSpace.s2) {
                Text(overview.name)
                    .font(ShieldFont.title)
                    .tracking(ShieldTracking.title)
                    .foregroundStyle(Color.ink)
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityAddTraits(.isHeader)
                if overview.name != overview.iso2 {
                    Text(overview.iso2)
                        .font(ShieldFont.meta.monospaced())
                        .foregroundStyle(Color.muted)
                        .accessibilityHidden(true)
                }
                Spacer(minLength: ShieldSpace.s2)
                Button(action: close) {
                    Image(systemName: "xmark.circle.fill")
                        .font(.title3)
                        .symbolRenderingMode(.hierarchical)
                        .foregroundStyle(Color.muted)
                }
                .buttonStyle(.plain)
                // No Escape shortcut here: the map screen's Escape goes search → event →
                // country, so an open event closes before its country.
                .accessibilityLabel("Close \(overview.name)")
            }
            ViewThatFits(in: .horizontal) {
                HStack(spacing: ShieldSpace.s2) {
                    status.fixedSize()
                    if overview.isSampleData { SampleDataBadge().fixedSize() }
                }
                VStack(alignment: .leading, spacing: ShieldSpace.s1) {
                    status
                    if overview.isSampleData { SampleDataBadge().fixedSize() }
                }
            }
        }
    }

    /// "Updated 2 min ago", re-read every 10 s; the failure when a refresh failed.
    private var status: some View {
        TimelineView(.periodic(from: .now, by: 10)) { context in
            Group {
                switch overview.phase {
                case .loading where overview.summary == nil:
                    Text("Loading…")
                case .failed(let failure) where overview.summary != nil:
                    Text("\(failure.message) Showing data from \(Freshness.relative(overview.summary!.generatedAt, now: context.date)).")
                        .foregroundStyle(Color.warning)
                default:
                    if let summary = overview.summary {
                        Text("Updated \(Freshness.relative(summary.generatedAt, now: context.date))")
                    }
                }
            }
            .font(ShieldFont.meta)
            .foregroundStyle(Color.muted)
            .fixedSize(horizontal: false, vertical: true)
        }
    }

    // MARK: States

    @ViewBuilder
    private var content: some View {
        if let summary = overview.summary {
            loaded(summary)
        } else {
            switch overview.phase {
            case .notFound:
                message("We have no boundary for this place.",
                        detail: "Aegis Terra can only summarise countries it has borders for.")
            case .failed(let failure):
                VStack(alignment: .leading, spacing: ShieldSpace.s3) {
                    message(failure.message, detail: nil, color: Color.danger)
                    Button("Retry") { Task { await overview.load() } }
                        .font(ShieldFont.body.weight(.semibold))
                        .buttonStyle(.plain)
                        .foregroundStyle(Color.accent)
                }
            default:
                ProgressView().frame(maxWidth: .infinity, alignment: .leading)
            }
        }
    }

    @ViewBuilder
    private func loaded(_ summary: CountrySummary) -> some View {
        StatStrip(cells: Self.cells(summary))
        if summary.country.activeEvents == 0 {
            // DESIGN.md §10.2: the mascot's EmptyState, then the coverage notice says what isn't covered.
            EmptyState {
                Text("No active events in \(summary.country.name) from covered sources.")
                    .font(ShieldFont.body)
                    .foregroundStyle(Color.ink)
            }
            CoverageNotice(countryName: summary.country.name, coverage: summary.coverage, sources: model.sources)
        } else {
            section("By type and severity") {
                TypeSeverityMatrix(rows: summary.matrix, selection: Binding(
                    get: { overview.cell }, set: { overview.cell = $0 }))
            }
            CoverageNotice(countryName: summary.country.name, coverage: summary.coverage, sources: model.sources)
            if !summary.notableChanges.isEmpty { changes(summary) }
            events(summary)
        }
    }

    static func cells(_ s: CountrySummary) -> [StatStrip.Cell] {
        let warnings = s.warningCount
        return [
            .init(value: s.country.activeEvents, label: "Active"),
            .init(value: s.highSeverityCount, label: "Severe+"),
            .init(value: warnings.value, isLowerBound: warnings.isLowerBound, label: "Official warnings"),
        ]
    }

    // MARK: Recent changes

    private func changes(_ summary: CountrySummary) -> some View {
        let all = summary.notableChanges
        let shown = showsAllChanges ? all : Array(all.prefix(Self.changesShown))
        return section("Changed in the last 24 h") {
            VStack(alignment: .leading, spacing: 0) {
                ForEach(shown) { change in
                    changeRow(change, event: summary.events.first { $0.id == change.eventID })
                }
                if all.count > Self.changesShown {
                    Button(showsAllChanges ? "Show fewer" : "Show all \(all.count)") {
                        withAnimation(ShieldMotion.standard) { showsAllChanges.toggle() }
                    }
                    .font(ShieldFont.meta.weight(.semibold))
                    .buttonStyle(.plain)
                    .foregroundStyle(Color.accent)
                    .padding(.top, ShieldSpace.s2)
                }
            }
        }
    }

    /// The event's title, then what changed and when. A row opens its event when the overview
    /// lists it (a capped list may not).
    @ViewBuilder
    private func changeRow(_ change: RecentChange, event: HazardEvent?) -> some View {
        let row = VStack(alignment: .leading, spacing: 2) {
            Text(change.title)
                .font(ShieldFont.body)
                .foregroundStyle(Color.ink)
            Text("\(change.words) · \(Freshness.relative(change.at))")
                .font(ShieldFont.meta)
                .foregroundStyle(Color.muted)
        }
        .fixedSize(horizontal: false, vertical: true)
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.vertical, ShieldSpace.s2)
        .contentShape(Rectangle())
        .accessibilityElement(children: .combine)
        if let event {
            Button { selectEvent(event) } label: { row }
                .buttonStyle(.plain)
                .accessibilityAddTraits(.isButton)
        } else {
            row
        }
    }

    // MARK: Events

    private func events(_ summary: CountrySummary) -> some View {
        let listed = overview.listedEvents
        return section("Events") {
            VStack(alignment: .leading, spacing: ShieldSpace.s2) {
                if let cell = overview.cell {
                    Button { overview.cell = nil } label: {
                        HStack(spacing: ShieldSpace.s1) {
                            Text("\(cell.type.label) · \(cell.severity.label)")
                            Image(systemName: "xmark").font(.system(size: 9, weight: .bold))
                        }
                        .font(ShieldFont.meta.weight(.semibold))
                        .foregroundStyle(Color.ink)
                        .padding(.horizontal, ShieldSpace.s3)
                        .frame(minHeight: 28)
                        .overlay(Capsule().strokeBorder(Color.accent.opacity(0.6)))
                        .contentShape(Capsule())
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel("Remove filter: \(cell.type.label), \(cell.severity.label)")
                }
                if let note = capNote(summary, listed: listed.count) {
                    Text(note)
                        .font(ShieldFont.meta)
                        .foregroundStyle(Color.muted)
                        .fixedSize(horizontal: false, vertical: true)
                }
                LazyVStack(alignment: .leading, spacing: 0) {
                    ForEach(listed) { event in
                        Button { selectEvent(event) } label: {
                            EventRow(event: event, source: model.sources[event.source],
                                     isStale: model.isStale(event),
                                     isSelected: model.selectedEventID == event.id,
                                     place: event.nearbyWords(iso2: overview.iso2, countryName: overview.name))
                        }
                        .buttonStyle(.plain)
                        .padding(.vertical, ShieldSpace.s1)
                    }
                }
            }
        }
    }

    /// Says when the list holds fewer than the numbers above it: the server lists the 100 most
    /// severe (`CountrySummary.eventLimit`).
    private func capNote(_ summary: CountrySummary, listed: Int) -> String? {
        guard summary.isListCapped else { return nil }
        if let cell = overview.cell {
            let total = summary.count(in: cell)
            guard listed < total else { return nil }
            return "Showing \(listed) of \(total). The list holds the \(summary.events.count) most severe events in \(summary.country.name)."
        }
        return "Showing the \(summary.events.count) most severe of \(summary.country.activeEvents) events."
    }

    // MARK: Layout

    private func section(_ title: String, @ViewBuilder content: () -> some View) -> some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s2) {
            Text(title.uppercased())
                .font(ShieldFont.micro)
                .tracking(ShieldTracking.micro)
                .foregroundStyle(Color.muted)
                .accessibilityAddTraits(.isHeader)
            content()
        }
    }

    private func message(_ text: String, detail: String?, color: Color = Color.ink) -> some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s1) {
            Text(text).font(ShieldFont.body).foregroundStyle(color)
            if let detail { Text(detail).font(ShieldFont.meta).foregroundStyle(Color.muted) }
        }
        .fixedSize(horizontal: false, vertical: true)
    }
}
