"use client";

/**
 * settings-dialog.tsx — Settings Dialog with Account + Exchange API Key tabs (Task 3.1.4).
 *
 * Tabs:
 *  1. Account — PUT /account/profile (change username and/or password).
 *     On success (200): force-logout → redirect /login.
 *  2. Exchange API Key — GET/POST/DELETE /exchange/api-keys.
 *     Secret Key is Write-Only: masked on display, never returned from API.
 *
 * Note on field masking (task 3.1.4 requirement):
 *   - api_key_masked from server: "****************************Eh8A"
 *   - secret_key INPUT: type="password" + eye toggle (never shows server value)
 *
 * WBS 3.1.4 · api.yaml §/account/profile §/exchange/api-keys
 */

import { useState, useEffect, FormEvent } from "react";
import { Eye, EyeOff, Loader2, Trash2, CheckCircle2, AlertCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Tabs,
  TabsList,
  TabsTrigger,
  TabsContent,
} from "@/components/ui/tabs";
import {
  updateProfile,
  getApiKeys,
  saveApiKeys,
  deleteApiKeys,
  ApiKeyInfo,
} from "@/lib/api-client";
import { useAuth } from "@/lib/auth";

// ─── Props ────────────────────────────────────────────────────────────────────

interface SettingsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

// ─── Inline status components ─────────────────────────────────────────────────

function ErrorBanner({ message }: { message: string }) {
  return (
    <div className="flex items-start gap-2 rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
      <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
      <span>{message}</span>
    </div>
  );
}

function SuccessBanner({ message }: { message: string }) {
  return (
    <div className="flex items-start gap-2 rounded-md border border-green-500/40 bg-green-500/10 px-3 py-2 text-sm text-green-400">
      <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0" />
      <span>{message}</span>
    </div>
  );
}

// ─── Account Tab ──────────────────────────────────────────────────────────────

function AccountTab({ onForceLogout }: { onForceLogout: () => void }) {
  const { user } = useAuth();

  const [currentPassword, setCurrentPassword] = useState("");
  const [newUsername, setNewUsername] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!currentPassword.trim()) { setError("Current password is required."); return; }
    if (!newUsername.trim() && !newPassword) { setError("Provide at least a new username or new password."); return; }
    if (newPassword && newPassword !== confirmPassword) { setError("New passwords do not match."); return; }

    setIsLoading(true);
    setError("");
    setSuccess("");

    const result = await updateProfile({
      current_password: currentPassword,
      new_username: newUsername.trim() || undefined,
      new_password: newPassword || undefined,
      confirm_password: newPassword ? confirmPassword : undefined,
    });

    setIsLoading(false);

    if (result.ok) {
      setSuccess("Profile updated. You will be signed out now…");
      setTimeout(onForceLogout, 1500);
    } else {
      const msg =
        result.code === "INVALID_CURRENT_PASSWORD"
          ? "Current password is incorrect."
          : result.code === "PASSWORD_MISMATCH"
            ? "New passwords do not match."
            : result.message || "An error occurred. Please try again.";
      setError(msg);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4 pt-2">
      {/* Current username (read-only display) */}
      {user && (
        <div className="space-y-1">
          <Label className="text-xs text-muted-foreground">Current username</Label>
          <p className="text-sm font-medium text-foreground pl-1">{user.username}</p>
        </div>
      )}

      <div className="space-y-1.5">
        <Label htmlFor="current-password">Current password <span className="text-destructive">*</span></Label>
        <Input id="current-password" type="password" autoComplete="current-password"
          value={currentPassword} onChange={e => setCurrentPassword(e.target.value)}
          placeholder="Enter current password" disabled={isLoading} />
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="new-username">New username <span className="text-muted-foreground text-xs">(optional)</span></Label>
        <Input id="new-username" type="text" autoComplete="username"
          value={newUsername} onChange={e => setNewUsername(e.target.value)}
          placeholder="Leave blank to keep current" disabled={isLoading} />
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="new-password">New password <span className="text-muted-foreground text-xs">(optional)</span></Label>
        <Input id="new-password" type="password" autoComplete="new-password"
          value={newPassword} onChange={e => setNewPassword(e.target.value)}
          placeholder="Leave blank to keep current" disabled={isLoading} />
      </div>

      {newPassword && (
        <div className="space-y-1.5">
          <Label htmlFor="confirm-password">Confirm new password</Label>
          <Input id="confirm-password" type="password" autoComplete="new-password"
            value={confirmPassword} onChange={e => setConfirmPassword(e.target.value)}
            placeholder="Repeat new password" disabled={isLoading} />
        </div>
      )}

      {error && <ErrorBanner message={error} />}
      {success && <SuccessBanner message={success} />}

      <Button type="submit" disabled={isLoading} className="w-full">
        {isLoading ? <><Loader2 className="mr-2 h-4 w-4 animate-spin" />Saving…</> : "Save changes"}
      </Button>
    </form>
  );
}

// ─── Exchange API Key Tab ─────────────────────────────────────────────────────

/** Show only the last `n` chars of a masked key, prefixed with "…" if longer. */
function tailMask(mask: string, n = 42): string {
  if (mask.length <= n) return mask;
  return `\u2026${mask.slice(-n)}`;
}

