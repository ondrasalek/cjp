# CJP – Otevřená česká slovníková a tvaroslovná databáze (Go + SQLite + React)

Rychlá lokální databáze českého jazyka, REST API v **Go** nad **SQLite (FTS5)** a responzivní webové rozhraní ve **Vite + React 19 + TypeScript + Tailwind CSS v4**.

Projekt vznikl jako čistě **open-source alternativa** postavená výhradně nad veřejně dostupnými otevřenými lingvistickými zdroji (bez scrapování uzavřených příruček a bez závislosti na externích API za běhu).

---

## Co aplikace a API umí (~300 000 hesel)

- **Slovníková hesla (`~299 500`)**: slovní druhy, jmenný rod, životnost, slovesný vid, výslovnost (IPA + audio nahrávky z Wikimedia Commons), dělení na slabiky a etymologie.
- **Skloňování, časování a stupňování (`~965 000` explicitních tvarů + Hunspell)**: přehledné tabulky 1.–7. pádu v jednotném i množném čísle, časování sloves (přítomný/budoucí čas, rozkazovací způsob, příčestí, přechodníky), stupňování přídavných jmen a příslovcí a dynamické rozgenerování tvarů z afixových pravidel `cs_CZ`.
- **Automatická lemmatizace a bezdiakritické dotazy**: vyhledá základní heslo i při zadání ohnutého tvaru (`ženami` → `žena`, `nejkrásnějšími` → `krásný`) nebo textu bez diakritiky (`zena` → `žena`). Homonymní ohnuté tvary jiných hesel (např. *žena* jako přechodník slovesa *hnát*) jsou přehledně vysvětleny.
- **Překlady s odvozením ze základního slova (`~235 000` překladů)**: překlady do `EN`, `RU`, `DE`, `SK`, `FR` a dalších jazyků. Pokud odvozené nebo stupňované heslo nemá vlastní překlad (např. `horizontálněji` → `horizontálnější` → `horizontální`, `kuchyňský` → `kuchyně`), automaticky dohledá překlad ze základního slova a transparentně u něj uvede zdrojové lemma.
- **Synonyma, antonyma a příbuzná slova (`~450 000` vazeb)**: sémantické vazby propojené na jedno kliknutí.
- **Fulltextové hledání ve významech (`~66 000` definic přes SQLite FTS5)**: hledání slov podle popisu (např. `psovita selma`, `veda o vesmiru`, `hudebni nastroj`) včetně filtrování českých předložek, tvaroslovného krácení koncovek a podpory bez diakritiky.

---

## Architektura projektu

```text
cjp/
├── cmd/
│   ├── cjp-etl/          # CLI nástroj pro stažení otevřených dat a sestavení SQLite databáze
│   └── cjp-api/          # HTTP REST API server v Go (čisté Go, CGO_ENABLED=0)
├── internal/
│   ├── api/              # HTTP routování, CORS, JSON endpointy
│   ├── db/               # SQLite schéma, FTS5 vyhledávání, skloňovací tabulky, odvozování překladů
│   ├── etl/              # Parser Kaikki Wiktionary JSONL, UniMorph TSV, Hunspell a tezauru
│   ├── hunspell/         # In-memory parser a generátor českých Hunspell afixů (.aff/.dic)
│   ├── hyphen/           # Dělení českých slov na slabiky (Liangův algoritmus nad hyph_cs_CZ)
│   └── textnorm/         # Unicode normalizace a odstraňování diakritiky
├── web/                  # Frontend (Vite + React 19 + TypeScript + Tailwind CSS v4 + Nginx)
├── compose.yaml          # Produkční/lokální orchestrace cjp-api + cjp-web
└── Dockerfile            # Multi-stage build Go binárek cjp-etl a cjp-api
```

---

## Rychlý start přes Docker Compose (doporučeno)

Stačí jediný příkaz. Při prvním spuštění si kontejner `cjp-api` automaticky stáhne surová otevřená data do `./data/raw` a během ~15 sekund vygeneruje indexovanou databázi `./data/cjp.db`:

```bash
docker compose up -d --build
```

