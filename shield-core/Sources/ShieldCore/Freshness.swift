import Foundation

/// How old a fact is, in the app's fixed wording: "12 min ago · 14:02 UTC" (DESIGN.md §2, §5).
public enum Freshness {
    /// Stale when the last update is older than three poll intervals (DECISIONS #8).
    public static func isStale(lastUpdate: Date, pollInterval: TimeInterval, now: Date = .now) -> Bool {
        now.timeIntervalSince(lastUpdate) > 3 * pollInterval
    }

    /// "now", "45 s ago", "12 min ago", "3 h ago", "2 d ago". Future dates read "in 12 min".
    public static func relative(_ date: Date, now: Date = .now) -> String {
        let delta = now.timeIntervalSince(date)
        let future = delta < 0
        let s = abs(delta)
        let text: String
        if s < 10 { return "now" }
        else if s < 60 { text = "\(Int(s)) s" }
        else if s < 3600 { text = "\(Int(s / 60)) min" }
        else if s < 86_400 { text = "\(Int(s / 3600)) h" }
        else { text = "\(Int(s / 86_400)) d" }
        return future ? "in \(text)" : "\(text) ago"
    }

    /// "14:02 UTC".
    public static func utcClock(_ date: Date) -> String {
        var cal = Calendar(identifier: .gregorian)
        cal.timeZone = TimeZone(identifier: "UTC")!
        let c = cal.dateComponents([.hour, .minute], from: date)
        return String(format: "%02d:%02d UTC", c.hour ?? 0, c.minute ?? 0)
    }

    /// "12 min ago · 14:02 UTC".
    public static func label(_ date: Date, now: Date = .now) -> String {
        "\(relative(date, now: now)) · \(utcClock(date))"
    }
}
