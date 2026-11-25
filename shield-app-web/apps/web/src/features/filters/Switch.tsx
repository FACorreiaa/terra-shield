import type { ReactElement } from "react";

import { Icon } from "@/design/icons.ts";

interface SwitchProps {
  label: string;
  icon: string;
  checked: boolean;
  disabled?: boolean;
  onToggle: () => void;
}

/** Label left, switch right (FilterSheet.swift `switchRow`), over a native checkbox with the switch role. */
function Switch({ label, icon, checked, disabled = false, onToggle }: SwitchProps): ReactElement {
  return (
    <label className="flex min-h-11 cursor-pointer items-center gap-3 py-1 has-[:disabled]:cursor-default">
      <Icon name={icon} aria-hidden className="size-4 shrink-0 text-muted" />
      <span className="flex-1 text-body text-ink">{label}</span>
      <input
        type="checkbox"
        role="switch"
        aria-checked={checked}
        checked={checked}
        disabled={disabled}
        onChange={onToggle}
        className="relative h-6 w-10 shrink-0 cursor-pointer appearance-none rounded-pill bg-hairline-strong transition-colors before:absolute before:top-0.5 before:left-0.5 before:size-5 before:rounded-full before:bg-ink before:transition-transform checked:bg-accent checked:before:translate-x-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent disabled:cursor-default disabled:opacity-50 motion-reduce:transition-none motion-reduce:before:transition-none"
      />
    </label>
  );
}

export { Switch };
