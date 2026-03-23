/**
 * [3.1.2] Sidebar — Icon-only 60px navigation (VS Code style).
 *
 * Layout:
 *   Top: Navigation icons (Trading, Strategies, New Strategy)
 *   Bottom: Settings icon (opens UIStore settings dialog)
 *
 * Active route: highlighted with accent color + bg-secondary.
 * Tooltips on hover via shadcn/ui Tooltip.
 */
"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { BarChart2, LayoutGrid, Plus, Settings } from "lucide-react";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useUIStore } from "@/store/ui-store";
import { cn } from "@/lib/utils";

// ---------------------------------------------------------------------------
// Nav item definitions
// ---------------------------------------------------------------------------

const NAV_ITEMS = [
  {
    href: "/trading",
    icon: BarChart2,
    label: "Giao dịch",
  },
  {
    href: "/strategies",
    icon: LayoutGrid,
    label: "Chiến lược",
  },
  {
    href: "/editor",
    icon: Plus,
    label: "Trình soạn thảo",
  },
] as const;

// ---------------------------------------------------------------------------
// Sidebar Nav Item
// ---------------------------------------------------------------------------

function SidebarNavItem({
  href,
  icon: Icon,
  label,
  isActive,
}: {
  href: string;
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  isActive: boolean;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Link
          href={href}
          aria-label={label}
          className={cn(
            "flex size-10 items-center justify-center rounded-lg transition-colors",
            isActive
              ? "bg-secondary text-primary"
              : "text-muted-foreground hover:bg-secondary hover:text-foreground"
          )}
        >
          <Icon className="size-[18px]" />
        </Link>
      </TooltipTrigger>
      <TooltipContent side="right" sideOffset={8}>
        {label}
      </TooltipContent>
    </Tooltip>
  );
}

// ---------------------------------------------------------------------------
// Sidebar component
// ---------------------------------------------------------------------------

export function Sidebar() {
  const pathname = usePathname();
  const openSettings = useUIStore((s) => s.openSettings);

  return (
    <aside
      className="flex w-[60px] shrink-0 flex-col items-center border-r border-border bg-card py-3"
      aria-label="Điều hướng chính"
    >
      {/* Top nav icons */}
      <nav className="flex flex-1 flex-col items-center gap-1">
        {NAV_ITEMS.map(({ href, icon, label }) => (
          <SidebarNavItem
            key={href}
            href={href}
            icon={icon}
            label={label}
            isActive={pathname.startsWith(href)}
          />
        ))}
      </nav>

      {/* Bottom: Settings */}
      <div className="flex flex-col items-center gap-1">
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              aria-label="Cài đặt"
              onClick={() => openSettings()}
              className="flex size-10 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
            >
              <Settings className="size-[18px]" />
            </button>
          </TooltipTrigger>
          <TooltipContent side="right" sideOffset={8}>
            Cài đặt
          </TooltipContent>
        </Tooltip>
      </div>
    </aside>
  );
}
