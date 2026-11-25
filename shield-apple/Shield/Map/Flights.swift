import MapKit
import ShieldCore
import SwiftUI

// MARK: - Map

/// One aircraft. Mutable, unlike events: a refresh moves it in place, so it neither flickers nor
/// loses its selection every minute.
final class AircraftAnnotation: NSObject, MKAnnotation {
    private(set) var aircraft: Aircraft
    @objc dynamic var coordinate: CLLocationCoordinate2D

    init(_ aircraft: Aircraft) {
        self.aircraft = aircraft
        coordinate = CLLocationCoordinate2D(latitude: aircraft.coordinate.latitude,
                                            longitude: aircraft.coordinate.longitude)
    }

    func update(_ aircraft: Aircraft) {
        self.aircraft = aircraft
        let c = aircraft.coordinate
        if c.latitude != coordinate.latitude || c.longitude != coordinate.longitude {
            coordinate = CLLocationCoordinate2D(latitude: c.latitude, longitude: c.longitude)
        }
    }

    var title: String? { aircraft.title }
}

/// The aircraft glyph, turned to its track. Below every hazard, and hidden by MapKit where
/// aircraft crowd each other: hazards are the app's subject, flights are context (ADR 28).
final class AircraftMarkerView: SwiftUIAnnotationView {
    static let reuseID = "aircraft"

    func configure(_ aircraft: Aircraft, isSelected: Bool) {
        // Set on every use, like the event markers, so a reused view can't keep another's.
        // Below every event marker, resolved ones (100) included: aircraft give way on collision.
        displayPriority = MKFeatureDisplayPriority(rawValue: 50)
        zPriority = .min
        collisionMode = .circle
        clusteringIdentifier = nil
        render(AircraftMarker(aircraft: aircraft, isSelected: isSelected), size: CGSize(width: 28, height: 28))
        speak(Self.spoken(aircraft))
    }

    /// Whether two positions draw and read the same, so a refresh that only moves an aircraft
    /// (or ages it) doesn't re-render its marker.
    static func looksSame(_ a: Aircraft, _ b: Aircraft) -> Bool {
        func track(_ x: Aircraft) -> Int? { x.trackDegrees.map { Int($0.rounded()) % 360 } }
        return track(a) == track(b) && a.title == b.title && a.altitudeFeet == b.altitudeFeet
            && a.onGround == b.onGround && a.military == b.military && a.emergency == b.emergency
    }

    static func spoken(_ a: Aircraft) -> String {
        var parts = ["Aircraft \(a.title)"]
        if a.onGround {
            parts.append("on the ground")
        } else if let feet = a.altitudeFeet {
            parts.append("\(feet.formatted(.number.locale(Locale(identifier: "en_US")))) feet")
        }
        if let track = a.trackDegrees { parts.append("track \(Int(track.rounded()) % 360) degrees") }
        if a.military { parts.append("military") }
        if let emergency = a.emergency { parts.append("emergency \(emergency)") }
        return parts.joined(separator: ", ")
    }
}

/// `airplane` points east at rest, so a track of 90° needs no turn. Without a track the
/// direction is unknown: a dot, not a guess.
struct AircraftMarker: View {
    let aircraft: Aircraft
    var isSelected = false

    var body: some View {
        ZStack {
            if isSelected {
                Circle().fill(Color.accent.opacity(0.25)).frame(width: 28, height: 28)
            }
            if let track = aircraft.trackDegrees {
                Image(systemName: "airplane")
                    .font(.system(size: 15, weight: .bold))
                    .foregroundStyle(Color.ink)
                    .shadow(color: .black.opacity(0.8), radius: 1.5)
                    .rotationEffect(.degrees(track - 90))
            } else {
                Circle()
                    .fill(Color.ink)
                    .frame(width: 7, height: 7)
                    .shadow(color: .black.opacity(0.8), radius: 1.5)
            }
        }
        .frame(width: 28, height: 28)
    }
}

// MARK: - Legend

/// The layer's state over the map: how many, how old, whether complete, and the ODbL attribution
/// whenever the layer is on (ADR 28). Re-reads the clock every 10 s, so the age is never frozen.
struct FlightsLegend: View {
    let model: LiveMapModel

