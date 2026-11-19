import ShieldCore
import SwiftUI

/// One source on the Sources screen (DESIGN.md §10.7): name, what it covers, licence and
/// attribution, how often we check it, and its health as a dot plus words.
struct SourceRow: View {
    let source: Source
    var now: Date = .now

    var body: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s2) {
            HStack(alignment: .firstTextBaseline, spacing: ShieldSpace.s2) {
                HealthDot(health: source.health(now: now))
                Text(source.name)
                    .font(ShieldFont.headline)
                    .foregroundStyle(Color.ink)
                    .fixedSize(horizontal: false, vertical: true)
            }
            if let coverage = source.coverage, !coverage.isEmpty {
                Text(coverage)
                    .font(ShieldFont.body)
                    .foregroundStyle(Color.ink)
                    .fixedSize(horizontal: false, vertical: true)
            }
            Text(source.statusLine(now: now))
                .font(ShieldFont.meta)
                .foregroundStyle(statusColor)
                .fixedSize(horizontal: false, vertical: true)
            Text(source.interval)
                .font(ShieldFont.meta)
                .foregroundStyle(Color.muted)
            VStack(alignment: .leading, spacing: ShieldSpace.s1) {
                Text("Licence: \(source.license)")
                Text("Attribution: \(source.attribution)")
            }
            .font(ShieldFont.meta)
            .foregroundStyle(Color.muted)
            .fixedSize(horizontal: false, vertical: true)
            HStack(spacing: ShieldSpace.s4) {
                Link(destination: source.termsUrl ?? source.url) {
                    Label(source.termsUrl == nil ? "Website" : "Terms", systemImage: "arrow.up.right.square")
                }
                if source.termsUrl != nil {
                    Link(destination: source.url) {
                        Label("Website", systemImage: "arrow.up.right.square")
                    }
                }
            }
            .font(ShieldFont.meta.weight(.semibold))
            .foregroundStyle(Color.accent)
        }
        .padding(.vertical, ShieldSpace.s3)
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private var statusColor: Color {
        switch source.health(now: now) {
        case .ok: Color.muted
        case .failing: Color.warning
        case .stale: Color.danger
        }
    }
}

/// `ok` / `warning` / `danger` (DESIGN.md §10.7); the words beside it say the same, so colour is
/// never the only signal (§14).
struct HealthDot: View {
    let health: SourceHealth

    var body: some View {
        Circle()
            .fill(color)
            .frame(width: 8, height: 8)
            .accessibilityLabel(label)
    }

    private var color: Color {
        switch health {
        case .ok: Color.ok
        case .failing: Color.warning
        case .stale: Color.danger
        }
    }

    private var label: String {
        switch health {
        case .ok: "Healthy"
        case .failing: "Last check failed"
        case .stale: "Stale"
        }
    }
}
