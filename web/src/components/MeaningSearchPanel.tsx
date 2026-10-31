import { useEffect, useId, useState } from "react";
import type { SearchHit } from "../types";
import { isPlainLeftClick, wordHref } from "../seo";

const SAMPLE_QUERIES = [
  "psovita selma",
  "dospely clovek",
  "veda o vesmiru",
  "hudebni nastroj",
];

export function MeaningSearchPanel({ onSelectWord }: { onSelectWord: (word: string) => void }) {
  const inputId = useId();
  const [query, setQuery] = useState("psovita selma");
  const [hits, setHits] = useState<SearchHit[]>([]);
  const [relaxed, setRelaxed] = useState(false);
  const [loading, setLoading] = useState(false);

  const runSearch = async (q: string) => {
    const clean = q.trim();
    if (!clean) return;
    setQuery(clean);
    setLoading(true);
    try {
      const res = await fetch(`/api/v1/search?q=${encodeURIComponent(clean)}&limit=20`);
      if (res.ok) {
        const data = await res.json();
        setHits(data.hits ?? []);
        setRelaxed(Boolean(data.relaxed));
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    runSearch("psovita selma");
  }, []);

  return (
    <div className="lexeme-card-container rounded-2xl border border-slate-200 bg-white shadow-xs dark:border-slate-800 dark:bg-slate-900 space-y-4 sm:space-y-5">
      <div className="space-y-1.5">
        <h2 className="text-[clamp(1.2rem,1rem+1.2vw,1.5rem)] font-bold text-slate-900 dark:text-slate-100 text-balance">
          Fulltextové hledání ve významech slov (FTS5)
        </h2>
        <p className="max-w-[72ch] text-[0.9rem] sm:text-[0.9375rem] text-slate-600 dark:text-slate-400 text-pretty">
          Hledejte česká slova podle popisu nebo definice. Díky indexu SQLite FTS5 s tokenizérem{" "}
          <code>unicode61</code> funguje vyhledávání i zcela bez diakritiky.
        </p>
      </div>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          runSearch(query);
        }}
        className="flex flex-col sm:flex-row gap-2 sm:gap-2.5 max-w-xl"
      >
        <label htmlFor={inputId} className="sr-only">
          Hledaný popis
        </label>
        <input
          id={inputId}
          type="text"
          inputMode="search"
          enterKeyHint="search"
          autoComplete="off"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Např. psovita selma, hudebni nastroj…"
          className="min-w-0 flex-1 rounded-xl border border-slate-300 bg-slate-50 px-3.5 py-2.5 text-base text-slate-900 focus:border-blue-600 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100"
        />
        <button
          type="submit"
          className="min-h-11 sm:min-h-10 w-full sm:w-auto shrink-0 rounded-xl bg-blue-700 px-5 py-2.5 text-[0.9375rem] font-semibold text-white transition hover:bg-blue-800 dark:bg-blue-500 dark:text-slate-950 dark:hover:bg-blue-400 cursor-pointer"
        >
          Vyhledat
        </button>
      </form>

      <div className="pill-cloud items-center gap-1.5 sm:gap-2">
        {SAMPLE_QUERIES.map((sq) => (
          <button
            key={sq}
            type="button"
            onClick={() => runSearch(sq)}
            className="min-h-9 rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-[0.8125rem] text-slate-700 transition hover:border-blue-600 hover:text-blue-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300 cursor-pointer"
          >
            {sq}
          </button>
        ))}
      </div>

      {loading ? (
        <p className="text-[0.9rem] text-slate-500">Prohledávám definice…</p>
      ) : hits.length === 0 ? (
        <p className="text-[0.9rem] text-amber-700 dark:text-amber-400">
          Žádné odpovídající definice nebyly nalezeny.
        </p>
      ) : (
        <div className="space-y-3">
          {relaxed && (
            <p className="rounded-xl border border-blue-200 bg-blue-50/80 px-3.5 py-2 text-[0.83rem] text-blue-950 dark:border-blue-900/60 dark:bg-blue-950/40 dark:text-blue-200">
              Přesná kombinace všech zadaných slov se v jedné definici nevyskytuje – zobrazuji nejbližší příbuzné definice:
            </p>
          )}
          <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,18rem),1fr))] gap-3">
            {hits.map((h, i) => (
              <div
                key={`${h.lexeme_id}-${i}`}
                className="min-w-0 rounded-xl border border-slate-200 bg-slate-50/70 p-3.5 sm:p-4 dark:border-slate-800 dark:bg-slate-950/50 space-y-1.5"
              >
                <div className="flex flex-wrap items-baseline gap-2">
                  <a
                    href={wordHref(h.lemma)}
                    onClick={(e) => {
                      if (isPlainLeftClick(e)) {
                        e.preventDefault();
                        onSelectWord(h.lemma);
                      }
                    }}
                    className="text-start text-[1.05rem] font-bold text-blue-700 hover:underline dark:text-blue-400"
                  >
                    {h.lemma}
                  </a>
                  {(h.pos_cs || h.pos) && (
                    <span className="text-[0.78rem] text-slate-500 dark:text-slate-400">
                      {h.pos_cs || h.pos}
                    </span>
                  )}
                </div>
                <p className="text-[0.875rem] sm:text-[0.9rem] text-slate-700 dark:text-slate-300 text-pretty">
                  {h.definition}
                </p>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
