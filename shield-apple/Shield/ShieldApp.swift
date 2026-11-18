import ShieldCore
import SwiftUI

@main
struct ShieldApp: App {
    /// Sample data until the API spec lands (BACKLOG S2.1). Swapping in the API-backed provider
    /// is this one line; `isSampleData` then turns the sample badges off.
    private let provider: any HazardEventProvider = SampleEventProvider()

    var body: some Scene {
        WindowGroup {
            ComponentGallery(provider: provider)
                .preferredColorScheme(.dark)
                .tint(.accent)
        }
        #if os(macOS)
        .defaultSize(width: 1100, height: 760)
        #endif
    }
}
