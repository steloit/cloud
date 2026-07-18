import type { Product } from "@/lib/api";

/**
 * PRE-A5 CANON SEAM — narrowed by the P4a ruling (2026-07-18): the
 * ecommerce canon is fully migrated (assets → Storage Binding, jobs →
 * Postgres $21 carrying the Jobs product; $208 preserved). The seam now
 * covers only (a) internal-tools' `files` storage record in world.ts
 * (M1-frame-coupled — the adaptive-rail exemplar needs two products) and
 * (b) frame-derived render surfaces (M3 env matrix columns, D7/D8 routes,
 * snav storage/queue workspaces) pending the S9 frame ruling. Deleting
 * this file is the P4b exit criterion, executed with S9.
 */
export type CanonProduct = Product | "storage" | "queue";
