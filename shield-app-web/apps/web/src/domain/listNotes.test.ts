import { describe, expect, it } from "vite-plus/test";

import { emptyWords, listFootnote } from "./listNotes.ts";

describe(listFootnote, () => {
  it("counts the clustered events, with or without listed ones", () => {
    expect(listFootnote({ clustered: 1, listed: 0, truncated: false })).toStrictEqual([
      "1 event is grouped on the map. Zoom in to list them.",
    ]);
    expect(listFootnote({ clustered: 12, listed: 4, truncated: false })).toStrictEqual([
      "12 events are also grouped on the map. Zoom in to list them.",
    ]);
  });

  it("says when the server cut the list short, and nothing when the list is whole", () => {
    expect(listFootnote({ clustered: 0, listed: 9, truncated: true })).toStrictEqual([
      "Showing the most severe events here. Zoom in to see them all.",
    ]);
    expect(listFootnote({ clustered: 0, listed: 9, truncated: false })).toStrictEqual([]);
  });
});

describe(emptyWords, () => {
  it("blames the filters first when they are on", () => {
    expect(emptyWords(true).title).toBe("No events in this view match your filters.");
    expect(emptyWords(false).title).toBe("No active events in this view from covered sources.");
  });
});
