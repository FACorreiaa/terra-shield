import Foundation

/// Build-time configuration from `Config/*.xcconfig`, read through Info.plist.
enum AppConfig {
    /// The Shield API. `nil` when the build has none (Beta/Release until an API host exists),
    /// in which case the app runs on sample data.
    static var apiBaseURL: URL? {
        guard let raw = Bundle.main.object(forInfoDictionaryKey: "ShieldAPIBaseURL") as? String,
              !raw.isEmpty else { return nil }
        return URL(string: raw)
    }
}
