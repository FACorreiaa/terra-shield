const DIACRITICS = /\p{Diacritic}/gu;

/** Lower case without accents, so "Côte" matches "cote" (Swift's `localizedStandardContains`). */
function fold(text: string): string {
  return text.normalize("NFD").replaceAll(DIACRITICS, "").toLowerCase();
}

export { fold };
