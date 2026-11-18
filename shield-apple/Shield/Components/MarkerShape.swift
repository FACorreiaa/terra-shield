import SwiftUI

/// The severity shapes (DESIGN.md §4): circle, diamond, triangle, octagon. Each fills its
/// frame so the four read as one family at the same size; `Severity.markerScale` sizes them.
struct MarkerShape: Shape {
    var kind: MarkerShapeKind

    func path(in rect: CGRect) -> Path {
        let r = rect.insetBy(dx: 0.5, dy: 0.5)
        switch kind {
        case .circle:
            return Path(ellipseIn: r.insetBy(dx: r.width * 0.06, dy: r.height * 0.06))
        case .diamond:
            return Path { p in
                p.move(to: CGPoint(x: r.midX, y: r.minY))
                p.addLine(to: CGPoint(x: r.maxX, y: r.midY))
                p.addLine(to: CGPoint(x: r.midX, y: r.maxY))
                p.addLine(to: CGPoint(x: r.minX, y: r.midY))
                p.closeSubpath()
            }
        case .triangle:
            // Point up, with the glyph's optical centre a little below the frame's centre.
            return Path { p in
                p.move(to: CGPoint(x: r.midX, y: r.minY))
                p.addLine(to: CGPoint(x: r.maxX, y: r.maxY - r.height * 0.06))
                p.addLine(to: CGPoint(x: r.minX, y: r.maxY - r.height * 0.06))
                p.closeSubpath()
            }
        case .octagon:
            // Regular octagon: each corner cut is w / (2 + √2).
            let k = r.width / (2 + 2.squareRoot())
            return Path { p in
                p.move(to: CGPoint(x: r.minX + k, y: r.minY))
                p.addLine(to: CGPoint(x: r.maxX - k, y: r.minY))
                p.addLine(to: CGPoint(x: r.maxX, y: r.minY + k))
                p.addLine(to: CGPoint(x: r.maxX, y: r.maxY - k))
                p.addLine(to: CGPoint(x: r.maxX - k, y: r.maxY))
                p.addLine(to: CGPoint(x: r.minX + k, y: r.maxY))
                p.addLine(to: CGPoint(x: r.minX, y: r.maxY - k))
                p.addLine(to: CGPoint(x: r.minX, y: r.minY + k))
                p.closeSubpath()
            }
        }
    }

    /// Where a glyph sits inside the shape, as a fraction of the frame (the triangle's is lower).
    var glyphOffsetY: CGFloat { kind == .triangle ? 0.12 : 0 }
    /// How much of the frame the glyph may use.
    var glyphScale: CGFloat {
        switch kind {
        case .circle, .octagon: 0.52
        case .diamond: 0.42
        case .triangle: 0.38
        }
    }
}
