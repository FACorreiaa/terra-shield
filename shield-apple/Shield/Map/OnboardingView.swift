import ShieldCore
import SwiftUI

/// First run, three cards, skippable (DESIGN.md §10.8). No account and no permission prompt:
/// location is asked only when the person taps Locate.
struct OnboardingView: View {
    var finish: () -> Void

    @State private var index: Int
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @Environment(\.dynamicTypeSize) private var typeSize

    init(startAt: Int = 0, finish: @escaping () -> Void) {
        self.finish = finish
        _index = State(initialValue: startAt)
    }

    private var isLast: Bool { index == Onboarding.cardCount - 1 }

    var body: some View {
        VStack(spacing: 0) {
            HStack {
                Spacer()
                // Skip on every card but the last, whose button does the same.
                Button("Skip", action: finish)
                    .font(ShieldFont.body.weight(.semibold))
                    .buttonStyle(.plain)
                    .foregroundStyle(Color.muted)
                    .frame(minWidth: 44, minHeight: 44)
                    .opacity(isLast ? 0 : 1)
                    .disabled(isLast)
                    .accessibilityHidden(isLast)
            }
            .padding(.horizontal, ShieldSpace.s4)

            cards
                .frame(maxWidth: .infinity, maxHeight: .infinity)

            VStack(spacing: ShieldSpace.s4) {
                PageDots(count: Onboarding.cardCount, index: index)
                Button(action: next) {
                    // Dark words on `accent`: white on it falls short of 4.5:1 (DESIGN.md §14).
                    Text(isLast ? "Show me the map" : "Next")
                        .font(ShieldFont.headline)
                        .foregroundStyle(Color.canvas)
                        .frame(maxWidth: 360)
                        .frame(minHeight: 50)
                        .background(Color.accent, in: RoundedRectangle(cornerRadius: ShieldRadius.md, style: .continuous))
                        .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .keyboardShortcut(.defaultAction)
            }
            .padding(.horizontal, ShieldSpace.s5)
            .padding(.bottom, ShieldSpace.s5)
        }
        .background { OnboardingBackground(showsEarth: index == 0) }
        .preferredColorScheme(.dark)
    }

    @ViewBuilder
    private var cards: some View {
        #if os(iOS)
        TabView(selection: $index) {
            ForEach(0..<Onboarding.cardCount, id: \.self) { card($0).tag($0) }
        }
        .tabViewStyle(.page(indexDisplayMode: .never))
        #else
        card(index)
            .id(index)
            .transition(.opacity)
        #endif
    }

    /// Centred while it fits; at large text sizes it scrolls from the top instead.
    private func card(_ n: Int) -> some View {
        ViewThatFits(in: .vertical) {
            cardContent(n)
                .frame(maxHeight: .infinity)
            ScrollView { cardContent(n) }
        }
    }

    @ViewBuilder
    private func cardContent(_ n: Int) -> some View {
        Group {
            switch n {
            case 0: welcome
            case 1: kinds
            default: disclaimer
            }
        }
        .padding(.horizontal, ShieldSpace.s5)
        .frame(maxWidth: 520)
        .frame(maxWidth: .infinity)
    }

    private func next() {
        guard !isLast else { return finish() }
        withAnimation(reduceMotion ? nil : ShieldMotion.standard) { index += 1 }
    }

    // MARK: Cards

    private var welcome: some View {
        VStack(spacing: ShieldSpace.s5) {
            Image("BrandLockup")
                .resizable()
                .interpolation(.high)
                .scaledToFit()
                .frame(maxWidth: 260)
                .accessibilityLabel("Aegis Terra")
            Text("Global hazard intelligence. Official sources, live.")
                .font(ShieldFont.title)
                .tracking(ShieldTracking.title)
                .foregroundStyle(Color.ink)
                .multilineTextAlignment(.center)
                .fixedSize(horizontal: false, vertical: true)
        }
    }

    private var kinds: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s5) {
            // Beside the chips while they fit; above them at accessibility sizes.
            let layout = typeSize.isAccessibilitySize
                ? AnyLayout(VStackLayout(alignment: .leading, spacing: ShieldSpace.s4))
                : AnyLayout(HStackLayout(alignment: .center, spacing: ShieldSpace.s5))
            layout {
                SentinelArt(height: 180)
                VStack(alignment: .leading, spacing: ShieldSpace.s2) {
                    ForEach(EventKind.allCases, id: \.self) { KindChip(kind: $0) }
                }
            }
            .frame(maxWidth: .infinity, alignment: typeSize.isAccessibilitySize ? .leading : .center)
            Text("Know what you're looking at: observed, official warning, forecast, satellite detection.")
                .font(ShieldFont.title)
                .tracking(ShieldTracking.title)
                .foregroundStyle(Color.ink)
                .fixedSize(horizontal: false, vertical: true)
        }
    }

    private var disclaimer: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s4) {
            Text("Before you start")
                .font(ShieldFont.title)
                .tracking(ShieldTracking.title)
                .foregroundStyle(Color.ink)
            DisclaimerBanner(style: .inline)
        }
    }
}

/// Orbital Earth behind the first card, under a radial vignette to `canvas`, kept low contrast
/// (DESIGN.md §12); plain `canvas` after it.
private struct OnboardingBackground: View {
    var showsEarth: Bool

    var body: some View {
        ZStack {
            Color.canvas
            if showsEarth {
                Image("BackgroundOrbitalEarth")
                    .resizable()
                    .scaledToFill()
                    .opacity(0.45)
                    .overlay {
                        RadialGradient(colors: [.clear, Color.canvas], center: .center,
                                       startRadius: 60, endRadius: 520)
                    }
                    .transition(.opacity)
                    .accessibilityHidden(true)
            }
        }
        .ignoresSafeArea()
        .animation(ShieldMotion.standard, value: showsEarth)
    }
}

private struct PageDots: View {
    var count: Int
    var index: Int

    var body: some View {
        HStack(spacing: ShieldSpace.s2) {
            ForEach(0..<count, id: \.self) { i in
                Circle()
                    .fill(i == index ? Color.ink : Color.ink.opacity(0.3))
                    .frame(width: 7, height: 7)
            }
        }
        .accessibilityElement()
        .accessibilityLabel("Card \(index + 1) of \(count)")
    }
}
