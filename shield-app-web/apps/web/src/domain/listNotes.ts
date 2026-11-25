/**
 * What the event list says it leaves out (EventList.swift `footnote`): the events the server sent as
 * clusters, and a response the server cut short. The list never looks more complete than it is.
 */

/** "3 events are also grouped on the map. Zoom in to list them." and the truncation line, as needed. */
function listFootnote({
  clustered,
  listed,
  truncated,
}: {
  clustered: number;
  listed: number;
  truncated: boolean;
}): string[] {
  const lines: string[] = [];
  if (clustered > 0) {
    const events = clustered === 1 ? "1 event is" : `${clustered} events are`;
    lines.push(
      listed === 0
        ? `${events} grouped on the map. Zoom in to list them.`
        : `${events} also grouped on the map. Zoom in to list them.`,
    );
  }
  if (truncated) {
    lines.push("Showing the most severe events here. Zoom in to see them all.");
  }
  return lines;
}

/** The empty view's words: with filters on, the filters may be what hides events (DESIGN.md §15). */
function emptyWords(filtered: boolean): { title: string; note: string } {
  return {
    title: filtered
      ? "No events in this view match your filters."
      : "No active events in this view from covered sources.",
    note: "Absence of data is not safety. Official warnings come from national services.",
  };
}

export { emptyWords, listFootnote };
