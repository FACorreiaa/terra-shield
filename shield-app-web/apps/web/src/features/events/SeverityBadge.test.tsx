import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vite-plus/test";

import { SeverityBadge } from "./SeverityBadge.tsx";

describe(SeverityBadge, () => {
  afterEach(cleanup);

  it("shows the word and the source value side by side, never merged", () => {
    render(<SeverityBadge severity="severe" sourceSeverity="M 6.1 mww, PAGER yellow" />);
    expect(screen.getByText("Severe")).toBeDefined();
    expect(screen.getByText("M 6.1 mww, PAGER yellow")).toBeDefined();
  });

  it("marks provisional severity and hides an empty source value", () => {
    const { container } = render(<SeverityBadge severity="minor" sourceSeverity="" provisional />);
    expect(screen.getByText("Provisional")).toBeDefined();
    expect(container.textContent).toBe("MinorProvisional");
  });
});
