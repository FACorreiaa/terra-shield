// swift-tools-version:6.2

import PackageDescription

/// Hosts swift-openapi-generator for `scripts/generate-api-client.sh`. Shield's server is Go, so
/// unlike Aaru there is no Swift server package to run the generator from. Nothing ships from here.
let package = Package(
    name: "shield-api-tools",
    platforms: [.macOS(.v15)],
    dependencies: [
        .package(url: "https://github.com/apple/swift-openapi-generator.git", exact: "1.13.0"),
    ],
    targets: []
)
