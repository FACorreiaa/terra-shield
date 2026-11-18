// swift-tools-version:6.2

import PackageDescription

/// The `/v1` client for the Apple app. Types and client are generated from
/// `backend/api/openapi.yaml` by `scripts/generate-api-client.sh` and committed, so Xcode needs no
/// build plugin. `APIEventProvider` maps them onto `ShieldCore` types. Pattern: apps/aaru/aaru-api.
let package = Package(
    name: "shield-api",
    platforms: [.macOS(.v15), .iOS(.v18)],
    products: [
        .library(name: "ShieldAPI", targets: ["ShieldAPI"]),
    ],
    dependencies: [
        .package(path: "../shield-core"),
        .package(url: "https://github.com/apple/swift-openapi-runtime.git", from: "1.7.0"),
        .package(url: "https://github.com/apple/swift-openapi-urlsession.git", from: "1.0.0"),
        .package(url: "https://github.com/apple/swift-http-types.git", from: "1.3.0"),
    ],
    targets: [
        .target(
            name: "ShieldAPI",
            dependencies: [
                .product(name: "ShieldCore", package: "shield-core"),
                .product(name: "OpenAPIRuntime", package: "swift-openapi-runtime"),
                .product(name: "OpenAPIURLSession", package: "swift-openapi-urlsession"),
                .product(name: "HTTPTypes", package: "swift-http-types"),
            ]
        ),
        .testTarget(
            name: "ShieldAPITests",
            dependencies: [
                "ShieldAPI",
                .product(name: "OpenAPIRuntime", package: "swift-openapi-runtime"),
                .product(name: "HTTPTypes", package: "swift-http-types"),
            ]
        ),
    ]
)
