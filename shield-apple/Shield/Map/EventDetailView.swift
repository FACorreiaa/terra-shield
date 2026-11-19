import ShieldCore
import SwiftUI

/// One event in full (DESIGN.md §10.3): what it is and where, both severities and why, kind and
/// times, key figures, who said so with the way to their record, and what changed. The iPhone
/// sheet shows it from medium (its top reads like the old summary) and scrolls at large; the Mac
/// and iPad show it in the inspector.
///
/// `event` is the one on the map (or the detail's own copy when the map no longer holds it);
/// `state` adds the source record and revisions as they load.
struct EventDetailView: View {
    let event: HazardEvent
    let state: LiveMapModel.DetailState?
    /// From `/v1/sources`, else the detail's own `sourceInfo`.
    let source: Source?
    let isStale: Bool
    /// Display name for an ISO code.
    var countryName: (String) -> String
    var openCountry: ((String) -> Void)? = nil
    var showOnMap: (() -> Void)? = nil
    var close: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s5) {
            header
            if isGone {
                GoneBanner(source: source)
            } else if isStale && !event.isResolved {
                StaleBanner(lastUpdate: source?.lastSuccessAt ?? event.fetchedAt)
            }
            SeverityBlock(event: event)
            kindAndTimes
            if let summary = event.summary, !summary.isEmpty {
                Text(summary)
                    .font(ShieldFont.body)
                    .foregroundStyle(Color.ink)
                    .fixedSize(horizontal: false, vertical: true)
            }
            KeyFigures(figures: event.keyFigures)
            if let showOnMap {
                Button(action: showOnMap) {
                    Label("Show on map", systemImage: "map")
                        .font(ShieldFont.body.weight(.semibold))
                }
                .buttonStyle(.plain)
                .foregroundStyle(Color.accent)
            }
            divider
            if let source {
                SourceBadge(source: source, url: event.url, style: .full, fetchedAt: event.fetchedAt)
            } else if let url = event.url {
                // The source list hasn't loaded: the official record still opens.
                OfficialSourceButton(url: url)
            }
            divider
            RevisionTimeline(state: state)
            divider
            footer
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private var isGone: Bool {
        if case .gone = state { true } else { false }
    }

    private var divider: some View {
        Rectangle().fill(Color.ink.opacity(ShieldAlpha.hairline)).frame(height: 1)
    }

    // MARK: Header band

    private var header: some View {
        HStack(alignment: .top, spacing: ShieldSpace.s3) {
            HazardMarker(type: event.type, severity: event.severity, kind: event.kind,
                         isStale: isStale, isResolved: event.isResolved || isGone, baseSize: 30, pulses: false)
                .frame(width: 44, height: 44)
            VStack(alignment: .leading, spacing: ShieldSpace.s1) {
                Text(event.title)
                    .font(ShieldFont.title)
                    .tracking(ShieldTracking.title)
                    .foregroundStyle(event.isResolved || isGone ? Color.muted : Color.ink)
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityAddTraits(.isHeader)
                places
            }
            Spacer(minLength: ShieldSpace.s2)
            Button(action: close) {
                Image(systemName: "xmark")
                    .font(.system(size: 13, weight: .bold))
                    .foregroundStyle(Color.muted)
                    .frame(width: 32, height: 32)
                    .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityLabel("Close")
        }
    }

    /// "Portugal, Spain" or "Off the coast of Chile, 38 km" (DESIGN.md §15): every country, none
    /// picked. Each opens its overview.
    @ViewBuilder private var places: some View {
        if !event.countries.isEmpty {
            HStack(spacing: ShieldSpace.s1) {
                Image(systemName: "location")
                    .foregroundStyle(Color.muted)
                    .accessibilityHidden(true)
                ForEach(Array(event.countries.enumerated()), id: \.element.iso2) { index, country in
                    let words = country.phrase(name: countryName(country.iso2)) + (index < event.countries.count - 1 ? "," : "")
                    if let openCountry {
                        Button(words) { openCountry(country.iso2) }
                            .buttonStyle(.plain)
                            .foregroundStyle(Color.accent)
                            .accessibilityHint("Opens the country overview")
                    } else {
                        Text(words).foregroundStyle(Color.muted)
                    }
                }
            }
            .font(ShieldFont.meta)
            .lineLimit(2)
        }
    }

    // MARK: Kind, status, times

    private var kindAndTimes: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s2) {
            HStack(spacing: ShieldSpace.s2) {
                KindChip(kind: event.kind)
                if event.isResolved {
                    ResolvedLabel(event: event)
                } else if !isGone {
                    Text("Active")
                        .font(ShieldFont.meta)
                        .foregroundStyle(Color.muted)
                }
            }
            TimelineView(.periodic(from: .now, by: 30)) { context in
                Grid(alignment: .leading, horizontalSpacing: ShieldSpace.s3, verticalSpacing: ShieldSpace.s1) {
                    ForEach(event.detailTimes(now: context.date)) { time in
                        GridRow {
                            Text(time.label)
                                .foregroundStyle(Color.muted)
                            Text(Freshness.stamp(time.date, now: context.date))
                                .foregroundStyle(Color.ink)
                                .monospacedDigit()
                        }
                        .accessibilityElement(children: .combine)
                    }
                }
                .font(ShieldFont.meta)
            }
        }
    }

    // MARK: Footer

    private var footer: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s3) {
            Text("Aegis Terra does not replace your national emergency service. Always follow official instructions.")
                .font(ShieldFont.meta)
                .foregroundStyle(Color.muted)
                .fixedSize(horizontal: false, vertical: true)
            if let url = event.url ?? source?.url {
                // No public event page until the web app ships, so this shares the source's record.
                ShareLink(item: url, subject: Text(event.title), message: Text(event.title)) {
                    Label("Share", systemImage: "square.and.arrow.up")
                        .font(ShieldFont.body.weight(.semibold))
                }
                .buttonStyle(.plain)
                .foregroundStyle(Color.accent)
            }
        }
    }
}

