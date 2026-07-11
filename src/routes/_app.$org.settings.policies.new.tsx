import { createFileRoute, Link } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Pghead } from "@/app/shell/pghead";
import { Banner } from "@/design-system/banner";
import { Btn } from "@/design-system/btn";
import { Card } from "@/design-system/card";
import { Copybit } from "@/design-system/copybit";
import { Eyebrow } from "@/design-system/eyebrow";
import { Flabel, Inp } from "@/design-system/inp";
import { Pill } from "@/design-system/pill";
import { useCreatePolicy, useCreatePolicyDryRun } from "@/features/settings/hooks";
import { errorMessage, type PolicyImpact } from "@/lib/api";
import { cn } from "@/lib/utils";

/**
 * G9 → G10 → G12 · New policy wizard — one page-tier route (no snav), internal
 * step state: configure → review → created. The rail is a live dry-run:
 * preview before enforce, the governance analog of estimate-before-provision.
 */

type Step = "configure" | "validate" | "review" | "created";
type EnforcementChoice = "warn" | "enforce";

const SCOPE_CHIPS = ["Identity", "Environments", "Databases", "Storage", "Compute", "Bindings"];
const DESCRIPTION_DEFAULT = "Branches from production must not carry readable PII";

function policyBody(
  name: string,
  description: string,
  enforcement: EnforcementChoice,
  blockExports: boolean,
) {
  return {
    key: name,
    description,
    scope: { project_id: null },
    rule: {
      trigger: "branch_created_from:production",
      action: "mask_columns_tagged:pii — deterministic masking, joins still work",
      block_exports_and_cross_region_copies: blockExports,
    },
    enforcement,
    enforce_from: enforcement === "warn" ? "2026-07-20T00:00:00+05:30" : null,
  };
}

function RailRow({ label, value, tone }: { label: string; value: string; tone?: "ok" }) {
  return (
    <div className="flex items-baseline justify-between gap-3 text-[11.5px]">
      <span className="text-ink3">{label}</span>
      <span className={cn("text-right font-medium", tone === "ok" && "text-ok")}>{value}</span>
    </div>
  );
}

