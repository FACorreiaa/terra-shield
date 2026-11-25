import { Link2 } from "lucide-react";
import type { ReactElement } from "react";
import { useCallback, useEffect, useState } from "react";

import { DisclaimerBanner } from "@/components/DisclaimerBanner.tsx";

/** How long "Link copied" stays. */
const COPIED_MS = 2500;

type Copy = "idle" | "copied" | "failed";

const COPY_WORDS: Readonly<Record<Copy, string>> = {
  idle: "",
  copied: "Link copied",
  failed: "The link couldn’t be copied.",
};

async function copied(link: string): Promise<Copy> {
  try {
    await navigator.clipboard.writeText(link);
    return "copied";
  } catch {
    return "failed";
  }
}

/** The disclaimer, then Share: the event's own address on Aegis Terra, `/e/<id>`, copied. */
function DetailFooter({ eventId }: { eventId: string }): ReactElement {
  const [copy, setCopy] = useState<Copy>("idle");
  const share = useCallback(() => {
    void (async () => {
      setCopy(await copied(`${globalThis.location.origin}/e/${eventId}`));
    })();
  }, [eventId]);
  useEffect(() => {
    if (copy === "idle") {
      return undefined;
    }
    const timer = setTimeout(() => {
      setCopy("idle");
    }, COPIED_MS);
    return () => {
      clearTimeout(timer);
    };
  }, [copy]);
  return (
    <footer className="flex flex-col gap-3">
      <DisclaimerBanner variant="footer" />
      <div className="flex flex-wrap items-center gap-3">
        <button
          type="button"
          onClick={share}
          className="inline-flex min-h-11 items-center gap-2 rounded-sm px-1 text-body font-semibold text-accent hover:underline focus-visible:outline-2 focus-visible:outline-accent"
        >
          <Link2 className="size-4" aria-hidden />
          Share
        </button>
        <output className="text-meta text-muted">{COPY_WORDS[copy]}</output>
      </div>
    </footer>
  );
}

export { DetailFooter };
