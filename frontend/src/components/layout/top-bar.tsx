"use client";

/**
 * top-bar.tsx — Sticky application top bar (Task 3.1.2).
 *
 * Left:  hamburger (mobile-only, opens sidebar) + QuantFlow logo + page title
 * Right: user info placeholder (will be wired to AuthContext in task 3.1.3)
 *
 * Page title is derived from pathname so active route is always reflected.
 *
 * WBS 3.1.2 · project_structure.md §3
 */

import { usePathname } from "next/navigation";
import { Menu, TrendingUp, User } from "lucide-react";
import { cn } from "@/lib/utils";

// ─── Route → title map ────────────────────────────────────────────────────────

const ROUTE_TITLES: Record<string, string> = {
  "/trading": "Trading",
  "/strategies": "Strategies",
  "/editor": "Editor",
};

function getPageTitle(pathname: string): string {
  for (const [prefix, title] of Object.entries(ROUTE_TITLES)) {
    if (pathname.startsWith(prefix)) return title;
  }
  return "QuantFlow";
}

// ─── Props ────────────────────────────────────────────────────────────────────

interface TopBarProps {
  /** Called when mobile hamburger is tapped */
  onMenuClick: () => void;
  className?: string;
}

// ─── Component ────────────────────────────────────────────────────────────────

export function TopBar({ onMenuClick, className }: TopBarProps) {
  const pathname = usePathname();
  const title = getPageTitle(pathname);

  return (
    <header
      className={cn(
        "h-12 flex items-center gap-3 px-3 border-b border-border bg-card shrink-0 z-40",
        className
      )}
    >
      {/* ── Left ── */}
      {/* Hamburger: mobile only */}
      <button
        onClick={onMenuClick}
        className="flex md:hidden h-8 w-8 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
        aria-label="Open navigation menu"
      >
        <Menu className="h-5 w-5" />
      </button>

      {/* Logo: visible on mobile (desktop logo is in sidebar) */}
      <div className="flex md:hidden items-center gap-2">
        <TrendingUp className="h-4 w-4 text-primary" />
      </div>

      {/* Page title */}
      <span className="text-sm font-semibold text-foreground">{title}</span>

      {/* ── Spacer ── */}
      <div className="flex-1" />

      {/* ── Right — User placeholder ─────────────────────────────────────────
          Will be enhanced in Task 3.1.3 with AuthContext data.
      ────────────────────────────────────────────────────────────────────── */}
      <div className="flex items-center gap-2">
        <div
          className="flex h-7 w-7 items-center justify-center rounded-full bg-primary/10 text-primary ring-1 ring-primary/30"
          title="User account"
        >
          <User className="h-4 w-4" />
        </div>
      </div>
    </header>
  );
}
