import { useCallback, useEffect, useState } from "react";
import { BookOpen, Code2, Sparkles, TextSearch } from "lucide-react";
import type { DataSourceInfo, FormMatch, MetaResponse, WordCardResponse } from "./types";
import { ThemeToggle } from "./components/ThemeToggle";
import { SearchBar } from "./components/SearchBar";
import { LexemeCard } from "./components/LexemeCard";
import { AnalyzePanel } from "./components/AnalyzePanel";
import { MeaningSearchPanel } from "./components/MeaningSearchPanel";
import { ApiDocsPanel } from "./components/ApiDocsPanel";
import { SeoGuideSection } from "./components/SeoGuideSection";
import { ApiQuickstartWidget, GitHubMark, GITHUB_REPO_URL, ProjectBadges } from "./components/HeroHighlights";
import {
  applyDocumentSEO,
  DEVELOPERS_PATH,
  isPlainLeftClick,
  parseLocationRoute,
  wordHref,
} from "./seo";

type TabKey = "dictionary" | "tools" | "fts" | "api";

const AVAILABLE_LANGS = [
  { code: "en", label: "EN", title: "Angličtina" },
  { code: "ru", label: "RU", title: "Ruština" },
  { code: "de", label: "DE", title: "Němčina" },
  { code: "sk", label: "SK", title: "Slovenština" },
  { code: "fr", label: "FR", title: "Francouzština" },
];

