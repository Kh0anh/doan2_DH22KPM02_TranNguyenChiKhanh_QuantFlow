import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

/** Format a number as PnL string with sign and currency suffix */
export function formatPnl(value: number, currency = "USDT"): string {
  const sign = value >= 0 ? "+" : "";
  return `${sign}${value.toFixed(2)} ${currency}`;
}

/** Return Tailwind text color class for a PnL value */
export function pnlColor(value: number): string {
  if (value > 0) return "text-success";
  if (value < 0) return "text-danger";
  return "text-muted-foreground";
}

/** Format a price value with adaptive decimal places and locale commas */
export function formatPrice(value: number, decimals?: number): string {
  const d = decimals ?? (value >= 1 ? 2 : value >= 0.01 ? 4 : 6);
  return value.toLocaleString("en-US", {
    minimumFractionDigits: d,
    maximumFractionDigits: d,
  });
}

/** Format an ISO date string to short date display */
export function formatDate(isoString: string): string {
  const d = new Date(isoString);
  return d.toLocaleDateString("en-US", {
    day: "2-digit",
    month: "2-digit",
    year: "2-digit",
  });
}

/** Format an ISO date string to date + time display */
export function formatDateTime(isoString: string): string {
  const d = new Date(isoString);
  return d.toLocaleString("en-US", {
    day: "2-digit",
    month: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

/** Format a number as a percent string with sign */
export function formatPercent(value: number): string {
  const sign = value >= 0 ? "+" : "";
  return `${sign}${value.toFixed(2)}%`;
}
