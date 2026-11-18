import SwiftUI

/// Required on every number that is not live (DESIGN.md §15). Uses `accent`, not amber or red,
/// so it can never be read as a hazard level.
struct SampleDataBadge: View {
    var body: some View {
        Label("SAMPLE DATA", systemImage: "flask")
            .labelStyle(.titleAndIcon)
            .font(ShieldFont.micro)
            .tracking(ShieldTracking.micro)
            .foregroundStyle(Color.accent)
            .padding(.horizontal, ShieldSpace.s2)
            .padding(.vertical, 3)
            .background(Color.accent.opacity(0.12), in: Capsule())
            .overlay(Capsule().strokeBorder(Color.accent.opacity(0.35), lineWidth: 1))
            .accessibilityLabel("Sample data. Not live.")
    }
}

#Preview(traits: .sizeThatFitsLayout) {
    SampleDataBadge()
        .padding(ShieldSpace.s5)
        .background(Color.canvas)
}
