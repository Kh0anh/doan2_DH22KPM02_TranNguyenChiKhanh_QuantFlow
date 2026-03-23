/**
 * QuantFlow Logo — reusable SVG icon component.
 * Shared across login page, top bar, and any future usage.
 */

import { cn } from "@/lib/utils";

interface QuantFlowLogoProps {
  /** Size of the SVG icon (Tailwind class, e.g. "size-4", "size-7") */
  iconSize?: string;
  /** Size of the container wrapper */
  containerSize?: string;
  /** Container border radius class */
  rounded?: string;
  /** Whether to show the "QuantFlow" text next to the icon */
  showText?: boolean;
  /** Text size class */
  textSize?: string;
  /** Additional className for the wrapper */
  className?: string;
}

export function QuantFlowLogo({
  iconSize = "size-4",
  containerSize = "size-7",
  rounded = "rounded-md",
  showText = false,
  textSize = "text-sm",
  className,
}: QuantFlowLogoProps) {
  return (
    <div className={cn("flex items-center gap-2", className)}>
      <div
        className={cn(
          "flex items-center justify-center bg-primary/10",
          containerSize,
          rounded
        )}
      >
        <svg
          viewBox="0 0 24 24"
          className={cn("fill-primary", iconSize)}
          aria-hidden="true"
        >
          <path d="M2 2h9v9H2V2zm11 0h9v9h-9V2zM2 13h9v9H2v-9zm13 4a4 4 0 1 1 0-8 4 4 0 0 1 0 8z" />
        </svg>
      </div>
      {showText && (
        <span
          className={cn(
            "font-semibold tracking-tight text-foreground",
            textSize
          )}
        >
          QuantFlow
        </span>
      )}
    </div>
  );
}
