/**
 * Money crosses the API as integer cents (ADR-025); the console renders it
 * through fmtMoney, always in mono. "$208" for whole dollars, "$199.10" otherwise.
 */
export function fmtMoney(cents: number): string {
  const hasCents = cents % 100 !== 0;
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: hasCents ? 2 : 0,
    maximumFractionDigits: hasCents ? 2 : 0,
  }).format(cents / 100);
}

/** "$208/mo" — the per-month grammar used across cards and rails. */
export function fmtMoneyPerMonth(cents: number): string {
  return `${fmtMoney(cents)}/mo`;
}

/** Canon times are IST (+05:30); the console shows wall-clock HH:MM. */
export function fmtTime(iso: string): string {
  return new Intl.DateTimeFormat("en-GB", {
    hour: "2-digit",
    minute: "2-digit",
    timeZone: "Asia/Kolkata",
  }).format(new Date(iso));
}
