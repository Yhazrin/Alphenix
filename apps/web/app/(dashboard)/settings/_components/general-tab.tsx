"use client";

import { useTheme } from "next-themes";
import { cn } from "@/lib/utils";
import { useScheme, type ThemeScheme } from "@/hooks/use-scheme";

interface SchemeColors {
  titleBar: string;
  content: string;
  sidebar: string;
  bar: string;
  barMuted: string;
  accent: string;
}

// Base zinc preview colors — grays are consistent across all themes
const LIGHT_GRAY = {
  titleBar: "#e8e8e8",
  content: "#ffffff",
  sidebar: "#f4f4f5",
  bar: "#e4e4e7",
  barMuted: "#d4d4d8",
};

const DARK_GRAY = {
  titleBar: "#333338",
  content: "#27272a",
  sidebar: "#1e1e21",
  bar: "#3f3f46",
  barMuted: "#52525b",
};

// Theme accent colors — only these change per scheme
const SCHEME_ACCENTS: Record<ThemeScheme, { light: string; dark: string }> = {
  zinc: { light: "#3b82f6", dark: "#60a5fa" },
  morandi: { light: "#b07898", dark: "#c88aaa" },
  ocean: { light: "#2888c8", dark: "#4aa8e8" },
  rose: { light: "#d04870", dark: "#e86888" },
};

function WindowMockup({
  gray,
  accent,
  className,
}: {
  gray: typeof LIGHT_GRAY;
  accent: string;
  className?: string;
}) {
  return (
    <div className={cn("flex h-full w-full flex-col", className)}>
      {/* Title bar */}
      <div
        className="flex items-center gap-[3px] px-2 py-1.5"
        style={{ backgroundColor: gray.titleBar }}
      >
        <span className="size-[6px] rounded-full bg-[#ff5f57]" aria-hidden="true" />
        <span className="size-[6px] rounded-full bg-[#febc2e]" aria-hidden="true" />
        <span className="size-[6px] rounded-full bg-[#28c840]" aria-hidden="true" />
      </div>
      {/* Content area */}
      <div className="flex flex-1" style={{ backgroundColor: gray.content }}>
        {/* Sidebar */}
        <div
          className="w-[30%] space-y-1 p-2"
          style={{ backgroundColor: gray.sidebar }}
        >
          <div
            className="h-1 w-3/4 rounded-full"
            style={{ backgroundColor: gray.bar }}
          />
          <div
            className="h-1 w-1/2 rounded-full"
            style={{ backgroundColor: accent }}
          />
        </div>
        {/* Main */}
        <div className="flex-1 space-y-1.5 p-2">
          <div
            className="h-1.5 w-4/5 rounded-full"
            style={{ backgroundColor: accent }}
          />
          <div
            className="h-1 w-full rounded-full"
            style={{ backgroundColor: gray.barMuted }}
          />
          <div
            className="h-1 w-3/5 rounded-full"
            style={{ backgroundColor: gray.barMuted }}
          />
        </div>
      </div>
    </div>
  );
}

const modeOptions = [
  { value: "light" as const, label: "Light" },
  { value: "dark" as const, label: "Dark" },
  { value: "system" as const, label: "System" },
];

const schemeOptions: { value: ThemeScheme; label: string }[] = [
  { value: "zinc", label: "Zinc" },
  { value: "morandi", label: "Morandi" },
  { value: "ocean", label: "Ocean" },
  { value: "rose", label: "Rose" },
];

export function AppearanceTab() {
  const { theme, setTheme } = useTheme();
  const { scheme, setScheme } = useScheme();

  const accent = SCHEME_ACCENTS[scheme];

  return (
    <div className="space-y-8">
      {/* Mode: Light / Dark / System */}
      <section className="space-y-4">
        <h2 className="text-sm font-semibold">Appearance</h2>
        <div className="flex gap-6" role="radiogroup" aria-label="Appearance mode">
          {modeOptions.map((opt) => {
            const active = theme === opt.value;
            return (
              <button
                key={opt.value}
                role="radio"
                aria-checked={active}
                aria-label={`Select ${opt.label} mode`}
                onClick={() => setTheme(opt.value)}
                className="group flex flex-col items-center gap-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-lg"
              >
                <div
                  className={cn(
                    "aspect-[4/3] w-36 overflow-hidden rounded-lg ring-1 transition-all",
                    active
                      ? "ring-2 ring-brand"
                      : "ring-border hover:ring-2 hover:ring-border"
                  )}
                >
                  {opt.value === "system" ? (
                    <div className="relative h-full w-full">
                      <WindowMockup
                        gray={LIGHT_GRAY}
                        accent={accent.light}
                        className="absolute inset-0"
                      />
                      <WindowMockup
                        gray={DARK_GRAY}
                        accent={accent.dark}
                        className="absolute inset-0 [clip-path:inset(0_0_0_50%)]"
                      />
                    </div>
                  ) : opt.value === "light" ? (
                    <WindowMockup gray={LIGHT_GRAY} accent={accent.light} />
                  ) : (
                    <WindowMockup gray={DARK_GRAY} accent={accent.dark} />
                  )}
                </div>
                <span
                  className={cn(
                    "text-sm transition-colors",
                    active
                      ? "font-medium text-foreground"
                      : "text-muted-foreground"
                  )}
                >
                  {opt.label}
                </span>
              </button>
            );
          })}
        </div>
      </section>

      {/* Scheme: Zinc / Morandi / Ocean / Rose */}
      <section className="space-y-4">
        <h2 className="text-sm font-semibold">Color Scheme</h2>
        <div className="flex gap-6" role="radiogroup" aria-label="Color scheme">
          {schemeOptions.map((opt) => {
            const active = scheme === opt.value;
            const schemeAccent = SCHEME_ACCENTS[opt.value];
            return (
              <button
                key={opt.value}
                role="radio"
                aria-checked={active}
                aria-label={`Select ${opt.label} color scheme`}
                onClick={() => setScheme(opt.value)}
                className="group flex flex-col items-center gap-2"
              >
                <div
                  className={cn(
                    "aspect-[4/3] w-32 overflow-hidden rounded-lg ring-1 transition-all",
                    active
                      ? "ring-2 ring-brand"
                      : "ring-border hover:ring-2 hover:ring-border"
                  )}
                >
                  <div className="relative h-full w-full">
                    <WindowMockup
                      gray={LIGHT_GRAY}
                      accent={schemeAccent.light}
                      className="absolute inset-0"
                    />
                    <WindowMockup
                      gray={DARK_GRAY}
                      accent={schemeAccent.dark}
                      className="absolute inset-0 [clip-path:inset(0_0_0_50%)]"
                    />
                  </div>
                </div>
                <span
                  className={cn(
                    "text-sm transition-colors",
                    active
                      ? "font-medium text-foreground"
                      : "text-muted-foreground"
                  )}
                >
                  {opt.label}
                </span>
              </button>
            );
          })}
        </div>
      </section>
    </div>
  );
}
