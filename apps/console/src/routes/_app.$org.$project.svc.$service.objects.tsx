import { createFileRoute } from "@tanstack/react-router";
import { Pghead } from "@/app/shell/pghead";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { EmptyState } from "@/design-system/empty-state";
import { Eyebrow } from "@/design-system/eyebrow";
import { Glyph } from "@/design-system/glyph";
import { Icon } from "@/design-system/icon";
import { Inp } from "@/design-system/inp";
import { useServices } from "@/features/services/hooks";
import { resolveEnvKey } from "@/lib/canon-env";
import { cn } from "@/lib/utils";

/**
 * D7 · Object Browser — 08-api has no object endpoints although D7 requires
 * them (finding: spec change needed before browsing can go live); the prefix
 * listing below is frame-fixed canon from the D7 frame.
 */

interface ObjectRow {
  key: string;
  folder?: boolean;
  size: string;
  modified: string;
  selected?: boolean;
}

const OBJECTS: ObjectRow[] = [
  { key: "sku_2213/", folder: true, size: "—", modified: "—" },
  { key: "img_88213_hero.webp", size: "214 KB", modified: "Jul 5 · 13:58", selected: true },
  { key: "img_88213_thumb.webp", size: "18 KB", modified: "Jul 5 · 13:58" },
  { key: "img_79104_hero.webp", size: "198 KB", modified: "Jul 4 · 09:12" },
  { key: "manifest.json", size: "2.1 KB", modified: "Jul 4 · 09:12" },
];

function ObjectsPage() {
  const { project, service } = Route.useParams();
  const { env } = Route.useSearch();
  const services = useServices(resolveEnvKey(project, env));

  const svc = services.data?.find((s) => s.name === service || s.id === service);
  if (!svc) return <main className="main" />;
  if (svc.product !== "storage") {
    return (
      <main className="main">
        <div className="pgpad">
          <Card className="p-4 text-12 text-ink2">
            The Object Browser is an Object Storage surface — {svc.name} is a {svc.product} service.
          </Card>
        </div>
      </main>
    );
  }

  // The prefix listing is `assets`' frame-fixed canon — any other bucket
  // starts honest: empty until the first upload.
  if (svc.name !== "assets") {
    return (
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Pghead
            before={<Glyph id="s-bucket" />}
            title="Object Storage · Object Browser"
            sub={<span className="mono">{svc.name} · 0 objects</span>}
          />
          <EmptyState
            icon="s-bucket"
            title="Bucket is empty"
            meaning="objects appear on first upload · lifecycle rules apply from day one"
            cta={
              <Btn variant="s" disabled disabledReason="No object endpoints in the spec (finding)">
                Upload
              </Btn>
            }
            cli={`steloit storage cp ./hero.webp ${svc.name}/`}
          />
        </div>
      </main>
    );
  }

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        {/* Finding: the frame shows the bare "Object Browser" title — the design-system
            "Area · Thing" h1 grammar wins per the audit's P1 ruling. */}
        <Pghead
          before={<Glyph id="s-bucket" />}
          title="Object Storage · Object Browser"
          sub={<span className="mono">{svc.name} / products/ · 214,033 objects in prefix</span>}
        />

        <div className="flex items-center gap-3">
          <div className="w-[260px]">
            <Inp placeholder="Filter by key…" aria-label="Filter by key" />
          </div>
          <span className="flex-1" />
          <Btn variant="s" disabled disabledReason="No object endpoints in the spec (finding)">
            Upload
          </Btn>
        </div>

        <div className="flex gap-3.5">
          <Card className="flex-1 self-start">
            <div className="tblwrap">
              <table className="tbl">
                <thead>
                  <tr>
                    <th>Key</th>
                    <th>Size</th>
                    <th>Modified</th>
                  </tr>
                </thead>
                <tbody>
                  {OBJECTS.map((o) => (
                    <tr key={o.key}>
                      <td className={cn("mono", o.selected && "bg-steel-tint")}>
                        <span className="flex items-center gap-2">
                          {o.folder ? <Icon id="s-doc" className="h-3.5 w-3.5 text-ink3" /> : null}
                          {o.key}
                        </span>
                      </td>
                      <td className={cn("mono", o.selected && "bg-steel-tint")}>{o.size}</td>
                      <td className={cn("mono", o.selected && "bg-steel-tint")}>{o.modified}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </Card>

          <Card className="flex w-[320px] shrink-0 flex-col gap-3 self-start p-4">
            <Eyebrow>Preview</Eyebrow>
            <div className="flex h-[140px] items-center justify-center rounded-lg border border-hair border-dashed">
              <span className="mono text-11 text-ink3">img_88213_hero.webp · 1200×800</span>
            </div>
            <div className="flex flex-col gap-2 text-11p5">
              <div className="flex items-center justify-between">
                <span className="text-ink3">Content type</span>
                <span className="mono">image/webp</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-ink3">Uploaded by</span>
                <span className="mono">api · bnd_api_assets</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-ink3">Versions</span>
                <span className="mono">2</span>
              </div>
            </div>
            <Copybit>Signed URL · expires 15 min</Copybit>
            <div className="flex items-center gap-2.5">
              <Btn variant="s" disabled disabledReason="No object endpoints in the spec (finding)">
                Download
              </Btn>
              <Btn
                variant="dgr"
                disabled
                disabledReason="No object endpoints in the spec (finding)"
              >
                Delete…
              </Btn>
            </div>
            <p className="border-hair border-t pt-2.5 text-10p5 text-ink3">
              Browser actions are audited exactly like API calls.
            </p>
          </Card>
        </div>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/$project/svc/$service/objects")({
  component: ObjectsPage,
});
