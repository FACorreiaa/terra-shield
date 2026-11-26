import { z } from "zod";

import { readStored, writeStored } from "@/lib/storage.ts";

/** The stream's cursor, kept across visits so a returning reader resumes where they left off. */
const KEY = "shield.live";
const VERSION = 1;
const SCHEMA = z.object({ cursor: z.string().regex(/^\d+$/u).optional() });
/** A burst (a source's first load) moves the cursor thousands of times a second; storage hears it at most once a second. */
const WRITE_EVERY_MS = 1000;

interface CursorStore {
  read: () => string | undefined;
  /** Moves the cursor forward; an older one (a deposed leader's late write) is ignored. */
  save: (cursor: string | undefined) => void;
  /** Forgets the cursor now (refused, or from before a database reset). */
  drop: () => void;
}

/** Whether `cursor` comes after `than` (none comes after nothing). Cursors are decimal strings of any length. */
function isAfter(cursor: string | undefined, than: string | undefined): boolean {
  if (cursor === undefined) {
    return false;
  }
  if (than === undefined) {
    return true;
  }
  return cursor.length === than.length ? cursor > than : cursor.length > than.length;
}

function stored(): string | undefined {
  return readStored(KEY, VERSION, SCHEMA, {}).cursor;
}

/**
 * The cursor in `localStorage` (`shield.live` v1), shared by the tabs: validated on read, which comes
 * at each connection (another tab may have led since), and written at most once a second.
 */
function cursorStore(): CursorStore {
  let current: string | undefined = undefined;
  let lastWrite = 0;
  let pending: ReturnType<typeof setTimeout> | undefined = undefined;
  const write = (): void => {
    pending = undefined;
    lastWrite = Date.now();
    // Another tab may have led since and moved it further: never move it back.
    if (isAfter(current, stored())) {
      writeStored(KEY, VERSION, { cursor: current });
    }
  };
  return {
    // A write still waiting holds the newest cursor; else storage does.
    read: () => (pending === undefined ? stored() : current),
    drop: () => {
      clearTimeout(pending);
      pending = undefined;
      current = undefined;
      writeStored(KEY, VERSION, {});
    },
    save: (cursor) => {
      current = cursor;
      if (pending !== undefined) {
        return;
      }
      const wait = lastWrite + WRITE_EVERY_MS - Date.now();
      if (wait <= 0) {
        write();
      } else {
        pending = setTimeout(write, wait);
      }
    },
  };
}

export type { CursorStore };
export { cursorStore, isAfter };
