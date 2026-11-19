import Foundation
import Observation

/// One country's overview (DESIGN.md §10.2): the summary, its load state, and the matrix cell
/// that filters its list.
///
/// One instance per country opened, so an answer for a country already left behind can never
/// land in the one on screen. Reloads of the same country are ordered by a generation number.
@MainActor @Observable
public final class CountryOverview: Identifiable {
    public enum Phase: Equatable, Sendable {
        case loading
        case loaded
        /// The server has no boundary for this code (404): "We have no boundary for this place."
        case notFound
        /// The last load failed. A summary loaded before is kept and shown with its age.
        case failed(LoadFailure)
    }

    /// Two letters, upper case.
    public let iso2: String
    public private(set) var summary: CountrySummary?
    public private(set) var phase = Phase.loading
    /// The matrix cell the list is filtered to; nil for every event.
    public var cell: MatrixCell?

    public nonisolated var id: String { iso2 }

    @ObservationIgnored private let known: Country?
    @ObservationIgnored private let provider: any HazardEventProvider
    @ObservationIgnored private let now: @Sendable () -> Date
    @ObservationIgnored private var generation = 0

    /// Nil when `iso2` isn't two letters; `known` is what the picker had, for the name and the
    /// map while the summary loads.
    public init?(iso2: String, known: Country? = nil, provider: any HazardEventProvider,
                 now: @escaping @Sendable () -> Date = { .now }) {
        guard let code = Country.normalizedCode(iso2) else { return nil }
        self.iso2 = code
        self.known = known
        self.provider = provider
        self.now = now
    }

    public var country: Country? { summary?.country ?? known }
    public var name: String { country?.name ?? iso2 }
    public var isSampleData: Bool { provider.isSampleData }

    /// The listed events in the chosen cell, or all of them.
    public var listedEvents: [HazardEvent] { summary?.events(in: cell) ?? [] }

    public func load() async {
        generation += 1
        let mine = generation
        if summary == nil { phase = .loading }
        do {
            let result = try await provider.countrySummary(iso2: iso2)
            guard mine == generation else { return }
            guard let result else {
                summary = nil
                cell = nil
                phase = .notFound
                return
            }
            summary = result
            // A cell that emptied since it was chosen would leave an unexplained empty list.
            if let cell, result.count(in: cell) == 0 { self.cell = nil }
            phase = .loaded
        } catch {
            guard mine == generation, !(error is CancellationError), !Task.isCancelled else { return }
            phase = .failed(LoadFailure(error))
        }
    }

    /// Loads again when the summary is older than `maxAge`; the screen calls this when the map
    /// itself refreshed, so the two don't drift apart for long.
    public func refresh(ifOlderThan maxAge: TimeInterval) async {
        if case .loading = phase { return }
        if let summary, now().timeIntervalSince(summary.generatedAt) < maxAge { return }
        await load()
    }
}
