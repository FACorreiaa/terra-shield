import type { ReactElement } from "react";

import { resultWords } from "@/domain/search.ts";
import { SearchField } from "@/features/search/SearchField.tsx";

import { useMapSearch } from "./mapSearchContext.ts";
import { useSearchMatches } from "./useSearchMatches.ts";

/**
 * The search field, wired to the shared search; `cancel` on a phone. The result count is spoken
 * from a status that stays mounted with the field, so the first count is heard too.
 */
function MapSearchBox({ cancel = false }: { cancel?: boolean }): ReactElement {
  const { text, setText, editing, setEditing, end, field } = useMapSearch();
  const { countries, events, eventsLoaded } = useSearchMatches();
  const count = text.trim() === "" || !eventsLoaded ? "" : resultWords(countries.length + events.length);
  return (
    <>
      <SearchField
        text={text}
        editing={editing}
        inputRef={field}
        cancel={cancel}
        onText={setText}
        onEditing={setEditing}
        onEnd={end}
      />
      <output className="sr-only">{count}</output>
    </>
  );
}

export { MapSearchBox };
