import { useId, useRef } from "react";
import { ArrowDown, Volume2 } from "lucide-react";
import type { ExampleSentence, LexemeDetail, Relation } from "../types";
import { ConjugationTable, DeclensionTable } from "./GrammarTables";
import { isPlainLeftClick, wordHref } from "../seo";

const GENDER_LABELS: Record<string, string> = {
  m: "rod mužský",
  f: "rod ženský",
  n: "rod střední",
  "m/f": "rod mužský/ženský",
};

const ANIMACY_LABELS: Record<string, string> = {
  anim: "životný",
  inan: "neživotný",
};

const ASPECT_LABELS: Record<string, string> = {
  impf: "nedokonavé",
  pf: "dokonavé",
  biasp: "obouvidové",
};

export function SyllablePills({ syllables }: { syllables: string[] }) {
  return (
    <span
      className="inline-flex flex-wrap items-center gap-1"
      aria-label={`Dělení na slabiky: ${syllables.join("-")}`}
    >
      {syllables.map((part, idx) => (
        <span key={`${part}-${idx}`} className="inline-flex items-center gap-1">
          {idx > 0 && <span className="font-bold text-slate-400">·</span>}
          <span className="rounded-md bg-slate-100 px-2 py-0.5 text-[0.85rem] font-semibold text-slate-800 dark:bg-slate-800 dark:text-slate-200">
            {part}
          </span>
        </span>
      ))}
    </span>
  );
}

function SenseExamples({ examples }: { examples: ExampleSentence[] }) {
  // Prefer showing up to 1 concise inline example (<= 150 chars), and tuck long literary excerpts into <details>
  const shortInline = examples[0] && examples[0].text.length <= 150 ? examples[0] : null;
  const extraExamples = shortInline ? examples.slice(1, 3) : examples.slice(0, 2);

  return (
    <div className="mt-1.5 space-y-1.5">
      {shortInline && (
        <blockquote className="border-l-2 border-slate-300 pl-3 text-[0.875rem] text-slate-600 dark:border-slate-700 dark:text-slate-400">
          <p className="italic text-pretty">{shortInline.text}</p>
          {shortInline.ref && (
            <cite className="mt-0.5 block text-[0.76rem] not-italic text-slate-500">
              — {shortInline.ref}
            </cite>
          )}
        </blockquote>
      )}

      {extraExamples.length > 0 && (
        <details className="group text-[0.82rem]">
          <summary className="cursor-pointer select-none font-medium text-slate-500 hover:text-blue-700 dark:text-slate-400 dark:hover:text-blue-300">
            {shortInline
              ? `Další citace (${extraExamples.length})`
              : `Zobrazit citaci (${extraExamples.length})`}
          </summary>
          <div className="mt-1.5 space-y-1.5">
            {extraExamples.map((ex, idx) => (
              <blockquote
                key={idx}
                className="border-l-2 border-slate-300 pl-3 text-[0.85rem] text-slate-600 dark:border-slate-700 dark:text-slate-400"
              >
                <p className="italic text-pretty">{ex.text}</p>
                {ex.ref && (
                  <cite className="mt-0.5 block text-[0.75rem] not-italic text-slate-500">
                    — {ex.ref}
                  </cite>
                )}
              </blockquote>
            ))}
          </div>
        </details>
      )}
    </div>
  );
}

function RelationPills({
  title,
  items,
  onSelectWord,
}: {
  title: string;
  items?: Relation[];
  onSelectWord: (word: string) => void;
}) {
  if (!items || items.length === 0) return null;

  const unique: Relation[] = [];
  const seen = new Set<string>();
  for (const r of items) {
    if (!seen.has(r.word)) {
      seen.add(r.word);
      unique.push(r);
    }
  }

  return (
    <section className="space-y-2">
      <h3 className="text-[0.98rem] sm:text-[1.02rem] font-bold text-slate-900 dark:text-slate-100">
        {title}
      </h3>
      <div className="pill-cloud gap-1.5">
        {unique.slice(0, 28).map((rel) => (
          <a
            key={rel.word}
            href={wordHref(rel.word)}
            onClick={(e) => {
              if (isPlainLeftClick(e)) {
                e.preventDefault();
                onSelectWord(rel.word);
              }
            }}
            className="inline-flex min-h-8.5 items-center rounded-lg border border-slate-200 bg-slate-50 px-2.5 py-1 text-start text-[0.83rem] font-medium text-slate-800 no-underline transition hover:border-blue-600 hover:text-blue-700 dark:border-slate-700 dark:bg-slate-800/80 dark:text-slate-200 dark:hover:border-blue-400 dark:hover:text-blue-300"
          >
            {rel.word}
            {rel.qualifier && (
              <span className="ml-1 text-[0.74rem] text-slate-500 dark:text-slate-400">
                ({rel.qualifier})
              </span>
            )}
          </a>
        ))}
      </div>
    </section>
  );
}

