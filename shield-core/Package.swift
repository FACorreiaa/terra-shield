// swift-tools-version:6.2

import PackageDescription

/// Domain types and rules shared by the Apple app (and its tests): hazard events, severity,
/// kind, freshness, country phrasing, and the provider protocol the UI reads through.
/// No UI and no networking. The generated API client lives in `shield-api`.
let package = Package(
    name: "shield-core",
    platforms: [.macOS(.v15), .iOS(.v18)],
    products: [
        .library(name: "ShieldCore", targets: ["ShieldCore"]),
    ],
    targets: [
        .target(name: "ShieldCore"),
        .testTarget(name: "ShieldCoreTests", dependencies: ["ShieldCore"]),
    ]
)
