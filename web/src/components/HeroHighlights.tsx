import { useState } from "react";
import { Check, Copy, Terminal } from "lucide-react";
import { DEVELOPERS_PATH, isPlainLeftClick } from "../seo";

export const GITHUB_REPO_URL = "https://github.com/ondrasalek/cjp";

export function GitHubMark({ className = "size-4" }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" className={className}>
      <path d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.53 1.032 1.53 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0 1 12 6.844a9.59 9.59 0 0 1 2.504.337c1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.02 10.02 0 0 0 22 12.017C22 6.484 17.522 2 12 2Z" />
    </svg>
  );
}

const badgeBase =
  "inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-[0.7rem] font-bold uppercase tracking-wide no-underline leading-5 transition";

/** Visual shortcut row: FREE · BEZ REKLAM · OPEN SOURCE · VEŘEJNÉ API */
export function ProjectBadges({ onOpenApi }: { onOpenApi: () => void }) {
  return (
    <ul aria-label="Vlastnosti projektu" className="flex flex-wrap items-center gap-1.5">
      <li>
        <span className={`${badgeBase} bg-emerald-600 text-white dark:bg-emerald-500 dark:text-emerald-950`}>
          Free
        </span>
      </li>
      <li>
        <span className={`${badgeBase} bg-blue-700 text-white dark:bg-blue-500 dark:text-slate-950`}>
          Bez reklam
        </span>
      </li>
      <li>
        <a
          href={GITHUB_REPO_URL}
          target="_blank"
          rel="noopener noreferrer"
          className={`${badgeBase} bg-slate-900 text-white hover:bg-slate-700 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white`}
        >
          <GitHubMark className="size-3.5" />
          Open source
        </a>
      </li>
      <li>
        <a
          href={DEVELOPERS_PATH}
          onClick={(e) => {
            if (isPlainLeftClick(e)) {
              e.preventDefault();
              onOpenApi();
            }
          }}
          className={`${badgeBase} bg-violet-700 text-white hover:bg-violet-600 dark:bg-violet-400 dark:text-violet-950 dark:hover:bg-violet-300`}
        >
          Veřejné API
        </a>
      </li>
    </ul>
  );
}

/** Small terminal widget showing a copy-pasteable API call. */
export function ApiQuickstartWidget({ onOpenApi }: { onOpenApi: () => void }) {
  const [copied, setCopied] = useState(false);
  const origin = typeof window !== "undefined" ? window.location.origin : "https://slovotvar.cz";
  const command = `curl ${origin}/api/v1/words/pes`;

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1800);
    } catch {
      // Clipboard unavailable (insecure context); the command stays selectable.
    }
  };

  return (
    <aside
      aria-labelledby="api-widget-heading"
      className="overflow-hidden rounded-2xl border border-slate-800 bg-slate-950 text-slate-100 shadow-2xs"
    >
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-slate-800 px-4 py-2">
        <h2
          id="api-widget-heading"
          className="inline-flex items-center gap-2 text-[0.8rem] font-semibold text-slate-300"
        >
          <Terminal className="size-4 text-violet-400" aria-hidden="true" />
          Vyzkoušejte bezplatné API v terminálu
        </h2>
        <a
          href={DEVELOPERS_PATH}
          onClick={(e) => {
            if (isPlainLeftClick(e)) {
              e.preventDefault();
              onOpenApi();
            }
          }}
          className="text-[0.78rem] font-semibold text-violet-300 no-underline hover:text-violet-200"
        >
          API &amp; Dokumentace →
        </a>
      </div>
      <div className="flex items-center gap-3 px-4 py-3">
        <pre className="min-w-0 flex-1 overflow-x-auto font-mono text-[0.82rem] sm:text-[0.875rem] leading-relaxed">
          <code>
            <span className="text-slate-500"># Bez registrace a bez API klíče</span>
            {"\n"}
            <span className="select-none text-emerald-400">$ </span>
            {command}
          </code>
        </pre>
        <button
          type="button"
          onClick={copy}
          aria-label={copied ? "Příkaz zkopírován" : "Kopírovat příkaz"}
          className="inline-grid size-9 shrink-0 place-items-center rounded-lg border border-slate-700 text-slate-300 transition hover:bg-slate-800 hover:text-white cursor-pointer"
        >
          {copied ? (
            <Check className="size-4 text-emerald-400" aria-hidden="true" />
          ) : (
            <Copy className="size-4" aria-hidden="true" />
          )}
        </button>
      </div>
    </aside>
  );
}
