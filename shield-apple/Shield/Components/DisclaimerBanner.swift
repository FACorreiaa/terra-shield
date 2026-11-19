import SwiftUI

/// The permanent disclaimer (DESIGN.md §2, DECISIONS #12), the one place its words live.
/// `inline`: a quiet panel at the top of a screen (Sources, onboarding). `footer`: muted text
/// closing a view (every event's detail).
struct DisclaimerBanner: View {
    enum Style { case inline, footer }

    static let text = "Aegis Terra does not replace your national emergency service. Always follow official instructions."

    var style: Style = .inline
    @Environment(\.dynamicTypeSize) private var typeSize

    var body: some View {
        switch style {
        case .footer:
            Text(Self.text)
                .font(ShieldFont.meta)
                .foregroundStyle(Color.muted)
                .fixedSize(horizontal: false, vertical: true)
        case .inline:
            Label {
                Text(Self.text)
                    .font(ShieldFont.body)
                    .foregroundStyle(Color.ink)
                    .fixedSize(horizontal: false, vertical: true)
            } icon: {
                // At accessibility sizes the words need the whole width.
                if !typeSize.isAccessibilitySize {
                    Image(systemName: "info.circle")
                        .foregroundStyle(Color.muted)
                }
            }
            .padding(ShieldSpace.s4)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(Color.raised, in: RoundedRectangle(cornerRadius: ShieldRadius.md, style: .continuous))
            .overlay(RoundedRectangle(cornerRadius: ShieldRadius.md, style: .continuous)
                .strokeBorder(Color.ink.opacity(ShieldAlpha.hairline)))
            .accessibilityElement(children: .combine)
        }
    }
}
