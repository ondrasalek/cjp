import { useEffect } from "react";
import { Moon, Sun } from "lucide-react";

export function ThemeToggle() {
  useEffect(() => {
    const onStorage = (e: StorageEvent) => {
      if (e.key === "color-scheme") {
        const meta = document.querySelector<HTMLMetaElement>('meta[name="color-scheme"]');
        if (meta) {
          meta.content = e.newValue ?? "light dark";
        }
      }
    };
    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, []);

  const handleToggle = () => {
    const meta = document.querySelector<HTMLMetaElement>('meta[name="color-scheme"]');
    if (!meta) return;

    const sysScheme = window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
    let stored: string | null = null;
    try {
      stored = localStorage.getItem("color-scheme");
    } catch {
      // Storage blocked; still update <meta> in place
    }

    const currentRendered = stored === "dark" || stored === "light" ? stored : sysScheme;
    const target = currentRendered === "dark" ? "light" : "dark";

    if (target === sysScheme) {
      try {
        localStorage.removeItem("color-scheme");
      } catch {}
      meta.content = "light dark";
    } else {
      try {
        localStorage.setItem("color-scheme", target);
      } catch {}
      meta.content = target;
    }
  };

  return (
    <button
      type="button"
      onClick={handleToggle}
      className="inline-grid size-10 sm:size-9 shrink-0 place-items-center rounded-xl sm:rounded-lg border border-slate-200 bg-white text-slate-700 transition hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:bg-slate-800 cursor-pointer"
    >
      <span className="theme-to-dark inline-flex items-center">
        <Moon className="size-4" aria-hidden="true" />
        <span className="sr-only">Přepnout na tmavý režim</span>
      </span>
      <span className="theme-to-light items-center">
        <Sun className="size-4" aria-hidden="true" />
        <span className="sr-only">Přepnout na světlý režim</span>
      </span>
    </button>
  );
}
