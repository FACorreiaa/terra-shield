import ShieldCore
import SwiftUI

/// Who said so (DECISIONS #11). `compact` is the name for rows; `full` adds attribution,
/// licence and the link to the official record for the detail view and Sources screen.
struct SourceBadge: View {
    enum Style { case compact, full }

    var source: Source
    /// The event's own page at the source, if any; falls back to the source's home page.
    var url: URL? = nil
    var style: Style = .compact
    /// When Shield last pulled this record (`full` only): "Fetched 2 min ago · 14:02 UTC".
    var fetchedAt: Date? = nil

    var body: some View {
        switch style {
        case .compact:
            Label(source.name, systemImage: "building.columns")
                .labelStyle(.titleAndIcon)
                .font(ShieldFont.meta)
                .foregroundStyle(Color.muted)
                .lineLimit(1)
                .accessibilityLabel("Source: \(source.name)")
        case .full:
            VStack(alignment: .leading, spacing: ShieldSpace.s1) {
                Text("SOURCE")
                    .font(ShieldFont.micro)
                    .tracking(ShieldTracking.micro)
                    .foregroundStyle(Color.faint)
                Text(source.name)
                    .font(ShieldFont.headline)
                    .foregroundStyle(Color.ink)
                Text("\(source.attribution) · \(source.license)")
                    .font(ShieldFont.meta)
                    .foregroundStyle(Color.muted)
                    .fixedSize(horizontal: false, vertical: true)
                if let fetchedAt {
                    TimelineView(.periodic(from: .now, by: 30)) { context in
                        Text("Fetched \(Freshness.stamp(fetchedAt, now: context.date))")
                            .font(ShieldFont.meta)
                            .foregroundStyle(Color.muted)
                            .monospacedDigit()
                    }
                }
                OfficialSourceButton(url: url ?? source.url)
                    .padding(.top, ShieldSpace.s2)
            }
            .accessibilityElement(children: .contain)
        }
    }
}

/// The primary action of an event (DESIGN.md §10.3 item 6): accent fill, `on-accent` text, ⌘L.
struct OfficialSourceButton: View {
    let url: URL
    @Environment(\.openURL) private var openURL

    var body: some View {
        Button {
            openURL(url)
        } label: {
            Label("Open official source", systemImage: "arrow.up.right.square")
                .font(ShieldFont.body.weight(.semibold))
                .foregroundStyle(Color.onAccent)
                .padding(.horizontal, ShieldSpace.s4)
                .padding(.vertical, ShieldSpace.s2 + 2)
                .frame(minHeight: 44)
                .background(Color.accent, in: RoundedRectangle(cornerRadius: ShieldRadius.sm, style: .continuous))
                .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .keyboardShortcut("l", modifiers: .command)
        .accessibilityHint(url.host() ?? "")
    }
}

#Preview("Source", traits: .sizeThatFitsLayout) {
    let s = SampleData.sources()[0]
    VStack(alignment: .leading, spacing: ShieldSpace.s4) {
        SourceBadge(source: s)
        SourceBadge(source: s, style: .full)
    }
    .padding(ShieldSpace.s5)
    .background(Color.panel)
}
