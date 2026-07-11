import { createFileRoute } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Icon } from "@/design-system/icon";
import { Pill } from "@/design-system/pill";
import { useServices } from "@/features/services/hooks";
import { cn } from "@/lib/utils";

/**
 * D2 · Table Viewer. 08-api has no table-data endpoint (spec gap, a finding):
 * the table list, filter, and rows below are frame-fixed canon from the D2
 * frame — pagination and inline edits stay inert until the endpoint exists.
 */

const TABLE_CHIPS = [
  "orders",
  "order_items",
  "products",
  "customers",
  "gift_cards",
  "gift_card_redemptions",
  "sessions",
  "+5",
];

/** id | customer_id | total | status | gift_card_id (null → NULL) | created_at */
const ROWS: Array<[string, string, string, string, string | null, string]> = [
  ["ord_92a1f0", "10422", "₹6,300.00", "pending", "gc_77b2", "2026-07-06 14:12:44"],
  ["ord_929be3", "8241", "₹860.00", "pending", null, "2026-07-06 13:59:10"],
  ["ord_928c17", "7719", "₹2,940.00", "pending", "gc_10ac", "2026-07-06 11:07:29"],
  ["ord_9271d5", "11038", "₹1,480.00", "pending", null, "2026-07-05 22:51:02"],
  ["ord_925402", "9963", "₹520.00", "pending", null, "2026-07-05 18:20:47"],
  ["ord_924eb9", "10422", "₹3,780.00", "pending", "gc_77b2", "2026-07-05 16:44:12"],
];

function TablesPage() {
  const { service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(env);
  const svc = services.data?.find((s) => s.name === service || s.id === service);

  if (!svc) return <main className="main" />;
  if (svc.product !== "postgres") {
    return (
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Card className="p-4 text-12 text-ink2">This tab belongs to PostgreSQL instances.</Card>
        </div>
      </main>
    );
  }

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        <Pghead
          title="Table Viewer"
          sub={
            <span>
              schema <span className="mono">public</span> · 12 tables · 21.4 GB
            </span>
          }
        >
          <Btn variant="s">+ Add filter</Btn>
          <Btn variant="s">
            Columns
            <Icon id="s-chevd" className="h-[11px] w-[11px]" />
          </Btn>
        </Pghead>

        <div className="chiprow">
          {TABLE_CHIPS.map((t) => (
            <span key={t} className={cn("chip", t === "orders" && "on")}>
              {t}
            </span>
          ))}
        </div>

        <div className="flex items-center gap-2">
          <Copybit>{"WHERE status = 'pending' AND created_at > now() - interval '7 days'"}</Copybit>
          <Pill tone="mut">2 filters</Pill>
        </div>

        <div className="tblwrap">
          <table className="tbl">
            <thead>
              <tr>
                <th>id</th>
                <th>customer_id</th>
                <th>total</th>
                <th>status</th>
                <th>gift_card_id</th>
                <th>created_at</th>
              </tr>
            </thead>
            <tbody>
              {ROWS.map(([id, customer, total, status, giftCard, created]) => (
                <tr key={id}>
                  <td className="mono text-11">{id}</td>
                  <td className="mono text-11">{customer}</td>
                  <td className="mono text-11">{total}</td>
                  <td>
                    <Pill tone="warn">{status}</Pill>
                  </td>
                  <td className="mono text-11 text-ink3">{giftCard ?? "NULL"}</td>
                  <td className="mono text-11 text-ink2">{created}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <div className="flex items-center gap-2.5 text-11 text-ink3">
          <span className="mono">rows 1–6 of 1,204,318</span>
          <Btn variant="s" className="h-6 px-2.5 text-10p5">
            ← Prev
          </Btn>
          <Btn variant="s" className="h-6 px-2.5 text-10p5">
            Next →
          </Btn>
          <span className="flex-1" />
          <span>Inline edits need a read-write role · every write is an audit event</span>
        </div>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/tables")({
  component: TablesPage,
});
