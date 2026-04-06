"use client";

import { useCallback, useSyncExternalStore } from "react";

export type ThemeScheme = "zinc" | "morandi" | "ocean" | "rose" | "jade";

const STORAGE_KEY = "multica-theme-scheme";
const SCHEMES: ThemeScheme[] = ["zinc", "morandi", "ocean", "rose", "jade"];

function getSnapshot(): ThemeScheme {
  if (typeof window === "undefined") return "zinc";
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored && SCHEMES.includes(stored as ThemeScheme)) {
      return stored as ThemeScheme;
    }
  } catch {}
  return "zinc";
}

let currentScheme = getSnapshot();
const listeners = new Set<() => void>();

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function emitChange() {
  for (const l of listeners) l();
}

function applySchemeClass(scheme: ThemeScheme) {
  const root = document.documentElement;
  for (const s of SCHEMES) {
    if (s !== "zinc") root.classList.remove(`theme-${s}`);
  }
  if (scheme !== "zinc") {
    root.classList.add(`theme-${scheme}`);
  }
}

export function useScheme() {
  const scheme = useSyncExternalStore(subscribe, getSnapshot, () => "zinc" as ThemeScheme);

  const setScheme = useCallback((next: ThemeScheme) => {
    currentScheme = next;
    try { localStorage.setItem(STORAGE_KEY, next); } catch {}
    applySchemeClass(next);
    emitChange();
  }, []);

  return { scheme, setScheme, schemes: SCHEMES };
}

/** Call once on client to apply persisted scheme on page load. */
export function initScheme() {
  const scheme = getSnapshot();
  currentScheme = scheme;
  applySchemeClass(scheme);
}
