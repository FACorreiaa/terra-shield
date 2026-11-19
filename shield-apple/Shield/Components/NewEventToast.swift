import ShieldCore
import SwiftUI

/// One line at the top of the map for a severe or extreme live arrival in view (DESIGN.md §10.1):
/// "New · Severe earthquake · M6.1 · Chile". Tapping it opens the event; no sound, no vibration.
struct NewEventToast: View {
    let toast: EventToast
    var open: () -> Void
    var dismiss: () -> Void

    var body: some View {
        HStack(spacing: ShieldSpace.s2) {
            HStack(spacing: ShieldSpace.s2) {
                HazardMarker(type: toast.event.type, severity: toast.event.severity, kind: toast.event.kind,
                             baseSize: 20, pulses: false)
                Text(toast.text)
                    .font(ShieldFont.body.weight(.semibold))
                    .foregroundStyle(Color.ink)
                    .lineLimit(1)
                    .truncationMode(.tail)
            }
            .contentShape(Rectangle())
            .onTapGesture(perform: open)
            .accessibilityElement(children: .combine)
            .accessibilityLabel(toast.text)
            .accessibilityAddTraits(.isButton)
            .accessibilityHint("Opens the event")
            .accessibilityAction { open() }

            Button(action: dismiss) {
                Image(systemName: "xmark")
                    .font(ShieldFont.meta.weight(.semibold))
                    .foregroundStyle(Color.muted)
                    .frame(width: 28, height: 28)
                    .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityLabel("Dismiss")
        }
        .padding(.leading, ShieldSpace.s3)
        .padding(.trailing, ShieldSpace.s1)
        .padding(.vertical, ShieldSpace.s1)
        .frame(minHeight: 44)
        .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: ShieldRadius.md))
        .overlay(RoundedRectangle(cornerRadius: ShieldRadius.md)
            .strokeBorder(toast.event.severity.color.opacity(0.6)))
        .frame(maxWidth: 560)
    }
}
