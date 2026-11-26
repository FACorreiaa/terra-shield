import { render } from "@testing-library/react";
import type { ReactElement } from "react";
import { describe, expect, it, vi } from "vite-plus/test";

describe("glyphMarkup", () => {
  it("gives every glyph's markup, the conflicts' crosshair included", async () => {
    expect.hasAssertions();
    const { glyphMarkup } = await import("./glyphMarkup.ts");
    expect(glyphMarkup("flame")).toContain("<path");
    expect(glyphMarkup("crosshair")).toContain("<circle");
  });

  it("has a type's glyph ready when first asked for during a render", async () => {
    expect.hasAssertions();
    vi.resetModules();
    // A fresh module, as on a first visit: nothing asked for yet.
    const { glyphMarkup } = await import("./glyphMarkup.ts");
    let markup = "";
    function Marker(): ReactElement {
      markup = glyphMarkup("crosshair");
      return <span />;
    }
    render(<Marker />);
    expect(markup).toContain("<circle");
  });
});