export function LexemeCard({
  entry,
  onSelectWord,
}: {
  entry: LexemeDetail;
  onSelectWord: (word: string) => void;
}) {
  const grammarId = useId();
  const grammarRef = useRef<HTMLDivElement | null>(null);
  const audioRef = useRef<HTMLAudioElement | null>(null);

  const playAudio = () => {
    if (!entry.audio_url) return;
    if (!audioRef.current || audioRef.current.src !== entry.audio_url) {
      audioRef.current = new Audio(entry.audio_url);
    }
    audioRef.current.currentTime = 0;
    audioRef.current.play().catch(() => {});
  };

  // Group translations by language code
  const translationsByLang = new Map<string, string[]>();
  if (entry.translations?.length) {
    for (const tr of entry.translations) {
      const code = tr.lang_code.toUpperCase();
      const arr = translationsByLang.get(code) ?? [];
      if (!arr.includes(tr.word)) arr.push(tr.word);
      translationsByLang.set(code, arr);
    }
  }

  const hasDeclension = Boolean(entry.inflection?.declension?.length);
  const hasConjugation = Boolean(entry.inflection?.conjugation);
  const hasDegrees = Boolean(entry.inflection?.degrees);
  const hasGenerated = Boolean(!entry.forms?.length && entry.generated_forms?.length);

  const hasGrammarPanel = hasDeclension || hasConjugation || hasDegrees || hasGenerated;

  const hasExtras =
    translationsByLang.size > 0 ||
    Boolean(entry.synonyms?.length) ||
    Boolean(entry.antonyms?.length) ||
    Boolean(entry.related?.length) ||
    Boolean(entry.etymology);

  const grammarJumpLabel = hasDeclension
    ? "Skloňování"
    : hasConjugation
      ? "Časování"
      : hasDegrees
        ? "Stupňování"
        : "Tvary";

  return (
    <article className="lexeme-card-container rounded-2xl border border-slate-200 bg-white shadow-xs dark:border-slate-800 dark:bg-slate-900">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:flex-wrap sm:items-center sm:justify-between gap-2.5 sm:gap-4 border-b border-slate-200 pb-3.5 mb-4 sm:mb-5 dark:border-slate-800">
        <div className="flex min-w-0 flex-wrap items-baseline gap-2">
          <h2 className="lexeme-title font-extrabold tracking-tight text-slate-900 dark:text-slate-50 break-words">
            {entry.lemma}
          </h2>
          {(entry.pos_cs || entry.pos) && (
            <span className="rounded-lg bg-blue-50 px-2.5 py-0.5 text-[0.8rem] font-semibold text-blue-800 dark:bg-blue-950/80 dark:text-blue-300">
              {entry.pos_cs || entry.pos}
            </span>
          )}
          {entry.gender && GENDER_LABELS[entry.gender] && (
            <span className="rounded-lg bg-slate-100 px-2.5 py-0.5 text-[0.8rem] font-medium text-slate-600 dark:bg-slate-800 dark:text-slate-300">
              {GENDER_LABELS[entry.gender]}
            </span>
          )}
          {entry.animacy && ANIMACY_LABELS[entry.animacy] && (
            <span className="rounded-lg bg-slate-100 px-2.5 py-0.5 text-[0.8rem] font-medium text-slate-600 dark:bg-slate-800 dark:text-slate-300">
              {ANIMACY_LABELS[entry.animacy]}
            </span>
          )}
          {entry.aspect && ASPECT_LABELS[entry.aspect] && (
            <span className="rounded-lg bg-slate-100 px-2.5 py-0.5 text-[0.8rem] font-medium text-slate-600 dark:bg-slate-800 dark:text-slate-300">
              {ASPECT_LABELS[entry.aspect]}
            </span>
          )}
        </div>

        <div className="flex flex-wrap items-center gap-2 sm:gap-3 text-[0.875rem] text-slate-600 dark:text-slate-400">
          {entry.syllables && entry.syllables.length > 0 && (
            <SyllablePills syllables={entry.syllables} />
          )}
          {entry.ipa && <span className="font-mono text-[0.875rem]">{entry.ipa}</span>}
          {entry.audio_url && (
            <button
              type="button"
              onClick={playAudio}
              className="inline-flex min-h-8.5 items-center gap-1.5 rounded-lg border border-slate-200 bg-slate-50 px-2.5 py-1 text-[0.8rem] font-medium text-slate-700 transition hover:border-blue-600 hover:text-blue-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:border-blue-400 cursor-pointer"
            >
              <Volume2 className="size-4 shrink-0" aria-hidden="true" />
              Výslovnost
            </button>
          )}
          {hasGrammarPanel && Boolean(entry.senses?.length) && (
            <a
              href={`#${grammarId}`}
              onClick={(e) => {
                e.preventDefault();
                grammarRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
              }}
              className="grammar-jump-link min-h-8.5 items-center gap-1 rounded-lg border border-blue-200 bg-blue-50 px-2.5 py-1 text-[0.8rem] font-semibold text-blue-800 no-underline transition hover:bg-blue-100 dark:border-blue-800 dark:bg-blue-950/70 dark:text-blue-300"
            >
              <ArrowDown className="size-3.5 shrink-0" aria-hidden="true" />
              {grammarJumpLabel}
            </a>
          )}
        </div>
      </div>

      {/* Container-Query 3-Area Grid:
          1) Meanings (senses)
          2) Grammar Tables (grammar) — immediately after meanings on narrow screens, right column on >= 43rem!
          3) Translations, Synonyms, Antonyms, Related, Etymology (extras)
       */}
      <div
        className={
          hasGrammarPanel
            ? hasExtras
              ? "lexeme-grid lexeme-grid--split lexeme-grid--with-extras"
              : "lexeme-grid lexeme-grid--split"
            : "lexeme-grid"
        }
      >
        {/* Area 1: Meanings */}
        <div className={hasGrammarPanel ? "lexeme-area-senses min-w-0" : "min-w-0"}>
          {entry.senses && entry.senses.length > 0 ? (
            <section className="space-y-2">
              <h3 className="text-[0.98rem] sm:text-[1.02rem] font-bold text-slate-900 dark:text-slate-100">
                Významy a příklady užití
              </h3>
              <ol className="list-decimal space-y-2.5 pl-5 max-w-[72ch]">
                {entry.senses.map((sn, i) => (
                  <li
                    key={`${sn.sense_index}-${i}`}
                    className="text-[0.9375rem] sm:text-[0.96875rem] text-slate-800 dark:text-slate-200 text-pretty"
                  >
                    {sn.labels && sn.labels.length > 0 && (
                      <span className="mr-1.5 inline-flex flex-wrap gap-1">
                        {sn.labels.slice(0, 4).map((lbl) => (
                          <span
                            key={lbl}
                            className="rounded bg-slate-100 px-1.5 py-0.5 text-[0.74rem] italic text-slate-600 dark:bg-slate-800 dark:text-slate-400"
                          >
                            {lbl}
                          </span>
                        ))}
                      </span>
                    )}
                    <span>{sn.definition}</span>

                    {sn.examples && sn.examples.length > 0 && (
                      <SenseExamples examples={sn.examples} />
                    )}
                  </li>
                ))}
              </ol>
            </section>
          ) : (
            <div className="space-y-2 text-[0.9rem] text-slate-500 dark:text-slate-400">
              <p>
                Heslo je evidováno v otevřeném pravopisném a tvaroslovném slovníku ({entry.source}).
              </p>
              {entry.base_lemma && (
                <p className="flex flex-wrap items-center gap-1.5">
                  <span>Odvozeno ze základního slova:</span>
                  <a
                    href={wordHref(entry.base_lemma)}
                    onClick={(e) => {
                      if (isPlainLeftClick(e)) {
                        e.preventDefault();
                        onSelectWord(entry.base_lemma!);
                      }
                    }}
                    className="inline-flex min-h-8 items-center gap-1 rounded-lg border border-blue-200 bg-blue-50 px-2.5 py-0.5 text-[0.84rem] font-semibold text-blue-800 no-underline transition hover:bg-blue-100 dark:border-blue-800 dark:bg-blue-950/70 dark:text-blue-300"
                  >
                    {entry.base_lemma} →
                  </a>
                </p>
              )}
            </div>
          )}
        </div>

        {/* Area 2: Declension / Conjugation / Degrees */}
        {hasGrammarPanel && (
          <div
            id={grammarId}
            ref={grammarRef}
            className="lexeme-area-grammar min-w-0 scroll-mt-24 space-y-4"
          >
            {entry.inflection?.declension && entry.inflection.declension.length > 0 && (
              <DeclensionTable lemma={entry.lemma} rows={entry.inflection.declension} />
            )}

            {entry.inflection?.conjugation && (
              <ConjugationTable
                lemma={entry.lemma}
                conjugation={entry.inflection.conjugation}
              />
            )}

            {entry.inflection?.degrees && (
              <section className="rounded-xl border border-slate-200 p-3.5 sm:p-4 dark:border-slate-700/80 space-y-2">
                <h3 className="text-[0.95rem] font-bold text-slate-900 dark:text-slate-100">
                  Stupňování
                </h3>
                <ul className="space-y-1 text-[0.875rem] sm:text-[0.9rem]">
                  {[
                    ["1", "1. stupeň (pozitiv)"],
                    ["2", "2. stupeň (komparativ)"],
                    ["3", "3. stupeň (superlativ)"],
                  ].map(([deg, label]) => {
                    const vals = entry.inflection?.degrees?.[deg];
                    if (!vals?.length) return null;
                    return (
                      <li key={deg}>
                        <strong className="text-slate-600 dark:text-slate-400">
                          {label}:{" "}
                        </strong>
                        <span className="text-slate-900 dark:text-slate-100">
                          {vals.join(", ")}
                        </span>
                      </li>
                    );
                  })}
                </ul>
              </section>
            )}

            {(!entry.forms || entry.forms.length === 0) &&
              entry.generated_forms &&
              entry.generated_forms.length > 0 && (
                <section className="rounded-xl border border-slate-200 p-3.5 sm:p-4 dark:border-slate-800 space-y-2.5">
                  <h3 className="text-[0.95rem] font-bold text-slate-900 dark:text-slate-100">
                    Rozgenerované tvary ({entry.generated_forms.length})
                  </h3>
                  <div className="pill-cloud gap-1.5">
                    {entry.generated_forms.slice(0, 40).map((f) => (
                      <span
                        key={f}
                        className="rounded-md bg-slate-100 px-2 py-0.5 text-[0.82rem] text-slate-800 dark:bg-slate-800 dark:text-slate-200"
                      >
                        {f}
                      </span>
                    ))}
                  </div>
                </section>
              )}
          </div>
        )}

        {/* Area 3: Translations, Synonyms, Antonyms, Related, Etymology */}
        {hasExtras && (
          <div
            className={
              hasGrammarPanel
                ? "lexeme-area-extras min-w-0 space-y-5"
                : "min-w-0 space-y-5"
            }
          >
            {translationsByLang.size > 0 && (
              <section className="space-y-2">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <h3 className="text-[0.98rem] sm:text-[1.02rem] font-bold text-slate-900 dark:text-slate-100">
                    Překlady
                  </h3>
                  {entry.translations_from_lemma && (
                    <span className="inline-flex flex-wrap items-center gap-1.5 text-[0.8rem] text-slate-500 dark:text-slate-400">
                      <span>ze základního slova</span>
                      <a
                        href={wordHref(entry.translations_from_lemma)}
                        onClick={(e) => {
                          if (isPlainLeftClick(e)) {
                            e.preventDefault();
                            onSelectWord(entry.translations_from_lemma!);
                          }
                        }}
                        className="inline-flex min-h-7 items-center rounded-md border border-blue-200 bg-blue-50 px-2 py-0.5 font-semibold text-blue-800 no-underline transition hover:bg-blue-100 dark:border-blue-800 dark:bg-blue-950/70 dark:text-blue-300"
                      >
                        {entry.translations_from_lemma} →
                      </a>
                    </span>
                  )}
                </div>
                <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,13rem),1fr))] gap-2">
                  {Array.from(translationsByLang.entries()).map(([lang, words]) => (
                    <div
                      key={lang}
                      className="min-w-0 rounded-xl bg-slate-50 px-3.5 py-2 text-[0.875rem] dark:bg-slate-800/60"
                    >
                      <span className="mr-2 font-bold text-blue-700 dark:text-blue-400">
                        {lang}
                      </span>
                      <span className="text-slate-800 dark:text-slate-200">
                        {words.join(", ")}
                      </span>
                    </div>
                  ))}
                </div>
              </section>
            )}

            <RelationPills
              title="Synonyma"
              items={entry.synonyms}
              onSelectWord={onSelectWord}
            />
            <RelationPills
              title="Antonyma"
              items={entry.antonyms}
              onSelectWord={onSelectWord}
            />
            <RelationPills
              title="Příbuzná slova a spojení"
              items={entry.related}
              onSelectWord={onSelectWord}
            />

            {entry.etymology && (
              <details className="rounded-xl border border-slate-200 px-3.5 sm:px-4 py-2 dark:border-slate-800">
                <summary className="cursor-pointer py-0.5 text-[0.88rem] font-semibold text-slate-700 dark:text-slate-300">
                  Původ slova (etymologie)
                </summary>
                <p className="mt-2 max-w-[70ch] text-[0.875rem] text-slate-600 dark:text-slate-400 text-pretty">
                  {entry.etymology}
                </p>
              </details>
            )}
          </div>
        )}
      </div>
    </article>
  );
}
