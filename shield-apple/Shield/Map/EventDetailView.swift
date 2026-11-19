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
    /// The type's background behind the header (DESIGN.md §12); the gallery turns it off.
    var headerArt = true
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
            EventMapSnippet(event: event, geometry: loadedGeometry,
                            isFaded: event.isResolved || isGone || isStale, showOnMap: showOnMap)
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
                // Gone: the event's own page is likely dead, so the button opens the source's home.
                SourceBadge(source: source, url: isGone ? nil : event.url, style: .full, checkedAt: source.lastSuccessAt)
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

    private var loadedGeometry: EventGeometry? {
        if case .loaded(let detail) = state { detail.geometry } else { nil }
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
                         isStale: isStale && !isGone, isResolved: event.isResolved, baseSize: 30, pulses: false)
                // Gone fades without a badge: neither resolved (checkmark) nor waiting on its source.
                .saturation(isGone ? 0 : 1)
                .opacity(isGone ? ShieldAlpha.stale : 1)
                .frame(width: 44, height: 44)
            VStack(alignment: .leading, spacing: ShieldSpace.s1) {
                Text(event.title)
                    .font(ShieldFont.title)
                    .tracking(ShieldTracking.title)
                    .foregroundStyle(event.isResolved || isGone ? Color.muted : Color.ink)
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityAddTraits(.isHeader)
                places
                if let note = event.locationNote {
                    Text(note)
                        .font(ShieldFont.meta)
                        .foregroundStyle(Color.muted)
                        .fixedSize(horizontal: false, vertical: true)
                }
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
        .background {
            if headerArt, let art = event.type.headerArt {
                HeaderArt(name: art, isFaded: event.isResolved || isGone)
            }
        }
    }

    /// "Portugal, Spain" or "Off the coast of Chile, 38 km" (DESIGN.md §15): every country, none
    /// picked. One text, so a long list wraps; each name is a link that opens its overview.
    @ViewBuilder private var places: some View {
        if !event.countries.isEmpty {
            Label {
                Text(placesText)
                    .fixedSize(horizontal: false, vertical: true)
                    .environment(\.openURL, OpenURLAction { url in
                        guard url.scheme == Self.countryScheme, let iso2 = url.host() else { return .systemAction }
                        openCountry?(iso2.uppercased())
                        return .handled
                    })
            } icon: {
                Image(systemName: "location").foregroundStyle(Color.muted)
            }
            .labelStyle(.titleAndIcon)
            .font(ShieldFont.meta)
            .foregroundStyle(Color.muted)
        }
    }

    private static let countryScheme = "aegis-country"

    private var placesText: AttributedString {
        var text = AttributedString()
        for (index, country) in event.countries.enumerated() {
            if index > 0 { text += AttributedString(", ") }
            var name = AttributedString(country.phrase(name: countryName(country.iso2)))
            if openCountry != nil, let url = URL(string: "\(Self.countryScheme)://\(country.iso2.lowercased())") {
                name.link = url
                name.foregroundColor = .accent
            }
            text += name
        }
        return text
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
            DisclaimerBanner(style: .footer)
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

// MARK: - Header art (DESIGN.md §12)

extension HazardType {
    /// The detail header's background, for the types §12 gives one; nil keeps the plain panel.
    var headerArt: String? {
        switch self {
        case .storm, .tropicalCyclone, .heavyRain, .flood, .wind: "HeaderPrecipitation"
        case .wildfire, .heat, .drought, .volcano: "HeaderWildfire"
        default: nil
        }
    }
}

/// The type's art at 30%, bled past the header's padding and faded out radially (§12's vignette)
/// to transparent rather than to a colour, so it melts into the sheet or inspector alike.
/// Decoration only: hidden from VoiceOver, and desaturated with a resolved or gone event.
private struct HeaderArt: View {
    let name: String
    let isFaded: Bool

    var body: some View {
        Image(name)
            .resizable()
            .scaledToFill()
            .opacity(0.3)
            .saturation(isFaded ? 0 : 1)
            .mask {
                EllipticalGradient(colors: [.black, .clear], center: .center,
                                   startRadiusFraction: 0.1, endRadiusFraction: 0.5)
            }
            .padding(-ShieldSpace.s5)
            .accessibilityHidden(true)
            .allowsHitTesting(false)
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
            Banner(icon: "questionmark.folder",
                   text: "This event is no longer published by its source. Open official source goes to \(source?.name ?? "the source")’s home page.",
                   color: .muted)
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
