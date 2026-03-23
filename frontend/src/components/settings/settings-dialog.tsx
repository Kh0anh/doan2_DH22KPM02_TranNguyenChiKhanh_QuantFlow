/**
 * [3.1.4] Settings Dialog — Account Tab + Exchange API Key Tab.
 *
 * Controlled by useUIStore: settingsOpen / closeSettings().
 *
 * ── Tab "Tài khoản" (UC-02) ─────────────────────────────────────────────
 *   PUT /api/v1/account/profile  { current_password, new_username?, new_password?, confirm_password? }
 *   200 → Force Logout: toast "Cập nhật thành công." → logout() after 2s
 *   400 PASSWORD_MISMATCH / MISSING_CHANGE_FIELDS  → inline error
 *   401 INVALID_CURRENT_PASSWORD                   → inline error on current_password field
 *
 * ── Tab "Kết nối sàn" (UC-03) ────────────────────────────────────────────
 *   GET    /api/v1/exchange/api-keys  → load current status on open
 *   POST   /api/v1/exchange/api-keys  { exchange, api_key, secret_key } → save / overwrite
 *   DELETE /api/v1/exchange/api-keys  → delete config (409 = active bots)
 *   422 EXCHANGE_VALIDATION_FAILED    → red alert banner
 *   Secret Key is Write-Only — never displayed plain-text.
 */
"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Eye, EyeOff, Loader2, User, Link } from "lucide-react";
import { toast } from "sonner";

import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { Badge } from "@/components/ui/badge";
import { useUIStore } from "@/store/ui-store";
import { useAuth } from "@/lib/auth";
import type { ApiKeyInfo } from "@/types";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type SettingsTab = "account" | "exchange";

// ---------------------------------------------------------------------------
// Password Input with visibility toggle
// ---------------------------------------------------------------------------

interface PasswordInputProps {
  id: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  disabled?: boolean;
  autoComplete?: string;
}

