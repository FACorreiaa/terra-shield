import ShieldCore
import SwiftUI

/// Sources & attribution (DESIGN.md §10.7): the disclaimer, then every active source with its
/// health. Refreshes on open; a failed refresh keeps the last list and says so.
struct SourcesView: View {
    let model: LiveMapModel
    var done: () -> Void

    @State private var isLoading = false

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: ShieldSpace.s4) {
                DisclaimerBanner(style: .inline)
                content
            }
            .padding(ShieldSpace.s5)
        }
        .background(Color.panel)
        .navigationTitle("Sources")
        #if os(iOS)
        .navigationBarTitleDisplayMode(.inline)
        #endif
        .toolbar {
            ToolbarItem(placement: .confirmationAction) {
                Button("Done", action: done)
                    .keyboardShortcut(.cancelAction)
            }
        }
        .refreshable { await model.refreshSources() }
        .task {
            isLoading = true
            await model.refreshSources()
            isLoading = false
        }
    }

    @ViewBuilder
    private var content: some View {
        let sources = Source.listed(model.sourceList)
        if sources.isEmpty {
            if isLoading {
                HStack(spacing: ShieldSpace.s2) {
                    ProgressView()
                    Text("Loading sources…").font(ShieldFont.body).foregroundStyle(Color.muted)
                }
                .padding(.top, ShieldSpace.s4)
            } else if model.sourcesRefreshFailed {
                VStack(alignment: .leading, spacing: ShieldSpace.s3) {
                    Text("Couldn't load the sources.")
                        .font(ShieldFont.body)
                        .foregroundStyle(Color.ink)
                    Button("Try again") { Task { await model.refreshSources() } }
                        .font(ShieldFont.body.weight(.semibold))
                }
            }
        } else {
            if model.sourcesRefreshFailed {
                Label("Couldn't refresh. Showing the last list loaded.", systemImage: "exclamationmark.triangle")
                    .font(ShieldFont.meta)
                    .foregroundStyle(Color.warning)
            }
            Text("\(sources.count) sources")
                .font(ShieldFont.micro)
                .tracking(ShieldTracking.micro)
                .foregroundStyle(Color.muted)
                .textCase(.uppercase)
            // Rows age in place: the relative times move on without a reload.
            TimelineView(.periodic(from: .now, by: 30)) { context in
                VStack(alignment: .leading, spacing: 0) {
                    ForEach(Array(sources.enumerated()), id: \.element.id) { index, source in
                        if index > 0 {
                            Rectangle().fill(Color.ink.opacity(ShieldAlpha.hairline)).frame(height: 1)
                        }
                        SourceRow(source: source, now: context.date)
                    }
                }
            }
            Text("Absence of data is not safety.")
                .font(ShieldFont.meta)
                .foregroundStyle(Color.muted)
                .padding(.top, ShieldSpace.s2)
        }
    }
}
