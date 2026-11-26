import { createElement } from "react";
import { flushSync } from "react-dom";
import { createRoot } from "react-dom/client";

import { Icon } from "@/design/icons.ts";
import { hazardTypeTokens } from "@/design/tokens.gen.ts";

const cache = new Map<string, string>();

function render(name: string): string {
  const host = document.createElement("div");
  const root = createRoot(host);
  flushSync(() => {
    root.render(createElement(Icon, { name }));
  });
  const markup = host.querySelector("svg")?.innerHTML ?? "";
  root.unmount();
  return markup;
}

/**
 * The inner SVG markup (paths, circles, lines) of a token-named Lucide icon, for drawing the type
 * glyph into a canvas marker. Rendered once per name into a detached node and cached; no
 * server renderer and no second copy of the icon set.
 *
 * `flushSync` can't flush while React is rendering, and `MarkerImage` asks during render, so every
 * type's glyph is rendered here when the module loads, and an empty answer is never kept: it
 * would leave that type's markers blank for the rest of the visit.
 */
function glyphMarkup(name: string): string {
  const cached = cache.get(name);
  if (cached !== undefined) {
    return cached;
  }
  const markup = render(name);
  if (markup !== "") {
    cache.set(name, markup);
  }
  return markup;
}

for (const { icon } of Object.values(hazardTypeTokens)) {
  glyphMarkup(icon);
}

export { glyphMarkup };
