import ShieldCore
import SwiftUI

/// One event in a list (VIEWS.md catalog; DESIGN.md §10.1): the type glyph in its severity
/// shape, the title, then kind, source and freshness. A stale row fades like its marker and
/// says so in words.
struct EventRow: View {
    let event: HazardEvent
    /// From `/v1/sources`; `nil` until they load, and the row then shows the source id.
    var source: Source?
    var isStale = false
    var isSelected = false
    /// Where it is, when the list around it doesn't say: "Near Chile, 38 km offshore" in Chile's
    /// overview (ADR 4).
    var place: String? = nil

    var body: some View {
        HStack(alignment: .top, spacing: ShieldSpace.s3) {
            HazardMarker(type: event.type, severity: event.severity, kind: event.kind,
                         isStale: isStale, baseSize: 22, pulses: false)
                .frame(width: 32, height: 32)
            VStack(alignment: .leading, spacing: ShieldSpace.s1) {
                Text(event.title)
                    .font(ShieldFont.headline)
                    .foregroundStyle(isStale ? Color.muted : Color.ink)
                    .lineLimit(2)
                    .fixedSize(horizontal: false, vertical: true)
                if let place {
                    Label(place, systemImage: "location")
                        .font(ShieldFont.meta)
                        .foregroundStyle(Color.muted)
                        .labelStyle(.titleAndIcon)
                }
                ViewThatFits(in: .horizontal) {
                    HStack(spacing: ShieldSpace.s2) { kindAndSource }
                    VStack(alignment: .leading, spacing: ShieldSpace.s1) { kindAndSource }
                }
                FreshnessLabel(updatedAt: event.updatedAt, fetchedAt: lastFetch,
                               pollInterval: pollInterval, observedAt: event.observedAt)
            }
            Spacer(minLength: 0)
        }
        .padding(.vertical, ShieldSpace.s1)
        .contentShape(Rectangle())
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(accessibilityText)
        .accessibilityAddTraits(isSelected ? [.isButton, .isSelected] : .isButton)
    }

    @ViewBuilder private var kindAndSource: some View {
        KindChip(kind: event.kind)
        if let source {
            SourceBadge(source: source)
        } else {
            Text(event.source.uppercased())
                .font(ShieldFont.meta)
                .foregroundStyle(Color.muted)
                .lineLimit(1)
        }
    }

    /// The same clock the marker's stale rule reads: the source's last successful poll.
    private var lastFetch: Date { source?.lastSuccessAt ?? event.fetchedAt }
    private var pollInterval: TimeInterval { source?.pollInterval ?? 600 }

    /// Like the marker's label, plus what a list can add (DESIGN.md §14): "M6.1 earthquake off
    /// Coquimbo. Severe earthquake, observed, updated 12 min ago, source USGS."
    private var accessibilityText: String {
        var parts = [
            "\(event.severity.label) \(event.type.label.lowercased())",
            event.kind.label.lowercased(),
            isStale ? "stale, last update \(Freshness.relative(lastFetch))"
                    : "updated \(Freshness.relative(event.updatedAt))",
        ]
        parts.append("source \(source?.name ?? event.source)")
        if let place { parts.insert(place, at: 0) }
        return "\(event.title). " + parts.joined(separator: ", ")
    }
}

#Preview("EventRow", traits: .sizeThatFitsLayout) {
    let events = SampleData.events()
    let sources = Dictionary(uniqueKeysWithValues: SampleData.sources().map { ($0.id, $0) })
    return VStack(alignment: .leading, spacing: ShieldSpace.s2) {
        SampleDataBadge()
        ForEach(events.prefix(3)) { event in
            EventRow(event: event, source: sources[event.source])
        }
        if let event = events.last {
            EventRow(event: event, source: nil, isStale: true)
        }
    }
    .padding(ShieldSpace.s5)
    .frame(width: 360)
    .background(Color.panel)
}