    var body: some View {
        TimelineView(.periodic(from: .now, by: 10)) { context in
            VStack(alignment: .leading, spacing: ShieldSpace.s1) {
                HStack(spacing: ShieldSpace.s2) {
                    Image(systemName: "airplane")
                        .font(.system(size: 11, weight: .semibold))
                        .accessibilityHidden(true)
                    Text(headline)
                        .font(ShieldFont.meta.weight(.semibold))
                }
                .foregroundStyle(model.flightsFailed ? Color.warning : Color.ink)
                ForEach(lines(now: context.date), id: \.self) { line in
                    Text(line)
                        .font(ShieldFont.meta)
                        .foregroundStyle(Color.muted)
                        .fixedSize(horizontal: false, vertical: true)
                }
                Text(attribution)
                    .font(ShieldFont.micro)
                    .foregroundStyle(Color.muted)
            }
            .padding(.horizontal, ShieldSpace.s3)
            .padding(.vertical, ShieldSpace.s2)
            .frame(maxWidth: 360, alignment: .leading)
            .background(.ultraThinMaterial, in: RoundedRectangle(cornerRadius: ShieldRadius.md))
            .overlay(RoundedRectangle(cornerRadius: ShieldRadius.md).strokeBorder(Color.ink.opacity(ShieldAlpha.hairline)))
            .accessibilityElement(children: .combine)
        }
    }

    private var headline: String {
        if model.flightsNeedZoom { return "Flights · zoom in to see them" }
        if model.flightsFailed, model.flights == nil { return "Flights didn’t load" }
        guard let flights = model.flights else { return "Flights · loading…" }
        let n = flights.aircraft.count
        return "Flights · " + (n == 0 ? "none in this view" : n == 1 ? "1 in this view" : "\(n) in this view")
    }

    private func lines(now: Date) -> [String] {
        if model.flightsNeedZoom { return ["Shown for an area up to about 5° across."] }
        guard let flights = model.flights else { return [] }
        var out = [flights.ageLine(now: now)]
        if !flights.complete { out.append("Incomplete: part of this view has no recent data.") }
        if model.flightsFailed { out.append("The last refresh failed; these positions are older.") }
        return out
    }

    private var attribution: String {
        let layer = model.flights ?? .empty
        return "\(layer.attribution) · \(layer.license)"
    }
}

// MARK: - Detail

/// The selected aircraft in the sheet (iPhone) or inspector (Mac, iPad): a position report,
/// never a flight page — no route, no history (ADR 28).
struct AircraftDetailView: View {
    let aircraft: Aircraft
    let layer: AircraftLayer?
    var close: () -> Void

    var body: some View {
        TimelineView(.periodic(from: .now, by: 10)) { context in
            VStack(alignment: .leading, spacing: ShieldSpace.s4) {
                HStack(alignment: .firstTextBaseline) {
                    VStack(alignment: .leading, spacing: ShieldSpace.s1) {
                        Text("AIRCRAFT")
                            .font(ShieldFont.micro)
                            .tracking(ShieldTracking.micro)
                            .foregroundStyle(Color.muted)
                        Text(aircraft.title)
                            .font(ShieldFont.title)
                            .foregroundStyle(Color.ink)
                    }
                    Spacer()
                    Button(action: close) {
                        Image(systemName: "xmark.circle.fill")
                            .font(.system(size: 22))
                            .foregroundStyle(Color.muted)
                    }
                    .buttonStyle(.plain)
                    .accessibilityLabel("Close")
                }
                VStack(alignment: .leading, spacing: ShieldSpace.s1) {
                    ForEach(aircraft.notes(now: context.date), id: \.self) { note in
                        Text(note)
                            .font(ShieldFont.meta.weight(note.hasPrefix("Emergency") ? .semibold : .regular))
                            .foregroundStyle(note.hasPrefix("Emergency") ? Color.warning : Color.muted)
                    }
                }
                Grid(alignment: .leading, horizontalSpacing: ShieldSpace.s4, verticalSpacing: ShieldSpace.s2) {
                    ForEach(aircraft.figures, id: \.self) { figure in
                        GridRow {
                            Text(figure.label)
                                .font(ShieldFont.meta)
                                .foregroundStyle(Color.muted)
                            Text(figure.value)
                                .font(ShieldFont.body.monospacedDigit())
                                .foregroundStyle(Color.ink)
                                .textSelection(.enabled)
                        }
                    }
                }
                Rectangle().fill(Color.ink.opacity(ShieldAlpha.hairline)).frame(height: 1)
                VStack(alignment: .leading, spacing: ShieldSpace.s1) {
                    Text("Live positions from adsb.lol, a community network of receivers. Not every aircraft is covered, and positions can be minutes old.")
                        .font(ShieldFont.meta)
                        .foregroundStyle(Color.muted)
                        .fixedSize(horizontal: false, vertical: true)
                    let l = layer ?? .empty
                    if let url = l.licenseURL {
                        Link("\(l.attribution) · \(l.license)", destination: url)
                            .font(ShieldFont.meta)
                    } else {
                        Text("\(l.attribution) · \(l.license)").font(ShieldFont.meta)
                    }
                }
            }
        }
    }
}
