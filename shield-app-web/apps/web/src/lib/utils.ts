import type { ClassValue } from "clsx";
import { clsx } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

/**
 * tailwind-merge only knows Tailwind's default scale; without this it reads the token sizes
 * (`text-meta`, …) as colours and drops them next to `text-ink`.
 */
const twMerge = extendTailwindMerge({
  extend: {
    classGroups: {
      "font-size": [{ text: ["stat-hero", "title", "headline", "body", "meta", "micro"] }],
    },
  },
});

function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}

export { cn };
