import { createElement } from "react";
import { flushSync } from "react-dom";
import { createRoot } from "react-dom/client";

import { Icon } from "@/design/icons.ts";

const cache = new Map<string, string>();

/**
 * The inner SVG markup (paths, circles, lines) of a token-named Lucide icon, for drawing the type
 * glyph into a canvas marker. Rendered once per name into a detached node and cached; no
 * server renderer and no second copy of the icon set.
 */
function glyphMarkup(name: string): string {
  const cached = cache.get(name);
  if (cached !== undefined) {
    return cached;
  }
  const host = document.createElement("div");
  const root = createRoot(host);
  flushSync(() => {
    root.render(createElement(Icon, { name }));
  });
  const markup = host.querySelector("svg")?.innerHTML ?? "";
  root.unmount();
  cache.set(name, markup);
  return markup;
}

export { glyphMarkup };
