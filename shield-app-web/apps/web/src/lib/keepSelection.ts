/** A link to another section keeps the selected event open beside it, as on the Mac. */
function keepSelection(previous: { event?: string | undefined }): { event?: string | undefined } {
  return { event: previous.event };
}

export { keepSelection };
