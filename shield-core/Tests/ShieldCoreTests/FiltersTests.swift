import Foundation
import Testing
@testable import ShieldCore

/// A store on a suite of its own: Swift Testing runs tests in parallel.
private func freshStore(_ name: String = #function) -> FilterStore {
    let suite = "shield.tests.\(name).\(UUID().uuidString)"
    return FilterStore(suiteName: suite)
}

@Suite struct ActiveFilterTests {
    @Test func theDefaultHasNoActiveFilters() {
        #expect(EventQuery().isDefault)
        #expect(EventQuery(country: "PT").isDefault) // a place, not a filter
    }

    @Test func labelsReadAsChips() {
        var q = EventQuery()
        q.types = [.earthquake]
        q.minimumSeverity = .severe
        q.kinds = [.warning]
        q.window = 7 * 86400
        #expect(q.activeFilters == [.types, .severity, .kinds, .window])
        #expect(q.activeFilters.map(q.label) == ["Earthquake", "Severe+", "Warnings only", "7 d"])

        q.types = [.flood, .storm]
        #expect(q.label(for: .types) == "Flood, Storm")
        q.types = [.flood, .storm, .wildfire]
        #expect(q.label(for: .types) == "3 types")
        q.kinds = [.observed, .forecast]
        #expect(q.label(for: .kinds) == "Observed + forecasts")
        q.window = 3600
        #expect(q.label(for: .window) == "1 h")
        q.window = nil
        #expect(q.label(for: .window) == "Any time")
    }

    @Test func removingResetsOneDimension() {
        var q = EventQuery()
        q.minimumSeverity = .extreme
        q.window = 3600
        let r = q.removing(.severity)
        #expect(r.minimumSeverity == .minor)
        #expect(r.window == 3600)
        #expect(r.removing(.window).isDefault)
    }
}

@Suite struct QuickFilterTests {
    @Test func aCellTapFiltersAndASecondTapUndoes() {
        let q = EventQuery()
        let high = q.toggling(.highSeverity)
        #expect(high.minimumSeverity == .severe && high.isOn(.highSeverity))
        #expect(high.toggling(.highSeverity).isDefault)

        let warnings = q.toggling(.warnings)
        #expect(warnings.kinds == [.warning] && warnings.isOn(.warnings))
        #expect(warnings.toggling(.warnings).isDefault)
        // Each cell touches only its own dimension.
        #expect(high.toggling(.warnings).minimumSeverity == .severe)
    }
}

@Suite struct FilterStoreTests {
    @Test func roundTrips() {
        let store = freshStore()
        var q = EventQuery()
        q.types = [.earthquake, .tsunami]
        q.minimumSeverity = .moderate
        q.kinds = [.warning, .forecast]
        q.window = 6 * 3600
        store.save(SavedFilters(query: q, showsDetections: true))
        #expect(store.load() == SavedFilters(query: q, showsDetections: true))
    }

    @Test func aTypeTheSaveDidNotKnowShowsByDefault() throws {
        // A blob from an older version: it excluded only floods, and knew nothing of later types.
        let store = freshStore()
        let old = #"{"version":1,"excludedTypes":["flood","no_such_type"],"minimumSeverity":"minor","excludedKinds":[],"window":86400,"showsDetections":false}"#
        UserDefaults(suiteName: store.suiteName)!.set(Data(old.utf8), forKey: store.key)
        let loaded = store.load().query
        #expect(!loaded.types.contains(.flood))
        #expect(loaded.types.count == HazardType.hazards.count - 1)
    }

