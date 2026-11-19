import ShieldCore
import SwiftUI

/// How old the fact is: "Updated 12 min ago · 14:02 UTC" (DESIGN.md §2). Stale, meaning older
/// than three poll intervals (DECISIONS #8), switches to `warning` with a clock and the words
/// "Stale · last update 3 h ago", so it never relies on colour alone. Refreshes itself.
struct FreshnessLabel: View {
    var updatedAt: Date
    /// When Shield last pulled this record from the source. Drives the stale rule.
    var fetchedAt: Date
    var pollInterval: TimeInterval
    var observedAt: Date? = nil

    var body: some View {
        TimelineView(.periodic(from: .now, by: 30)) { context in
            let now = context.date
            let stale = Freshness.isStale(lastUpdate: fetchedAt, pollInterval: pollInterval, now: now)
            Label {
                Text(text(now: now, stale: stale))
                    .monospacedDigit()
            } icon: {
                if stale { Image(systemName: "clock.badge.exclamationmark") }
            }
            .labelStyle(.titleAndIcon)
            .font(ShieldFont.meta)
            .foregroundStyle(stale ? Color.warning : Color.muted)
            .lineLimit(1)
        }
    }

    private func text(now: Date, stale: Bool) -> String {
        if stale {
            return "Stale · last update \(Freshness.relative(fetchedAt, now: now))"
        }
        return "Updated \(Freshness.label(updatedAt, now: now))"
    }
}

/// A resolved event's status line, in place of its freshness: "Resolved · ended 3 h ago", in `ok`
/// with a checkmark (DESIGN.md §5). Lists and cards only; `ok` never goes on the map.
struct ResolvedLabel: View {
    let event: HazardEvent

    var body: some View {
        TimelineView(.periodic(from: .now, by: 30)) { context in
            Label(event.resolvedWords(now: context.date) ?? "", systemImage: "checkmark.circle")
                .labelStyle(.titleAndIcon)
                .font(ShieldFont.meta)
                .foregroundStyle(Color.ok)
                .monospacedDigit()
                .lineLimit(1)
        }
    }
}

#Preview("Freshness", traits: .sizeThatFitsLayout) {
    VStack(alignment: .leading, spacing: ShieldSpace.s2) {
        FreshnessLabel(updatedAt: .now.addingTimeInterval(-720), fetchedAt: .now.addingTimeInterval(-60), pollInterval: 60)
        FreshnessLabel(updatedAt: .now.addingTimeInterval(-3 * 3600), fetchedAt: .now.addingTimeInterval(-3 * 3600), pollInterval: 900)
    }
    .padding(ShieldSpace.s5)
    .background(Color.panel)
}
