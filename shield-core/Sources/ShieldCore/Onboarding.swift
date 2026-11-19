/// When the first-run cards show (DESIGN.md §10.8). Release builds pass an empty environment.
public enum Onboarding {
    /// The `UserDefaults` key set once the person finishes or skips the cards.
    public static let completedKey = "onboarding.completed"
    public static let cardCount = 3

    /// First run only. An automation launch (any `SHIELD_*` variable) expects the map, so it skips
    /// the cards unless `SHIELD_ONBOARDING` asks for them.
    public static func shouldShow(completed: Bool, environment: [String: String]) -> Bool {
        if environment["SHIELD_ONBOARDING"] != nil { return true }
        if environment.keys.contains(where: { $0.hasPrefix("SHIELD_") }) { return false }
        return !completed
    }

    /// `SHIELD_ONBOARDING=2` opens on the second card; anything else on the first.
    public static func firstCard(environment: [String: String]) -> Int {
        guard let n = environment["SHIELD_ONBOARDING"].flatMap(Int.init), (1...cardCount).contains(n) else { return 0 }
        return n - 1
    }
}
