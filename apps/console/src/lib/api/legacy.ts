import type { Product } from "@/lib/api";

/**
 * PRE-A5 CANON SEAM (P4). The frozen `Product` enum is [postgres, valkey,
 * web, worker] (ADR-0004/A5); the canon demo world still carries `storage`
 * and `queue` services pending the S9 item-6 ruling and the P4 canon
 * migration. Render-plane code widens through this type so the canon world
 * keeps rendering; the create plane conforms to the frozen enum. Deleting
 * this file is the P4 exit criterion.
 */
export type CanonProduct = Product | "storage" | "queue";
