import { Layers } from "lucide-react";
import type { ChangeEvent, ReactElement, RefObject } from "react";
import { useCallback, useEffect, useId, useRef, useState } from "react";

import type { Projection } from "./basemapStyle.ts";
import { controlButton } from "./controlStyles.ts";

interface LayersMenuProps {
  projection: Projection;
  onProjectionChange: (projection: Projection) => void;
}

const PROJECTIONS: readonly { value: Projection; label: string }[] = [
  { value: "globe", label: "Globe" },
  { value: "mercator", label: "Flat" },
];

/**
 * While the panel is open: Escape closes it and puts focus back on its button, and a press
 * outside `root` closes it. Escape is taken in the capture phase and stopped there, so an open
 * menu closes before anything else on the page reacts to the same key.
 */
function useDismiss(
  open: boolean,
  refs: { root: RefObject<HTMLElement | null>; button: RefObject<HTMLElement | null> },
  close: () => void,
): void {
  const { root, button } = refs;
  useEffect(() => {
    if (!open) {
      return undefined;
    }
    const onPointerDown = (event: PointerEvent): void => {
      if (event.target instanceof Node && root.current?.contains(event.target) !== true) {
        close();
      }
    };
    const onKeyDown = (event: KeyboardEvent): void => {
      if (event.key === "Escape") {
        event.stopPropagation();
        close();
        button.current?.focus();
      }
    };
    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeyDown, { capture: true });
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown, { capture: true });
    };
  }, [open, root, button, close]);
}

/** Globe or flat, as a two-segment control over native radios (arrow keys move between them). */
function ProjectionChoice({ projection, onProjectionChange }: LayersMenuProps): ReactElement {
  const name = useId();
  const choose = useCallback(
    (event: ChangeEvent<HTMLInputElement>) => {
      const next = PROJECTIONS.find((option) => option.value === event.target.value);
      if (next !== undefined) {
        onProjectionChange(next.value);
      }
    },
    [onProjectionChange],
  );
  return (
    <fieldset>
      <legend className="mb-2 text-meta text-muted">Map</legend>
      <div className="grid grid-cols-2 gap-1 rounded-sm bg-canvas/60 p-1">
        {PROJECTIONS.map((option) => (
          <label
            key={option.value}
            className="flex h-11 cursor-pointer items-center justify-center rounded-xs text-body text-muted transition-colors hover:text-ink has-checked:bg-raised has-checked:text-ink has-focus-visible:outline-2 has-focus-visible:outline-accent"
          >
            <input
              type="radio"
              name={name}
              value={option.value}
              checked={projection === option.value}
              onChange={choose}
              className="sr-only"
            />
            {option.label}
          </label>
        ))}
      </div>
    </fieldset>
  );
}

function LayersPanel({ id, projection, onProjectionChange }: LayersMenuProps & { id: string }): ReactElement {
  return (
    <div
      id={id}
      className="absolute top-0 right-full mr-2 w-56 rounded-md border border-hairline-strong p-3 shadow-floating material-floating contrast-more:bg-panel contrast-more:backdrop-filter-none"
    >
      <ProjectionChoice projection={projection} onProjectionChange={onProjectionChange} />
    </div>
  );
}

/**
 * The layers button and its panel: the map's projection. The data layers (heat, flights,
 * conflicts) are one-press switches in the controls column, as in the apps.
 */
function LayersMenu({ projection, onProjectionChange }: LayersMenuProps): ReactElement {
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const button = useRef<HTMLButtonElement>(null);
  const panelId = useId();
  const close = useCallback(() => {
    setOpen(false);
  }, []);
  const toggle = useCallback(() => {
    setOpen((was) => !was);
  }, []);
  useDismiss(open, { root, button }, close);

  return (
    <div ref={root} className="relative">
      <button
        ref={button}
        type="button"
        className={controlButton}
        aria-label="Layers"
        title="Layers"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={toggle}
      >
        <Layers className="size-4" strokeWidth={2.25} aria-hidden />
      </button>
      {open ? <LayersPanel id={panelId} projection={projection} onProjectionChange={onProjectionChange} /> : undefined}
    </div>
  );
}

export { LayersMenu };