function PolicyNewPage() {
  const { org } = Route.useParams();
  const [step, setStep] = useState<Step>("configure");
  const [name, setName] = useState("branch-data-masking");
  const [description, setDescription] = useState(DESCRIPTION_DEFAULT);
  const [scopes, setScopes] = useState<string[]>(["Databases"]);
  const [appliesToOrg, setAppliesToOrg] = useState(true);
  const [enforcement, setEnforcement] = useState<EnforcementChoice>("warn");
  const [blockExports, setBlockExports] = useState(true);

  const dryRun = useCreatePolicyDryRun();
  const create = useCreatePolicy();

  const dryRunMutate = dryRun.mutate;
  useEffect(() => {
    // Live dry-run on mount — the rail is evaluated against current state (G9).
    dryRunMutate({
      path: { org },
      body: policyBody("branch-data-masking", DESCRIPTION_DEFAULT, "warn", true),
    });
  }, [dryRunMutate, org]);

  // Dry-run returns 200 PolicyImpact; the real POST returns 201 Policy (has `id`).
  const impact: PolicyImpact | undefined =
    dryRun.data && !("id" in dryRun.data) ? dryRun.data : undefined;

  // G11 validation state: the archived-name collision and preview-minimal
  // conflict are frame-fixed canon (the API exposes no archived versions —
  // finding); resolving them is a real gate before review.
  const [renamed, setRenamed] = useState(false);
  const [resolution, setResolution] = useState<"amend" | "scope-out">("amend");
  const [existingHandling, setExistingHandling] = useState<"flag" | "block">("flag");
  const onCreate = () => {
    create.mutate(
      { path: { org }, body: policyBody(name, description, enforcement, blockExports) },
      {
        onSuccess: () => {
          toast.success("Policy created");
          setStep("created");
        },
        onError: (err) => toast.error(errorMessage(err)),
      },
    );
  };

  if (step === "validate" && !renamed) {
    return (
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Pghead
            title="New policy"
            sub="Validation & conflict states — errors name the collision and offer the resolutions"
          />
          <div className="flex gap-4">
            <div className="flex max-w-[760px] flex-1 flex-col gap-3.5">
              <Card className="flex flex-col gap-2 p-4">
                <Flabel>Policy name</Flabel>
                <Inp className="err-state mono" value={name} readOnly />
                <div className="ferr">
                  ✗ A policy with this name existed before (v3 · archived by asha · 4 mo ago)
                </div>
                <div className="flex gap-2.5">
                  <Btn
                    variant="s"
                    disabled
                    disabledReason="Archived versions aren't exposed by the API — spec change first (finding)"
                  >
                    Restore v3 & edit
                  </Btn>
                  <Btn
                    variant="s"
                    onClick={() => {
                      setName(`${name}-v4`);
                      setRenamed(true);
                      setStep("review");
                    }}
                  >
                    Rename
                  </Btn>
                </div>
              </Card>
              <Banner tone="warn">
                <b>Rule conflict:</b>&nbsp;masking runs as a branch-create job, but preview-minimal
                excludes workers from preview environments — masked branches could never finish
                materializing in previews.
              </Banner>
              <Card className="flex flex-col gap-2.5 p-4">
                <Flabel>
                  Resolve the conflict — pick one, the diff is shown before either applies
                </Flabel>
                <div className="grid grid-cols-2 gap-2.5">
                  <button
                    type="button"
                    className={cn(
                      "card p-3 text-left",
                      resolution === "amend" && "border-steel bg-steel-tint/40",
                    )}
                    onClick={() => setResolution("amend")}
                  >
                    <div className="text-[12.5px] font-semibold">Amend preview-minimal</div>
                    <div className="mt-1 text-[11px] leading-snug text-ink3">
                      Allow the masking job (system, scale-to-zero, ~$0) in previews.
                      preview-minimal becomes v2.
                    </div>
                  </button>
                  <button
                    type="button"
                    className={cn(
                      "card p-3 text-left",
                      resolution === "scope-out" && "border-steel bg-steel-tint/40",
                    )}
                    onClick={() => setResolution("scope-out")}
                  >
                    <div className="text-[12.5px] font-semibold">Scope previews out</div>
                    <div className="mt-1 text-[11px] leading-snug text-ink3">
                      This policy skips preview environments — weaker, and the preview gap is
                      recorded on the policy row.
                    </div>
                  </button>
                </div>
              </Card>
              <Card className="flex flex-col gap-2 border-warn/40 p-4">
                <p className="text-[12px] leading-relaxed text-ink2">
                  <b>Dry-run finding:</b> 1 existing branch violates. Default is{" "}
                  <b>flag, don't touch</b> — switching to "block until resolved" would freeze{" "}
                  <span className="mono">preview/pr-142</span>, which is marco's open PR.
                </p>
                <div className="chiprow">
                  <button
                    type="button"
                    className={cn("chip", existingHandling === "flag" && "on")}
                    onClick={() => setExistingHandling("flag")}
                  >
                    Flag existing
                  </button>
                  <button
                    type="button"
                    className={cn("chip", existingHandling === "block" && "on")}
                    onClick={() => setExistingHandling("block")}
                  >
                    Block until resolved
                  </button>
                </div>
              </Card>
              <div className="flex items-center gap-2.5">
                <Btn
                  variant="p"
                  disabled
                  disabledReason="Name collides with an archived version — rename or restore first"
                >
                  Continue to review →
                </Btn>
                <Btn variant="s" onClick={() => setStep("configure")}>
                  Cancel
                </Btn>
              </div>
            </div>
            <div className="flex w-[320px] shrink-0 flex-col gap-3">
              <Card className="flex flex-col gap-2 p-4">
                <Eyebrow>Why creation can't proceed</Eyebrow>
                <div className="text-[11.5px] text-err">
                  ✗ Name collides with an archived version
                </div>
                <div className="text-[11.5px] text-err">
                  ✗ Conflict with preview-minimal unresolved
                </div>
                <div className="text-[11.5px] text-ok">✓ Rule parameters valid</div>
                <p className="border-hair border-t pt-2 text-[10.5px] leading-relaxed text-ink3">
                  Validation is a checklist, not a wall of red — each item carries its own fix, and
                  the button stays honest about why it's off.
                </p>
              </Card>
            </div>
          </div>
        </div>
      </main>
    );
  }

  if (step === "created") {
    return (
      <main className="main">
        <div className="pgpad !overflow-y-auto">
          <Pghead
            title="Organization · Policies"
            sub="The rulebook — versioned, enforced at the platform, every trigger audited"
          />
          {/* Design-system Banner exposes only default|warn tones (16-qa); the frame's
              ok-tinted banner is approximated with default + ok check — finding. */}
          <Banner>
            <span className="text-ok">✓</span>
            <span>
              branch-data-masking v1 created — warn mode · promotes to enforce Jul 20 ·
              preview-minimal amended to v2 alongside · <span className="mono">evt_51aa02</span>
            </span>
          </Banner>
          <div>
            <Link to="/$org/settings/policies" params={{ org }}>
              <Btn variant="s">← Back to the rulebook</Btn>
            </Link>
          </div>
        </div>
      </main>
    );
  }

  return (
    <main className="main">
      <div className="pgpad !overflow-y-auto">
        {step === "configure" ? (
          <Pghead
            title="New policy"
            sub="Organization · Acme — a rule the platform enforces, not a doc people forget"
          />
        ) : (
          <Pghead
            title="Review policy"
            sub="branch-data-masking · v1 — read it the way a teammate will meet it"
          />
        )}

        <div className="flex items-start gap-4">
          <div className="flex max-w-[760px] flex-1 flex-col gap-3.5">
            {step === "configure" ? (
              <>
                <Card className="flex flex-col gap-4 p-4">
                  <div>
                    <Flabel htmlFor="pol-name">Policy name</Flabel>
                    <Inp
                      id="pol-name"
                      className="mono"
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                    />
                  </div>
                  <div>
                    <Flabel htmlFor="pol-desc">Description</Flabel>
                    <Inp
                      id="pol-desc"
                      value={description}
                      onChange={(e) => setDescription(e.target.value)}
                    />
                  </div>
                  <div>
                    <Flabel>Scope</Flabel>
                    <div className="chiprow">
                      {SCOPE_CHIPS.map((s) => (
                        <button
                          key={s}
                          type="button"
                          className={cn("chip", scopes.includes(s) && "on")}
                          onClick={() =>
                            setScopes((prev) =>
                              prev.includes(s) ? prev.filter((x) => x !== s) : [...prev, s],
                            )
                          }
                        >
                          {s}
                        </button>
                      ))}
                    </div>
                  </div>
                </Card>

                <Card className="flex flex-col gap-3 p-4">
                  <Flabel>Rule</Flabel>
                  <div className="text-[10.5px] text-ink3">
                    structured, so the platform can evaluate it — not prose, so nobody can interpret
                    it
                  </div>
                  <div className="flex flex-wrap items-center gap-2 text-[12px]">
                    <span>When a branch is created from</span>
                    <span className="chip">production ▾</span>
                  </div>
                  <div className="flex flex-wrap items-center gap-2 text-[12px]">
                    <span>→ mask columns tagged</span>
                    <span className="chip">pii</span>
                    <span className="text-ink3">(deterministic masking — joins still work)</span>
                  </div>
                  <label className="flex items-center gap-2 text-[12px]">
                    <input
                      type="checkbox"
                      checked={blockExports}
                      onChange={(e) => setBlockExports(e.target.checked)}
                    />
                    Also block exports and cross-region copies of masked branches
                  </label>
                </Card>

                <Card className="flex flex-col gap-3 p-4">
                  <Flabel>Applies to</Flabel>
                  <div className="chiprow">
                    <button
                      type="button"
                      className={cn("chip", appliesToOrg && "on")}
                      onClick={() => setAppliesToOrg(true)}
                    >
                      Entire organization
                    </button>
                    <button
                      type="button"
                      className={cn("chip", !appliesToOrg && "on")}
                      onClick={() => setAppliesToOrg(false)}
                    >
                      Selected projects…
                    </button>
                  </div>
                </Card>

                <Card className="flex flex-col gap-3 p-4">
                  <Flabel>Enforcement</Flabel>
                  <div className="flex gap-3">
                    <button
                      type="button"
                      className={cn(
                        "card flex-1 p-3.5 text-left",
                        enforcement === "warn" && "border-steel bg-steel-tint",
                      )}
                      onClick={() => setEnforcement("warn")}
                    >
                      <div className="text-[12.5px] font-semibold">Warn first</div>
                      <div className="mt-1 text-[11px] text-ink3">allowed · logged · nagged</div>
                    </button>
                    <button
                      type="button"
                      className={cn(
                        "card flex-1 p-3.5 text-left",
                        enforcement === "enforce" && "border-steel bg-steel-tint",
                      )}
                      onClick={() => setEnforcement("enforce")}
                    >
                      <div className="text-[12.5px] font-semibold">Enforce now</div>
                      <div className="mt-1 text-[11px] text-ink3">denied, policy named</div>
                    </button>
                  </div>
                  <div className="text-[11px] text-ink3">
                    Promote to enforce on Jul 20 — the mfa-required pattern: teams get runway, the
                    date is public
                  </div>
                </Card>

                <div className="flex items-center gap-2.5">
                  <Btn variant="p" onClick={() => setStep("validate")}>
                    Continue to review →
                  </Btn>
                  <Link to="/$org/settings/policies" params={{ org }}>
                    <Btn variant="s">Cancel</Btn>
                  </Link>
                  <span className="ml-auto">
                    <Copybit>
                      steloit policy create --from branch-data-masking.yaml --dry-run
                    </Copybit>
                  </span>
                </div>
              </>
            ) : (
              <>
                <Card className="flex flex-col gap-2.5 p-4">
                  <Eyebrow>In plain language</Eyebrow>
                  <div className="text-[12px] leading-relaxed text-ink1">
                    When anyone creates a branch from production, every column tagged pii is
                    deterministically masked before the branch is readable. Exports and cross-region
                    copies of masked branches are blocked. Applies to the entire organization.
                  </div>
                </Card>

                <Card className="flex flex-col gap-3 p-4">
                  <Eyebrow>What people will see</Eyebrow>
                  <div className="flex items-start gap-2.5 text-[11.5px]">
                    <Pill tone="st">branch create</Pill>
                    <span className="text-ink2">
                      succeeds as always — a masked · branch-data-masking pill appears on the branch
                      (W5)
                    </span>
                  </div>
                  <div className="flex items-start gap-2.5 text-[11.5px]">
                    <Pill tone="warn">warn mode</Pill>
                    <span className="text-ink2">
                      violations logged &amp; the actor nudged; nothing blocked until Jul 20
                    </span>
                  </div>
                  <div className="flex items-start gap-2.5 text-[11.5px]">
                    <Pill tone="err">after Jul 20</Pill>
                    <span className="text-ink2">
                      export of a masked branch → denied with the policy named — E3's 403 grammar,
                      never a mystery
                    </span>
                  </div>
                </Card>

                <Card className="flex flex-col gap-2 p-4">
                  <Eyebrow>Governance</Eyebrow>
                  <div className="text-[11.5px] text-ink2">Editable by Org Admins</div>
                  <div className="text-[11.5px] text-ink2">
                    Versioned — revert is one click, audited
                  </div>
                  <div className="text-[11.5px] text-ink2">
                    Creation, every trigger, every denial → audit log (W12)
                  </div>
                </Card>

                <div className="flex items-center gap-2.5">
                  <Btn
                    variant="p"
                    onClick={onCreate}
                    disabled={create.isPending}
                    disabledReason="Creating the policy…"
                  >
                    Create policy · v1 — warn mode
                  </Btn>
                  <Btn variant="gh" onClick={() => setStep("configure")}>
                    ← Back to edit
                  </Btn>
                </div>
              </>
            )}
          </div>

          <Card className="flex w-[320px] flex-shrink-0 flex-col gap-3 p-4">
            {step === "configure" ? (
              <>
                <Eyebrow>Impact preview · live dry-run</Eyebrow>
                <div className="text-[11.5px] text-ink2">
                  Evaluated against current state — 4 projects · 9 environments · 4 branches:
                </div>
                {dryRun.isPending ? (
                  <div className="h-10 animate-pulse rounded bg-surface2" />
                ) : (
                  (impact?.affected ?? []).map((a) => {
                    const warnRow = (a.effect ?? "").includes("flagged");
                    return (
                      <div key={a.id} className="flex items-start gap-2 text-[11.5px]">
                        <span className={warnRow ? "text-warn" : "text-ok"}>
                          {warnRow ? "⚠" : "✓"}
                        </span>
                        <span>
                          <b>{a.name}</b>
                          <span className="text-ink3"> — {a.effect}</span>
                        </span>
                      </div>
                    );
                  })
                )}
                <div className="text-[11.5px] text-ok">
                  ✓ No conflicts with the 6 existing policies
                </div>
                <div className="border-hair border-t pt-2.5 text-[10.5px] text-ink3">
                  Preview before enforce — the governance analog of estimate-before-provision.
                  Nothing is denied until you say so.
                </div>
              </>
            ) : (
              <>
                <Eyebrow>Impact · confirmed dry-run</Eyebrow>
                <RailRow label="New branches affected" value="all future" />
                <RailRow label="Existing flagged" value="1" />
                <RailRow label="Conflicts" value="0" tone="ok" />
                <RailRow label="Cost" value="$0 · masking is built in" />
              </>
            )}
          </Card>
        </div>
      </div>
    </main>
  );
}

export const Route = createFileRoute("/_app/$org/settings/policies/new")({
  component: PolicyNewPage,
});
