import { follow } from "./connect.ts";
import { cursorStore } from "./cursorStore.ts";
import { FeedNode } from "./feedNode.ts";

const CHANNEL = "shield-live";

let node: FeedNode | undefined = undefined;

/**
 * The page's live feed, started on first use and kept for the page's life: one stream per browser,
 * shared by every tab.
 */
function liveFeed(): FeedNode {
  if (node === undefined) {
    const shared = typeof navigator.locks === "object" && typeof BroadcastChannel === "function";
    node = new FeedNode({
      locks: shared ? navigator.locks : undefined,
      channel: shared ? new BroadcastChannel(CHANNEL) : undefined,
      follow,
      cursor: cursorStore(),
      now: () => Date.now(),
      visible: () => document.visibilityState === "visible",
    });
    node.start();
  }
  return node;
}

export type { FeedStatus } from "./feedNode.ts";
export { liveFeed };