// MARK: - Severity

/// `SEVERITY · AEGIS / SOURCE`: the app's badge beside the source's value, verbatim and
/// uncoloured (DESIGN.md §4), then why the app's level reads as it does.
private struct SeverityBlock: View {
    let event: HazardEvent

    var body: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s2) {
            Text("SEVERITY · AEGIS / SOURCE")
                .font(ShieldFont.micro)
                .tracking(ShieldTracking.micro)
                .foregroundStyle(Color.faint)
            HStack(alignment: .firstTextBaseline, spacing: ShieldSpace.s2) {
                SeverityBadge(severity: event.severity, isProvisional: event.showsProvisional)
                    .fixedSize() // the source's text wraps, the badge never does
                if let source = event.sourceSeverity, !source.isEmpty {
                    Text(source)
                        .font(ShieldFont.meta)
                        .foregroundStyle(Color.muted)
                        .fixedSize(horizontal: false, vertical: true)
                        .accessibilityLabel("Source says \(source)")
                }
            }
            if let rating = event.severityRating {
                // Neutral: it explains the level, it isn't one (DESIGN.md §1).
                Label(rating, systemImage: event.isPrescribed ? "flame" : "ruler")
                    .labelStyle(.titleAndIcon)
                    .font(ShieldFont.meta.weight(.semibold))
                    .foregroundStyle(Color.ink)
            }
            if let note = event.severityNote {
                Text(note)
                    .font(ShieldFont.meta)
                    .foregroundStyle(Color.muted)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
    }
}

// MARK: - Key figures

/// Type-specific facts from `attrs`, only those present (DESIGN.md §10.3 item 4). The lead
/// figure is large; the rest are label and value.
private struct KeyFigures: View {
    let figures: [KeyFigure]

    var body: some View {
        if !figures.isEmpty {
            VStack(alignment: .leading, spacing: ShieldSpace.s3) {
                if let lead = figures.first(where: \.isLead) {
                    VStack(alignment: .leading, spacing: 0) {
                        Text(lead.value)
                            .font(ShieldFont.statHero)
                            .tracking(ShieldTracking.statHero)
                            .foregroundStyle(Color.ink)
                            .lineLimit(1)
                            .minimumScaleFactor(0.6)
                        Text(lead.label)
                            .font(ShieldFont.meta)
                            .foregroundStyle(Color.muted)
                    }
                    .accessibilityElement(children: .combine)
                }
                let rest = figures.filter { !$0.isLead }
                if !rest.isEmpty {
                    Grid(alignment: .topLeading, horizontalSpacing: ShieldSpace.s3, verticalSpacing: ShieldSpace.s2) {
                        ForEach(rest) { figure in
                            GridRow {
                                Text(figure.label)
                                    .foregroundStyle(Color.muted)
                                Text(figure.value)
                                    .foregroundStyle(Color.ink)
                                    .fixedSize(horizontal: false, vertical: true)
                            }
                            .accessibilityElement(children: .combine)
                        }
                    }
                    .font(ShieldFont.meta)
                }
            }
        }
    }
}

// MARK: - Timeline

