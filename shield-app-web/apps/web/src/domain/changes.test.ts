import { describe, expect, it } from "vite-plus/test";
import { z } from "zod";

import type { ChangeStyle } from "./changes.ts";
import { changeSentence, notableChanges, parseChanges, timeline } from "./changes.ts";

const AT = "2026-11-16T14:20:00Z";
const changesObject = z.record(z.string(), z.unknown());

function words(json: string, style: ChangeStyle = "summary"): string {
  const object = changesObject.parse(JSON.parse(json));
  return changeSentence(parseChanges(object), style);
}

describe(parseChanges, () => {
  it("reads a real revision in reading order", () => {
    // From the local API, HR, 2026-11-19: a flood that came back.
    const object = changesObject.parse(
      JSON.parse(
        '{"endedAt": {"from": "2026-11-14T01:00:00Z", "to": null}, "geometry": {"changed": true}, "status": {"from": "resolved", "to": "active"}}',
      ),
    );
    expect(parseChanges(object)).toStrictEqual([
      { field: "status", from: "resolved", to: "active" },
      { field: "geometry" },
      { field: "endedAt", to: undefined },
    ]);
  });

  it("drops unknown or unreadable fields", () => {
    const object = { population: { from: 1, to: 2 }, severity: { from: "minor", to: "catastrophic" } };
    expect(parseChanges(object)).toStrictEqual([]);
  });
});

describe(changeSentence, () => {
  it.each([
    // The end time going away is what "active again" means; it isn't said twice.
    [
      '{"endedAt": {"from": "2026-11-14T01:00:00Z", "to": null}, "geometry": {"changed": true}, "status": {"from": "resolved", "to": "active"}}',
      "Active again · area updated",
    ],
    ['{"severity": {"from": "moderate", "to": "severe"}}', "Raised to Severe"],
    ['{"severity": {"from": "severe", "to": "minor"}}', "Lowered to Minor"],
    ['{"severity": {"from": null, "to": "minor"}}', "Now Minor"],
    ['{"sourceSeverity": {"from": "Yellow warning", "to": "Orange warning"}}', "Source now says “Orange warning”"],
    ['{"sourceSeverity": {"from": "Yellow warning", "to": ""}}', "Source severity cleared"],
    ['{"population": {"from": 1, "to": 2}}', "Updated"],
    ['{"endedAt": {"from": null, "to": "2026-11-19T10:09:07.189239Z"}}', "End time updated"],
    ['{"endedAt": {"from": "2026-11-19T10:09:07Z", "to": null}}', "End time removed"],
    [
      '{"title": {"from": "a", "to": "b"}, "severity": {"from": "minor", "to": "severe"}}',
      "Raised to Severe · renamed",
    ],
  ])("reads a summary of %s as %s", (json, expected) => {
    expect(words(json)).toBe(expected);
  });

  it.each([
    ['{"severity": {"from": "moderate", "to": "severe"}}', "Severity Moderate → Severe"],
    [
      '{"geometry": {"changed": true}, "severity": {"from": "minor", "to": "severe"}}',
      "Severity Minor → Severe · area updated",
    ],
    [
      '{"geometry": {"changed": true}, "sourceSeverity": {"from": "48470 ha", "to": "48484 ha"}}',
      "Source now says “48484 ha” · area updated",
    ],
    [
      '{"status": {"from": "active", "to": "resolved"}, "endedAt": {"from": null, "to": "2026-11-19T10:09:07Z"}}',
      "Resolved",
    ],
    ['{"severity": {"from": null, "to": "minor"}}', "Severity now Minor"],
  ])("reads a timeline entry of %s as %s", (json, expected) => {
    expect(words(json, "timeline")).toBe(expected);
  });
});

describe(notableChanges, () => {
  it("leaves out renames alone", () => {
    const renamed = {
      eventId: "a",
      title: "Orange wind warning · Lička-senjska",
      at: AT,
      changes: { title: { from: "Orange wind warning", to: "Orange wind warning · Lička-senjska" } },
    };
    const raised = {
      eventId: "b",
      title: "Flood",
      at: AT,
      changes: { title: { from: "a", to: "b" }, severity: { from: "minor", to: "severe" } },
    };
    const notable = notableChanges([renamed, raised]);
    expect(notable.map((change) => change.eventId)).toStrictEqual(["b"]);
    expect(notable.map((change) => changeSentence(change.changes))).toStrictEqual(["Raised to Severe · renamed"]);
  });
});

describe(timeline, () => {
  it("leaves out renames and empty revisions", () => {
    const revisions = timeline([
      { at: AT, changes: { title: { from: "a", to: "b" } } },
      { at: "2026-11-16T14:19:00Z", changes: {} },
      { at: "2026-11-16T14:18:00Z", changes: { geometry: { changed: true } } },
    ]);
    expect(revisions.map((revision) => changeSentence(revision.changes, "timeline"))).toStrictEqual(["Area updated"]);
    expect(revisions[0]?.at).toStrictEqual(new Date("2026-11-16T14:18:00Z"));
  });
});