export function App() {
  const [routeState, setRouteState] = useState(() => parseLocationRoute());
  const [activeTab, setActiveTab] = useState<TabKey>(() =>
    routeState.isDevelopersRoute ? "api" : "dictionary"
  );
  const word = routeState.word;
  const isWordRoute = routeState.isWordRoute;
  const [selectedLangs, setSelectedLangs] = useState<string[]>(["en", "ru"]);
  const [cardData, setCardData] = useState<WordCardResponse | null>(null);
  const [loadingWord, setLoadingWord] = useState(false);

  const [statsText, setStatsText] = useState("Načítám databázi…");
  const [sources, setSources] = useState<DataSourceInfo[]>([]);

  useEffect(() => {
    fetch("/api/v1/meta")
      .then((r) => (r.ok ? r.json() : null))
      .then((data: MetaResponse | null) => {
        if (!data) return;
        const fmt = new Intl.NumberFormat("cs-CZ");
        setStatsText(
          `${fmt.format(data.stats.lexemes)} hesel • ${fmt.format(data.stats.word_forms)} tvarů`
        );
        setSources(data.stats.sources ?? []);
      })
      .catch(() => setStatsText("API offline"));
  }, []);

  const fetchWordCard = useCallback(async (targetWord: string, langs: string[]) => {
    const clean = targetWord.trim();
    if (!clean) return;
    setLoadingWord(true);
    try {
      const params = new URLSearchParams();
      if (langs.length > 0) params.set("langs", langs.join(","));
      const res = await fetch(`/api/v1/words/${encodeURIComponent(clean)}?${params.toString()}`);
      const json: WordCardResponse = await res.json();
      setCardData(json);
    } catch {
      setCardData({
        query: clean,
        exact_lemma_match: false,
        error: "Nepodařilo se spojit se serverem API.",
      });
    } finally {
      setLoadingWord(false);
    }
  }, []);

  useEffect(() => {
    fetchWordCard(word, selectedLangs);
  }, [word, selectedLangs, fetchWordCard]);

  useEffect(() => {
    const onPopState = () => {
      const next = parseLocationRoute();
      setRouteState(next);
      setActiveTab(next.isDevelopersRoute ? "api" : "dictionary");
    };
    window.addEventListener("popstate", onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, []);

  useEffect(() => {
    applyDocumentSEO({
      activeTab,
      isWordRoute,
      queryWord: word,
      cardData: cardData && cardData.query === word ? cardData : null,
    });
  }, [activeTab, isWordRoute, word, cardData]);

  const handleSelectWord = (nextWord: string) => {
    const clean = nextWord.trim();
    if (!clean) return;
    setRouteState({ word: clean, isWordRoute: true });
    setActiveTab("dictionary");
    window.history.pushState({ q: clean }, "", wordHref(clean));
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  const handleGoHome = () => {
    setRouteState({ word: "žena", isWordRoute: false });
    setActiveTab("dictionary");
    window.history.pushState({}, "", "/");
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  /** Switches tabs; the API tab owns its own crawlable URL (/vyvojari). */
  const selectTab = (key: TabKey) => {
    setActiveTab(key);
    const onDevelopersUrl = window.location.pathname.replace(/\/+$/, "") === DEVELOPERS_PATH;
    if (key === "api" && !onDevelopersUrl) {
      window.history.pushState({}, "", DEVELOPERS_PATH);
    } else if (key !== "api" && onDevelopersUrl) {
      setRouteState({ word: "žena", isWordRoute: false });
      window.history.pushState({}, "", "/");
    }
  };

  const openApiDocs = () => {
    selectTab("api");
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  const toggleLang = (code: string) => {
    setSelectedLangs((prev) =>
      prev.includes(code) ? prev.filter((c) => c !== code) : [...prev, code]
    );
  };

  const navItems: {
    key: TabKey;
    label: string;
    shortLabel: string;
    Icon: typeof BookOpen;
  }[] = [
    { key: "dictionary", label: "Slovník & tvary", shortLabel: "Slovník", Icon: BookOpen },
    { key: "tools", label: "Analýza & slabiky", shortLabel: "Analýza", Icon: Sparkles },
    { key: "fts", label: "Hledat ve významech", shortLabel: "Významy", Icon: TextSearch },
    { key: "api", label: "API & Dokumentace", shortLabel: "API", Icon: Code2 },
  ];

  const formatFormMatchLabel = (fm: FormMatch) => {
    const nonfiniteLabel =
      fm.nonfinite === "transgr"
        ? fm.tense === "pres"
          ? "přechodník přítomný"
          : fm.tense === "past"
            ? "přechodník minulý"
            : "přechodník"
        : fm.nonfinite === "inf"
          ? "infinitiv"
          : fm.nonfinite === "part"
            ? "příčestí"
            : "";

    const parts = [
      nonfiniteLabel,
      fm.case ? `${fm.case}. pád (${fm.case_name})` : "",
      fm.number === "sg" ? "č. jednotné" : fm.number === "pl" ? "č. množné" : "",
      fm.gender === "m"
        ? "rod mužský"
        : fm.gender === "f"
          ? "rod ženský"
          : fm.gender === "n"
            ? "rod střední"
            : "",
      fm.person ? `${fm.person}. osoba` : "",
      fm.mood === "imp" ? "rozkazovací zp." : "",
      fm.degree === "2" ? "2. stupeň" : fm.degree === "3" ? "3. stupeň" : "",
    ].filter(Boolean);

    return parts.length > 0 ? `${fm.lemma} (${parts.join(", ")})` : fm.lemma;
  };

  const inflectedSummary = (() => {
    if (!cardData || cardData.exact_lemma_match || !cardData.form_matches?.length) return null;
    const items = cardData.form_matches.slice(0, 4).map(formatFormMatchLabel);
    return Array.from(new Set(items)).join(" • ");
  })();

  const { primaryEntries, secondaryInflectedEntries, secondaryInflectedSummary } = (() => {
    const all = cardData?.entries ?? [];
    if (!cardData || !cardData.exact_lemma_match) {
      return {
        primaryEntries: all,
        secondaryInflectedEntries: [],
        secondaryInflectedSummary: "",
      };
    }
    const qLower = cardData.query.trim().toLowerCase();
    const primary = all.filter((e) => e.lemma.toLowerCase() === qLower);
    const secondary = all.filter((e) => e.lemma.toLowerCase() !== qLower);
    if (primary.length === 0) {
      return {
        primaryEntries: all,
        secondaryInflectedEntries: [],
        secondaryInflectedSummary: "",
      };
    }
    const secIDs = new Set(secondary.map((e) => e.id));
    const secLabels = (cardData.form_matches ?? [])
      .filter((fm) => secIDs.has(fm.lexeme_id))
      .map(formatFormMatchLabel);
    const uniqueLabels =
      secLabels.length > 0
        ? Array.from(new Set(secLabels))
        : secondary.map((e) => e.lemma);

    return {
      primaryEntries: primary,
      secondaryInflectedEntries: secondary,
      secondaryInflectedSummary: uniqueLabels.slice(0, 4).join(" • "),
    };
  })();

  const langFilterControl = (
    <div
      role="group"
      aria-label="Jazyky překladů"
      className="flex flex-wrap items-center gap-1.5 text-[0.8125rem]"
    >
      <span className="mr-0.5 font-medium text-slate-500 dark:text-slate-400">Překlady:</span>
      {AVAILABLE_LANGS.map((l) => {
        const checked = selectedLangs.includes(l.code);
        return (
          <label
            key={l.code}
            title={l.title}
            className={`inline-flex min-h-8.5 items-center justify-center rounded-full border px-2.5 py-1 text-[0.78rem] font-bold transition cursor-pointer select-none has-focus-visible:ring-2 has-focus-visible:ring-blue-500 ${
              checked
                ? "border-blue-700 bg-blue-700 text-white shadow-2xs dark:border-blue-500 dark:bg-blue-500 dark:text-slate-950"
                : "border-slate-200 bg-white text-slate-500 hover:border-slate-300 hover:text-slate-800 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-400 dark:hover:text-slate-200"
            }`}
          >
            <input
              type="checkbox"
              checked={checked}
              onChange={() => toggleLang(l.code)}
              className="sr-only"
            />
            {l.label}
          </label>
        );
      })}
    </div>
  );

  return (
    <div className="min-h-dvh flex flex-col">
      {/* Sticky Header: single clean row from md (768px) upward, ultra-compact 2-row bar on phones */}
      <header className="sticky top-0 z-20 border-b border-slate-200 bg-white/95 backdrop-blur-md dark:border-slate-800 dark:bg-slate-900/95">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between gap-3 py-2.5">
            <a
              href="/"
              onClick={(e) => {
                if (isPlainLeftClick(e)) {
                  e.preventDefault();
                  handleGoHome();
                }
              }}
              className="inline-flex shrink-0 items-center gap-2.5 text-inherit no-underline"
            >
              <span
                aria-hidden="true"
                className="inline-grid size-8 sm:size-9 shrink-0 place-items-center rounded-xl bg-blue-700 text-[0.88rem] font-extrabold text-white dark:bg-blue-500 dark:text-slate-950"
              >
                S
              </span>
              <span className="flex flex-col leading-tight">
                <strong className="whitespace-nowrap text-[0.95rem] sm:text-[1rem] text-slate-900 dark:text-slate-100">
                  Slovotvar.cz
                </strong>
                <span className="hidden xl:inline whitespace-nowrap text-[0.74rem] text-slate-500 dark:text-slate-400">
                  Open source • zdarma • bez reklam
                </span>
              </span>
            </a>

            {/* Single-line Navigation from md (768px) upward — never wraps onto 2 lines */}
            <nav aria-label="Hlavní sekce" className="hidden md:block">
              <ul className="flex items-center gap-1 rounded-xl bg-slate-100/90 p-1 dark:bg-slate-800/80">
                {navItems.map(({ key, label, shortLabel, Icon }) => {
                  const active = activeTab === key;
                  return (
                    <li key={key}>
                      <NavControl
                        tab={key}
                        active={active}
                        onSelect={selectTab}
                        className={`inline-flex items-center gap-1.5 whitespace-nowrap rounded-lg px-3 lg:px-3.5 py-1.5 text-[0.84rem] font-semibold no-underline transition cursor-pointer ${
                          active
                            ? "bg-white text-blue-800 shadow-2xs dark:bg-slate-900 dark:text-blue-300"
                            : "text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-100"
                        }`}
                      >
                        <Icon className="size-4 shrink-0" aria-hidden="true" />
                        <span className="lg:hidden">{shortLabel}</span>
                        <span className="hidden lg:inline">{label}</span>
                      </NavControl>
                    </li>
                  );
                })}
              </ul>
            </nav>

            <div className="flex shrink-0 items-center gap-2">
              <span className="hidden 2xl:inline-block whitespace-nowrap rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-[0.75rem] font-medium text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300">
                {statsText}
              </span>
              <a
                href={GITHUB_REPO_URL}
                target="_blank"
                rel="noopener noreferrer"
                aria-label="Zdrojový kód na GitHubu (open source)"
                title="Open source na GitHubu"
                className="inline-flex h-10 sm:h-9 min-w-10 sm:min-w-9 shrink-0 items-center justify-center gap-1.5 rounded-xl sm:rounded-lg border border-slate-200 bg-white px-2.5 text-[0.8rem] font-semibold text-slate-700 no-underline transition hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800"
              >
                <GitHubMark className="size-4" />
                <span className="hidden xl:inline">GitHub</span>
              </a>
              <ThemeToggle />
            </div>
          </div>

          {/* Compact single-line mobile segmented bar (< 768px) */}
          <nav aria-label="Hlavní sekce" className="pb-2 md:hidden">
            <ul className="grid grid-cols-4 gap-1 rounded-xl bg-slate-100 p-1 dark:bg-slate-800/90">
              {navItems.map(({ key, shortLabel, Icon }) => {
                const active = activeTab === key;
                return (
                  <li key={key} className="min-w-0">
                    <NavControl
                      tab={key}
                      active={active}
                      onSelect={selectTab}
                      className={`inline-flex min-h-8.5 w-full items-center justify-center gap-1 whitespace-nowrap rounded-lg px-1.5 py-1 text-[0.78rem] font-semibold no-underline transition cursor-pointer ${
                        active
                          ? "bg-white text-blue-800 shadow-2xs dark:bg-slate-900 dark:text-blue-300"
                          : "text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-100"
                      }`}
                    >
                      <Icon className="hidden min-[420px]:block size-3.5 shrink-0" aria-hidden="true" />
                      <span className="truncate">{shortLabel}</span>
                    </NavControl>
                  </li>
                );
              })}
            </ul>
          </nav>
        </div>
      </header>

      {/* Main Content */}
      <main className="mx-auto w-full max-w-7xl flex-1 px-4 sm:px-6 lg:px-8 py-4 sm:py-6 space-y-4 sm:space-y-5">
        {activeTab === "dictionary" && (
          <>
            <section className="space-y-3">
              <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
                <h1 className="text-[clamp(1.2rem,1rem+1.1vw,1.75rem)] font-extrabold leading-snug tracking-tight text-slate-900 dark:text-slate-50">
                  Skloňování a časování českých slov online
                </h1>
                <span className="hidden sm:inline-block 2xl:hidden whitespace-nowrap text-[0.78rem] font-medium text-slate-500 dark:text-slate-400">
                  {statsText}
                </span>
              </div>

              <ProjectBadges onOpenApi={openApiDocs} />

              <p className="max-w-[70ch] text-[0.92rem] sm:text-[1rem] leading-relaxed text-slate-600 dark:text-slate-300 text-pretty">
                Bezplatný <strong className="text-slate-900 dark:text-slate-100">open-source</strong> slovník
                pro automatické skloňování a časování slov.{" "}
                <strong className="text-slate-900 dark:text-slate-100">100% bez reklam</strong> a s{" "}
                <strong className="text-slate-900 dark:text-slate-100">veřejným API</strong> pro vývojáře.
              </p>

              <SearchBar
                query={word}
                onSelectWord={handleSelectWord}
                langFilter={langFilterControl}
              />

              {!isWordRoute && <ApiQuickstartWidget onOpenApi={openApiDocs} />}
            </section>

            <section className="space-y-4">
              {loadingWord ? (
                <p className="text-[0.95rem] text-slate-500">Načítám heslo „{word}“…</p>
              ) : cardData?.error ? (
                <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3.5 text-[0.92rem] sm:text-[0.95rem] text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/60 dark:text-amber-200">
                  {cardData.error === "word not found" ? (
                    <>
                      <strong>Slovo „{cardData.query}“ nebylo v databázi nalezeno.</strong>
                      {cardData.syllables && cardData.syllables.length > 0 && (
                        <span className="block sm:inline sm:ml-2 mt-1 sm:mt-0">
                          Algoritmické dělení na slabiky:{" "}
                          <strong>{cardData.syllables.join("-")}</strong>
                        </span>
                      )}
                    </>
                  ) : cardData.error === "rate_limited" ? (
                    <strong>Příliš mnoho požadavků – zkuste to prosím za okamžik.</strong>
                  ) : (
                    <strong>{cardData.error}</strong>
                  )}
                </div>
              ) : (
                cardData && (
                  <div className="space-y-4 sm:space-y-5">
                    {inflectedSummary && (
                      <div className="rounded-xl border border-blue-200 bg-blue-50/90 px-3.5 sm:px-4 py-2.5 text-[0.875rem] sm:text-[0.92rem] text-blue-950 dark:border-blue-900/70 dark:bg-blue-950/50 dark:text-blue-200">
                        Zadaný výraz <strong>„{cardData.query}“</strong> (
                        {(cardData.syllables ?? []).join("-")}) je ohnutý tvar základního hesla:{" "}
                        <strong>{inflectedSummary}</strong>
                      </div>
                    )}

                    {cardData.ascii_fallback && (
                      <div className="rounded-xl border border-blue-200 bg-blue-50/90 px-3.5 sm:px-4 py-2.5 text-[0.875rem] sm:text-[0.92rem] text-blue-950 dark:border-blue-900/70 dark:bg-blue-950/50 dark:text-blue-200">
                        Zobrazeny výsledky nalezené doplněním diakritiky pro dotaz{" "}
                        <strong>„{cardData.query}“</strong>.
                      </div>
                    )}

                    {primaryEntries.map((entry) => (
                      <LexemeCard
                        key={entry.id}
                        entry={entry}
                        onSelectWord={handleSelectWord}
                      />
                    ))}

                    {secondaryInflectedEntries.length > 0 && (
                      <details className="group rounded-2xl border border-slate-200 bg-slate-50/85 px-4 py-3 text-[0.875rem] sm:text-[0.92rem] dark:border-slate-800 dark:bg-slate-900/60">
                        <summary className="cursor-pointer select-none font-medium text-slate-700 hover:text-blue-700 dark:text-slate-300 dark:hover:text-blue-400">
                          Výraz <strong>„{cardData.query}“</strong> je v češtině také ohnutým tvarem jiného hesla:{" "}
                          <strong className="text-blue-700 dark:text-blue-400">
                            {secondaryInflectedSummary}
                          </strong>{" "}
                          <span className="text-[0.8rem] font-normal text-slate-500 group-open:hidden">
                            — rozbalit kartu
                          </span>
                        </summary>
                        <div className="mt-3.5 space-y-4">
                          {secondaryInflectedEntries.map((entry) => (
                            <LexemeCard
                              key={entry.id}
                              entry={entry}
                              onSelectWord={handleSelectWord}
                            />
                          ))}
                        </div>
                      </details>
                    )}
                  </div>
                )
              )}
            </section>

            <SeoGuideSection onSelectWord={handleSelectWord} />
          </>
        )}

        {activeTab === "tools" && <AnalyzePanel onSelectWord={handleSelectWord} />}
        {activeTab === "fts" && <MeaningSearchPanel onSelectWord={handleSelectWord} />}
        {activeTab === "api" && <ApiDocsPanel sources={sources} />}
      </main>

      <footer className="mt-8 border-t border-slate-200 bg-white py-4 text-[0.8125rem] text-slate-500 dark:border-slate-800 dark:bg-slate-900 dark:text-slate-400">
        <div className="mx-auto max-w-7xl space-y-2.5 px-4 sm:px-6 lg:px-8">
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2">
            <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1 text-slate-600 dark:text-slate-300">
              <span className="font-semibold text-slate-800 dark:text-slate-100">© 2026 OSLKS</span>
              <span aria-hidden="true" className="text-slate-300 dark:text-slate-700">
                •
              </span>
              <a
                href="https://slks.cz"
                target="_blank"
                rel="noopener noreferrer"
                className="font-medium text-blue-700 hover:underline dark:text-blue-400"
              >
                slks.cz
              </a>
              <span aria-hidden="true" className="text-slate-300 dark:text-slate-700">
                •
              </span>
              <a
                href="https://github.com/ondrasalek"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1 font-medium text-blue-700 hover:underline dark:text-blue-400"
              >
                <svg
                  viewBox="0 0 24 24"
                  fill="currentColor"
                  aria-hidden="true"
                  className="size-3.5"
                >
                  <path d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.53 1.032 1.53 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0 1 12 6.844a9.59 9.59 0 0 1 2.504.337c1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.02 10.02 0 0 0 22 12.017C22 6.484 17.522 2 12 2Z" />
                </svg>
                github.com/ondrasalek
              </a>
            </div>

            <div className="flex items-center justify-between sm:justify-end gap-3">
              <span className="text-[0.75rem] sm:hidden">{statsText}</span>
              <a
                href={DEVELOPERS_PATH}
                onClick={(e) => {
                  if (isPlainLeftClick(e)) {
                    e.preventDefault();
                    openApiDocs();
                  }
                }}
                className="shrink-0 font-medium text-blue-700 hover:underline dark:text-blue-400"
              >
                API &amp; Dokumentace
              </a>
              <a
                href={GITHUB_REPO_URL}
                target="_blank"
                rel="noopener noreferrer"
                className="shrink-0 font-medium text-blue-700 hover:underline dark:text-blue-400"
              >
                Zdrojový kód (MIT)
              </a>
            </div>
          </div>

          <p className="text-[0.76rem] text-slate-500 dark:text-slate-400 text-pretty">
            Otevřená jazyková data:{" "}
            <a
              href="https://cs.wiktionary.org"
              target="_blank"
              rel="noopener noreferrer"
              className="underline hover:text-slate-800 dark:hover:text-slate-200"
            >
              <cite className="not-italic">Český Wikislovník</cite>
            </a>{" "}
            (CC BY-SA 4.0),{" "}
            <a
              href="https://github.com/unimorph/ces"
              target="_blank"
              rel="noopener noreferrer"
              className="underline hover:text-slate-800 dark:hover:text-slate-200"
            >
              <cite className="not-italic">UniMorph</cite>
            </a>{" "}
            (CC BY-SA 3.0),{" "}
            <a
              href="https://github.com/LibreOffice/dictionaries/tree/master/cs_CZ"
              target="_blank"
              rel="noopener noreferrer"
              className="underline hover:text-slate-800 dark:hover:text-slate-200"
            >
              <cite className="not-italic">LibreOffice cs_CZ</cite>
            </a>{" "}
            (GPL/GFDL).
          </p>
        </div>
      </footer>
    </div>
  );
}

/** Tab control: the API tab is a real crawlable link (/vyvojari), others are in-page buttons. */
function NavControl({
  tab,
  active,
  onSelect,
  className,
  children,
}: {
  tab: TabKey;
  active: boolean;
  onSelect: (tab: TabKey) => void;
  className: string;
  children: React.ReactNode;
}) {
  if (tab === "api") {
    return (
      <a
        href={DEVELOPERS_PATH}
        aria-current={active ? "page" : undefined}
        onClick={(e) => {
          if (isPlainLeftClick(e)) {
            e.preventDefault();
            onSelect(tab);
          }
        }}
        className={className}
      >
        {children}
      </a>
    );
  }
  return (
    <button type="button" aria-pressed={active} onClick={() => onSelect(tab)} className={className}>
      {children}
    </button>
  );
}