/// What changed, newest first, each in words with its time (DESIGN.md §10.3 item 7).
private struct RevisionTimeline: View {
    let state: LiveMapModel.DetailState?

    var body: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s3) {
            Text("TIMELINE")
                .font(ShieldFont.micro)
                .tracking(ShieldTracking.micro)
                .foregroundStyle(Color.faint)
            switch state {
            case .loaded(let detail) where !detail.timeline.isEmpty:
                TimelineView(.periodic(from: .now, by: 30)) { context in
                    VStack(alignment: .leading, spacing: 0) {
                        ForEach(Array(detail.timeline.enumerated()), id: \.element.id) { index, revision in
                            row(revision, isLast: index == detail.timeline.count - 1, now: context.date)
                        }
                    }
                }
            case .loaded:
                message("No changes recorded yet.")
            case .loading, nil:
                HStack(spacing: ShieldSpace.s2) {
                    ProgressView().controlSize(.small)
                    message("Loading history…")
                }
            case .failed:
                message("The history couldn’t be loaded.")
            case .gone:
                message("No history: the source no longer publishes this event.")
            }
        }
    }

    private func row(_ revision: EventRevision, isLast: Bool, now: Date) -> some View {
        HStack(alignment: .top, spacing: ShieldSpace.s3) {
            // A dot on a hairline rail; the rail stops at the oldest entry.
            VStack(spacing: 0) {
                Circle()
                    .fill(Color.muted)
                    .frame(width: 7, height: 7)
                    .padding(.top, 5)
                if !isLast {
                    Rectangle()
                        .fill(Color.ink.opacity(ShieldAlpha.hairlineStrong))
                        .frame(width: 1)
                }
            }
            .frame(width: 7)
            VStack(alignment: .leading, spacing: 2) {
                Text(revision.words)
                    .font(ShieldFont.body)
                    .foregroundStyle(Color.ink)
                    .fixedSize(horizontal: false, vertical: true)
                Text(Freshness.stamp(revision.at, now: now))
                    .font(ShieldFont.meta)
                    .foregroundStyle(Color.muted)
            }
            .padding(.bottom, isLast ? 0 : ShieldSpace.s3)
        }
        .accessibilityElement(children: .combine)
    }

    private func message(_ text: String) -> some View {
        Text(text)
            .font(ShieldFont.meta)
            .foregroundStyle(Color.muted)
    }
}

// MARK: - Banners

/// The event's source has missed three polls (DECISIONS #8).
private struct StaleBanner: View {
    let lastUpdate: Date

    var body: some View {
        TimelineView(.periodic(from: .now, by: 30)) { context in
            Banner(icon: "clock.badge.exclamationmark",
                   text: "Stale · this source last updated \(Freshness.relative(lastUpdate, now: context.date)). The event may have changed since.",
                   color: .warning)
        }
    }
}

/// A 404 or a retraction on the stream (DESIGN.md §10.3 "Gone").
private struct GoneBanner: View {
    let source: Source?

    var body: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s2) {
            Banner(icon: "questionmark.folder", text: "This event is no longer published by its source.", color: .muted)
            if let source {
                Link(destination: source.url) {
                    Label("Open \(source.name)", systemImage: "arrow.up.right.square")
                        .font(ShieldFont.meta.weight(.semibold))
                }
                .foregroundStyle(Color.accent)
            }
        }
    }
}

private struct Banner: View {
    let icon: String
    let text: String
    let color: Color

    var body: some View {
        Label {
            Text(text).fixedSize(horizontal: false, vertical: true)
        } icon: {
            Image(systemName: icon)
        }
        .font(ShieldFont.meta)
        .foregroundStyle(color)
        .padding(ShieldSpace.s3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Color.raised.opacity(0.5), in: RoundedRectangle(cornerRadius: ShieldRadius.sm))
        .overlay(RoundedRectangle(cornerRadius: ShieldRadius.sm).strokeBorder(Color.ink.opacity(ShieldAlpha.hairline)))
    }
}

#Preview("Event detail", traits: .sizeThatFitsLayout) {
    let event = SampleData.events()[0]
    ScrollView {
        VStack(alignment: .leading) {
            SampleDataBadge()
            EventDetailView(event: event, state: .loaded(EventDetail(event: event, geometry: .point(event.centroid))),
                            source: SampleData.sources()[0], isStale: false,
                            countryName: { Locale(identifier: "en").localizedString(forRegionCode: $0) ?? $0 },
                            close: {})
        }
        .padding(ShieldSpace.s5)
    }
    .frame(width: 380, height: 900)
    .background(Color.panel)
}
