import type { ReactElement, RefObject } from "react";

import { SHORTCUT_HELP } from "./shortcuts.ts";

/**
 * The `?` sheet: every shortcut on one card. A native modal `<dialog>`, so focus stays inside and
 * Escape closes it.
 */
function ShortcutsHelp({ dialog }: { dialog: RefObject<HTMLDialogElement | null> }): ReactElement {
  return (
    <dialog
      ref={dialog}
      aria-labelledby="shortcuts-title"
      className="m-auto w-[min(26rem,calc(100%-2rem))] rounded-lg border border-hairline-strong bg-panel p-0 text-ink shadow-floating backdrop:bg-canvas/60"
    >
      <form method="dialog" className="flex flex-col gap-4 p-5">
        <h2 id="shortcuts-title" className="text-title">
          Keyboard shortcuts
        </h2>
        <dl className="grid grid-cols-[auto_1fr] items-baseline gap-x-4 gap-y-2.5">
          {SHORTCUT_HELP.map((entry) => (
            <div key={entry.keys} className="contents">
              <dt>
                <kbd className="rounded-xs border border-hairline-strong bg-raised px-1.5 py-0.5 font-mono text-meta text-ink">
                  {entry.keys}
                </kbd>
              </dt>
              <dd className="text-body text-muted">{entry.does}</dd>
            </div>
          ))}
        </dl>
        <p className="text-meta text-muted">Shortcuts work when you are not typing in a field.</p>
        <button
          type="submit"
          className="h-11 self-end rounded-sm px-4 text-body font-semibold text-accent hover:bg-ink/8 focus-visible:outline-2 focus-visible:outline-accent"
        >
          Done
        </button>
      </form>
    </dialog>
  );
}

export { ShortcutsHelp };
