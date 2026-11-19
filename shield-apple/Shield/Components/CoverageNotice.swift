import ShieldCore
import SwiftUI

/// Who covers a country, and what that leaves out (DESIGN.md §10.2, ADR 7). Always shown: a
/// quiet map is only reassuring if you know who is watching.
///
/// It says only what the API knows: whether an official warning feed covers the country, and
/// which sources do. It never names a missing national service, since nothing says which one.
struct CoverageNotice: View {
    let countryName: String
    let coverage: Coverage
    /// From `/v1/sources`, for names; an id shows until they load.
    let sources: [String: Source]

    var body: some View {
        VStack(alignment: .leading, spacing: ShieldSpace.s2) {
            if coverage.officialWarnings {
                Label {
                    Text("Official warnings cover \(countryName).")
                        .foregroundStyle(Color.ink)
                } icon: {
                    Image(systemName: "checkmark.seal").foregroundStyle(Color.muted)
                }
            } else {
                Label {
                    Text("Limited coverage: no official warning service for \(countryName) feeds Aegis Terra, so warnings may be missing.")
                        .foregroundStyle(Color.ink)
                } icon: {
                    Image(systemName: "exclamationmark.triangle").foregroundStyle(Color.warning)
                }
            }
            if !names.isEmpty {
                Text("Sources: \(names.formatted(.list(type: .and))).")
                    .foregroundStyle(Color.muted)
            }
            Text("Absence of data is not safety.")
                .foregroundStyle(Color.muted)
        }
        .font(ShieldFont.meta)
        .fixedSize(horizontal: false, vertical: true)
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(ShieldSpace.s3)
        .background(Color.ink.opacity(ShieldAlpha.hairline / 2), in: RoundedRectangle(cornerRadius: ShieldRadius.md))
        .overlay(RoundedRectangle(cornerRadius: ShieldRadius.md)
            .strokeBorder(coverage.officialWarnings ? Color.ink.opacity(ShieldAlpha.hairline) : Color.warning.opacity(0.6)))
        .accessibilityElement(children: .combine)
    }

    private var names: [String] {
        coverage.sources.map { sources[$0]?.name ?? $0.uppercased() }
    }
}

#Preview("CoverageNotice", traits: .sizeThatFitsLayout) {
    let sources = Dictionary(uniqueKeysWithValues: SampleData.sources().map { ($0.id, $0) })
    VStack(spacing: ShieldSpace.s4) {
        CoverageNotice(countryName: "Portugal",
                       coverage: Coverage(officialWarnings: true, sources: ["meteoalarm", "usgs", "gdacs"]),
                       sources: sources)
        CoverageNotice(countryName: "Chile",
                       coverage: Coverage(officialWarnings: false, sources: ["usgs", "gdacs", "eonet"]),
                       sources: sources)
    }
    .padding(ShieldSpace.s5)
    .frame(width: 360)
    .background(Color.panel)
}
