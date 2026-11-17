import { LEGAL_TITLES, type LegalPageKey } from "../seo";

export const OPERATOR_NAME = "Ondřej Šálek";
export const CONTACT_EMAIL = "podpora@slovotvar.cz";

const linkClass = "font-medium text-blue-700 hover:underline dark:text-blue-400";

function Contact() {
  return (
    <a href={`mailto:${CONTACT_EMAIL}`} className={linkClass}>
      {CONTACT_EMAIL}
    </a>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="space-y-2">
      <h2 className="text-[1.05rem] font-bold text-slate-900 dark:text-slate-50">{title}</h2>
      {children}
    </section>
  );
}

function Terms() {
  return (
    <>
      <Section title="1. Provozovatel">
        <p>
          Provozovatelem služby Slovotvar.cz je {OPERATOR_NAME}. Kontakt a podpora:{" "}
          <Contact />.
        </p>
      </Section>
      <Section title="2. Předmět služby">
        <p>
          Slovotvar.cz je online slovník pro skloňování a časování českých slov s veřejným API.
          Služba je poskytována „tak, jak je“, bez záruky úplnosti, správnosti a nepřetržité
          dostupnosti. Provozovatel může službu, limity či rozsah API kdykoli změnit.
        </p>
      </Section>
      <Section title="3. Pravidla používání">
        <ul className="list-disc space-y-1 pl-5">
          <li>Službu i API je zakázáno používat způsobem, který ohrožuje její provoz.</li>
          <li>Dodržujte stanovené limity požadavků; při jejich překročení může být přístup omezen.</li>
          <li>Je zakázáno obcházet omezení, zneužívat přístupové klíče nebo narušovat zabezpečení.</li>
        </ul>
      </Section>
      <Section title="4. Licence dat a zdrojový kód">
        <p>
          Zdrojový kód je dostupný pod licencí MIT. Jazyková data pocházejí z otevřených zdrojů
          (Český Wikislovník, UniMorph, LibreOffice cs_CZ) a zůstávají pod svými původními
          licencemi (CC BY-SA, GPL/GFDL). Při dalším šíření dat je nutné uvést zdroj a dodržet
          podmínky těchto licencí. Podrobnosti jsou v dokumentaci API.
        </p>
      </Section>
      <Section title="5. Odpovědnost">
        <p>
          Provozovatel neodpovídá za škody vzniklé používáním služby nebo spoléháním na její
          výstupy, v rozsahu, v jakém to zákon připouští.
        </p>
      </Section>
      <Section title="6. Kontakt a změny podmínek">
        <p>
          Dotazy, připomínky a nahlášení problémů posílejte na <Contact />. Tyto podmínky mohou
          být aktualizovány; platí vždy aktuální znění zveřejněné na této stránce.
        </p>
      </Section>
    </>
  );
}

function Privacy() {
  return (
    <>
      <Section title="1. Správce údajů">
        <p>
          Správcem osobních údajů je {OPERATOR_NAME}. Kontakt pro dotazy k ochraně soukromí:{" "}
          <Contact />.
        </p>
      </Section>
      <Section title="2. Jaké údaje zpracováváme">
        <ul className="list-disc space-y-1 pl-5">
          <li>
            <strong>Technické údaje</strong> – IP adresa a údaje o požadavku (čas, adresa,
            prohlížeč), které jsou nezbytné pro provoz, zabezpečení a omezování počtu požadavků.
          </li>
          <li>
            <strong>Komunikace</strong> – e-mail a obsah zpráv, které nám pošlete na podporu.
          </li>
          <li>
            <strong>Platební a fakturační údaje</strong> – pouze pokud využíváte placené funkce
            API; platby zpracovává externí poskytovatel plateb a čísla karet neukládáme.
          </li>
        </ul>
        <p>Web nepoužívá reklamní ani sledovací cookies.</p>
      </Section>
      <Section title="3. Účel a právní základ">
        <p>
          Údaje zpracováváme pro provoz a zabezpečení služby (oprávněný zájem), vyřizování
          vašich dotazů (oprávněný zájem, případně plnění smlouvy) a plnění zákonných povinností.
        </p>
      </Section>
      <Section title="4. Příjemci údajů">
        <p>
          Údaje mohou zpracovávat poskytovatelé infrastruktury (hosting a síťová ochrana webu)
          a poskytovatel plateb. Údaje neprodáváme.
        </p>
      </Section>
      <Section title="5. Doba uchování">
        <p>
          Technické záznamy uchováváme jen po dobu nezbytnou pro provoz a zabezpečení. Komunikaci
          a doklady po dobu nutnou k vyřízení a podle zákonných lhůt.
        </p>
      </Section>
      <Section title="6. Vaše práva">
        <p>
          Máte právo na přístup k údajům, jejich opravu, výmaz, omezení zpracování, přenositelnost
          a právo vznést námitku. Žádosti posílejte na <Contact />. Máte také právo podat
          stížnost u Úřadu pro ochranu osobních údajů (
          <a
            href="https://uoou.gov.cz"
            target="_blank"
            rel="noopener noreferrer"
            className={linkClass}
          >
            uoou.gov.cz
          </a>
          ).
        </p>
      </Section>
    </>
  );
}

export function LegalPage({ page }: { page: LegalPageKey }) {
  return (
    <article className="mx-auto max-w-3xl space-y-5 text-[0.95rem] leading-relaxed text-slate-700 dark:text-slate-300">
      <h1 className="text-[clamp(1.3rem,1.1rem+1.1vw,1.9rem)] font-extrabold tracking-tight text-slate-900 dark:text-slate-50">
        {LEGAL_TITLES[page]}
      </h1>
      {page === "podminky" ? <Terms /> : <Privacy />}
    </article>
  );
}
