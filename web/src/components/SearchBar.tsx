import { useEffect, useId, useRef, useState } from "react";
import { Search, X } from "lucide-react";
import type { AutocompleteItem } from "../types";

interface SearchBarProps {
  query: string;
  onSelectWord: (word: string) => void;
  langFilter?: React.ReactNode;
}

const QUICK_EXAMPLES = [
  { label: "žena", word: "žena" },
  { label: "ženami", word: "ženami" },
  { label: "dělat", word: "dělat" },
  { label: "dobrý", word: "dobrý" },
  { label: "stát", word: "stát" },
  { label: "horizontálněji", word: "horizontálněji" },
];

export function SearchBar({ query, onSelectWord, langFilter }: SearchBarProps) {
  const inputId = useId();
  const listboxId = useId();

  const [value, setValue] = useState(query);
  const [suggestions, setSuggestions] = useState<AutocompleteItem[]>([]);
  const [activeIndex, setActiveIndex] = useState(-1);
  const [open, setOpen] = useState(false);

  const formRef = useRef<HTMLFormElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const lastCompositionEndAt = useRef<number | null>(null);

  useEffect(() => {
    setValue(query);
  }, [query]);

  useEffect(() => {
    const handleOutside = (e: MouseEvent) => {
      if (formRef.current && !formRef.current.contains(e.target as Node)) {
        setOpen(false);
        setActiveIndex(-1);
      }
    };
    document.addEventListener("mousedown", handleOutside);
    return () => document.removeEventListener("mousedown", handleOutside);
  }, []);

  useEffect(() => {
    const trimmed = value.trim();
    if (!open || trimmed.length === 0) {
      setSuggestions([]);
      return;
    }

    const controller = new AbortController();
    const timer = window.setTimeout(async () => {
      try {
        const res = await fetch(`/api/v1/autocomplete?q=${encodeURIComponent(trimmed)}&limit=8`, {
          signal: controller.signal,
        });
        if (!res.ok) return;
        const data = await res.json();
        setSuggestions(data.suggestions ?? []);
        setActiveIndex(-1);
      } catch (err) {
        if ((err as Error).name !== "AbortError") {
          setSuggestions([]);
        }
      }
    }, 100);

    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [value, open]);

  const submitWord = (word: string) => {
    const clean = word.trim();
    if (!clean) return;
    setOpen(false);
    setActiveIndex(-1);
    inputRef.current?.blur();
    onSelectWord(clean);
  };

  const handleKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "ArrowDown" && suggestions.length > 0) {
      event.preventDefault();
      setOpen(true);
      setActiveIndex((prev) => (prev + 1) % suggestions.length);
    } else if (event.key === "ArrowUp" && suggestions.length > 0) {
      event.preventDefault();
      setOpen(true);
      setActiveIndex((prev) => (prev - 1 + suggestions.length) % suggestions.length);
    } else if (event.key === "Escape") {
      setOpen(false);
      setActiveIndex(-1);
    } else if (event.key === "Enter") {
      event.preventDefault();
      const native = event.nativeEvent;
      if (
        native.isComposing ||
        event.keyCode === 229 ||
        (lastCompositionEndAt.current !== null &&
          Math.abs(native.timeStamp - lastCompositionEndAt.current) < 50)
      ) {
        return;
      }
      if (open && activeIndex >= 0 && suggestions[activeIndex]) {
        submitWord(suggestions[activeIndex].lemma);
      } else {
        formRef.current?.requestSubmit();
      }
    }
  };

  const showList = open && suggestions.length > 0;

  return (
    <search className="w-full space-y-2.5">
      <div className="flex flex-col lg:flex-row lg:items-center gap-3">
        <form
          ref={formRef}
          onSubmit={(e) => {
            e.preventDefault();
            submitWord(value);
          }}
          className="search-container relative min-w-0 flex-1"
        >
          <label htmlFor={inputId} className="sr-only">
            Hledat české slovo nebo tvar
          </label>

          <div className="flex items-center gap-2 rounded-2xl border-2 border-slate-200 bg-white py-1 pr-1.5 pl-3.5 shadow-2xs transition focus-within:border-blue-600 dark:border-slate-700 dark:bg-slate-900 dark:focus-within:border-blue-400">
            <Search className="size-4.5 shrink-0 text-slate-400" aria-hidden="true" />
            <input
              ref={inputRef}
              id={inputId}
              type="text"
              inputMode="search"
              enterKeyHint="search"
              autoComplete="off"
              autoCorrect="off"
              spellCheck={false}
              value={value}
              onChange={(e) => {
                setValue(e.target.value);
                setOpen(true);
              }}
              onFocus={() => {
                if (value.trim().length > 0) setOpen(true);
              }}
              onCompositionEnd={(e) => {
                lastCompositionEndAt.current = e.nativeEvent.timeStamp;
              }}
              onKeyDown={handleKeyDown}
              placeholder="Zadejte slovo či tvar (např. žena, ženami, dělat)…"
              aria-autocomplete="list"
              aria-controls={listboxId}
              aria-expanded={showList}
              aria-activedescendant={
                showList && activeIndex >= 0 ? `${listboxId}-opt-${activeIndex}` : undefined
              }
              className="min-w-0 flex-1 bg-transparent py-1.5 text-base text-slate-900 placeholder:text-slate-400 focus:outline-none dark:text-slate-100"
            />

            {value.length > 0 && (
              <button
                type="button"
                onClick={() => {
                  setValue("");
                  setSuggestions([]);
                  setOpen(false);
                  inputRef.current?.focus();
                }}
                aria-label="Vymazat zadání"
                className="inline-grid size-8 shrink-0 place-items-center rounded-lg text-slate-400 transition hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-800 dark:hover:text-slate-200 cursor-pointer"
              >
                <X className="size-4" aria-hidden="true" />
              </button>
            )}

            <button
              type="submit"
              aria-label="Hledat"
              className="inline-flex min-h-9.5 shrink-0 items-center justify-center rounded-xl bg-blue-700 px-3.5 sm:px-5 py-1.5 text-[0.9rem] font-semibold text-white transition hover:bg-blue-800 dark:bg-blue-500 dark:text-slate-950 dark:hover:bg-blue-400 cursor-pointer"
            >
              <Search className="search-submit-icon size-4 shrink-0" aria-hidden="true" />
              <span className="search-submit-label">Hledat</span>
            </button>
          </div>

          {showList && (
            <ul
              id={listboxId}
              role="listbox"
              className="absolute inset-x-0 top-[calc(100%+0.4rem)] z-30 max-h-[min(22rem,52dvh)] overflow-y-auto overscroll-contain [scrollbar-gutter:stable] rounded-2xl border border-slate-200 bg-white p-1.5 shadow-xl dark:border-slate-700 dark:bg-slate-900"
            >
              {suggestions.map((item, idx) => {
                const selected = idx === activeIndex;
                return (
                  <li
                    key={`${item.lemma}-${item.pos}-${idx}`}
                    id={`${listboxId}-opt-${idx}`}
                    role="option"
                    aria-selected={selected}
                    onMouseDown={(e) => {
                      e.preventDefault();
                      submitWord(item.lemma);
                    }}
                    className={`flex min-h-11 flex-col sm:flex-row sm:items-baseline sm:justify-between gap-0.5 sm:gap-3 rounded-xl px-3.5 py-2 cursor-pointer transition ${
                      selected
                        ? "bg-blue-50 text-blue-900 dark:bg-blue-950/70 dark:text-blue-200"
                        : "hover:bg-slate-100 dark:hover:bg-slate-800/80"
                    }`}
                  >
                    <span className="flex min-w-0 items-baseline gap-2">
                      <span className="font-bold text-slate-900 dark:text-slate-100">
                        {item.lemma}
                      </span>
                      {(item.pos_cs || item.pos) && (
                        <span className="shrink-0 text-[0.8125rem] font-medium text-blue-700 dark:text-blue-400">
                          {item.pos_cs || item.pos}
                        </span>
                      )}
                    </span>
                    {item.preview && (
                      <span className="min-w-0 sm:max-w-[38ch] truncate text-[0.8125rem] text-slate-500 dark:text-slate-400">
                        {item.preview}
                      </span>
                    )}
                  </li>
                );
              })}
            </ul>
          )}
        </form>

        {langFilter && <div className="shrink-0">{langFilter}</div>}
      </div>

      <div className="pill-cloud items-center gap-1.5">
        <span className="mr-0.5 text-[0.8rem] text-slate-500 dark:text-slate-400">
          Ukázky:
        </span>
        {QUICK_EXAMPLES.map((ex) => (
          <button
            key={ex.word}
            type="button"
            onClick={() => submitWord(ex.word)}
            className="rounded-full border border-slate-200 bg-white px-2.5 py-0.5 text-[0.8rem] font-medium text-slate-700 transition hover:border-blue-600 hover:text-blue-700 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-300 dark:hover:border-blue-400 dark:hover:text-blue-300 cursor-pointer"
          >
            {ex.label}
          </button>
        ))}
      </div>
    </search>
  );
}
