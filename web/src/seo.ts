import type { LexemeDetail, WordCardResponse } from "./types";

export const SITE_NAME = "Slovotvar.cz";

export const DEVELOPERS_PATH = "/vyvojari";

export const HOME_TITLE = "Slovotvar.cz | Skloňování online & Bezplatné API bez reklam";

export const HOME_DESCRIPTION =
  "Otevřený (Open Source) český slovník pro automatické skloňování a časování slov. Zcela zdarma, bez reklam a s rychlým veřejným API rozhraním pro vývojáře.";

export function wordHref(word: string): string {
  return `/slovo/${encodeURIComponent(word.trim())}`;
}

export function isPlainLeftClick(e: React.MouseEvent<HTMLAnchorElement>): boolean {
  return e.button === 0 && !e.metaKey && !e.ctrlKey && !e.shiftKey && !e.altKey;
}

export interface RouteState {
  word: string;
  isWordRoute: boolean;
  isDevelopersRoute?: boolean;
}

export function parseLocationRoute(): RouteState {
  const path = window.location.pathname;
  if (path.replace(/\/+$/, "") === DEVELOPERS_PATH) {
    return { word: "žena", isWordRoute: false, isDevelopersRoute: true };
  }
  if (path.startsWith("/slovo/")) {
    const raw = path.slice("/slovo/".length).replace(/\/+$/, "");
    if (raw) {
      try {
        const decoded = decodeURIComponent(raw).trim();
        if (decoded) {
          return { word: decoded, isWordRoute: true };
        }
      } catch {
        // Malformed URI sequence; fall through
      }
    }
  }

  const q = new URLSearchParams(window.location.search).get("q")?.trim();
  if (q) {
    return { word: q, isWordRoute: true };
  }

  return { word: "žena", isWordRoute: false };
}

function setMetaContent(selector: string, content: string) {
  const el = document.querySelector<HTMLMetaElement>(selector);
  if (el) {
    el.setAttribute("content", content);
  }
}

function setCanonical(url: string) {
  let link = document.querySelector<HTMLLinkElement>('link[rel="canonical"]');
  if (!link) {
    link = document.createElement("link");
    link.rel = "canonical";
    document.head.appendChild(link);
  }
  link.href = url;
}

function collectSampleWordForms(entry: LexemeDetail): string[] {
  const seen = new Set<string>([entry.lemma]);
  const out: string[] = [];
  const push = (w: string) => {
    const clean = w.trim();
    if (clean && !seen.has(clean)) {
      seen.add(clean);
      out.push(clean);
    }
  };

  for (const row of entry.inflection?.declension ?? []) {
    for (const s of row.singular ?? []) push(s);
    for (const p of row.plural ?? []) push(p);
  }
  if (entry.inflection?.conjugation) {
    for (const vals of Object.values(entry.inflection.conjugation.present_or_future ?? {})) {
      for (const v of vals) push(v);
    }
    for (const vals of Object.values(entry.inflection.conjugation.imperative ?? {})) {
      for (const v of vals) push(v);
    }
    for (const v of entry.inflection.conjugation.past_participle ?? []) push(v);
  }
  if (entry.inflection?.degrees) {
    for (const vals of Object.values(entry.inflection.degrees)) {
      for (const v of vals) push(v);
    }
  }
  for (const g of entry.generated_forms ?? []) {
    push(g);
  }
  return out.slice(0, 7);
}

const FAQ_ENTITIES = [
  {
    "@type": "Question",
    name: "Jak zjistit správný tvar slova a skloňování v češtině?",
    acceptedAnswer: {
      "@type": "Answer",
      text: "Zadejte libovolné české slovo v základním nebo ohnutém tvaru (i bez diakritiky). Slovník okamžitě zobrazí tabulku skloňování ve všech 7 pádech jednotného i množného čísla, časování sloves, stupňování a platné slovotvary.",
    },
  },
  {
    "@type": "Question",
    name: "Pozná vyhledávání ohnutý tvar slova (slovotvary) a vrátí základní heslo?",
    acceptedAnswer: {
      "@type": "Answer",
      text: "Ano. Morfologický analyzátor rozpozná ohnutý tvar slova (např. ženami, nejkrásnějšími, horizontálněji), určí pád, číslo, osobu či stupeň a zobrazí kartu výchozího českého hesla včetně překladů.",
    },
  },
  {
    "@type": "Question",
    name: "Jak funguje dělení českých slov na slabiky?",
    acceptedAnswer: {
      "@type": "Answer",
      text: "Každé české slovo je rozděleno na slabiky kombinací slovníkových dat Českého Wikislovníku a typografického algoritmu českých vzorů dělení slov (csTeX / LibreOffice).",
    },
  },
];

