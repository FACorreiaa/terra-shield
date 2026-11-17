import type { VariantProps } from "class-variance-authority";
import { cva } from "class-variance-authority";
import type { ComponentProps, ReactElement } from "react";

import { cn } from "@/lib/utils.ts";

/** shadcn/ui Button, restyled onto Aegis Terra tokens (no hex, no default palette). */
const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 rounded-sm text-body font-semibold whitespace-nowrap transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      variant: {
        primary: "bg-accent text-on-accent hover:bg-accent/90",
        secondary: "border border-ink/10 bg-raised text-ink hover:bg-raised/80",
        ghost: "text-muted hover:bg-raised hover:text-ink",
      },
      size: {
        sm: "h-8 px-3",
        md: "h-10 px-4",
        icon: "size-9",
      },
    },
    defaultVariants: { variant: "secondary", size: "md" },
  },
);

function Button({
  className,
  variant,
  size,
  type = "button",
  ...props
}: ComponentProps<"button"> & VariantProps<typeof buttonVariants>): ReactElement {
  return <button type={type} className={cn(buttonVariants({ variant, size }), className)} {...props} />;
}

export { Button };
