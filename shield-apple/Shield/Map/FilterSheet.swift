import ShieldCore
import SwiftUI

/// The filter sheet (DESIGN.md §10.6): hazard types, minimum severity, kinds, time window and
/// status. Changes apply as they are made, and the map reloads behind the sheet. "Reset" shows
/// when anything differs from the default.
struct FilterSheet: View {
    @Bindable var model: LiveMapModel
    var done: () -> Void

    private var isDefault: Bool { model.query.isDefault && !model.showsDetections }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: ShieldSpace.s6) {
                section("Hazard type", trailing: model.query.types.count < HazardType.hazards.count
                        ? AnyView(Button("All") { model.query.types = Set(HazardType.hazards) }
                            .buttonStyle(.plain).foregroundStyle(Color.accent).font(ShieldFont.meta.weight(.semibold)))
                        : nil) {
                    typeGrid
                }
                section("Minimum severity") { severityPicker }
                section("Kind") { kindToggles }
                section("Updated in the last") { windowPicker }
                section("Status") { statusToggle }
                if model.provider.supportsHistoric {
                    section("Historic sites") {
                        historicToggles
                        // The layer's own description (ADR 30): the phone's legend leaves it out.
                        if let note = model.historic?.note {
                            Text(note)
                                .font(ShieldFont.meta)
                                .foregroundStyle(Color.muted)
                                .fixedSize(horizontal: false, vertical: true)
                        }
                    }
                }
            }
            .padding(ShieldSpace.s5)
        }
        .background(Color.panel)
        .navigationTitle("Filters")
        #if os(iOS)
        .navigationBarTitleDisplayMode(.inline)
        #endif
        .toolbar {
            ToolbarItem(placement: .cancellationAction) {
                if !isDefault {
                    Button("Reset") {
                        model.query = EventQuery(country: model.query.country)
                        model.showsDetections = false
                    }
                }
            }
            ToolbarItem(placement: .confirmationAction) {
                Button("Done", action: done).fontWeight(.semibold)
            }
        }
        .tint(.accent)
    }

    // MARK: Type

    private var typeGrid: some View {
        LazyVGrid(columns: [GridItem(.adaptive(minimum: 96), spacing: ShieldSpace.s2)], spacing: ShieldSpace.s2) {
            ForEach(HazardType.hazards, id: \.self) { type in
                let on = model.query.types.contains(type)
                Button {
                    // The last type can't be turned off: a map filtered to nothing helps no one.
                    if on, model.query.types.count == 1 { return }
                    if on { model.query.types.remove(type) } else { model.query.types.insert(type) }
                } label: {
                    VStack(spacing: ShieldSpace.s1) {
                        Image(systemName: type.sfSymbol)
                            .font(.system(size: 18, weight: .semibold))
                            .frame(height: 22)
                        Text(type.label)
                            .font(ShieldFont.micro)
                            .multilineTextAlignment(.center)
                            .lineLimit(2)
                            .fixedSize(horizontal: false, vertical: true)
                    }
                    .foregroundStyle(on ? Color.ink : Color.muted)
                    .frame(maxWidth: .infinity, minHeight: 64)
                    .padding(.horizontal, ShieldSpace.s1)
                    .toggleSurface(isOn: on)
                }
                .buttonStyle(.plain)
                .accessibilityLabel(type.label)
                .accessibilityAddTraits(on ? [.isSelected] : [])
                .accessibilityHint(on && model.query.types.count == 1 ? "The last type can't be turned off" : "")
            }
        }
    }

    // MARK: Severity

    /// Four steps, each with the severity's own shape and colour (DESIGN.md §4).
    private var severityPicker: some View {
        HStack(spacing: ShieldSpace.s2) {
            ForEach(Severity.allCases, id: \.self) { severity in
                let selected = model.query.minimumSeverity == severity
                let included = severity >= model.query.minimumSeverity
                Button { model.query.minimumSeverity = severity } label: {
                    VStack(spacing: ShieldSpace.s1) {
                        Image(systemName: severity.sfSymbol)
                            .font(.system(size: 14, weight: .bold))
                            .foregroundStyle(included ? severity.color : Color.muted)
                        Text(severity == .minor ? "All" : "\(severity.label)+")
                            .font(ShieldFont.meta.weight(selected ? .semibold : .regular))
                            .foregroundStyle(selected ? Color.ink : Color.muted)
                            .lineLimit(1)
                            .minimumScaleFactor(0.8)
                    }
                    .frame(maxWidth: .infinity, minHeight: 52)
                    .toggleSurface(isOn: selected)
                }
                .buttonStyle(.plain)
                .accessibilityLabel(severity == .minor ? "All severities" : "\(severity.label) and above")
                .accessibilityAddTraits(selected ? [.isSelected] : [])
            }
        }
    }

    // MARK: Kind

    private var kindToggles: some View {
        VStack(spacing: 0) {
            ForEach(EventQuery.eventKinds, id: \.self) { kind in
                let on = model.query.kinds.contains(kind)
                kindRow(kind, isOn: Binding(
                    get: { on },
                    set: { newValue in
                        if newValue { model.query.kinds.insert(kind) }
                        // The last kind stays on.
                        else if model.query.kinds.count > 1 { model.query.kinds.remove(kind) }
                    }
                ), disabled: on && model.query.kinds.count == 1)
                Divider().overlay(Color.ink.opacity(ShieldAlpha.hairline))
            }
            // The FIRMS layer, the same switch as the map's ⌥⌘H control. Off by default (§5).
            kindRow(.detection, isOn: $model.showsDetections, disabled: false)
        }
        .padding(.horizontal, ShieldSpace.s3)
        .background(Color.ink.opacity(ShieldAlpha.hairline / 2), in: RoundedRectangle(cornerRadius: ShieldRadius.md))
    }

    private func kindRow(_ kind: EventKind, isOn: Binding<Bool>, disabled: Bool) -> some View {
        switchRow(kind.label, symbol: kind.sfSymbol, isOn: isOn, disabled: disabled)
    }

    // MARK: Historic sites

    /// The layer's switch (the map's ⌥⌘S control) and its groups (ADR 30). The groups stay
    /// settable with the layer off, and the last one stays on: the API reads none as all.
    private var historicToggles: some View {
        VStack(spacing: 0) {
            switchRow("Show on the map", symbol: "building.columns", isOn: $model.showsHistoric, disabled: false)
            ForEach(HistoricKind.allCases, id: \.self) { kind in
                Divider().overlay(Color.ink.opacity(ShieldAlpha.hairline))
                let on = model.historicKinds.contains(kind)
                switchRow(kind.label, symbol: kind.sfSymbol, isOn: Binding(
                    get: { on },
                    set: { newValue in
                        if newValue { model.historicKinds.insert(kind) } else { model.historicKinds.remove(kind) }
                    }
                ), disabled: on && model.historicKinds.count == 1)
            }
            Divider().overlay(Color.ink.opacity(ShieldAlpha.hairline))
            // With the groups above, as on the web: only UNESCO World Heritage Sites of them.
            switchRow("World Heritage Sites only", symbol: "checkmark.seal",
                      isOn: $model.historicWorldHeritageOnly, disabled: false)
        }
        .padding(.horizontal, ShieldSpace.s3)
        .background(Color.ink.opacity(ShieldAlpha.hairline / 2), in: RoundedRectangle(cornerRadius: ShieldRadius.md))
    }

    // MARK: Status

    /// Active events always show; this adds the resolved ones, faded with a checkmark. The
    /// window applies to their last update, as it does to active events, not to when they ended.
    private var statusToggle: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s2) {
            switchRow("Include resolved", symbol: "checkmark.circle", isOn: $model.query.includeResolved, disabled: false)
                .padding(.horizontal, ShieldSpace.s3)
                .background(Color.ink.opacity(ShieldAlpha.hairline / 2), in: RoundedRectangle(cornerRadius: ShieldRadius.md))
            Text("Adds events that ended or were cancelled in the window, shown faded.")
                .font(ShieldFont.meta)
                .foregroundStyle(Color.muted)
                .fixedSize(horizontal: false, vertical: true)
        }
    }

    private func switchRow(_ label: String, symbol: String, isOn: Binding<Bool>, disabled: Bool) -> some View {
        // Label left, switch right on both platforms (the Mac's default is a centred checkbox,
        // and its switch hugs the label).
        HStack(spacing: ShieldSpace.s3) {
            Image(systemName: symbol)
                .foregroundStyle(Color.muted)
                .frame(width: 22)
                .accessibilityHidden(true)
            Text(label).font(ShieldFont.body).foregroundStyle(Color.ink)
                .accessibilityHidden(true) // the Toggle carries the name
            Spacer(minLength: ShieldSpace.s2)
            Toggle(label, isOn: isOn)
                .labelsHidden()
                .toggleStyle(.switch)
                .tint(Color.accent)
                .disabled(disabled)
        }
        .padding(.vertical, ShieldSpace.s2)
    }

    // MARK: Window

    private var windowPicker: some View {
        Picker("Updated in the last", selection: $model.query.window) {
            ForEach(EventQuery.windows, id: \.self) { window in
                Text(EventQuery.windowLabel(window)).tag(Optional(window))
            }
        }
        .pickerStyle(.segmented)
        .labelsHidden()
    }

    // MARK: Layout

    private func section(_ title: String, trailing: AnyView? = nil, @ViewBuilder content: () -> some View) -> some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s3) {
            HStack {
                Text(title.uppercased())
                    .font(ShieldFont.micro)
                    .tracking(ShieldTracking.micro)
                    .foregroundStyle(Color.muted)
                    .accessibilityAddTraits(.isHeader)
                Spacer()
                trailing
            }
            content()
        }
    }
}

private extension View {
    /// A tile that reads as on with a raised fill and an accent hairline, off as a plain outline.
    func toggleSurface(isOn: Bool) -> some View {
        self
            .background(isOn ? Color.raised : Color.clear, in: RoundedRectangle(cornerRadius: ShieldRadius.sm))
            .overlay(RoundedRectangle(cornerRadius: ShieldRadius.sm)
                .strokeBorder(isOn ? Color.accent.opacity(0.6) : Color.ink.opacity(ShieldAlpha.hairline)))
            .contentShape(RoundedRectangle(cornerRadius: ShieldRadius.sm))
    }
}
