import { useEffect, useId, useState } from "react";
import { CheckCircle2, XCircle } from "lucide-react";
import type { AnalyzeResponse, HyphenateResponse } from "../types";
import { SyllablePills } from "./LexemeCard";

const SAMPLE_WORDS = [
  "nejkrásnějšími",
  "kuchyňskými",
  "čokoláda",
  "nejneobhospodařovávatelnější",
  "Aakjaerovými",
];

export function AnalyzePanel({ onSelectWord }: { onSelectWord: (word: string) => void }) {
  const inputId = useId();
  const [input, setInput] = useState("nejkrásnějšími");
  const [analyzeData, setAnalyzeData] = useState<AnalyzeResponse | null>(null);
  const [hyphData, setHyphData] = useState<HyphenateResponse | null>(null);
  const [loading, setLoading] = useState(false);

  const runAnalysis = async (word: string) => {
    const clean = word.trim();
    if (!clean) return;
    setInput(clean);
    setLoading(true);
    try {
      const [aRes, hRes] = await Promise.all([
        fetch(`/api/v1/analyze?q=${encodeURIComponent(clean)}`),
        fetch(`/api/v1/hyphenate?q=${encodeURIComponent(clean)}`),
      ]);
      if (aRes.ok) setAnalyzeData(await aRes.json());
      if (hRes.ok) setHyphData(await hRes.json());
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    runAnalysis("nejkrásnějšími");
  }, []);

  return (
    <div className="lexeme-card-container rounded-2xl border border-slate-200 bg-white shadow-xs dark:border-slate-800 dark:bg-slate-900 space-y-4 sm:space-y-5">
      <div className="space-y-1.5">
        <h2 className="text-[clamp(1.2rem,1rem+1.2vw,1.5rem)] font-bold text-slate-900 dark:text-slate-100 text-balance">
          Morfologická analýza a dělení na slabiky
        </h2>
        <p className="max-w-[72ch] text-[0.9rem] sm:text-[0.9375rem] text-slate-600 dark:text-slate-400 text-pretty">
          Ověřte platnost libovolného českého slovního tvaru (včetně stupňovaných tvarů, předpon{" "}
          <code>ne-</code> / <code>nej-</code> či přivlastňovacích přípon) a rozložte slovo na
          slabiky.
        </p>
      </div>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          runAnalysis(input);
        }}
        className="flex flex-col sm:flex-row gap-2 sm:gap-2.5 max-w-xl"
      >
        <label htmlFor={inputId} className="sr-only">
          Slovo k analýze
        </label>
        <input
          id={inputId}
          type="text"
          autoComplete="off"
          autoCorrect="off"
          spellCheck={false}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="Zadejte libovolný český tvar…"
          className="min-w-0 flex-1 rounded-xl border border-slate-300 bg-slate-50 px-3.5 py-2.5 text-base text-slate-900 focus:border-blue-600 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100"
        />
        <button
          type="submit"
          className="min-h-11 sm:min-h-10 w-full sm:w-auto shrink-0 rounded-xl bg-blue-700 px-5 py-2.5 text-[0.9375rem] font-semibold text-white transition hover:bg-blue-800 dark:bg-blue-500 dark:text-slate-950 dark:hover:bg-blue-400 cursor-pointer"
        >
          Analyzovat
        </button>
      </form>

      <div className="pill-cloud items-center gap-1.5 sm:gap-2">
        {SAMPLE_WORDS.map((w) => (
          <button
            key={w}
            type="button"
            onClick={() => runAnalysis(w)}
            className="min-h-9 max-w-full truncate rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-[0.8125rem] text-slate-700 transition hover:border-blue-600 hover:text-blue-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300 cursor-pointer"
          >
            {w}
          </button>
        ))}
      </div>

      {loading && (
        <p className="text-[0.9rem] text-slate-500">Probíhá morfologická analýza…</p>
      )}

      {!loading && analyzeData && (
        <div className="space-y-4 sm:space-y-5 pt-1 sm:pt-2">
          <div
            className={`flex items-start sm:items-center gap-2.5 rounded-xl px-3.5 sm:px-4 py-3 text-[0.9rem] sm:text-[0.95rem] font-medium ${
              analyzeData.valid
                ? "bg-emerald-50 text-emerald-900 dark:bg-emerald-950/60 dark:text-emerald-300"
                : "bg-amber-50 text-amber-900 dark:bg-amber-950/60 dark:text-amber-300"
            }`}
          >
            {analyzeData.valid ? (
              <CheckCircle2 className="mt-0.5 sm:mt-0 size-5 shrink-0" aria-hidden="true" />
            ) : (
              <XCircle className="mt-0.5 sm:mt-0 size-5 shrink-0" aria-hidden="true" />
            )}
            <span className="min-w-0 break-words">
              {analyzeData.valid
                ? `Výraz „${analyzeData.query}“ je platný český tvar.`
                : `Výraz „${analyzeData.query}“ nebyl rozpoznán ve slovníku.`}
            </span>
          </div>

          {hyphData && (
            <section className="space-y-2">
              <h3 className="text-[0.98rem] sm:text-[1rem] font-bold text-slate-900 dark:text-slate-100">
                Dělení na slabiky ({hyphData.syllable_count})
              </h3>
              <SyllablePills syllables={hyphData.syllables} />
            </section>
          )}

          {analyzeData.lemmas && analyzeData.lemmas.length > 0 && (
            <section className="space-y-2">
              <h3 className="text-[0.98rem] sm:text-[1rem] font-bold text-slate-900 dark:text-slate-100">
                Nalezená základní hesla (lemmata)
              </h3>
              <div className="pill-cloud gap-2">
                {analyzeData.lemmas.map((lem) => (
                  <button
                    key={lem}
                    type="button"
                    onClick={() => onSelectWord(lem)}
                    className="min-h-10 rounded-lg border border-blue-200 bg-blue-50 px-3 py-1.5 text-[0.875rem] sm:text-[0.9rem] font-semibold text-blue-800 transition hover:bg-blue-100 dark:border-blue-800 dark:bg-blue-950/70 dark:text-blue-300 cursor-pointer"
                  >
                    {lem} → otevřít kartu
                  </button>
                ))}
              </div>
            </section>
          )}

          {analyzeData.morphology && analyzeData.morphology.length > 0 && (
            <section className="space-y-2">
              <h3 className="text-[0.98rem] sm:text-[1rem] font-bold text-slate-900 dark:text-slate-100">
                Mluvnické určení
              </h3>
              <ul className="list-disc space-y-1.5 pl-5 text-[0.875rem] sm:text-[0.9rem] text-slate-700 dark:text-slate-300">
                {analyzeData.morphology.map((m, idx) => {
                  const parts = [
                    m.pos_cs || m.pos,
                    m.case ? `${m.case}. pád (${m.case_name})` : "",
                    m.number === "sg" ? "jednotné č." : m.number === "pl" ? "množné č." : "",
                    m.person ? `${m.person}. osoba` : "",
                    m.degree ? `${m.degree}. stupeň` : "",
                  ].filter(Boolean);
                  return (
                    <li key={`${m.lexeme_id}-${idx}`}>
                      <strong>{m.lemma}</strong>:{" "}
                      {parts.length > 0 ? parts.join(", ") : "odvozený/základní tvar"}{" "}
                      <span className="text-slate-400">({m.source})</span>
                    </li>
                  );
                })}
              </ul>
            </section>
          )}
        </div>
      )}
    </div>
  );
}
