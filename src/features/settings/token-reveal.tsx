import { Btn } from "@/design-system/btn";
import { Copybit } from "@/design-system/copybit";
import { Glyph } from "@/design-system/glyph";

/**
 * U7 · Token reveal — the only time the plaintext exists in the UI.
 * Reused by P5 personal tokens and G8 org keys (the sub line differs);
 * no scrim dismiss on purpose — the single exit is "Done — I've stored it".
 */
export function TokenReveal({
  sub,
  token,
  onDone,
}: {
  /** e.g. "laptop-cli · full — your roles · expires in 90 d" (P5) or "{name} · org key" (G8). */
  sub: string;
  /** Plaintext from TokenCreated.token — shown once, never again. */
  token: string;
  onDone: () => void;
}) {
  return (
    <div
      className="fixed inset-0 z-40 flex items-center justify-center bg-[rgba(6,9,12,0.44)]"
      role="dialog"
      aria-label="Token created"
    >
      <div className="flex w-[460px] flex-col gap-4 rounded-xl border border-hair bg-surface p-5 shadow-e2">
        <div className="flex items-center gap-3">
          <Glyph id="s-key" />
          <div>
            <div className="text-[14px] font-semibold">Token created — copy it now</div>
            <div className="hsub">{sub}</div>
          </div>
        </div>
        <Copybit>{token}</Copybit>
        <div className="rounded-lg border border-hair bg-surface2 px-3.5 py-3 text-[11.5px] leading-relaxed text-ink2">
          Shown once. Steloit stores a hash, never the token. It acts as you — it carries your roles
          and shrinks the moment your roles do. Revoke anytime from this page.
        </div>
        <Btn variant="p" className="justify-center" onClick={onDone}>
          Done — I've stored it
        </Btn>
      </div>
    </div>
  );
}
