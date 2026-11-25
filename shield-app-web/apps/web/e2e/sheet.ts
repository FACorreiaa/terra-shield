import type { Locator, Page } from "@playwright/test";

import { expect } from "./fixtures.ts";

/** Waits until the element stops moving: two polls in a row at the same height. */
async function settled(element: Locator): Promise<void> {
  let last = Number.NaN;
  await expect
    .poll(async () => {
      const box = await element.boundingBox();
      const top = box?.y ?? Number.NaN;
      const still = top === last;
      last = top;
      return still;
    })
    .toBe(true);
}

/**
 * Lifts the phone sheet from collapsed to half height and waits for the spring to settle, so the
 * next tap lands on what it aims at rather than where the sheet was a frame ago. It uses the
 * handle's Enter rather than a tap: the handle's own tap has its test, and on a loaded machine a
 * synthetic touch here was the one flaky step.
 */
async function openHalf(page: Page): Promise<void> {
  const handle = page.getByRole("button", { name: /^Resize sheet/u });
  await handle.press("Enter");
  await expect(handle).toHaveAccessibleName("Resize sheet, now half height");
  await settled(handle);
}

export { openHalf, settled };
