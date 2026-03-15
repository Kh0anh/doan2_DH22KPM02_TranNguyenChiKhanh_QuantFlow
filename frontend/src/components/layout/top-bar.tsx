"use client";

/**
 * top-bar.tsx — Sticky application top bar (Task 3.1.2, wired in 3.1.3).
 *
 * Left:  hamburger (mobile-only) + page title from pathname
 * Right: username from AuthContext + logout button
 *
 * WBS 3.1.2 / 3.1.3 · project_structure.md §3
 */

import { usePathname, useRouter } from "next/navigation";
import { LogOut, Menu, TrendingUp } from "lucide-react";
import { cn } from "@/lib/utils";
import { useAuth } from "@/lib/auth";

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
  const router = useRouter();
  const { user, logout } = useAuth();
  const title = getPageTitle(pathname);

  async function handleLogout() {
    await logout();
    router.replace("/login");
  }

  return (
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

      {/* ── Right — User info + Logout ── */}
      <div className="flex items-center gap-2">
        {user && (
          <span className="hidden sm:block text-xs text-muted-foreground">
            {user.username}
          </span>
        )}

        <button
          onClick={handleLogout}
          className="flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
          title="Sign out"
          aria-label="Sign out"
        >
          <LogOut className="h-4 w-4" />
        </button>
      </div>
    </header>
  );
}
