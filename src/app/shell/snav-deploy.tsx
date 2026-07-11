import { Icon } from "@/design-system/icon";
import { Dot } from "@/design-system/pill";
import { Nfoot, NitLink, Snav } from "./snav";

/**
 * Snav variant — the Deploy domain (DP1–DP3): cross-service deployments and
 * previews for the whole project. The footer mirrors the in-flight rollout
 * (#143 canary) so it's visible from anywhere in the domain.
 */
export function SnavDeploy({
  org,
  project,
  env,
  active,
}: {
  org: string;
  project: string;
  env: string;
  active: "deployments" | "previews";
}) {
  const params = { org, project };
  const search = { env };

  return (
    <Snav>
      <div className="snhead">
        <span className="glyph">
          <Icon id="s-deploy" />
        </span>
        <div>
          <div className="t">Deploy</div>
          <div className="u">{project} · all services</div>
        </div>
      </div>
      <NitLink
        to="/$org/$project/deploy"
        params={params}
        search={search}
        icon="s-deploy"
        label="Deployments"
        on={active === "deployments"}
      />
      <NitLink
        to="/$org/$project/deploy/previews"
        params={params}
        search={search}
        icon="s-branch"
        label="Previews"
        count="1"
        on={active === "previews"}
      />
      <Nfoot>
        <div className="flex items-center gap-1.5 px-1.5">
          <Dot tone="prov" />
          <span>rolling out</span>
          <span className="mono ml-auto">#143 · 33%</span>
        </div>
        <div className="px-1.5 pt-1">
          fix/orders-query → production · gates healthy · follows #142
        </div>
      </Nfoot>
    </Snav>
  );
}
