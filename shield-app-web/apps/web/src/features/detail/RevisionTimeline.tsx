import type { ReactElement } from "react";

import type { EventRevision } from "@/domain/changes.ts";
import { changeSentence, timeline } from "@/domain/changes.ts";
import { stamp } from "@/domain/freshness.ts";

import type { DetailState } from "./detailState.ts";
import { Eyebrow } from "./Eyebrow.tsx";

function Message({ children }: { children: string }): ReactElement {
  return <p className="text-meta text-muted">{children}</p>;
}

const MESSAGES: Readonly<Record<Exclude<DetailState["kind"], "loaded">, string>> = {
  loading: "Loading history…",
  failed: "The history couldn’t be loaded.",
  gone: "No history: the source no longer publishes this event.",
};

function Revisions({ revisions, now }: { revisions: readonly EventRevision[]; now: Date }): ReactElement {
  if (revisions.length === 0) {
    return <Message>No changes recorded yet.</Message>;
  }
  const last = revisions.length - 1;
  return (
    <ol className="flex flex-col">
      {revisions.map((revision, index) => (
        <li key={`${revision.at.toISOString()} ${changeSentence(revision.changes, "timeline")}`} className="flex gap-3">
          {/* A dot on a hairline rail; the rail stops at the oldest entry. */}
          <span aria-hidden className="flex w-[7px] shrink-0 flex-col items-center">
            <span className="mt-[7px] size-[7px] rounded-full bg-muted" />
            {index === last ? undefined : <span className="w-px flex-1 bg-hairline-strong" />}
          </span>
          <span className={`flex flex-col gap-0.5 ${index === last ? "" : "pb-3"}`}>
            <span className="text-body text-ink">{changeSentence(revision.changes, "timeline")}</span>
            <span className="font-mono text-meta text-muted">{stamp(revision.at, now)}</span>
          </span>
        </li>
      ))}
    </ol>
  );
}

/** What changed, newest first, each in words with its time (DESIGN.md §10.3 item 7). */
function RevisionTimeline({ state, now }: { state: DetailState; now: Date }): ReactElement {
  return (
    <section aria-label="Timeline" className="flex flex-col gap-3">
      <Eyebrow>Timeline</Eyebrow>
      {state.kind === "loaded" ? (
        <Revisions revisions={timeline(state.detail.revisions)} now={now} />
      ) : (
        <Message>{MESSAGES[state.kind]}</Message>
      )}
    </section>
  );
}

export { RevisionTimeline };