Po spuštění běží:
- **Webové rozhraní (`cjp-web`)**: [http://localhost:5173](http://localhost:5173)
- **REST API (`cjp-api`)**: [http://localhost:8088](http://localhost:8088) *(dostupné také přes reverzní proxy webu na `http://localhost:5173/api/v1/...`)*

Porty na hostiteli lze změnit proměnnými prostředí:

```bash
WEB_PORT=8080 API_PORT=8088 docker compose up -d --build
```

---

## Lokální vývoj bez Dockeru

### Požadavky
- **Go 1.24+** (nevyžaduje C kompilátor – používá čistě Go SQLite driver `modernc.org/sqlite`)
- **Node.js 22+** a `npm`

### Postup

```bash
# 1. Stažení otevřených slovníkových dat do data/raw/
make fetch

# 2. Sestavení SQLite databáze data/cjp.db (~15 sekund)
make build-db

# 3. Spuštění Go REST API na http://localhost:8088
make run

# 4. V druhém terminálu spuštění Vite dev serveru na http://localhost:5173
cd web && npm install && npm run dev
```

Spuštění automatizovaných testů:

```bash
make test
```

---

## Nasazení na vlastní doménu (Coolify / Caddy / Traefik)

Soubor `compose.yaml` je připravený přímo pro **Coolify** (i libovolný jiný produkční Docker Compose orchestrátor):
- Používá pojmenovaný Docker volume `cjp-data:/data`, takže databáze se při prvním nasazení sama stáhne a sestaví (healthcheck má `start_period: 300s`) a při všech dalších deployích už zůstává trvale uložená.
- Lokální porty (`5173:80`, `8088:8080`) a bind-mount `./data:/data` jsou oddělené v `compose.override.yaml`, takže lokální `docker compose up` funguje dál beze změny a v Coolify nevznikají kolize portů při rolling updatech.
- **V Coolify**: přidejte projekt jako *Docker Compose*, nechte `compose.yaml` a doménu (např. `https://slovnik.slks.cz` s cílovým portem `:80`) přiřaďte službě **`cjp-web`**. Službu `cjp-api` není potřeba vystavovat ven – `cjp-web` do ní interně proxuje `/api/` i `/healthz`.

Příklad s vlastním **Caddy** (`Caddyfile`) mimo Coolify:

```caddyfile
slovnik.example.cz {
    reverse_proxy 127.0.0.1:5173
}
```

---

## Přehled REST API

| Metoda a cesta | Popis | Ukázka |
| :--- | :--- | :--- |
| `GET /api/v1/words/{word}` | Kompletní slovníková karta (významy, skloňování/časování, slabiky, IPA, synonyma, překlady s parametrem `?langs=en,ru,de,sk,fr`) | `/api/v1/words/žena?langs=en,de` |
| `GET /api/v1/analyze` | Morfologická analýza tvaru, ověření platnosti a převod na základní lemmata | `/api/v1/analyze?q=nejkrásnějšími` |
| `GET /api/v1/autocomplete` | Rychlý prefixový našeptávač s podporou dotazů s diakritikou i bez ní | `/api/v1/autocomplete?q=zen&limit=8` |
| `GET /api/v1/hyphenate` | Dělení českého slova na slabiky (slovník + Liangův algoritmus) | `/api/v1/hyphenate?q=čokoláda` |
| `GET /api/v1/translations/{word}` | Překlady slova seskupené podle významů (včetně fallbacku ze základního slova) | `/api/v1/translations/horizontálněji?langs=en,de` |
| `GET /api/v1/search` | Fulltextové vyhledávání v definicích (SQLite FTS5 + morfologické rozšíření) | `/api/v1/search?q=veda+o+vesmiru` |
| `GET /api/v1/meta` | Statistiky databáze a licenční atribuce všech naimportovaných zdrojů | `/api/v1/meta` |
| `GET /healthz` | Healthcheck endpoint | `/healthz` |

---

## Otevřené zdroje dat a licence

Zdrojový kód tohoto projektu (Go backend, ETL pipeline, React frontend) je licencován pod licencí **MIT** (viz [LICENSE](./LICENSE)).

Samotná jazyková data nejsou součástí Git repozitáře – stahují se skriptem `cjp-etl` z následujících otevřených zdrojů, které výslovně umožňují bezplatné i veřejné webové provozování při zachování uvedení původu (atribuce) a podmínek příslušných licencí:

| Zdroj dat | Licence | Využití v projektu |
| :--- | :--- | :--- |
| **[Český Wikislovník](https://cs.wiktionary.org)** *(extrakce [Kaikki.org](https://kaikki.org/dictionary/downloads/cs/))* | `CC BY-SA 4.0` | Definice, příklady, skloňování, časování, IPA, audio odkazy, synonyma, překlady |
| **[UniMorph Czech](https://github.com/unimorph/ces)** *(větev Wiktionary)* | `CC BY-SA 3.0` | Doplnění morfologických paradigmat |
| **[LibreOffice `hyph_cs_CZ.dic`](https://github.com/LibreOffice/dictionaries/tree/master/cs_CZ)** *(Pavel Janík, csTeX)* | `GPL` | Vzory pro algoritmické dělení českých slov na slabiky |
| **[LibreOffice `cs_CZ` Hunspell + Tezaurus](https://github.com/LibreOffice/dictionaries/tree/master/cs_CZ)** *(Petr Kolář, Miroslav Pošta a kol.)* | `GPL` / `GFDL` | Pravopisný slovník, afixová pravidla tvarů a rozšíření českých synonym |

> **Poznámka k licencím při dalším šíření dat:**
> - Veřejné provozování tohoto open-source webu a API na vlastní doméně je plně v souladu se všemi výše uvedenými licencemi – atribuce zdrojů je automaticky zobrazena v patičce webu, v záložce *REST API* i v endpointu `/api/v1/meta`.
> - Pokud by někdo chtěl distribuovat předsestavený soubor `cjp.db` bez dat pod licencí GPL/GFDL (pouze data `CC BY-SA`), stačí sestavit databázi příkazem `make build-db-cc` (`--include-gpl=false`) a spustit API s `CJP_USE_HUNSPELL=false`.

---

## Autor

© 2026 **OSLKS** — [slks.cz](https://slks.cz) • [github.com/ondrasalek](https://github.com/ondrasalek)
