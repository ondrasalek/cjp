import { isPlainLeftClick, wordHref } from "../seo";

const FEATURED_WORD_GROUPS = [
  {
    category: "Podstatná jména (skloňování 1.–7. pád)",
    words: ["žena", "muž", "předseda", "soudce", "město", "moře", "píseň", "kost"],
  },
  {
    category: "Slovesa (časování a přechodníky)",
    words: ["být", "mít", "dělat", "hnát", "jít", "psát", "vědět", "chtít"],
  },
  {
    category: "Přídavná jména a příslovce (stupňování)",
    words: ["dobrý", "krásný", "malý", "velký", "horizontální", "rychle", "dobře"],
  },
  {
    category: "Homonyma a zajímavé slovotvary",
    words: ["stát", "tři", "kolej", "zámek", "koruna", "ženami", "horizontálněji"],
  },
];

export function SeoGuideSection({ onSelectWord }: { onSelectWord: (word: string) => void }) {
  return (
    <section
      aria-labelledby="cjp-seo-guide-heading"
      className="mt-6 rounded-2xl border border-slate-200 bg-white p-4 sm:p-6 shadow-2xs dark:border-slate-800 dark:bg-slate-900 space-y-4"
    >
      <div className="space-y-1.5">
        <h2
          id="cjp-seo-guide-heading"
          className="text-[1.05rem] sm:text-[1.15rem] font-bold text-slate-900 dark:text-slate-100 text-balance"
        >
          Slovotvar.cz – český slovník, tvar slova a slovotvary online
        </h2>
        <p className="max-w-[85ch] text-[0.85rem] sm:text-[0.875rem] text-slate-600 dark:text-slate-400 text-pretty">
          Otevřená jazyková databáze umožňuje okamžitě ověřit jakékoliv <strong>české slovo</strong>,
          zjistit správný <strong>tvar slova</strong> (všechny <strong>slovotvary</strong> v jednotném i
          množném čísle), tabulky pro <strong>skloňování</strong> v 1.–7. pádu,{" "}
          <strong>časování</strong> českých sloves, <strong>stupňování</strong> přídavných jmen a
          příslovcí, <strong>dělení na slabiky</strong>, výslovnost (IPA), synonyma, antonyma i
          překlady do cizích jazyků.
        </p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        {FEATURED_WORD_GROUPS.map((group) => (
          <div
            key={group.category}
            className="rounded-xl border border-slate-200/80 bg-slate-50/70 p-3 dark:border-slate-800 dark:bg-slate-950/50 space-y-2"
          >
            <h3 className="text-[0.82rem] font-semibold text-slate-700 dark:text-slate-300">
              {group.category}
            </h3>
            <div className="pill-cloud gap-1.5">
              {group.words.map((w) => (
                <a
                  key={w}
                  href={wordHref(w)}
                  onClick={(e) => {
                    if (isPlainLeftClick(e)) {
                      e.preventDefault();
                      onSelectWord(w);
                    }
                  }}
                  className="rounded-md border border-slate-200 bg-white px-2 py-0.5 text-[0.8rem] font-medium text-blue-700 no-underline transition hover:border-blue-600 hover:bg-blue-50 dark:border-slate-700 dark:bg-slate-900 dark:text-blue-400 dark:hover:border-blue-400"
                >
                  {w}
                </a>
              ))}
            </div>
          </div>
        ))}
      </div>

      <div className="space-y-2 pt-1 text-[0.85rem]">
        <details className="rounded-xl border border-slate-200 px-3.5 py-2.5 dark:border-slate-800">
          <summary className="cursor-pointer select-none font-semibold text-slate-800 dark:text-slate-200">
            Jak najít správný tvar slova (slovo tvar) a pád v češtině?
          </summary>
          <p className="mt-2 text-slate-600 dark:text-slate-400 text-pretty">
            Do vyhledávání můžete zadat základní české slovo (např. <em>žena</em>, <em>předseda</em>,{" "}
            <em>dělat</em>) i libovolný ohnutý tvar slova (např. <em>ženami</em>,{" "}
            <em>nejkrásnějšími</em>, <em>horizontálněji</em>) — a to česky s háčky a čárkami i zcela bez
            diakritiky. Slovník automaticky určí výchozí heslo a vypíše kompletní slovotvary.
          </p>
        </details>

        <details className="rounded-xl border border-slate-200 px-3.5 py-2.5 dark:border-slate-800">
          <summary className="cursor-pointer select-none font-semibold text-slate-800 dark:text-slate-200">
            Odkud pocházejí české slovotvary, dělení na slabiky a překlady?
          </summary>
          <p className="mt-2 text-slate-600 dark:text-slate-400 text-pretty">
            Databáze kombinuje otevřená data Českého Wikislovníku (CC BY-SA 4.0), morfologické tabulky
            UniMorph Czech (CC BY-SA 3.0), český slovník Hunspell a tezaurus synonym z projektu
            LibreOffice cs_CZ. Dělení českých slov na slabiky využívá slovníkové záznamy doplněné o
            české typografické vzory csTeX.
          </p>
        </details>
      </div>
    </section>
  );
}
