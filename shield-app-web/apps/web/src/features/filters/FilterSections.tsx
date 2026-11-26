import type { EventKind, Severity } from "@shield/api-client";
import { SEVERITY_ORDER } from "@shield/api-client";
import type { ChangeEvent, ReactElement, ReactNode } from "react";
import { useCallback, useId } from "react";

import { Icon } from "@/design/icons.ts";
import { KIND_ICONS } from "@/design/kindIcons.ts";
import { hazardTypeTokens, kindTokens, severityTokens } from "@/design/tokens.gen.ts";
import type { EventQuery } from "@/domain/filters.ts";
import { EVENT_KINDS, WINDOWS, windowLabel } from "@/domain/filters.ts";
import type { EventType } from "@/domain/taxonomy.ts";
import { HAZARD_TYPES, isAtLeast } from "@/domain/taxonomy.ts";
import { cn } from "@/lib/utils.ts";

import { toggleShowsDetections, useSavedFilters } from "./filterPrefs.ts";
import { Switch } from "./Switch.tsx";

interface SectionProps {
  query: EventQuery;
  onChange: (query: EventQuery) => void;
}

/** On: a raised fill and an accent hairline; off: a plain outline (FilterSheet.swift `toggleSurface`). */
const TILE =
  "flex flex-col items-center justify-center gap-1 rounded-sm border px-1 text-center focus-visible:outline-2 focus-visible:outline-accent has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-accent";
const TILE_ON = "border-accent/60 bg-raised text-ink";
const TILE_OFF = "border-hairline-strong text-muted hover:bg-ink/5";
const SEVERITY_TEXT: Readonly<Record<Severity, string>> = {
  minor: "text-severity-minor",
  moderate: "text-severity-moderate",
  severe: "text-severity-severe",
  extreme: "text-severity-extreme",
};

/** A titled group; `onAll`, when given, adds the "All" button that turns every option back on. */
function Section({
  title,
  onAll,
  children,
}: {
  title: string;
  onAll?: (() => void) | undefined;
  children: ReactNode;
}): ReactElement {
  const titleId = useId();
  return (
    // Named by its heading rather than a <legend>, so "All" can share the heading's row in normal flow.
    <fieldset aria-labelledby={titleId} className="flex min-w-0 flex-col gap-3">
      <div className="flex min-h-11 items-center justify-between">
        <span id={titleId} className="text-micro text-muted uppercase">
          {title}
        </span>
        {onAll === undefined ? undefined : (
          <button
            type="button"
            onClick={onAll}
            className="-mr-2 min-h-11 rounded-sm px-2 text-meta font-semibold text-accent focus-visible:outline-2 focus-visible:outline-accent"
          >
            All
          </button>
        )}
      </div>
      {children}
    </fieldset>
  );
}

interface TypeToggleProps extends SectionProps {
  type: EventType;
}

function TypeToggle({ type, query, onChange }: TypeToggleProps): ReactElement {
  const on = query.types.has(type);
  const last = on && query.types.size === 1;
  const toggle = useCallback(() => {
    // The last type can't be turned off: a map filtered to nothing helps no one.
    if (last) {
      return;
    }
    const types = new Set(query.types);
    if (on) {
      types.delete(type);
    } else {
      types.add(type);
    }
    onChange({ ...query, types });
  }, [last, on, type, query, onChange]);
  const token = hazardTypeTokens[type];
  return (
    <button
      type="button"
      aria-pressed={on}
      title={last ? "The last type can’t be turned off" : undefined}
      onClick={toggle}
      className={cn(TILE, "min-h-16", on ? TILE_ON : TILE_OFF)}
    >
      <Icon name={token.icon} aria-hidden className="size-5" />
      <span className="text-micro">{token.label}</span>
    </button>
  );
}

/** Every hazard type as a toggle grid; conflicts are their own layer (ADR 27), never here. */
function TypeSection({ query, onChange }: SectionProps): ReactElement {
  const all = useCallback(() => {
    onChange({ ...query, types: new Set(HAZARD_TYPES) });
  }, [query, onChange]);
  return (
    <Section title="Hazard type" onAll={query.types.size < HAZARD_TYPES.length ? all : undefined}>
      <div className="grid grid-cols-[repeat(auto-fill,minmax(6rem,1fr))] gap-2">
        {HAZARD_TYPES.map((type) => (
          <TypeToggle key={type} type={type} query={query} onChange={onChange} />
        ))}
      </div>
    </Section>
  );
}

