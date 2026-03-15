"use client";

/**
 * top-bar.tsx — Sticky application top bar (Task 3.1.2, wired in 3.1.3/3.1.4).
 *
 * Left:  hamburger (mobile-only) + page title from pathname
 * Right: Settings icon → SettingsDialog | username | LogOut button
 *
 * WBS 3.1.2 / 3.1.3 / 3.1.4 · project_structure.md §3
 */

import { useState } from "react";
import { usePathname } from "next/navigation";
import { LogOut, Menu, Settings, TrendingUp } from "lucide-react";
import { cn } from "@/lib/utils";
import { useAuth } from "@/lib/auth";
import { SettingsDialog } from "@/components/layout/settings-dialog";

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
  onMenuClick: () => void;
  className?: string;
}

// ─── Component ────────────────────────────────────────────────────────────────

export function TopBar({ onMenuClick, className }: TopBarProps) {
  const pathname = usePathname();
  const { user, logout } = useAuth();
  const title = getPageTitle(pathname);

  const [settingsOpen, setSettingsOpen] = useState(false);

  return (
    <>
      <header
        className={cn(
          "h-12 flex items-center gap-3 px-3 border-b border-border bg-card shrink-0 z-40",
          className
        )}
      >
        {/* ── Left ── */}
        <button
          onClick={onMenuClick}
          className="flex md:hidden h-8 w-8 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
          aria-label="Open navigation menu"
        >
          <Menu className="h-5 w-5" />
        </button>

        <div className="flex md:hidden items-center gap-2">
          <TrendingUp className="h-4 w-4 text-primary" />
        </div>

        <span className="text-sm font-semibold text-foreground">{title}</span>

        <div className="flex-1" />

        {/* ── Right — Settings + User + Logout ── */}
        <div className="flex items-center gap-1">
          {user && (
            <span className="hidden sm:block text-xs text-muted-foreground mr-1">
              {user.username}
            </span>
          )}

          {/* Settings button */}
          <button
            onClick={() => setSettingsOpen(true)}
            className="flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
            title="Settings"
            aria-label="Open settings"
          >
            <Settings className="h-4 w-4" />
          </button>

          {/* Logout button */}
          <button
            onClick={() => logout()}
            className="flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
            title="Sign out"
            aria-label="Sign out"
          >
            <LogOut className="h-4 w-4" />
          </button>
        </div>
      </header>

      {/* Settings Dialog */}
      <SettingsDialog open={settingsOpen} onOpenChange={setSettingsOpen} />
    </>
  );
}
