import SwiftUI

/// An empty list with the Sentinel beside its words (DESIGN.md §11): no events in view, a country
/// with no active events, later no saved regions and no alerts. Never on the map canvas, at
/// least 120 pt tall, once per screen. The words say what isn't covered, never "no threats".
struct EmptyState<Words: View, Actions: View>: View {
    @ViewBuilder var words: Words
    @ViewBuilder var actions: Actions

    @Environment(\.dynamicTypeSize) private var typeSize

    var body: some View {
        // Side by side while the words have room; stacked at accessibility sizes.
        let layout = typeSize.isAccessibilitySize
            ? AnyLayout(VStackLayout(alignment: .leading, spacing: ShieldSpace.s3))
            : AnyLayout(HStackLayout(alignment: .center, spacing: ShieldSpace.s4))
        layout {
            SentinelArt(height: 128)
            VStack(alignment: .leading, spacing: ShieldSpace.s2) {
                words
                    .fixedSize(horizontal: false, vertical: true)
                // In a row while they fit, else one under another.
                ViewThatFits(in: .horizontal) {
                    HStack(spacing: ShieldSpace.s5) { actions }
                    VStack(alignment: .leading, spacing: ShieldSpace.s2) { actions }
                }
                    .font(ShieldFont.body.weight(.semibold))
                    .buttonStyle(.plain)
                    .foregroundStyle(Color.accent)
                    .padding(.top, ShieldSpace.s1)
            }
            .frame(maxWidth: .infinity, alignment: .leading)
        }
    }
}

extension EmptyState where Actions == EmptyView {
    init(@ViewBuilder words: () -> Words) {
        self.words = words()
        self.actions = EmptyView()
    }
}

/// The Sentinel mascot, Version B (the canonical pose), as-is.
struct SentinelArt: View {
    var height: CGFloat

    var body: some View {
        let h = max(height, 120)
        // The square art has wide transparent margins. It lays out narrower than it draws, so
        // the margins overlap the gap instead of taking the words' room; nothing is cropped.
        Image("Sentinel")
            .resizable()
            .interpolation(.high)
            .scaledToFit()
            .frame(width: h, height: h)
            .frame(width: h * 0.72, height: h)
            .accessibilityHidden(true)
    }
}
