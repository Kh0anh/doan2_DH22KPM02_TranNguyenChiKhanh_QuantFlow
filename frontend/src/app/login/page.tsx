"use client";

/**
 * Login Page — Task 3.1.1 (auth guard wired in 3.1.3)
 *
 * - Form + Brute-force error handling per api.yaml §POST /auth/login
 * - Auth guard: if already authenticated → redirect /trading
 *
 * WBS 3.1.1 / 3.1.3 · SRS FR-ACCESS-01 · api.yaml §Authentication
 */

import { useState, useEffect, FormEvent } from "react";
import { useRouter } from "next/navigation";
import { Loader2, Lock, AlertTriangle, TrendingUp } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { authLogin } from "@/lib/api-client";
import { useAuth } from "@/lib/auth";

// ─── Error state types ────────────────────────────────────────────────────────

type ErrorState =
  | { kind: "none" }
  | { kind: "invalid"; remaining: number }
  | { kind: "locked"; until: Date }
  | { kind: "server" };

// ─── Countdown hook ───────────────────────────────────────────────────────────

function useCountdown(until: Date | null): string | null {
  const [display, setDisplay] = useState<string | null>(null);

  useEffect(() => {
    if (!until) { setDisplay(null); return; }
    const tick = () => {
      const diff = Math.max(0, Math.floor((until.getTime() - Date.now()) / 1000));
      if (diff === 0) { setDisplay(null); return; }
      const m = Math.floor(diff / 60);
      const s = diff % 60;
      setDisplay(m > 0 ? `${m}m ${s}s` : `${s}s`);
    };
    tick();
    const id = setInterval(tick, 1000);
    return () => clearInterval(id);
  }, [until]);

  return display;
}

// ─── Login Page ───────────────────────────────────────────────────────────────

export default function LoginPage() {
  const router = useRouter();
  const { isAuthenticated, isLoading: authLoading } = useAuth();

  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorState>({ kind: "none" });

  // ── Auth guard: redirect if already logged in ──────────────────────────────
  useEffect(() => {
    if (!authLoading && isAuthenticated) {
      router.replace("/trading");
    }
  }, [authLoading, isAuthenticated, router]);

  const lockedUntil = error.kind === "locked" ? error.until : null;
  const countdown = useCountdown(lockedUntil);

  useEffect(() => {
    if (error.kind === "locked" && countdown === null) {
      setError({ kind: "none" });
    }
  }, [error.kind, countdown]);

  // ── Submit handler ─────────────────────────────────────────────────────────

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (isLoading) return;
    if (error.kind === "locked" && countdown !== null) return;

    setIsLoading(true);
    setError({ kind: "none" });

    try {
      const result = await authLogin(username.trim(), password);
      if (result.ok) {
        router.push("/trading");
        return;
      }
      if (result.status === 401) {
        setError({ kind: "invalid", remaining: result.data.error.remaining_attempts ?? 0 });
        setPassword("");
      } else if (result.status === 403) {
        const rawDate = result.data.error.locked_until;
        const until = rawDate ? new Date(rawDate) : new Date(Date.now() + 15 * 60 * 1000);
        setError({ kind: "locked", until });
      } else {
        setError({ kind: "server" });
      }
    } catch {
      setError({ kind: "server" });
    } finally {
      setIsLoading(false);
    }
  }

  const isLocked = error.kind === "locked" && countdown !== null;
  const submitDisabled = isLoading || isLocked;

  // Show minimal loading while AuthContext resolves initial /auth/me
  if (authLoading) {
    return (
      <div className="flex h-full items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  // ── Render ─────────────────────────────────────────────────────────────────

  return (
    <div className="w-full max-w-sm space-y-6 p-8 rounded-xl border border-border bg-card shadow-lg">
      {/* Header */}
      <div className="space-y-2 text-center">
        <div className="flex justify-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary/10 ring-1 ring-primary/30">
            <TrendingUp className="h-6 w-6 text-primary" />
          </div>
        </div>
        <h1 className="text-2xl font-semibold tracking-tight text-foreground">QuantFlow</h1>
        <p className="text-sm text-muted-foreground">Sign in to your account</p>
      </div>

      {/* Error banners */}
      {error.kind === "invalid" && (
        <div className="flex items-start gap-3 rounded-lg border border-warning/40 bg-warning/10 px-4 py-3 text-sm text-warning">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
          <span>
            Incorrect username or password.{" "}
            {error.remaining > 0 ? (
              <><strong>{error.remaining}</strong> attempt{error.remaining !== 1 ? "s" : ""} remaining.</>
            ) : (
              "Account will be locked on the next failed attempt."
            )}
          </span>
        </div>
      )}
      {error.kind === "locked" && (
        <div className="flex items-start gap-3 rounded-lg border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          <Lock className="mt-0.5 h-4 w-4 shrink-0" />
          <span>
            Account temporarily locked.{" "}
            {countdown ? <>Try again in <strong>{countdown}</strong>.</> : "You may try again now."}
          </span>
        </div>
      )}
      {error.kind === "server" && (
        <div className="flex items-start gap-3 rounded-lg border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
          <span>A server error occurred. Please try again.</span>
        </div>
      )}

      {/* Form */}
      <form onSubmit={handleSubmit} className="space-y-4" noValidate>
        <div className="space-y-1.5">
          <Label htmlFor="username">Username</Label>
          <Input
            id="username"
            type="text"
            autoComplete="username"
            autoFocus
            required
            disabled={isLoading}
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="Enter your username"
            className="h-10"
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="password">Password</Label>
          <Input
            id="password"
            type="password"
            autoComplete="current-password"
            required
            disabled={isLoading}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="Enter your password"
            className="h-10"
          />
        </div>
        <Button type="submit" disabled={submitDisabled} className="w-full h-10 font-medium">
          {isLoading ? (
            <><Loader2 className="mr-2 h-4 w-4 animate-spin" />Signing in…</>
          ) : isLocked ? (
            <><Lock className="mr-2 h-4 w-4" />Locked — {countdown}</>
          ) : (
            "Sign in"
          )}
        </Button>
      </form>

      {/* Footer */}
      <p className="text-center text-xs text-muted-foreground">
        QuantFlow · Low-code Crypto Trading Platform
      </p>
    </div>
  );
}
