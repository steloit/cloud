/**
 * "Canon now" — frozen just after the incident's last event (deploy #143,
 * 14:50 IST, 2026-07-02) so relative times read like the gallery frames.
 */
export const CANON_NOW = new Date("2026-07-02T15:00:00+05:30");

export function relTime(iso: string): string {
  const then = new Date(iso).getTime();
  const mins = Math.max(0, Math.round((CANON_NOW.getTime() - then) / 60_000));
  if (mins < 60) return `${mins} min ago`;
  const hours = Math.round(mins / 60);
  if (hours < 48) return `${hours} h ago`;
  const days = Math.round(hours / 24);
  if (days < 60) return `${days} days ago`;
  return `${Math.round(days / 30)} mo ago`;
}

/** "age 7 mo" — service identity chip. */
export function ageOf(iso: string): string {
  const months = Math.round((CANON_NOW.getTime() - new Date(iso).getTime()) / (30 * 86_400_000));
  if (months < 1) return "age <1 mo";
  return `age ${months} mo`;
}
