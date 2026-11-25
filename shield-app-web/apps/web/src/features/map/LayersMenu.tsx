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

/** Closes an open panel on Escape (focus back to its button) or a press outside `root`. */
function useDismiss(
  root: RefObject<HTMLElement | null>,
  button: RefObject<HTMLElement | null>,
  close: () => void,
): void {
  useEffect(() => {
    const onPointerDown = (event: PointerEvent): void => {
      if (event.target instanceof Node && root.current?.contains(event.target) !== true) {
        close();
      }
    };
    const onKeyDown = (event: KeyboardEvent): void => {
      if (event.key === "Escape") {
        close();
        button.current?.focus();
      }
    };
    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [root, button, close]);
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
 * The layers button and its panel. W2 holds the map's projection; the hazard layers (heat,
 * flights, conflicts) join it in W7.
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
  useDismiss(root, button, close);

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
