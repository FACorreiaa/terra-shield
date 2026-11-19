import ShieldAPI
import ShieldCore
import SwiftUI

@main
struct ShieldApp: App {
    /// The API when the build has one (Debug: localhost, BACKLOG "Local-first"), else sample
    /// data, which `MapScreen` then badges (DESIGN.md §15).
    @State private var model: LiveMapModel = {
        let filters = FilterStore()
        #if DEBUG
        // Screenshot runs start clean (`SHIELD_RESET_FILTERS=1`): saved filters would leak
        // from one launch into the next.
        if ProcessInfo.processInfo.environment["SHIELD_RESET_FILTERS"] == "1" { filters.clear() }
        #endif
        // The person's filters, kept across launches (S3.3).
        let model = LiveMapModel(provider: ShieldApp.makeProvider(), filters: filters)
        #if DEBUG
        // Screenshot automation: start with heat detections on (`SHIELD_DETECTIONS=1`) …
        let env = ProcessInfo.processInfo.environment
        if env["SHIELD_DETECTIONS"] == "1" { model.showsDetections = true }
        // … and with an event selected (`SHIELD_SELECT=<event id>`).
        if let id = env["SHIELD_SELECT"].flatMap(UUID.init(uuidString:)) { model.selectedEventID = id }
        // … and with a set of filters on (`SHIELD_FILTER_DEMO=1`): moderate and up, warnings
        // and forecasts, earthquakes and floods, last 7 days.
        if env["SHIELD_FILTER_DEMO"] == "1" {
            var q = EventQuery()
            q.minimumSeverity = .moderate
            q.kinds = [.warning, .forecast]
            q.types = [.earthquake, .flood]
            q.window = 7 * 86400
            model.query = q
        }
        // … and with resolved events included (`SHIELD_RESOLVED=1`).
        if env["SHIELD_RESOLVED"] == "1" { model.query.includeResolved = true }
        #endif
        return model
    }()

    var body: some Scene {
        WindowGroup {
            Group {
                #if DEBUG
                // Screenshot automation: `SHIELD_GALLERY=1` (or `=detail`) opens the component gallery instead.
                if ProcessInfo.processInfo.environment["SHIELD_GALLERY"] != nil {
                    ComponentGallery(provider: SampleEventProvider())
                } else {
                    MapScreen(model: model)
                }
                #else
                MapScreen(model: model)
                #endif
            }
            .preferredColorScheme(.dark)
            .tint(.accent)
        }
        #if os(macOS)
        .defaultSize(width: 1200, height: 800)
        #endif

        #if os(macOS) && DEBUG
        Window("Component Gallery", id: "gallery") {
            ComponentGallery(provider: SampleEventProvider())
                .preferredColorScheme(.dark)
                .tint(.accent)
        }
        .defaultSize(width: 1100, height: 760)
        #endif
    }

    private static func makeProvider() -> any HazardEventProvider {
        guard let url = AppConfig.apiBaseURL else { return SampleEventProvider() }
        // Polls /v1/events and follows /v1/stream for live changes (S3.6).
        return APIEventProvider(client: .shield(baseURL: url), stream: LiveStreamConfig(baseURL: url))
    }
}
