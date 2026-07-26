import { useState } from "react";
import { Icon } from "./icon";

/** Mono command chip — click copies; if it can be pasted into a terminal, it is mono. */
export function Copybit({ children }: { children: string }) {
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    await navigator.clipboard.writeText(children);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <button type="button" className="copybit" onClick={copy} aria-label={`Copy: ${children}`}>
      {children}
      <Icon id={copied ? "s-check" : "s-copy"} />
    </button>
  );
}
