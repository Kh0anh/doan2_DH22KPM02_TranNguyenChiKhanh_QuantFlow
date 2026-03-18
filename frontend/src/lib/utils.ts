// ============================================================
// QuantFlow — Utility Functions
// Task: F-0.4 — Setup Constants & Utilities
// ============================================================

import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

import type { BotStatus } from '@/types/bot';
import type { StrategyStatus } from '@/types/strategy';
import { BOT_STATUS_CONFIG, STRATEGY_STATUS_CONFIG } from './constants';

// ─── Tailwind Merge (required by shadcn/ui) ────────────────

/** Merge Tailwind classes with conflict resolution */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

// ─── Price Formatters ──────────────────────────────────────

const priceFormatter = new Intl.NumberFormat('en-US', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

const preciseFormatter = new Intl.NumberFormat('en-US', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 8,
});

/** Format number as price (e.g., 64,500.00) */
export function formatPrice(value: number | string): string {
  const num = typeof value === 'string' ? parseFloat(value) : value;
  if (isNaN(num)) return '—';
  // Use 8 decimals for small prices (< 1), 2 for large
  return num < 1 && num > 0
    ? preciseFormatter.format(num)
    : priceFormatter.format(num);
}

/** Format volume with compact notation (e.g., 28.65B, 1.2M) */
export function formatVolume(value: number | string): string {
  const num = typeof value === 'string' ? parseFloat(value) : value;
  if (isNaN(num)) return '—';

  if (num >= 1_000_000_000) return `${(num / 1_000_000_000).toFixed(2)}B`;
  if (num >= 1_000_000) return `${(num / 1_000_000).toFixed(2)}M`;
  if (num >= 1_000) return `${(num / 1_000).toFixed(2)}K`;
  return priceFormatter.format(num);
}

/** Format percentage with sign (e.g., +2.41%, -1.20%) */
export function formatPercent(value: number | string): string {
  const num = typeof value === 'string' ? parseFloat(value) : value;
  if (isNaN(num)) return '—';
  const sign = num > 0 ? '+' : '';
  return `${sign}${num.toFixed(2)}%`;
}

/** Format PnL value with sign (e.g., +128.95, -42.30) */
export function formatPnl(value: number | string): string {
  const num = typeof value === 'string' ? parseFloat(value) : value;
  if (isNaN(num)) return '—';
  const sign = num > 0 ? '+' : '';
  return `${sign}${priceFormatter.format(num)}`;
}

// ─── Date Formatters ───────────────────────────────────────

const dateFormatter = new Intl.DateTimeFormat('vi-VN', {
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
});

const dateTimeFormatter = new Intl.DateTimeFormat('vi-VN', {
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
});

const timeFormatter = new Intl.DateTimeFormat('vi-VN', {
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  hour12: false,
});

/** Format ISO string to date (e.g., 17/03/2026) */
export function formatDate(iso: string): string {
  try {
    return dateFormatter.format(new Date(iso));
  } catch {
    return '—';
  }
}

/** Format ISO string to datetime (e.g., 17/03/2026 10:30) */
export function formatDateTime(iso: string): string {
  try {
    return dateTimeFormatter.format(new Date(iso));
  } catch {
    return '—';
  }
}

/** Format ISO string to time only (e.g., 10:30:15) */
export function formatTime(iso: string): string {
  try {
    return timeFormatter.format(new Date(iso));
  } catch {
    return '—';
  }
}

/** Format ISO string to relative time (e.g., 5 phút trước) */
export function formatRelativeTime(iso: string): string {
  try {
    const now = Date.now();
    const then = new Date(iso).getTime();
    const diffMs = now - then;

    if (diffMs < 0) return 'vừa xong';

    const seconds = Math.floor(diffMs / 1000);
    if (seconds < 60) return 'vừa xong';

    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes} phút trước`;

    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours} giờ trước`;

    const days = Math.floor(hours / 24);
    if (days < 30) return `${days} ngày trước`;

    const months = Math.floor(days / 30);
    if (months < 12) return `${months} tháng trước`;

    return `${Math.floor(months / 12)} năm trước`;
  } catch {
    return '—';
  }
}

// ─── Color / Status Helpers ────────────────────────────────

/** Return CSS class for PnL value: 'price-up' (green) or 'price-down' (red) */
export function getPnlColor(value: number): string {
  if (value > 0) return 'price-up';
  if (value < 0) return 'price-down';
  return 'text-muted-foreground';
}

/** Return CSS class for price change percent */
export function getPriceChangeColor(value: number): string {
  if (value > 0) return 'price-up';
  if (value < 0) return 'price-down';
  return 'text-muted-foreground';
}

/** Get bot status display config (label, color, dotColor) */
export function getBotStatusConfig(status: BotStatus) {
  return BOT_STATUS_CONFIG[status] ?? BOT_STATUS_CONFIG.Stopped;
}

/** Get strategy status display config (label, color, dotColor) */
export function getStrategyStatusConfig(status: StrategyStatus) {
  return STRATEGY_STATUS_CONFIG[status] ?? STRATEGY_STATUS_CONFIG.Draft;
}

// ─── String Helpers ────────────────────────────────────────

/** Truncate string with ellipsis (e.g., "abcdefgh..." → "abcd...fgh") */
export function truncateMiddle(
  str: string,
  startChars = 6,
  endChars = 4
): string {
  if (str.length <= startChars + endChars + 3) return str;
  return `${str.slice(0, startChars)}...${str.slice(-endChars)}`;
}

/** Mask API key (show last 4 chars only) */
export function maskApiKey(key: string): string {
  if (key.length <= 4) return key;
  return `${'*'.repeat(key.length - 4)}${key.slice(-4)}`;
}

// ─── Misc Helpers ──────────────────────────────────────────

/** Sleep utility for delays (e.g., reconnection backoff) */
export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/** Generate a simple unique ID (not UUID, for local UI keys) */
export function generateId(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
}
