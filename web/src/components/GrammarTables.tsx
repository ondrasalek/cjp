import { useId } from "react";
import type { CaseRow, ConjugationTableData } from "../types";

export function DeclensionTable({ lemma, rows }: { lemma: string; rows: CaseRow[] }) {
  const captionId = useId();

  return (
    <div
      tabIndex={0}
      role="region"
      aria-labelledby={captionId}
      className="table-wrapper rounded-xl border border-slate-200 dark:border-slate-700/80"
    >
      <table className="grammar-table declension-table text-[0.9rem]">
        <caption
          id={captionId}
          className="border-b border-slate-200 bg-slate-50 px-4 py-2.5 text-start font-bold text-slate-800 dark:border-slate-700/80 dark:bg-slate-800/70 dark:text-slate-100"
        >
          Skloňování – {lemma}
        </caption>
        <thead>
          <tr className="bg-slate-50 text-slate-600 dark:bg-slate-800 dark:text-slate-300">
            <th scope="col" className="border-b border-slate-200 px-4 py-2 text-start font-semibold dark:border-slate-700">
              Pád
            </th>
            <th scope="col" className="border-b border-slate-200 px-4 py-2 text-start font-semibold dark:border-slate-700">
              Jednotné č.
            </th>
            <th scope="col" className="border-b border-slate-200 px-4 py-2 text-start font-semibold dark:border-slate-700">
              Množné č.
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
          {rows.map((r) => (
            <tr key={r.case}>
              <th
                scope="row"
                className="px-4 py-2 text-start font-semibold whitespace-nowrap text-slate-600 dark:text-slate-300"
              >
                {r.case}. {r.case_name}
              </th>
              <td className="px-4 py-2 text-slate-900 dark:text-slate-100">
                {r.singular && r.singular.length > 0 ? r.singular.join(", ") : "—"}
              </td>
              <td className="px-4 py-2 text-slate-900 dark:text-slate-100">
                {r.plural && r.plural.length > 0 ? r.plural.join(", ") : "—"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function ConjugationTable({
  lemma,
  conjugation,
}: {
  lemma: string;
  conjugation: ConjugationTableData;
}) {
  const captionId = useId();

  const rows: { label: string; values: string[] }[] = [];
  if (conjugation.infinitive?.length) {
    rows.push({ label: "Infinitiv", values: conjugation.infinitive });
  }
  if (conjugation.present_or_future) {
    const keys: [string, string][] = [
      ["1.sg", "1. os. č. j."],
      ["2.sg", "2. os. č. j."],
      ["3.sg", "3. os. č. j."],
      ["1.pl", "1. os. č. mn."],
      ["2.pl", "2. os. č. mn."],
      ["3.pl", "3. os. č. mn."],
    ];
    for (const [k, label] of keys) {
      const vals = conjugation.present_or_future[k];
      if (vals?.length) rows.push({ label, values: vals });
    }
  }
  if (conjugation.imperative) {
    const imp = [
      ...(conjugation.imperative["2.sg"] ?? []),
      ...(conjugation.imperative["1.pl"] ?? []),
      ...(conjugation.imperative["2.pl"] ?? []),
    ];
    if (imp.length > 0) rows.push({ label: "Rozkazovací zp.", values: imp });
  }
  if (conjugation.past_participle?.length) {
    rows.push({ label: "Příčestí činné", values: conjugation.past_participle });
  }
  if (conjugation.passive_participle?.length) {
    rows.push({ label: "Příčestí trpné", values: conjugation.passive_participle });
  }
  if (conjugation.transgressive?.length) {
    rows.push({ label: "Přechodníky", values: conjugation.transgressive });
  }

  return (
    <div
      tabIndex={0}
      role="region"
      aria-labelledby={captionId}
      className="table-wrapper rounded-xl border border-slate-200 dark:border-slate-700/80"
    >
      <table className="grammar-table conjugation-table text-[0.9rem]">
        <caption
          id={captionId}
          className="border-b border-slate-200 bg-slate-50 px-4 py-2.5 text-start font-bold text-slate-800 dark:border-slate-700/80 dark:bg-slate-800/70 dark:text-slate-100"
        >
          Časování – {lemma}
        </caption>
        <thead>
          <tr className="bg-slate-50 text-slate-600 dark:bg-slate-800 dark:text-slate-300">
            <th scope="col" className="border-b border-slate-200 px-4 py-2 text-start font-semibold dark:border-slate-700">
              Kategorie
            </th>
            <th scope="col" className="border-b border-slate-200 px-4 py-2 text-start font-semibold dark:border-slate-700">
              Tvary
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
          {rows.map((r) => (
            <tr key={r.label}>
              <th
                scope="row"
                className="px-4 py-2 text-start font-semibold whitespace-nowrap text-slate-600 dark:text-slate-300"
              >
                {r.label}
              </th>
              <td className="px-4 py-2 text-slate-900 dark:text-slate-100">
                {r.values.join(", ")}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