function ExchangeKeyTab({ isVisible }: { isVisible: boolean }) {
  const [keyInfo, setKeyInfo] = useState<ApiKeyInfo | null | undefined>(undefined); // undefined = loading
  const [apiKey, setApiKey] = useState("");
  const [secretKey, setSecretKey] = useState("");
  const [showSecret, setShowSecret] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");

  // Load current key info when tab becomes visible
  useEffect(() => {
    if (!isVisible) return;
    setKeyInfo(undefined); // reset to loading
    getApiKeys().then(result => {
      setKeyInfo(result.ok ? result.data : null);
    });
  }, [isVisible]);

  async function handleSave(e: FormEvent) {
    e.preventDefault();
    if (!apiKey.trim() || !secretKey.trim()) { setError("API Key and Secret Key are required."); return; }

    setIsLoading(true);
    setError("");
    setSuccess("");

    const result = await saveApiKeys({ exchange: "Binance", api_key: apiKey.trim(), secret_key: secretKey.trim() });
    setIsLoading(false);

    if (result.ok) {
      setKeyInfo(result.data);
      setApiKey("");
      setSecretKey("");
      setSuccess("Exchange connected successfully.");
    } else {
      const msg =
        result.code === "EXCHANGE_VALIDATION_FAILED"
          ? "Binance rejected the key — invalid API key or missing Futures Trading permission."
          : result.code === "INVALID_KEY_FORMAT"
            ? "Invalid API key format. Please check and try again."
            : result.message || "An error occurred. Please try again.";
      setError(msg);
    }
  }

  async function handleDelete() {
    if (!confirm("Remove exchange API key configuration?")) return;
    setIsDeleting(true);
    setError("");
    setSuccess("");

    const result = await deleteApiKeys();
    setIsDeleting(false);

    if (result.ok) {
      setKeyInfo(null);
      setSuccess("Exchange configuration removed.");
    } else {
      const msg =
        result.code === "ACTIVE_BOTS_EXIST"
          ? "Cannot remove while bots are running. Stop all bots first."
          : result.message || "Failed to remove configuration.";
      setError(msg);
    }
  }

  return (
    <div className="space-y-4 pt-2">
      {/* Current status */}
      {keyInfo === undefined ? (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" /> Loading…
        </div>
      ) : keyInfo !== null ? (
        <div className="rounded-md border border-border bg-muted/30 p-3 space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-xs text-muted-foreground">Status</span>
            <span className="text-xs font-medium text-green-400 flex items-center gap-1">
              <span className="inline-block h-1.5 w-1.5 rounded-full bg-green-400" />
              {keyInfo.status}
            </span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-xs text-muted-foreground">Exchange</span>
            <span className="text-xs font-medium text-foreground">{keyInfo.exchange}</span>
          </div>
          <div className="flex items-center justify-between gap-2">
            <span className="text-xs text-muted-foreground shrink-0 whitespace-nowrap">API Key</span>
            <span className="text-xs font-mono text-foreground">{tailMask(keyInfo.api_key_masked)}</span>
          </div>
        </div>
      ) : (
        <p className="text-sm text-muted-foreground">No exchange connected.</p>
      )}

      {/* Form to add/update key */}
      <form onSubmit={handleSave} className="space-y-3">
        <p className="text-xs text-muted-foreground">
          {keyInfo ? "Update API key configuration:" : "Connect an exchange:"}
        </p>

        <div className="space-y-1.5">
          <Label htmlFor="exc-api-key">API Key</Label>
          <Input id="exc-api-key" type="text" autoComplete="off"
            value={apiKey} onChange={e => setApiKey(e.target.value)}
            placeholder="Binance API Key" disabled={isLoading} />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="exc-secret-key">
            Secret Key
            <span className="ml-1.5 text-xs text-muted-foreground">(write-only)</span>
          </Label>
          <div className="relative">
            <Input
              id="exc-secret-key"
              type={showSecret ? "text" : "password"}
              autoComplete="off"
              value={secretKey}
              onChange={e => setSecretKey(e.target.value)}
              placeholder="Binance Secret Key"
              disabled={isLoading}
              className="pr-9"
            />
            <button
              type="button"
              onClick={() => setShowSecret(v => !v)}
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
              tabIndex={-1}
              aria-label={showSecret ? "Hide secret key" : "Show secret key"}
            >
              {showSecret ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
            </button>
          </div>
        </div>

        {error && <ErrorBanner message={error} />}
        {success && <SuccessBanner message={success} />}

        <div className="flex gap-2">
          <Button type="submit" disabled={isLoading} className="flex-1">
            {isLoading ? <><Loader2 className="mr-2 h-4 w-4 animate-spin" />Connecting…</> : keyInfo ? "Update" : "Connect"}
          </Button>
          {keyInfo && (
            <Button type="button" variant="outline" size="icon" onClick={handleDelete} disabled={isDeleting}
              title="Remove exchange API key" aria-label="Remove exchange API key">
              {isDeleting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4 text-destructive" />}
            </Button>
          )}
        </div>
      </form>
    </div>
  );
}

// ─── Main Dialog ──────────────────────────────────────────────────────────────

export function SettingsDialog({ open, onOpenChange }: SettingsDialogProps) {
  const { logout } = useAuth();
  const [activeTab, setActiveTab] = useState("account");

  // Reset to account tab when dialog re-opens
  useEffect(() => {
    if (open) setActiveTab("account");
  }, [open]);

  async function handleForceLogout() {
    onOpenChange(false);
    await logout();
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Settings</DialogTitle>
        </DialogHeader>

        <Tabs value={activeTab} onValueChange={setActiveTab}>
          <TabsList className="w-full">
            <TabsTrigger value="account" className="flex-1">Account</TabsTrigger>
            <TabsTrigger value="exchange" className="flex-1">Exchange API Key</TabsTrigger>
          </TabsList>

          <TabsContent value="account">
            <AccountTab onForceLogout={handleForceLogout} />
          </TabsContent>

          <TabsContent value="exchange">
            <ExchangeKeyTab isVisible={activeTab === "exchange" && open} />
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  );
}
