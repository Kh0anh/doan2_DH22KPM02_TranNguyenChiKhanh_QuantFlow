"use client";

/**
 * sidebar.tsx — Main navigation sidebar (Task 3.1.2).
 *
 * Behaviour:
 * - Desktop: icon-strip (w-14) by default; expands to w-56 on toggle.
 * - Mobile: hidden by default; slides in as a fixed overlay when isOpen=true.
 *
 * Nav items:
 *   /trading    — BarChart2   (Trading & Monitoring)
 *   /strategies — Layers      (Strategy List)
 *   /editor/new — Code2       (Strategy Editor)
 *
 * Active state derived from usePathname().
 * Collapsed icon labels shown via radix-ui Tooltip.
 *
 * WBS 3.1.2 · project_structure.md §3
 */

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  BarChart2,
  ChevronLeft,
  ChevronRight,
  Code2,
  Layers,
  TrendingUp,
} from "lucide-react";
import { cn } from "@/lib/utils";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

// ─── Nav definition ──────────────────────────────────────────────────────────

const NAV_ITEMS = [
  {
    href: "/trading",
    label: "Trading",
    icon: BarChart2,
    matchPrefix: "/trading",
  },
  {
    href: "/strategies",
    label: "Strategies",
    icon: Layers,
    matchPrefix: "/strategies",
  },
  {
    href: "/editor/new",
    label: "Editor",
    icon: Code2,
    matchPrefix: "/editor",
  },
] as const;

// ─── Props ────────────────────────────────────────────────────────────────────

interface SidebarProps {
  /** Desktop collapse state (true = expanded w-56, false = icon-only w-14) */
  expanded: boolean;
  /** Mobile open state (true = overlay visible) */
  isOpen: boolean;
  onToggleExpanded: () => void;
  onClose: () => void;
}

// ─── Component ────────────────────────────────────────────────────────────────

export function Sidebar({
  expanded,
  isOpen,
  onToggleExpanded,
  onClose,
}: SidebarProps) {
  const pathname = usePathname();

  const NavContent = (
    <nav className="flex flex-col gap-1 flex-1 px-2 py-3">
      {NAV_ITEMS.map(({ href, label, icon: Icon, matchPrefix }) => {
        const isActive = pathname.startsWith(matchPrefix);

        const linkEl = (
          <Link
            key={href}
            href={href}
            onClick={onClose}
            className={cn(
              "flex items-center gap-3 rounded-md px-2 py-2 text-sm font-medium transition-colors",
              "hover:bg-accent hover:text-accent-foreground",
              isActive
                ? "bg-primary/10 text-primary"
                : "text-muted-foreground"
            )}
          >
            <Icon className="shrink-0 h-5 w-5" />
            {/* Label: always shown on mobile overlay; on desktop shown when expanded */}
            <span
              className={cn(
                "truncate transition-all duration-200",
                expanded ? "opacity-100 w-auto" : "opacity-0 w-0 overflow-hidden md:hidden"
              )}
            >
              {label}
            </span>
          </Link>
        );

        // Wrap with Tooltip only on desktop collapsed mode
        if (!expanded) {
          return (
            <Tooltip key={href}>
              <TooltipTrigger asChild>{linkEl}</TooltipTrigger>
              <TooltipContent side="right">{label}</TooltipContent>
            </Tooltip>
          );
        }

        return linkEl;
      })}
    </nav>
  );

  // ── Desktop sidebar ─────────────────────────────────────────────────────────
  const desktopSidebar = (
    <aside
      className={cn(
        "hidden md:flex flex-col h-full border-r border-border bg-card transition-all duration-200 shrink-0",
        expanded ? "w-56" : "w-14"
      )}
    >
      {/* Logo row */}
      <div
        className={cn(
          "h-12 flex items-center border-b border-border shrink-0 px-2",
          expanded ? "justify-between" : "justify-center"
        )}
      >
        {expanded && (
          <div className="flex items-center gap-2 pl-1">
            <TrendingUp className="h-5 w-5 text-primary" />
            <span className="text-sm font-semibold text-foreground">QuantFlow</span>
          </div>
        )}
        {/* Collapse / expand toggle */}
        <button
          onClick={onToggleExpanded}
          className="flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
          aria-label={expanded ? "Collapse sidebar" : "Expand sidebar"}
        >
          {expanded ? (
            <ChevronLeft className="h-4 w-4" />
          ) : (
            <ChevronRight className="h-4 w-4" />
          )}
        </button>
      </div>

      {NavContent}
    </aside>
  );

  // ── Mobile overlay sidebar ──────────────────────────────────────────────────
  const mobileSidebar = (
    <>
      {/* Backdrop */}
      {isOpen && (
        <div
          className="fixed inset-0 z-20 bg-background/80 backdrop-blur-sm md:hidden"
          onClick={onClose}
          aria-hidden
        />
      )}

      {/* Drawer */}
      <aside
        className={cn(
          "fixed inset-y-0 left-0 z-30 flex flex-col w-56 bg-card border-r border-border",
          "transform transition-transform duration-200 md:hidden",
          isOpen ? "translate-x-0" : "-translate-x-full"
        )}
      >
        {/* Logo row */}
        <div className="h-12 flex items-center gap-2 px-4 border-b border-border shrink-0">
          <TrendingUp className="h-5 w-5 text-primary" />
          <span className="text-sm font-semibold text-foreground">QuantFlow</span>
        </div>
        {NavContent}
      </aside>
    </>
  );

  return (
    <>
      {desktopSidebar}
      {mobileSidebar}
    </>
  );
}
