/**
 * [3.1.1] Login Page — form + brute-force error handling (UC-01, FR-ACCESS-01).
 *
 * Flows:
 *  - Mount: GET /auth/me → redirect /trading if session is valid
 *  - Submit: POST /auth/login
 *      200 → redirect /trading
 *      401 INVALID_CREDENTIALS → destructive alert + remaining_attempts
 *      403 ACCOUNT_LOCKED → warning alert + mm:ss countdown + disable form
 *      5xx/network → sonner toast.error()
 */
"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { toast } from "sonner";
import {
  Eye,
  EyeOff,
  LogIn,
  Loader2,
  AlertCircle,
  Lock,
} from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { cn } from "@/lib/utils";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type LoginError =
  | { type: "invalid"; message: string; remaining_attempts: number }
  | { type: "locked"; message: string; locked_until: string }
  | null;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Format total seconds to mm:ss display */
function formatCountdown(totalSeconds: number): string {
  const m = Math.floor(totalSeconds / 60)
    .toString()
    .padStart(2, "0");
  const s = (totalSeconds % 60).toString().padStart(2, "0");
  return `${m}:${s}`;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export default function LoginPage() {
  const router = useRouter();
  const searchParams = useSearchParams();

  // Form field state
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);

  // UI state
  const [isLoading, setIsLoading] = useState(false);
  const [isCheckingSession, setIsCheckingSession] = useState(true);
  const [shake, setShake] = useState(false);

  // Error / lockout state
  const [loginError, setLoginError] = useState<LoginError>(null);
  const [countdown, setCountdown] = useState(0);

  // -------------------------------------------------------------------------
  // On mount: check if already authenticated → redirect to /trading
  // Skip session check if arrived from logout to prevent redirect loop.
  // -------------------------------------------------------------------------
  useEffect(() => {
    // User just logged out — skip the /auth/me check entirely
    if (searchParams.get("from") === "logout") {
      setIsCheckingSession(false);
      return;
    }

    let cancelled = false;

    async function checkSession() {
      try {
        const res = await fetch("/api/v1/auth/me", {
          credentials: "include",
        });
        if (!cancelled && res.ok) {
          router.replace("/trading");
          return;
        }
      } catch {
        // Not authenticated or network error — fall through to show form
      } finally {
        if (!cancelled) setIsCheckingSession(false);
      }
    }

    checkSession();
    return () => {
      cancelled = true;
    };
  }, [router, searchParams]);

  // -------------------------------------------------------------------------
  // Countdown timer — active only while account is locked
  // -------------------------------------------------------------------------
  useEffect(() => {
    if (loginError?.type !== "locked") return;

    const lockedUntilMs = new Date(loginError.locked_until).getTime();

    const tick = () => {
      const remaining = Math.max(
        0,
        Math.round((lockedUntilMs - Date.now()) / 1000)
      );
      setCountdown(remaining);
      if (remaining === 0) {
        setLoginError(null);
      }
    };

    tick(); // immediate paint
    const timerId = setInterval(tick, 1000);
    return () => clearInterval(timerId);
  }, [loginError]);

  // -------------------------------------------------------------------------
  // Shake animation trigger
  // -------------------------------------------------------------------------
  const triggerShake = useCallback(() => {
    setShake(true);
    const t = setTimeout(() => setShake(false), 600);
    return () => clearTimeout(t);
  }, []);

  // -------------------------------------------------------------------------
  // Form submit handler
  // -------------------------------------------------------------------------
  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (isLoading) return;

    setIsLoading(true);
    setLoginError(null);

    try {
      const res = await fetch("/api/v1/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ username: username.trim(), password }),
      });

      if (res.ok) {
        router.replace("/trading");
        return;
      }

      // Parse error body safely
      const body = await res.json().catch(() => ({})) as {
        error?: {
          code?: string;
          message?: string;
          remaining_attempts?: number;
          locked_until?: string;
        };
      };

      if (res.status === 401) {
        // Wrong credentials
        setLoginError({
          type: "invalid",
          message:
            body?.error?.message ??
            "Tên đăng nhập hoặc mật khẩu không chính xác.",
          remaining_attempts: body?.error?.remaining_attempts ?? 0,
        });
        triggerShake();
      } else if (res.status === 403) {
        // Account locked (brute-force)
        setLoginError({
          type: "locked",
          message:
            body?.error?.message ??
            "Bạn đã nhập sai quá nhiều lần. Vui lòng thử lại sau 15 phút.",
          locked_until:
            body?.error?.locked_until ??
            new Date(Date.now() + 15 * 60 * 1000).toISOString(),
        });
      } else {
        toast.error("Lỗi máy chủ. Vui lòng thử lại sau.");
      }
    } catch {
      toast.error(
        "Không thể kết nối đến máy chủ. Vui lòng kiểm tra kết nối mạng."
      );
    } finally {
      setIsLoading(false);
    }
  }

  // -------------------------------------------------------------------------
  // Derived state
  // -------------------------------------------------------------------------
  const isLocked = loginError?.type === "locked";
  const isFormDisabled = isLoading || isLocked;
  const canSubmit = username.trim().length > 0 && password.length > 0;

  // -------------------------------------------------------------------------
  // Render — session check spinner
  // -------------------------------------------------------------------------
  if (isCheckingSession) {
    return (
      <div className="flex items-center justify-center">
        <Loader2 className="size-5 animate-spin text-muted-foreground" />
      </div>
    );
  }

  // -------------------------------------------------------------------------
  // Render — login form
  // -------------------------------------------------------------------------
  return (
    <Card
      className={cn(
        "relative z-10 w-full max-w-sm",
        shake && "animate-shake"
      )}
    >
      {/* ── Logo & Tagline ── */}
      <CardHeader className="items-center justify-items-center text-center pb-2">
        {/* QuantFlow icon mark */}
        <div className="mb-3 flex size-12 items-center justify-center rounded-xl bg-primary/10">
          <svg
            viewBox="0 0 24 24"
            className="size-7 fill-primary"
            aria-hidden="true"
          >
            <path d="M2 2h9v9H2V2zm11 0h9v9h-9V2zM2 13h9v9H2v-9zm13 4a4 4 0 1 1 0-8 4 4 0 0 1 0 8z" />
          </svg>
        </div>
        <CardTitle className="text-xl font-semibold tracking-tight">
          QuantFlow
        </CardTitle>
        <CardDescription className="text-[13px]">
          Nền tảng Low-code giao dịch tự động
        </CardDescription>
      </CardHeader>

      <CardContent className="pt-4">
        <form onSubmit={handleSubmit} noValidate className="space-y-4">
          {/* ── Username ── */}
          <div className="space-y-1.5">
            <Label htmlFor="username">Tên đăng nhập</Label>
            <Input
              id="username"
              type="text"
              autoComplete="username"
              // eslint-disable-next-line jsx-a11y/no-autofocus
              autoFocus
              placeholder="Tên đăng nhập"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              disabled={isFormDisabled}
              required
            />
          </div>

          {/* ── Password ── */}
          <div className="space-y-1.5">
            <Label htmlFor="password">Mật khẩu</Label>
            <div className="relative">
              <Input
                id="password"
                type={showPassword ? "text" : "password"}
                autoComplete="current-password"
                placeholder="Mật khẩu"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                disabled={isFormDisabled}
                required
                className="pr-10"
              />
              <button
                type="button"
                className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground transition-colors hover:text-foreground disabled:pointer-events-none"
                onClick={() => setShowPassword((v) => !v)}
                aria-label={showPassword ? "Ẩn mật khẩu" : "Hiện mật khẩu"}
                tabIndex={-1}
                disabled={isFormDisabled}
              >
                {showPassword ? (
                  <EyeOff className="size-[15px]" aria-hidden="true" />
                ) : (
                  <Eye className="size-[15px]" aria-hidden="true" />
                )}
              </button>
            </div>
          </div>

          {/* ── Submit button ── */}
          <Button
            type="submit"
            className="w-full"
            disabled={isFormDisabled || !canSubmit}
          >
            {isLoading ? (
              <>
                <Loader2 className="size-4 animate-spin" aria-hidden="true" />
                Đang đăng nhập...
              </>
            ) : (
              <>
                <LogIn className="size-4" aria-hidden="true" />
                Đăng nhập
              </>
            )}
          </Button>

          {/* ── Error: invalid credentials (401) ── */}
          {loginError?.type === "invalid" && (
            <Alert variant="destructive">
              <AlertCircle className="size-4" aria-hidden="true" />
              <AlertDescription>
                <p>{loginError.message}</p>
                {loginError.remaining_attempts > 0 && (
                  <p className="mt-1 text-xs opacity-80">
                    Còn lại{" "}
                    <strong>{loginError.remaining_attempts}</strong> lần thử
                    trước khi tài khoản bị khóa.
                  </p>
                )}
              </AlertDescription>
            </Alert>
          )}

          {/* ── Error: account locked (403) ── */}
          {loginError?.type === "locked" && (
            <Alert className="border-warning/40 bg-warning/10 text-warning [&>svg]:text-warning">
              <Lock className="size-4" aria-hidden="true" />
              <AlertDescription className="text-warning/90">
                <p>{loginError.message}</p>
                {countdown > 0 && (
                  <p className="mt-1.5 text-sm">
                    Thử lại sau:{" "}
                    <span className="font-mono font-semibold tabular-nums">
                      {formatCountdown(countdown)}
                    </span>
                  </p>
                )}
              </AlertDescription>
            </Alert>
          )}
        </form>
      </CardContent>
    </Card>
  );
}
