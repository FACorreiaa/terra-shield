import Testing
@testable import ShieldCore

/// DESIGN.md §10.8: first run only, and never in the way of screenshot automation.
@Suite struct OnboardingTests {
    @Test func showsOnFirstRunOnly() {
        #expect(Onboarding.shouldShow(completed: false, environment: [:]))
        #expect(!Onboarding.shouldShow(completed: true, environment: [:]))
    }

    @Test func automationLaunchesSkipIt() {
        // Every SHIELD_* hook expects the map, not the cards.
        #expect(!Onboarding.shouldShow(completed: false, environment: ["SHIELD_SELECT": "x"]))
        #expect(!Onboarding.shouldShow(completed: false, environment: ["SHIELD_RESET_FILTERS": "1"]))
        // Other variables don't count.
        #expect(Onboarding.shouldShow(completed: false, environment: ["HOME": "/"]))
    }

    @Test func automationCanAskForIt() {
        #expect(Onboarding.shouldShow(completed: true, environment: ["SHIELD_ONBOARDING": "1"]))
        #expect(Onboarding.firstCard(environment: ["SHIELD_ONBOARDING": "3"]) == 2)
        #expect(Onboarding.firstCard(environment: ["SHIELD_ONBOARDING": "1"]) == 0)
        #expect(Onboarding.firstCard(environment: ["SHIELD_ONBOARDING": "9"]) == 0)
        #expect(Onboarding.firstCard(environment: [:]) == 0)
    }
}
