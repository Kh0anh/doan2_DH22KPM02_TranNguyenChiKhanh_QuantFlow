/**
 * [3.1.2] TopBar — 48px application header.
 * [3.1.3] Username and logout sourced from AuthContext (useAuth).
 *
 * Left:  QuantFlow logo mark + wordmark
 * Right: User dropdown (Avatar initials + username + Settings + Logout)
 *
 * Logout: delegates to logout() from AuthContext
 * Settings: Calls openSettings() from useUIStore → opens Settings Dialog (Task 3.1.4)
 */
"use client";

import { useState } from "react";
import { Settings, LogOut, Loader2 } from "lucide-react";
import { useAuth } from "@/lib/auth";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { useUIStore } from "@/store/ui-store";
import { ConnectionIndicator } from "@/components/shared/connection-indicator";
import { useSystemAlerts } from "@/lib/hooks/use-system-alerts";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Get initials from a username string (up to 2 chars). */
function getInitials(name: string): string {
  const parts = name.trim().split(/\s+/);
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}

// ---------------------------------------------------------------------------
// TopBar component
// ---------------------------------------------------------------------------

export function TopBar() {
  const { user, logout } = useAuth();
  const openSettings = useUIStore((s) => s.openSettings);
  const [isLoggingOut, setIsLoggingOut] = useState(false);

  // Mount global system alerts (Task 3.4.6)
  useSystemAlerts();

  const username = user?.username ?? "";

  async function handleLogout() {
    if (isLoggingOut) return;
    setIsLoggingOut(true);
    try {
      await logout();
    } finally {
      setIsLoggingOut(false);
    }
  }

  return (
    <header className="flex h-12 shrink-0 items-center justify-between border-b border-border bg-card px-4">
      {/* ── Logo ── */}
      <div className="flex items-center gap-2">
        <div className="flex size-7 items-center justify-center rounded-md bg-primary/10">
          <svg
            viewBox="0 0 24 24"
            className="size-4 fill-primary"
            aria-hidden="true"
          >
            <path d="M2 2h9v9H2V2zm11 0h9v9h-9V2zM2 13h9v9H2v-9zm13 4a4 4 0 1 1 0-8 4 4 0 0 1 0 8z" />
          </svg>
        </div>
        <span className="text-sm font-semibold tracking-tight text-foreground">
          QuantFlow
        </span>
      </div>

      {/* ── Right side: Connection indicator + User dropdown ── */}
      <div className="flex items-center gap-3">
        <ConnectionIndicator />
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            className="flex items-center gap-2 rounded-md px-2 py-1 text-sm transition-colors hover:bg-secondary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            aria-label="Menu người dùng"
          >
            <Avatar size="sm">
              <AvatarFallback className="bg-primary/15 text-primary text-xs font-medium">
                {getInitials(username)}
              </AvatarFallback>
            </Avatar>
            <span className="hidden text-[13px] font-medium text-foreground sm:block">
              {username}
            </span>
          </button>
        </DropdownMenuTrigger>

        <DropdownMenuContent align="end" className="w-48">
          <DropdownMenuLabel className="text-xs text-muted-foreground font-normal">
            Đăng nhập với
            <span className="block font-medium text-foreground truncate">
              {username}
            </span>
          </DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuGroup>
            <DropdownMenuItem onClick={() => openSettings()}>
              <Settings aria-hidden="true" />
              Cài đặt
            </DropdownMenuItem>
          </DropdownMenuGroup>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            variant="destructive"
            onClick={handleLogout}
            disabled={isLoggingOut}
          >
            {isLoggingOut ? (
              <Loader2 className="size-4 animate-spin" aria-hidden="true" />
            ) : (
              <LogOut aria-hidden="true" />
            )}
            Đăng xuất
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      </div>
    </header>
  );
}