export function applyDocumentSEO(params: {
  activeTab: "dictionary" | "tools" | "fts" | "api";
  isWordRoute: boolean;
  queryWord: string;
  cardData: WordCardResponse | null;
}) {
  const origin = window.location.origin;
  const { activeTab, isWordRoute, queryWord, cardData } = params;

  let title = HOME_TITLE;
  let description = HOME_DESCRIPTION;
  let canonicalUrl = `${origin}/`;
  let robots = "index, follow, max-snippet:-1, max-image-preview:large";
  let jsonLdGraph: Record<string, unknown>[] = [
    {
      "@type": "WebSite",
      name: SITE_NAME,
      alternateName: ["Slovotvar", "Český slovník a slovotvary", "Skloňování a časování online"],
      inLanguage: "cs",
      url: `${origin}/`,
      potentialAction: {
        "@type": "SearchAction",
        target: `${origin}/slovo/{search_term_string}`,
        "query-input": "required name=search_term_string",
      },
    },
    {
      "@type": "FAQPage",
      mainEntity: FAQ_ENTITIES,
    },
    {
      "@type": "WebAPI",
      name: "Slovotvar.cz REST API",
      description:
        "Bezplatné veřejné REST API pro skloňování, časování, slovotvary, dělení na slabiky, synonyma a překlady českých slov.",
      documentation: `${origin}${DEVELOPERS_PATH}`,
      isAccessibleForFree: true,
      inLanguage: "cs",
      license: "https://opensource.org/licenses/MIT",
      provider: { "@type": "Organization", name: "OSLKS", url: "https://slks.cz" },
      sameAs: ["https://github.com/ondrasalek/cjp"],
    },
  ];

  if (activeTab === "tools") {
    title = "Morfologická analýza, tvar slova a dělení na slabiky | Slovotvar.cz";
    description =
      "Ověřte, zda české slovo nebo tvar slova existuje, zjistěte mluvnické kategorie (pád, číslo, rod, vid) a rozdělte libovolný český výraz na slabiky.";
  } else if (activeTab === "fts") {
    title = "Hledání českých slov podle významu a definice | Slovotvar.cz";
    description =
      "Fulltextové vyhledávání českých slov podle popisu a definice významu (funguje česky s diakritikou i bez diakritiky).";
  } else if (activeTab === "api") {
    title = "API & Dokumentace – bezplatné API pro skloňování a časování | Slovotvar.cz";
    description =
      "Veřejné REST API zdarma a bez API klíče: skloňování, časování, slovotvary, dělení na slabiky, synonyma a překlady českých slov. Open source, bez reklam.";
    canonicalUrl = `${origin}${DEVELOPERS_PATH}`;
  } else if (isWordRoute) {
    const cleanQ = queryWord.trim();
    canonicalUrl = `${origin}${wordHref(cleanQ)}`;

    if (cardData?.error) {
      // Prevent Soft-404 indexing for unknown queries
      robots = "noindex, follow";
      title = `${cleanQ} – hledání slova | Slovotvar.cz`;
      description = `Výsledek hledání českého výrazu „${cleanQ}“ a algoritmické dělení na slabiky.`;
    } else if (cardData?.entries && cardData.entries.length > 0) {
      const primary = cardData.entries[0];
      const posLabel = primary.pos_cs || primary.pos || "české heslo";
      const grammarFocus =
        primary.pos === "verb"
          ? "časování, tvar slova, slovotvary"
          : primary.pos === "adj" || primary.pos === "adv"
            ? "stupňování, skloňování, tvar slova"
            : "skloňování, tvar slova, slovotvary";

      if (cardData.exact_lemma_match) {
        title = `${primary.lemma} – ${grammarFocus}, význam a slabiky | Slovotvar.cz`;
        canonicalUrl = `${origin}${wordHref(primary.lemma)}`;
      } else {
        title = `${cleanQ} – tvar slova „${primary.lemma}“ (${grammarFocus}) | Slovotvar.cz`;
      }

      const defSnippet = primary.senses?.[0]?.definition
        ? ` Význam: ${primary.senses[0].definition.slice(0, 110)}.`
        : "";
      const sampleForms = collectSampleWordForms(primary);
      const formsSnippet =
        sampleForms.length > 0 ? ` Slovotvary: ${sampleForms.join(", ")}.` : "";
      const syllSnippet = primary.hyphenation ? `, slabiky ${primary.hyphenation}` : "";

      description =
        `České slovo „${primary.lemma}“ (${posLabel}${syllSnippet}).${defSnippet}${formsSnippet}`.slice(
          0,
          280
        );

      jsonLdGraph = [
        jsonLdGraph[0],
        {
          "@type": "DefinedTerm",
          name: primary.lemma,
          description: primary.senses?.[0]?.definition || description,
          inLanguage: "cs",
          url: canonicalUrl,
          inDefinedTermSet: {
            "@type": "DefinedTermSet",
            name: "Česká slovníková a tvaroslovná databáze",
            url: `${origin}/`,
          },
        },
      ];
    }
  }

  document.title = title;
  setMetaContent('meta[name="description"]', description);
  setMetaContent('meta[name="robots"]', robots);
  setMetaContent('meta[property="og:title"]', title);
  setMetaContent('meta[property="og:description"]', description);
  setMetaContent('meta[name="twitter:title"]', title);
  setMetaContent('meta[name="twitter:description"]', description);
  setCanonical(canonicalUrl);

  const ldScript = document.getElementById("seo-jsonld");
  if (ldScript) {
    ldScript.textContent = JSON.stringify({
      "@context": "https://schema.org",
      "@graph": jsonLdGraph,
    });
  }
}
