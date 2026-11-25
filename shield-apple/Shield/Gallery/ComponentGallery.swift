import ShieldCore
import SwiftUI

/// S3.1's root: the core components on sample data, on both platforms. S3.2 replaces it with
/// the live map; it then stays reachable from Debug builds as a visual check.
struct ComponentGallery: View {
    let provider: any HazardEventProvider

    @State private var events: [HazardEvent] = []
    @State private var sources: [String: Source] = [:]
    @State private var selected: HazardEvent.ID?

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: ShieldSpace.s6) {
                header
                // Screenshot automation: `SHIELD_GALLERY=detail` puts the detail states first,
                // `=tsunami` the tsunami forecast and warning.
                switch ProcessInfo.processInfo.environment["SHIELD_GALLERY"] {
                case "detail": section("Event detail states") { detailStates }
                case "tsunami": section("Tsunami detail") { tsunamiDetails }
                case "sources": section("Sources") { sourceStates }
                case "empty": section("Empty states") { emptyStates }
                default: EmptyView()
                }
                // Two columns when there is room (macOS, iPad landscape), one on iPhone.
                ViewThatFits(in: .horizontal) {
                    HStack(alignment: .top, spacing: ShieldSpace.s6) {
                        componentColumn.frame(minWidth: 400, maxWidth: 480)
                        section("Events") { eventList }.frame(minWidth: 400)
                    }
                    VStack(alignment: .leading, spacing: ShieldSpace.s6) {
                        componentColumn
                        section("Events") { eventList }
                    }
                }
            }
            .padding(ShieldSpace.s5)
            .frame(maxWidth: 1200, alignment: .leading)
            .frame(maxWidth: .infinity)
        }
        .background(Color.canvas.ignoresSafeArea())
        .task { await load() }
    }

    private var componentColumn: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s6) {
            section("Stat strip") {
                VStack(alignment: .leading, spacing: ShieldSpace.s4) {
                    StatStrip(cells: StatStrip.Cell.liveMap(snapshot), isSample: provider.isSampleData)
                    StatStrip(cells: StatStrip.Cell.liveMap(snapshot), style: .compact)
                }
            }
            section("Markers") { markerGrid }
            section("Severity") { severityList }
            section("Kind") { kindRow }
            section("Event detail states") { detailStates }
            section("Sources") { sourceStates }
            section("Empty states") { emptyStates }
        }
    }

    /// The detail's states that live data rarely shows (DESIGN.md §10.3): gone, loading and
    /// failed, on an event across three countries and offshore of a fourth, so the place line wraps.
    @ViewBuilder private var detailStates: some View {
        if var event = events.first {
            let _ = event.countries = [
                EventCountry(iso2: "PT", relation: .within), EventCountry(iso2: "ES", relation: .within),
                EventCountry(iso2: "FR", relation: .within), EventCountry(iso2: "MA", relation: .nearby, distanceKm: 38),
            ]
            let states: [(String, LiveMapModel.DetailState)] = [("Gone", .gone(event)), ("Loading", .loading), ("Failed", .failed)]
            ViewThatFits(in: .horizontal) {
                HStack(alignment: .top, spacing: ShieldSpace.s4) { detailCards(event, states) }
                VStack(alignment: .leading, spacing: ShieldSpace.s4) { detailCards(event, states) }
            }
        }
    }

    /// A PTWC threat forecast and an NTWC warning, shaped like #25's attrs (sample data).
    /// DESIGN.md §10.7: both disclaimer styles and a source row in each health (sample data).
    private var sourceStates: some View {
        let now = Date.now
        func source(_ name: String, success: TimeInterval?, error: TimeInterval? = nil, stale: Bool,
                    poll: Int, cadence: String? = nil, coverage: String? = nil) -> Source {
            var s = Source(id: name, name: name, url: URL(string: "https://example.org")!,
                           license: "CC BY 4.0", attribution: "\(name) contributors",
                           pollIntervalSeconds: poll,
                           lastSuccessAt: success.map { now.addingTimeInterval($0) },
                           lastErrorAt: error.map { now.addingTimeInterval($0) }, stale: stale)
            s.cadence = cadence
            s.coverage = coverage
            s.termsUrl = coverage == nil ? nil : URL(string: "https://example.org/terms")
            return s
        }
        let rows = [
            source("Healthy source", success: -120, error: -3000, stale: false, poll: 600,
                   coverage: "Earthquakes M2.5+ worldwide"),
            source("Failing source", success: -3600, error: -180, stale: false, poll: 300),
            source("Stale source", success: -19 * 3600, stale: true, poll: 600),
            source("Monthly source", success: -1800, stale: false, poll: 21_600, cadence: "monthly release",
                   coverage: "Armed-conflict events, monthly research estimates"),
            {
                // Fetched only while its layer is on (adsb.lol, #43).
                var s = source("On-demand source", success: nil, stale: false, poll: 60,
                               coverage: "Live aircraft positions")
                s.onDemand = true
                return s
            }(),
        ]
        return VStack(alignment: .leading, spacing: ShieldSpace.s3) {
            DisclaimerBanner(style: .inline)
            DisclaimerBanner(style: .footer)
            // The on-demand row first: it's the newest case (#43).
            ForEach(rows.filter { $0.onDemand == true } + rows.filter { $0.onDemand != true }) {
                SourceRow(source: $0, now: now)
            }
        }
        .padding(ShieldSpace.s4)
        .frame(maxWidth: 420)
        .background(Color.panel, in: RoundedRectangle(cornerRadius: ShieldRadius.md))
    }

    /// DESIGN.md §11: the Sentinel beside the words of an empty list or country.
    private var emptyStates: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s5) {
            EmptyState {
                EmptyViewWords(filtered: false)
            } actions: {
                Button("Zoom out") {}
            }
            EmptyState {
                EmptyViewWords(filtered: true)
            } actions: {
                Button("Reset filters") {}
                Button("Zoom out") {}
            }
            EmptyState {
                Text("No active events in Iceland from covered sources.")
                    .font(ShieldFont.body).foregroundStyle(Color.ink)
            }
        }
        .padding(ShieldSpace.s4)
        .frame(maxWidth: 420)
        .background(Color.panel, in: RoundedRectangle(cornerRadius: ShieldRadius.md))
    }

    private var tsunamiDetails: some View {
        let now = Date.now
        func event(kind: EventKind, severity: Severity, title: String, provisional: Bool,
                   countries: [EventCountry], attrs: [String: JSONValue]) -> HazardEvent {
            HazardEvent(id: UUID(), source: "tsunami", type: .tsunami, kind: kind, severity: severity,
                        sourceSeverity: kind == .forecast ? "Threat forecast" : "Tsunami Warning",
                        severityProvisional: provisional, title: title,
                        summary: kind == .forecast ? "Guidance for national authorities, not a warning. Hazardous tsunami waves are forecast for some coasts." : nil,
                        centroid: Coordinate(latitude: 52.5, longitude: 160), countries: countries,
                        observedAt: now.addingTimeInterval(-1200), updatedAt: now.addingTimeInterval(-600),
                        fetchedAt: now, attrs: attrs)
        }
        let common: [String: JSONValue] = ["product": .string("WEPA40"), "bulletin": .number(3),
                                           "incident": .string("x"), "magnitude": .number(8.8),
                                           "magnitudeType": .string("Mww")]
        let forecast = event(kind: .forecast, severity: .severe, title: "Tsunami threat forecast · Kamchatka",
                             provisional: false,
                             countries: [EventCountry(iso2: "JP", relation: .within), EventCountry(iso2: "RU", relation: .within)],
                             attrs: common.merging([
                                "center": .string("PTWC"), "geometry": .string("epicenter"), "guidance": .bool(true),
                                "forecast": .array([
                                    .object(["band": .string("1–3 m"), "places": .array([.string("Japan"), .string("Russia")])]),
                                    .object(["band": .string("0.3–1 m"), "places": .array([.string("Hawaii"), .string("Chile"), .string("Peru")])]),
                                ]),
                                "unmatchedPlaces": .array([.string("Wake Island")]),
                             ]) { $1 })
        let warning = event(kind: .warning, severity: .extreme, title: "Tsunami Warning · Alaska",
                            provisional: false, countries: [EventCountry(iso2: "US", relation: .within)],
                            attrs: common.merging([
                                "center": .string("NTWC"), "geometry": .string("zones"),
                                "areas": .array([.string("Chignik Bay, Alaska to Samalga Pass, Alaska"),
                                                 .string("Samalga Pass, Alaska to Attu, Alaska")]),
                                "zonesUnresolved": .array([.string("PZZ530"), .string("BCZ921")]),
                            ]) { $1 })
        return HStack(alignment: .top, spacing: ShieldSpace.s4) {
            ForEach([forecast, warning]) { e in
                EventDetailView(event: e, state: .loaded(EventDetail(event: e, geometry: .point(e.centroid))),
                                source: nil, isStale: false,
                                countryName: { Locale(identifier: "en").localizedString(forRegionCode: $0) ?? $0 },
                                openCountry: { _ in }, close: {})
                    .padding(ShieldSpace.s4)
                    .frame(width: 320)
                    .background(Color.panel, in: RoundedRectangle(cornerRadius: ShieldRadius.md))
            }
        }
    }

    private func detailCards(_ event: HazardEvent, _ states: [(String, LiveMapModel.DetailState)]) -> some View {
        ForEach(states, id: \.0) { label, state in
            VStack(alignment: .leading, spacing: ShieldSpace.s2) {
                columnLabel(label)
                EventDetailView(event: event, state: state, source: sources[event.source], isStale: false,
                                countryName: { Locale(identifier: "en").localizedString(forRegionCode: $0) ?? $0 },
                                openCountry: { _ in }, close: {})
                    .padding(ShieldSpace.s4)
                    .frame(width: 320)
                    .background(Color.panel, in: RoundedRectangle(cornerRadius: ShieldRadius.md))
            }
        }
    }

    private var header: some View {
        HStack(alignment: .firstTextBaseline) {
            VStack(alignment: .leading, spacing: ShieldSpace.s1) {
                Text("AEGIS TERRA · COMPONENTS")
                    .font(ShieldFont.micro)
                    .tracking(ShieldTracking.micro)
                    .foregroundStyle(Color.faint)
                Text("Core components")
                    .font(ShieldFont.title)
                    .tracking(ShieldTracking.title)
                    .foregroundStyle(Color.ink)
            }
            Spacer()
            if provider.isSampleData { SampleDataBadge() }
        }
    }

    private func section<Content: View>(_ title: String, @ViewBuilder content: () -> Content) -> some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s3) {
            Text(title.uppercased())
                .font(ShieldFont.micro)
                .tracking(ShieldTracking.micro)
                .foregroundStyle(Color.muted)
            content()
                .padding(ShieldSpace.s4)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(Color.panel, in: RoundedRectangle(cornerRadius: ShieldRadius.md, style: .continuous))
                .overlay(
                    RoundedRectangle(cornerRadius: ShieldRadius.md, style: .continuous)
                        .strokeBorder(Color.ink.opacity(ShieldAlpha.hairline), lineWidth: 1)
                )
        }
    }

    /// Kind label above each row; columns are the four severities plus a stale example.
    private var markerGrid: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s3) {
            HStack(spacing: ShieldSpace.s2) {
                ForEach(Severity.allCases, id: \.self) { columnLabel($0.label) }
                columnLabel("Stale")
            }
            ForEach(EventKind.allCases, id: \.self) { kind in
                VStack(alignment: .leading, spacing: ShieldSpace.s1) {
                    Text(kind.label)
                        .font(ShieldFont.meta)
                        .foregroundStyle(Color.muted)
                    HStack(spacing: ShieldSpace.s2) {
                        ForEach(Severity.allCases, id: \.self) { s in
                            HazardMarker(type: .earthquake, severity: s, kind: kind)
                                .frame(width: Self.column, height: 48)
                        }
                        HazardMarker(type: .flood, severity: .severe, kind: kind, isStale: true)
                            .frame(width: Self.column, height: 48)
                    }
                }
            }
        }
    }

    private static let column: CGFloat = 54

    private func columnLabel(_ text: String) -> some View {
        Text(text)
            .font(ShieldFont.micro)
            .foregroundStyle(Color.faint)
            .lineLimit(1)
            .minimumScaleFactor(0.8)
            .frame(width: Self.column)
    }

    private var severityList: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s3) {
            SeverityBadge(severity: .extreme, sourceSeverity: "GDACS Red")
            SeverityBadge(severity: .severe, sourceSeverity: "M6.1 mww, PAGER yellow")
            SeverityBadge(severity: .moderate, sourceSeverity: "Level 2 · Yellow")
            SeverityBadge(severity: .minor)
        }
    }

    private var kindRow: some View {
        ViewThatFits(in: .horizontal) {
            HStack(spacing: ShieldSpace.s2) { kinds }
            VStack(alignment: .leading, spacing: ShieldSpace.s2) { kinds }
        }
    }

    @ViewBuilder private var kinds: some View {
        ForEach(EventKind.allCases, id: \.self) { KindChip(kind: $0) }
    }

    private var eventList: some View {
        VStack(alignment: .leading, spacing: 0) {
            ForEach(Array(events.enumerated()), id: \.element.id) { index, event in
                if index > 0 {
                    Rectangle().fill(Color.ink.opacity(ShieldAlpha.hairline)).frame(height: 1)
                }
                EventRow(event: event, source: sources[event.source],
                         isStale: sources[event.source]?.isStale() ?? false, isSelected: selected == event.id)
                    .padding(.vertical, ShieldSpace.s2)
                    .onTapGesture { selected = selected == event.id ? nil : event.id }
            }
        }
    }

    /// The loaded events as the map would receive them, for the stat strip.
    private var snapshot: MapSnapshot? {
        events.isEmpty ? nil : MapSnapshot(events: events, clusters: [], truncated: false, generatedAt: .now)
    }

    private func load() async {
        do {
            events = try await provider.events(matching: EventQuery())
            sources = Dictionary(uniqueKeysWithValues: try await provider.sources().map { ($0.id, $0) })
        } catch {
            events = []
        }
    }
}

#Preview {
    ComponentGallery(provider: SampleEventProvider())
        .preferredColorScheme(.dark)
}