    @Test func anUnreadableBlobGivesTheDefaults() {
        let store = freshStore()
        let defaults = UserDefaults(suiteName: store.suiteName)!
        defaults.set(Data("not json".utf8), forKey: store.key)
        #expect(store.load() == SavedFilters())
        defaults.set(Data(#"{"version":2}"#.utf8), forKey: store.key)
        #expect(store.load() == SavedFilters())
        // Everything excluded: never restore a query that shows nothing.
        let all = HazardType.hazards.map { "\"\($0.rawValue)\"" }.joined(separator: ",")
        defaults.set(Data(#"{"version":1,"excludedTypes":[\#(all)],"minimumSeverity":"bogus","excludedKinds":["observed","warning","forecast"],"window":null,"showsDetections":false}"#.utf8), forKey: store.key)
        let q = store.load().query
        #expect(q.types == EventQuery().types)
        #expect(q.kinds == EventQuery().kinds)
        #expect(q.minimumSeverity == .minor)
        #expect(q.window == TimeInterval(24 * 3600)) // null isn't a window the sheet offers
        // Nor is an arbitrary number.
        defaults.set(Data(#"{"version":1,"excludedTypes":[],"minimumSeverity":"minor","excludedKinds":[],"window":1234,"showsDetections":false}"#.utf8), forKey: store.key)
        #expect(store.load().query.window == TimeInterval(24 * 3600))
    }
}

@MainActor @Suite struct ModelFilterTests {
    @Test func savedFiltersLoadAtInitWithoutALoadOrASave() async {
        let store = freshStore()
        var q = EventQuery()
        q.minimumSeverity = .severe
        store.save(SavedFilters(query: q, showsDetections: true))
        let p = ScriptedProvider()
        let m = LiveMapModel(provider: p, filters: store, debounce: .zero)
        #expect(m.query.minimumSeverity == .severe)
        #expect(m.showsDetections)
        await m.settle()
        #expect(p.requests.isEmpty) // nothing loads before the map reports a viewport
    }

    @Test func changesAreSavedAndAnEmptySetIsRefused() async {
        let store = freshStore()
        let m = LiveMapModel(provider: ScriptedProvider(), filters: store, debounce: .zero)
        m.query.window = 3600
        m.showsDetections = true
        #expect(store.load() == SavedFilters(query: m.query, showsDetections: true))

        m.query.types = []
        #expect(m.query.types == EventQuery().types)
        m.query.kinds = [.detection] // heat detections are the layer, not a kind
        #expect(m.query.kinds == EventQuery().kinds)
        #expect(store.load().query.window == 3600)
    }
}

@Suite struct ResolvedFilterTests {
    @Test func statusIsAChipWithItsOwnReset() {
        let q = EventQuery(includeResolved: true)
        #expect(q.activeFilters == [.status])
        #expect(q.label(for: .status) == "With resolved")
        #expect(q.removing(.status).isDefault)
    }

    @Test func includeResolvedIsSaved() {
        let store = FilterStore(suiteName: "resolved-\(UUID())")
        store.save(SavedFilters(query: EventQuery(includeResolved: true)))
        #expect(store.load().query.includeResolved)
        store.clear()
    }

    @Test func aVersion1BlobWithoutStatusKeepsItsOtherFilters() {
        let store = FilterStore(suiteName: "resolved-old-\(UUID())")
        let defaults = UserDefaults(suiteName: store.suiteName)!
        defaults.set(Data(#"{"version":1,"excludedTypes":["flood"],"minimumSeverity":"severe","excludedKinds":[],"window":3600,"showsDetections":true}"#.utf8), forKey: store.key)
        let loaded = store.load()
        #expect(!loaded.query.includeResolved)
        #expect(!loaded.query.types.contains(.flood))
        #expect(loaded.query.minimumSeverity == .severe)
        #expect(loaded.query.window == 3600)
        #expect(loaded.showsDetections)
        store.clear()
    }
}

/// PR #23: the server windows a resolved event by when it ended (else its last update), an
/// active one by its last update. Live stream patches must agree.
@Suite struct ResolvedWindowTests {
    let now = Date(timeIntervalSince1970: 1_794_838_800)

    private func event(updated: TimeInterval, ended: TimeInterval?) -> HazardEvent {
        HazardEvent(id: UUID(), source: "meteoalarm", type: .wind, kind: .warning,
                    status: ended == nil ? .active : .resolved, severity: .moderate, sourceSeverity: nil,
                    title: "t", centroid: Coordinate(latitude: 0, longitude: 0),
                    observedAt: now.addingTimeInterval(updated), updatedAt: now.addingTimeInterval(updated),
                    endedAt: ended.map { now.addingTimeInterval($0) }, fetchedAt: now)
    }

    @Test func aResolvedEventIsWindowedByItsEnd() {
        let q = EventQuery(includeResolved: true, window: 24 * 3600)
        // Updated two days ago, ended an hour ago: resolved within the last 24 h.
        #expect(q.matches(event(updated: -48 * 3600, ended: -3600), now: now))
        // Updated an hour ago, ended two days ago: not.
        #expect(!q.matches(event(updated: -3600, ended: -48 * 3600), now: now))
    }

    @Test func anActiveEventIsStillWindowedByItsUpdate() {
        let q = EventQuery(includeResolved: true, window: 24 * 3600)
        #expect(q.matches(event(updated: -3600, ended: nil), now: now))
        #expect(!q.matches(event(updated: -48 * 3600, ended: nil), now: now))
    }
}