function PasswordInput({
  id,
  value,
  onChange,
  placeholder,
  disabled,
  autoComplete,
}: PasswordInputProps) {
  const [visible, setVisible] = useState(false);
  return (
    <div className="relative">
      <Input
        id={id}
        type={visible ? "text" : "password"}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        disabled={disabled}
        autoComplete={autoComplete}
        className="pr-9"
      />
      <button
        type="button"
        tabIndex={-1}
        onClick={() => setVisible((v) => !v)}
        disabled={disabled}
        className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground disabled:pointer-events-none"
        aria-label={visible ? "Ẩn mật khẩu" : "Hiện mật khẩu"}
      >
        {visible ? (
          <EyeOff className="size-4" aria-hidden="true" />
        ) : (
          <Eye className="size-4" aria-hidden="true" />
        )}
      </button>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Account Tab
// ---------------------------------------------------------------------------

function AccountTab() {
  const { user, logout } = useAuth();
  const router = useRouter();

  const [newUsername, setNewUsername] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [currentPassword, setCurrentPassword] = useState("");

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [isSubmitting, setIsSubmitting] = useState(false);

  function validate(): boolean {
    const errs: Record<string, string> = {};
    if (!newUsername && !newPassword) {
      errs.general = "Vui lòng cung cấp ít nhất Tên đăng nhập mới hoặc Mật khẩu mới.";
    }
    if (newPassword && newPassword !== confirmPassword) {
      errs.confirmPassword = "Mật khẩu xác nhận không trùng khớp.";
    }
    if (!currentPassword) {
      errs.currentPassword = "Mật khẩu hiện tại là bắt buộc.";
    }
    setErrors(errs);
    return Object.keys(errs).length === 0;
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!validate()) return;

    setIsSubmitting(true);
    setErrors({});

    try {
      const body: Record<string, string> = { current_password: currentPassword };
      if (newUsername) body.new_username = newUsername;
      if (newPassword) {
        body.new_password = newPassword;
        body.confirm_password = confirmPassword;
      }

      const res = await fetch("/api/v1/account/profile", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify(body),
      });

      const data = await res.json();

      if (res.ok) {
        toast.success("Cập nhật thành công. Vui lòng đăng nhập lại.");
        setTimeout(() => logout(), 2000);
        return;
      }

      const code = data?.error?.code;
      const message = data?.error?.message ?? "Đã có lỗi xảy ra.";

      if (code === "INVALID_CURRENT_PASSWORD") {
        setErrors({ currentPassword: message });
      } else if (code === "PASSWORD_MISMATCH") {
        setErrors({ confirmPassword: message });
      } else {
        setErrors({ general: message });
      }
    } catch {
      toast.error("Lỗi kết nối. Vui lòng thử lại.");
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-5">
      {/* Current username — read-only */}
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs text-muted-foreground">Tên đăng nhập hiện tại</Label>
        <div className="flex h-9 items-center rounded-md border border-border bg-secondary px-3 text-sm text-muted-foreground">
          {user?.username ?? "—"}
        </div>
      </div>

      <Separator />

      {/* New username */}
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="new-username" className="text-sm">
          Tên đăng nhập mới
          <span className="ml-1 text-xs text-muted-foreground">(tuỳ chọn)</span>
        </Label>
        <Input
          id="new-username"
          type="text"
          value={newUsername}
          onChange={(e) => setNewUsername(e.target.value)}
          placeholder="Để trống nếu không thay đổi"
          disabled={isSubmitting}
          autoComplete="username"
        />
      </div>

      {/* New password */}
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="new-password" className="text-sm">
          Mật khẩu mới
          <span className="ml-1 text-xs text-muted-foreground">(tuỳ chọn)</span>
        </Label>
        <PasswordInput
          id="new-password"
          value={newPassword}
          onChange={setNewPassword}
          placeholder="Để trống nếu không thay đổi"
          disabled={isSubmitting}
          autoComplete="new-password"
        />
      </div>

      {/* Confirm new password */}
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="confirm-password" className="text-sm">
          Nhập lại mật khẩu mới
        </Label>
        <PasswordInput
          id="confirm-password"
          value={confirmPassword}
          onChange={setConfirmPassword}
          placeholder="Nhập lại mật khẩu mới"
          disabled={isSubmitting || !newPassword}
          autoComplete="new-password"
        />
        {errors.confirmPassword && (
          <p className="text-xs text-destructive">{errors.confirmPassword}</p>
        )}
      </div>

      <Separator />

      {/* Current password — required */}
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="current-password" className="text-sm">
          Mật khẩu hiện tại
          <span className="ml-1 text-xs text-[var(--color-danger)]">*</span>
        </Label>
        <PasswordInput
          id="current-password"
          value={currentPassword}
          onChange={setCurrentPassword}
          placeholder="Mật khẩu hiện tại"
          disabled={isSubmitting}
          autoComplete="current-password"
        />
        {errors.currentPassword && (
          <p className="text-xs text-destructive">{errors.currentPassword}</p>
        )}
      </div>

      {errors.general && (
        <p className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">
          {errors.general}
        </p>
      )}

      <div className="flex justify-end">
        <Button type="submit" disabled={isSubmitting}>
          {isSubmitting && <Loader2 className="mr-2 size-4 animate-spin" aria-hidden="true" />}
          Lưu thay đổi
        </Button>
      </div>
    </form>
  );
}

// ---------------------------------------------------------------------------
// Exchange Tab
// ---------------------------------------------------------------------------

function ExchangeTab() {
  const [apiKeyInfo, setApiKeyInfo] = useState<ApiKeyInfo | null | undefined>(
    undefined // undefined = not loaded yet
  );
  const [isLoadingInfo, setIsLoadingInfo] = useState(true);

  const [apiKey, setApiKey] = useState("");
  const [secretKey, setSecretKey] = useState("");
  const [saveError, setSaveError] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);

  const [isDeleting, setIsDeleting] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

  // Load current API key status on mount
  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const res = await fetch("/api/v1/exchange/api-keys", {
          credentials: "include",
        });
        if (cancelled) return;
        if (!res.ok) {
          setApiKeyInfo(null);
          return;
        }
        const body = await res.json();
        const raw = body?.data;
        if (!raw) {
          setApiKeyInfo(null);
          return;
        }
        setApiKeyInfo({
          id: raw.id,
          exchange: raw.exchange,
          apiKeyMasked: raw.api_key_masked,
          status: raw.status,
          createdAt: raw.created_at,
          updatedAt: raw.updated_at,
        });
      } catch {
        if (!cancelled) setApiKeyInfo(null);
      } finally {
        if (!cancelled) setIsLoadingInfo(false);
      }
    }
    load();
    return () => { cancelled = true; };
  }, []);

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    setSaveError(null);

    if (!apiKey.trim() || !secretKey.trim()) {
      setSaveError("Vui lòng nhập đầy đủ API Key và Secret Key.");
      return;
    }

    setIsSaving(true);
    try {
      const res = await fetch("/api/v1/exchange/api-keys", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({
          exchange: "Binance",
          api_key: apiKey.trim(),
          secret_key: secretKey.trim(),
        }),
      });

      const body = await res.json();

      if (res.ok) {
        const raw = body?.data;
        if (raw) {
          setApiKeyInfo({
            id: raw.id,
            exchange: raw.exchange,
            apiKeyMasked: raw.api_key_masked,
            status: raw.status,
            createdAt: raw.created_at,
            updatedAt: raw.updated_at,
          });
        }
        setApiKey("");
        setSecretKey("");
        toast.success("Kết nối sàn thành công.");
        return;
      }

      const code = body?.error?.code;
      if (code === "EXCHANGE_VALIDATION_FAILED") {
        setSaveError("API Key không hợp lệ hoặc thiếu quyền hạn Futures Trading.");
      } else if (code === "INVALID_KEY_FORMAT") {
        setSaveError("Vui lòng kiểm tra lại định dạng API Key.");
      } else {
        setSaveError(body?.error?.message ?? "Đã có lỗi xảy ra. Vui lòng thử lại.");
      }
    } catch {
      toast.error("Lỗi kết nối. Vui lòng thử lại.");
    } finally {
      setIsSaving(false);
    }
  }

  async function handleDelete() {
    setIsDeleting(true);
    try {
      const res = await fetch("/api/v1/exchange/api-keys", {
        method: "DELETE",
        credentials: "include",
      });

      const body = await res.json();

      if (res.ok) {
        setApiKeyInfo(null);
        setShowDeleteConfirm(false);
        toast.success("Đã xóa cấu hình kết nối sàn.");
        return;
      }

      const code = body?.error?.code;
      if (code === "ACTIVE_BOTS_EXIST") {
        toast.error("Không thể xóa cấu hình khi còn Bot đang chạy. Vui lòng dừng tất cả Bot trước.");
      } else {
        toast.error(body?.error?.message ?? "Đã có lỗi xảy ra.");
      }
      setShowDeleteConfirm(false);
    } catch {
      toast.error("Lỗi kết nối. Vui lòng thử lại.");
    } finally {
      setIsDeleting(false);
    }
  }

  if (isLoadingInfo) {
    return (
      <div className="flex h-40 items-center justify-center">
        <Loader2 className="size-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  const isConnected = apiKeyInfo !== null && apiKeyInfo?.status === "Connected";

  return (
    <div className="flex flex-col gap-5">
      {/* Current connection status */}
      <div className="flex flex-col gap-3">
        {/* Exchange selector — disabled (Binance only) */}
        <div className="flex flex-col gap-1.5">
          <Label className="text-sm">Sàn giao dịch</Label>
          <div className="flex h-9 items-center rounded-md border border-border bg-secondary px-3 text-sm text-muted-foreground">
            Binance
          </div>
        </div>

        {/* Connection status */}
        <div className="flex items-center gap-2">
          <Label className="text-sm text-muted-foreground">Trạng thái:</Label>
          {isConnected ? (
            <Badge
              variant="outline"
              className="border-[var(--color-success)] text-[var(--color-success)] gap-1.5"
            >
              <span className="size-1.5 rounded-full bg-[var(--color-success)]" />
              Đã kết nối
            </Badge>
          ) : (
            <Badge
              variant="outline"
              className="border-[var(--color-danger)] text-[var(--color-danger)] gap-1.5"
            >
              <span className="size-1.5 rounded-full bg-[var(--color-danger)]" />
              Chưa kết nối
            </Badge>
          )}
        </div>

        {/* Masked API Key display */}
        {isConnected && apiKeyInfo && (
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs text-muted-foreground">API Key hiện tại</Label>
            <div className="flex h-9 items-center rounded-md border border-border bg-secondary px-3 font-mono text-xs text-muted-foreground">
              {typeof apiKeyInfo.apiKeyMasked === "string"
                ? "••••••••••••••••••••••••••••••••" + apiKeyInfo.apiKeyMasked.slice(-4)
                : "—"}
            </div>
          </div>
        )}
      </div>

      <Separator />

      {/* Update form */}
      <form onSubmit={handleSave} className="flex flex-col gap-4">
        <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
          {isConnected ? "Cập nhật API Key" : "Thêm API Key"}
        </p>

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="api-key" className="text-sm">API Key</Label>
          <Input
            id="api-key"
            type="text"
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            placeholder="Nhập API Key"
            disabled={isSaving}
            autoComplete="off"
            className="font-mono text-sm"
          />
        </div>

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="secret-key" className="text-sm whitespace-nowrap">
            Secret Key
          </Label>
          <Input
            id="secret-key"
            type="password"
            value={secretKey}
            onChange={(e) => setSecretKey(e.target.value)}
            placeholder="Nhập Secret Key"
            disabled={isSaving}
            autoComplete="off"
            className="font-mono text-sm"
          />
        </div>

        {/* Exchange validation error banner */}
        {saveError && (
          <div className="rounded-md border border-[var(--color-danger)]/40 bg-[var(--color-danger)]/10 px-3 py-2.5 text-xs text-[var(--color-danger)]">
            {saveError}
          </div>
        )}

        <div className="flex items-center justify-between pt-1">
          <Button
            type="button"
            variant="destructive"
            size="sm"
            disabled={!isConnected || isSaving || isDeleting}
            onClick={() => setShowDeleteConfirm(true)}
          >
            {isDeleting && (
              <Loader2 className="mr-2 size-3.5 animate-spin" aria-hidden="true" />
            )}
            Xóa kết nối
          </Button>

          <Button type="submit" disabled={isSaving || isDeleting}>
            {isSaving && (
              <Loader2 className="mr-2 size-4 animate-spin" aria-hidden="true" />
            )}
            Lưu cấu hình
          </Button>
        </div>
      </form>

      {/* Inline delete confirmation */}
      {showDeleteConfirm && (
        <div className="rounded-md border border-[var(--color-danger)]/40 bg-[var(--color-danger)]/10 p-3 text-xs text-[var(--color-danger)]">
          <p className="mb-2 font-medium">
            Xác nhận xóa cấu hình kết nối sàn? Hành động này không thể hoàn tác.
          </p>
          <div className="flex gap-2">
            <Button
              size="sm"
              variant="destructive"
              disabled={isDeleting}
              onClick={handleDelete}
            >
              {isDeleting && (
                <Loader2 className="mr-1.5 size-3.5 animate-spin" aria-hidden="true" />
              )}
              Xác nhận xóa
            </Button>
            <Button
              size="sm"
              variant="ghost"
              disabled={isDeleting}
              onClick={() => setShowDeleteConfirm(false)}
            >
              Hủy
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// SettingsDialog — root export
// ---------------------------------------------------------------------------

export function SettingsDialog() {
  const settingsOpen = useUIStore((s) => s.settingsOpen);
  const closeSettings = useUIStore((s) => s.closeSettings);
  const [activeTab, setActiveTab] = useState<SettingsTab>("account");

  // Reset to account tab every time the dialog opens
  useEffect(() => {
    if (settingsOpen) setActiveTab("account");
  }, [settingsOpen]);

  return (
    <Dialog open={settingsOpen} onOpenChange={(open) => !open && closeSettings()}>
      <DialogContent className="flex min-h-[480px] max-h-[90vh] w-full max-w-[640px] sm:max-w-[640px] flex-col gap-0 overflow-hidden p-0">
        <DialogHeader className="shrink-0 border-b border-border px-6 py-4">
          <DialogTitle className="text-base font-semibold">Cài đặt</DialogTitle>
        </DialogHeader>

        <div className="flex flex-1 overflow-hidden">
          {/* ── Left tab sidebar ── */}
          <nav className="flex w-40 shrink-0 flex-col gap-1 border-r border-border p-3">
            <button
              type="button"
              onClick={() => setActiveTab("account")}
              className={[
                "flex items-center gap-2.5 rounded-md px-3 py-2 text-left text-sm transition-colors",
                activeTab === "account"
                  ? "bg-secondary text-foreground font-medium"
                  : "text-muted-foreground hover:bg-secondary/60 hover:text-foreground",
              ].join(" ")}
            >
              <User className="size-4 shrink-0" aria-hidden="true" />
              Tài khoản
            </button>

            <button
              type="button"
              onClick={() => setActiveTab("exchange")}
              className={[
                "flex items-center gap-2.5 rounded-md px-3 py-2 text-left text-sm transition-colors",
                activeTab === "exchange"
                  ? "bg-secondary text-foreground font-medium"
                  : "text-muted-foreground hover:bg-secondary/60 hover:text-foreground",
              ].join(" ")}
            >
              <Link className="size-4 shrink-0" aria-hidden="true" />
              Kết nối sàn
            </button>
          </nav>

          {/* ── Right content ── */}
          <div className="flex-1 overflow-y-auto p-6">
            {activeTab === "account" && <AccountTab />}
            {activeTab === "exchange" && <ExchangeTab />}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
