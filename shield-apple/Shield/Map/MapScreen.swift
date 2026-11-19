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
    #if os(iOS)
    @Environment(\.horizontalSizeClass) private var sizeClass
    @State private var peekHeight = MapScreen.defaultPeekHeight
    @State private var detent = PresentationDetent.height(MapScreen.defaultPeekHeight)
    @State private var sheetHeight: CGFloat = 0
    #endif

    var body: some View {
        shell
            .background(Color.canvas)
            .task { await model.keepFresh() }
            #if os(macOS)
            .onKeyPress(.escape) {
                guard model.selectedEventID != nil else { return .ignored }
                model.selectedEventID = nil
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
        NavigationSplitView {
            VStack(spacing: 0) {
                StatStrip(cells: StatStrip.Cell.liveMap(model.snapshot),
                          isSample: model.provider.isSampleData)
                    .padding(.horizontal, ShieldSpace.s4)
                    .padding(.vertical, ShieldSpace.s3)
                Rectangle()
                    .fill(Color.ink.opacity(ShieldAlpha.hairline))
                    .frame(height: 1)
                EventList(model: model, zoomOut: fitWorld)
            }
            .navigationSplitViewColumnWidth(min: 220, ideal: 260, max: 300)
        } detail: {
            mapLayer(showsSelectedCard: true)
                .toolbar(removing: .title)
        }
    }

    #if os(iOS)
    /// The stat strip's height before it has been measured.
    private static let defaultPeekHeight: CGFloat = 116

    private var sheetShell: some View {
        // The map's attribution sits above the sheet wherever the map is still usable: at peek
        // and at medium. The notices sit above the peek.
        mapLayer(showsSelectedCard: false,
                 bottomCover: detent == .medium ? max(sheetHeight, peekHeight) : peekHeight)
            .safeAreaInset(edge: .bottom, spacing: 0) { Color.clear.frame(height: peekHeight) }
            .sheet(isPresented: .constant(true)) {
                MapSheet(model: model, zoomOut: fitWorld, peekHeightChanged: setPeekHeight,
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
            // A marker tapped while the sheet peeks: open far enough to show the event and
            // still leave the map in view.
            .onChange(of: model.selectedEventID, initial: true) { _, selected in
                guard selected != nil, detent == .height(peekHeight) else { return }
                withAnimation(ShieldMotion.spring) { detent = .medium }
            }
            #if DEBUG
            // Screenshot automation: `SHIELD_SHEET=peek|medium|large`.
            .onAppear {
                switch ProcessInfo.processInfo.environment["SHIELD_SHEET"] {
                case "medium": detent = .medium
                case "large": detent = .large
                default: break
                }
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

    // MARK: Map

    private func mapLayer(showsSelectedCard: Bool, bottomCover: CGFloat = 0) -> some View {
        ZStack {
            HazardMapView(model: model, fitWorldRequest: fitWorldRequest, bottomCover: bottomCover)
                .ignoresSafeArea()

            VStack(spacing: ShieldSpace.s2) {
                HStack(alignment: .top) {
                    StatusCapsule(model: model)
                    Spacer(minLength: ShieldSpace.s2)
                    MapControls(
                        isRefreshing: model.phase == .loading,
                        showsDetections: $model.showsDetections,
                        fitWorld: fitWorld,
                        refresh: { model.reload() }
                    )
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
                Spacer()
                bottom(showsSelectedCard: showsSelectedCard)
                    // Clear of the map's attribution ("Maps · Legal"), which must stay
                    // visible (DECISIONS #11).
                    .padding(.bottom, ShieldSpace.s6)
            }
            .padding(ShieldSpace.s3)
            .animation(ShieldMotion.standard, value: model.phase)
            .animation(ShieldMotion.spring, value: model.selectedEventID)
            .animation(ShieldMotion.standard, value: model.showsDetections)
        }
    }

    /// The selected event where there is no sheet to hold it, else the empty and truncated
    /// notices, which the sheet's list repeats for when it covers them.
    @ViewBuilder
    private func bottom(showsSelectedCard: Bool) -> some View {
        if showsSelectedCard, let event = model.selectedEvent {
            SelectedEventCard(
                event: event,
                source: model.sources[event.source],
                isStale: model.isStale(event),
                close: { model.selectedEventID = nil }
            )
            .transition(.move(edge: .bottom).combined(with: .opacity))
        } else if let snapshot = model.snapshot, snapshot.totalCount == 0 {
            EmptyViewNotice(zoomOut: fitWorld)
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
                    Circle()
                        .fill(dotColor)
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
        }
    }

    private func status(now: Date) -> String {
        switch model.phase {
        case .loading where model.lastSuccess == nil:
            return "Loading events…"
        case .failed(.offline):
            return model.lastSuccess.map { "Offline · showing data from \(Freshness.utcClock($0))" } ?? "Offline"
        default:
            guard let last = model.lastSuccess else { return "No data yet" }
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
    var fitWorld: () -> Void
    var refresh: () -> Void

    var body: some View {
        VStack(spacing: 0) {
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
    var zoomOut: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s2) {
            Text("No active events in this view from covered sources.")
                .font(ShieldFont.body)
                .foregroundStyle(Color.ink)
            Text("Absence of data is not safety. Official warnings come from national services.")
                .font(ShieldFont.meta)
                .foregroundStyle(Color.muted)
            Button("Zoom out", action: zoomOut)
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

// MARK: - Selected event

/// The tapped event, compact: title, both severities, kind, source and freshness. Full detail
/// is S3.5. Hosted by the iOS sheet and by `SelectedEventCard`.
struct SelectedEventSummary: View {
    let event: HazardEvent
    let source: Source?
    let isStale: Bool
    /// The source's own summary text, where there is room for it.
    var showsSummary = false
    var close: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s2) {
            HStack(alignment: .firstTextBaseline) {
                Text(event.title)
                    .font(ShieldFont.headline)
                    .foregroundStyle(Color.ink)
                    .lineLimit(2)
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
            SeverityBadge(severity: event.severity, sourceSeverity: event.sourceSeverity,
                          isProvisional: event.severityProvisional)
            HStack(spacing: ShieldSpace.s2) {
                KindChip(kind: event.kind)
                if let source { SourceBadge(source: source, url: event.url) }
            }
            if let source {
                FreshnessLabel(updatedAt: event.updatedAt, fetchedAt: event.fetchedAt,
                               pollInterval: source.pollInterval, observedAt: event.observedAt)
            }
            if isStale, source == nil {
                // FreshnessLabel says it when the source is known; say it here otherwise.
                Text("Stale · this source hasn’t updated recently")
                    .font(ShieldFont.meta)
                    .foregroundStyle(Color.warning)
            }
            if showsSummary, let summary = event.summary, !summary.isEmpty {
                Text(summary)
                    .font(ShieldFont.body)
                    .foregroundStyle(Color.ink)
                    .fixedSize(horizontal: false, vertical: true)
                    .padding(.top, ShieldSpace.s1)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

/// `SelectedEventSummary` as a card on the map, for the shells without a sheet.
private struct SelectedEventCard: View {
    let event: HazardEvent
    let source: Source?
    let isStale: Bool
    var close: () -> Void

    var body: some View {
        SelectedEventSummary(event: event, source: source, isStale: isStale, close: close)
            .padding(ShieldSpace.s4)
            .frame(maxWidth: 560, alignment: .leading)
            .background(Color.panel, in: RoundedRectangle(cornerRadius: ShieldRadius.lg))
            .overlay(RoundedRectangle(cornerRadius: ShieldRadius.lg).strokeBorder(Color.ink.opacity(ShieldAlpha.hairline)))
            .shadow(color: .black.opacity(0.4), radius: 12, y: 4)
    }
}
