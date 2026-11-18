import ShieldAPI
import ShieldCore
import SwiftUI

@main
struct ShieldApp: App {
    /// The API when the build has one (Debug: localhost, BACKLOG "Local-first"), else sample
    /// data, which `MapScreen` then badges (DESIGN.md §15).
    @State private var model: LiveMapModel = {
        let model = LiveMapModel(provider: ShieldApp.makeProvider())
        #if DEBUG
        // Screenshot automation: start with heat detections on (`SHIELD_DETECTIONS=1`) …
        let env = ProcessInfo.processInfo.environment
        if env["SHIELD_DETECTIONS"] == "1" { model.showsDetections = true }
        // … and with an event selected (`SHIELD_SELECT=<event id>`).
        if let id = env["SHIELD_SELECT"].flatMap(UUID.init(uuidString:)) { model.selectedEventID = id }
        #endif
        return model
    }()

    var body: some Scene {
        WindowGroup {
            MapScreen(model: model)
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
        return APIEventProvider(client: .shield(baseURL: url))
    }
}
