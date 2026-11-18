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
                Link(destination: url ?? source.url) {
                    Label("Open official source", systemImage: "arrow.up.right.square")
                        .font(ShieldFont.body.weight(.semibold))
                }
                .padding(.top, ShieldSpace.s1)
            }
            .accessibilityElement(children: .contain)
        }
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
