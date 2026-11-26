import { describe, expect, it } from "vite-plus/test";

import { makeEvent } from "@/domain/testEvents.ts";

import { drawnEvents, knownEvents } from "./layerEvents.ts";

const QUAKE = makeEvent({ id: "quake", type: "earthquake" });
const CONFLICT = makeEvent({ id: "conflict", type: "conflict" });
const LINKED = makeEvent({ id: "linked", type: "flood" });

describe(knownEvents, () => {
  it("lists hazards, then conflicts", () => {
    expect(knownEvents([QUAKE], [CONFLICT]).map((event) => event.id)).toStrictEqual(["quake", "conflict"]);
  });
});

describe(drawnEvents, () => {
  const known = [QUAKE];

  it("adds a selection that isn't in view, unless its source dropped it", () => {
    expect(drawnEvents(known, LINKED, { gone: false, conflictsShown: false })).toHaveLength(2);
    expect(drawnEvents(known, LINKED, { gone: true, conflictsShown: false })).toHaveLength(1);
  });

  it("draws no selected conflict while the conflicts layer is off", () => {
    expect(drawnEvents(known, CONFLICT, { gone: false, conflictsShown: false })).toStrictEqual(known);
    expect(drawnEvents(known, CONFLICT, { gone: false, conflictsShown: true })).toHaveLength(2);
  });
});