/** Four steps, each with the severity's own shape and colour (DESIGN.md §4), over native radios. */
function SeveritySection({ query, onChange }: SectionProps): ReactElement {
  const name = useId();
  const choose = useCallback(
    (event: ChangeEvent<HTMLInputElement>) => {
      const level = SEVERITY_ORDER.find((each) => each === event.target.value);
      if (level !== undefined) {
        onChange({ ...query, minimumSeverity: level });
      }
    },
    [query, onChange],
  );
  return (
    <Section title="Minimum severity">
      <div className="grid grid-cols-4 gap-2">
        {SEVERITY_ORDER.map((level) => {
          const selected = query.minimumSeverity === level;
          const token = severityTokens[level];
          return (
            <label key={level} className={cn(TILE, "min-h-13 cursor-pointer", selected ? TILE_ON : TILE_OFF)}>
              <input
                type="radio"
                name={name}
                value={level}
                checked={selected}
                onChange={choose}
                aria-label={level === "minor" ? "All severities" : `${token.label} and above`}
                className="sr-only"
              />
              <Icon
                name={token.icon}
                aria-hidden
                fill="currentColor"
                className={cn(
                  "size-3.5",
                  isAtLeast(level, query.minimumSeverity) ? SEVERITY_TEXT[level] : "text-muted",
                )}
              />
              <span className={cn("text-meta", selected && "font-semibold")}>
                {level === "minor" ? "All" : `${token.label}+`}
              </span>
            </label>
          );
        })}
      </div>
    </Section>
  );
}

interface KindRowProps extends SectionProps {
  kind: EventKind;
}

function KindRow({ kind, query, onChange }: KindRowProps): ReactElement {
  const on = query.kinds.has(kind);
  const toggle = useCallback(() => {
    const kinds = new Set(query.kinds);
    if (on) {
      kinds.delete(kind);
    } else {
      kinds.add(kind);
    }
    onChange({ ...query, kinds });
  }, [on, kind, query, onChange]);
  // The last kind stays on.
  return (
    <Switch
      label={kindTokens[kind].label}
      icon={KIND_ICONS[kind]}
      checked={on}
      disabled={on && query.kinds.size === 1}
      onToggle={toggle}
    />
  );
}

/** Satellite heat detections are the map's FIRMS layer, not events: the same switch as the map's control. */
function DetectionRow(): ReactElement {
  const { showsDetections } = useSavedFilters();
  return (
    <Switch
      label={kindTokens.detection.label}
      icon={KIND_ICONS.detection}
      checked={showsDetections}
      disabled={false}
      onToggle={toggleShowsDetections}
    />
  );
}

function KindSection({ query, onChange }: SectionProps): ReactElement {
  return (
    <Section title="Kind">
      <div className="divide-y divide-hairline rounded-md bg-ink/3 px-3">
        {EVENT_KINDS.map((kind) => (
          <KindRow key={kind} kind={kind} query={query} onChange={onChange} />
        ))}
        <DetectionRow />
      </div>
    </Section>
  );
}

function WindowSection({ query, onChange }: SectionProps): ReactElement {
  const name = useId();
  const choose = useCallback(
    (event: ChangeEvent<HTMLInputElement>) => {
      onChange({ ...query, windowSeconds: Number(event.target.value) });
    },
    [query, onChange],
  );
  return (
    <Section title="Updated in the last">
      <div className="grid grid-cols-4 gap-1 rounded-md bg-ink/5 p-1">
        {WINDOWS.map((seconds) => {
          const selected = query.windowSeconds === seconds;
          return (
            <label
              key={seconds}
              className={cn(
                TILE,
                "min-h-11 cursor-pointer border-transparent text-meta",
                selected ? "bg-raised font-semibold text-ink" : "text-muted",
              )}
            >
              <input
                type="radio"
                name={name}
                value={seconds}
                checked={selected}
                onChange={choose}
                className="sr-only"
              />
              {windowLabel(seconds)}
            </label>
          );
        })}
      </div>
    </Section>
  );
}

/** Active events always show; this adds the resolved ones, faded with a checkmark. */
function StatusSection({ query, onChange }: SectionProps): ReactElement {
  const toggle = useCallback(() => {
    onChange({ ...query, includeResolved: !query.includeResolved });
  }, [query, onChange]);
  return (
    <Section title="Status">
      <div className="rounded-md bg-ink/3 px-3">
        <Switch label="Include resolved" icon="circle-check" checked={query.includeResolved} onToggle={toggle} />
      </div>
      <p className="text-meta text-muted">Adds events that ended or were cancelled in the window, shown faded.</p>
    </Section>
  );
}

export { KindSection, SeveritySection, StatusSection, TypeSection, WindowSection };
