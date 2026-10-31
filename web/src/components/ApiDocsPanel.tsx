import { useState } from "react";
import { Check, Copy } from "lucide-react";
import type { DataSourceInfo } from "../types";

const ENDPOINTS = [
  {
    route: "GET /api/v1/words/{word}?langs=en,ru",
    desc: "Kompletní slovníková karta (skloňování, časování, IPA, slabiky, významy, synonyma, překlady). Rozpozná i ohnuté tvary a dotazy bez diakritiky.",
    sample: "/api/v1/words/žena?langs=en,ru",
  },
  {
    route: "GET /api/v1/analyze?q={word}",
    desc: "Ověření platnosti českého tvaru, převod na základní lemmata a výpis mluvnických kategorií.",
    sample: "/api/v1/analyze?q=ženami",
  },
  {
    route: "GET /api/v1/autocomplete?q={prefix}&limit=10",
    desc: "Bleskový našeptávač hesel s automatickou podporou dotazů s diakritikou i bez ní.",
    sample: "/api/v1/autocomplete?q=zen&limit=8",
  },
  {
    route: "GET /api/v1/hyphenate?q={word}",
    desc: "Dělení libovolného českého slova na slabiky (kombinace slovníku a Liangova algoritmu).",
    sample: "/api/v1/hyphenate?q=astronaut",
  },
  {
    route: "GET /api/v1/translations/{word}?langs=en,ru,de",
    desc: "Překlady českého slova seskupené podle hesel a jednotlivých významů.",
    sample: "/api/v1/translations/pes?langs=en,ru,de",
  },
  {
    route: "GET /api/v1/search?q={query}",
    desc: "Fulltextové vyhledávání v definicích hesel přes SQLite FTS5.",
    sample: "/api/v1/search?q=psovita+selma",
  },
];

export function ApiDocsPanel({ sources }: { sources: DataSourceInfo[] }) {
  const [copiedRoute, setCopiedRoute] = useState<string | null>(null);
  const formatNum = (n: number) => new Intl.NumberFormat("cs-CZ").format(n);

  const handleCopy = async (route: string, cmd: string) => {
    try {
      await navigator.clipboard.writeText(cmd);
      setCopiedRoute(route);
      window.setTimeout(() => {
        setCopiedRoute((prev) => (prev === route ? null : prev));
      }, 1800);
    } catch {
      // Clipboard API unavailable
    }
  };

  return (
    <div className="space-y-5 sm:space-y-6">
      <div className="lexeme-card-container rounded-2xl border border-slate-200 bg-white shadow-xs dark:border-slate-800 dark:bg-slate-900 space-y-4 sm:space-y-5">
        <div className="space-y-1.5">
          <h2 className="text-[clamp(1.2rem,1rem+1.2vw,1.5rem)] font-bold text-slate-900 dark:text-slate-100 text-balance">
            Přehled REST API endpointů
          </h2>
          <p className="max-w-[72ch] text-[0.9rem] sm:text-[0.9375rem] text-slate-600 dark:text-slate-400 text-pretty">
            Backend v Go obsluhuje dotazy přímo nad lokální SQLite databází s odezvou v řádu stovek
            mikrosekund. Kliknutím na ukázku si otevřete živou JSON odpověď.
          </p>
        </div>

        <div className="endpoint-grid">
          {ENDPOINTS.map((ep) => {
            const curlCmd = `curl "${window.location.origin}${ep.sample}"`;
            const isCopied = copiedRoute === ep.route;
            return (
              <article
                key={ep.route}
                className="endpoint-card min-w-0 rounded-xl border border-slate-200 bg-slate-50/70 p-3.5 sm:p-4 dark:border-slate-800 dark:bg-slate-950/60 space-y-2.5 md:space-y-0"
              >
                <h3 className="min-w-0 font-mono text-[0.82rem] sm:text-[0.9rem] font-bold text-blue-700 dark:text-blue-400 break-all sm:break-words">
                  {ep.route}
                </h3>
                <p className="text-[0.85rem] sm:text-[0.875rem] text-slate-600 dark:text-slate-400 text-pretty">
                  {ep.desc}
                </p>
                <pre
                  tabIndex={0}
                  className="min-w-0 self-end overflow-x-auto overscroll-x-contain rounded-lg bg-slate-200/70 px-3 py-2 text-[0.76rem] sm:text-[0.8rem] dark:bg-slate-900"
                >
                  <code>{curlCmd}</code>
                </pre>
                <div className="flex flex-wrap items-center justify-between gap-2 pt-0.5">
                  <a
                    href={ep.sample}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex min-h-8 items-center text-[0.8125rem] font-semibold text-blue-700 hover:underline dark:text-blue-400"
                  >
                    Otevřít živý JSON →
                  </a>
                  <button
                    type="button"
                    onClick={() => handleCopy(ep.route, curlCmd)}
                    className="inline-flex min-h-8 items-center gap-1 rounded-lg border border-slate-200 bg-white px-2.5 py-1 text-[0.75rem] font-medium text-slate-600 transition hover:border-blue-600 hover:text-blue-700 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-300 cursor-pointer"
                  >
                    {isCopied ? (
                      <>
                        <Check className="size-3.5 text-emerald-600 dark:text-emerald-400" aria-hidden="true" />
                        Zkopírováno
                      </>
                    ) : (
                      <>
                        <Copy className="size-3.5" aria-hidden="true" />
                        Kopírovat curl
                      </>
                    )}
                  </button>
                </div>
              </article>
            );
          })}
        </div>
      </div>

      <div className="lexeme-card-container rounded-2xl border border-slate-200 bg-white shadow-xs dark:border-slate-800 dark:bg-slate-900 space-y-4">
        <h2 className="text-[clamp(1.2rem,1rem+1.2vw,1.5rem)] font-bold text-slate-900 dark:text-slate-100 text-balance">
          Otevřené zdroje dat a licenční atribuce
        </h2>
        <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,18rem),1fr))] gap-3.5 sm:gap-4">
          {sources.map((src) => (
            <article
              key={src.id}
              className="min-w-0 rounded-xl border border-slate-200 p-3.5 sm:p-4 dark:border-slate-800 space-y-2"
            >
              <div className="flex flex-wrap items-start justify-between gap-2">
                <h3 className="min-w-0 text-[0.95rem] sm:text-[0.98rem] font-bold text-slate-900 dark:text-slate-100">
                  {src.name}
                </h3>
                <span className="shrink-0 rounded-md bg-blue-50 px-2 py-0.5 text-[0.75rem] font-semibold text-blue-800 dark:bg-blue-950 dark:text-blue-300">
                  {src.license}
                </span>
              </div>
              <p className="text-[0.85rem] text-slate-600 dark:text-slate-400 text-pretty">
                {src.attribution}
              </p>
              <div className="flex flex-wrap items-center justify-between gap-2 text-[0.8rem] text-slate-500">
                <span>
                  Naimportováno záznamů: <strong>{formatNum(src.record_count)}</strong>
                </span>
                {src.url && (
                  <a
                    href={src.url.split(" / ")[0]}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="font-medium text-blue-700 hover:underline dark:text-blue-400"
                  >
                    Zdroj projektu →
                  </a>
                )}
              </div>
            </article>
          ))}
        </div>
      </div>
    </div>
  );
}
