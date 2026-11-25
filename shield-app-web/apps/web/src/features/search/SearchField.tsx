import { Search, X } from "lucide-react";
import type { ChangeEvent, KeyboardEvent, ReactElement, RefObject, SyntheticEvent } from "react";
import { useCallback, useEffect } from "react";

import { SEARCH_WORDS } from "@/domain/search.ts";

interface SearchFieldProps {
  text: string;
  editing: boolean;
  inputRef: RefObject<HTMLInputElement | null>;
  /** Shows Cancel while editing, as on iPhone; elsewhere Escape ends the search. */
  cancel: boolean;
  onText: (text: string) => void;
  onEditing: (editing: boolean) => void;
  onEnd: () => void;
}

/**
 * Keeps focus in the field: a press on Clear or Cancel would otherwise blur it before the click
 * lands. On both pointer and mouse down, since Safari moves focus on the latter.
 */
function keepFocus(event: SyntheticEvent): void {
  event.preventDefault();
}

/** Focus and blur report editing; so does a field removed while focused (the shell changed), which sends no blur. */
function useEditing(onEditing: (editing: boolean) => void): { focus: () => void; blur: () => void } {
  const focus = useCallback(() => {
    onEditing(true);
  }, [onEditing]);
  const blur = useCallback(() => {
    onEditing(false);
  }, [onEditing]);
  useEffect(
    () => () => {
      onEditing(false);
    },
    [onEditing],
  );
  return { focus, blur };
}

/**
 * The search field (MapSearch.swift `MapSearchField`): magnifier, text, Clear and, on a phone while
 * editing, Cancel. The whole box is a label, so a press anywhere in it puts the cursor in the text.
 */
function SearchField({ text, editing, inputRef, cancel, onText, onEditing, onEnd }: SearchFieldProps): ReactElement {
  const change = useCallback(
    (event: ChangeEvent<HTMLInputElement>) => {
      onText(event.target.value);
    },
    [onText],
  );
  const { focus, blur } = useEditing(onEditing);
  const keys = useCallback(
    (event: KeyboardEvent<HTMLInputElement>) => {
      if (event.key === "Escape") {
        event.preventDefault();
        onEnd();
      }
    },
    [onEnd],
  );
  const clear = useCallback(() => {
    onText("");
    inputRef.current?.focus();
  }, [onText, inputRef]);

  return (
    <div className="flex items-center gap-2">
      <label
        data-search-field
        className="flex h-11 min-w-0 flex-1 cursor-text items-center gap-2 rounded-sm bg-ink/8 pl-3 text-muted focus-within:outline-2 focus-within:outline-accent"
      >
        <Search aria-hidden className="size-4 shrink-0" />
        {/* 1rem: iOS zooms the page into any field set smaller. */}
        <input
          ref={inputRef}
          type="search"
          value={text}
          onChange={change}
          onFocus={focus}
          onBlur={blur}
          onKeyDown={keys}
          aria-label={SEARCH_WORDS.placeholder}
          placeholder={SEARCH_WORDS.placeholder}
          enterKeyHint="search"
          autoComplete="off"
          autoCapitalize="none"
          spellCheck={false}
          className="h-full min-w-0 flex-1 bg-transparent pr-3 text-[1rem] text-ink outline-none select-text placeholder:text-muted [&::-webkit-search-cancel-button]:appearance-none"
        />
        {text === "" ? undefined : (
          <button
            type="button"
            aria-label="Clear search"
            onPointerDown={keepFocus}
            onMouseDown={keepFocus}
            onClick={clear}
            className="-ml-3 flex size-11 shrink-0 items-center justify-center rounded-sm hover:text-ink focus-visible:outline-2 focus-visible:outline-accent"
          >
            <X aria-hidden className="size-4" />
          </button>
        )}
      </label>
      {cancel && editing ? (
        <button
          type="button"
          onPointerDown={keepFocus}
          onMouseDown={keepFocus}
          onClick={onEnd}
          className="min-h-11 shrink-0 rounded-sm px-1 text-body text-accent focus-visible:outline-2 focus-visible:outline-accent"
        >
          Cancel
        </button>
      ) : undefined}
    </div>
  );
}

export { SearchField };
