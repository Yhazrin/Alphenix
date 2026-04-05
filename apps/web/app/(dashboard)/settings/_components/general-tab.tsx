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

const LIGHT_COLORS: SchemeColors = {
  titleBar: "#e8e8e8",
  content: "#ffffff",
  sidebar: "#f4f4f5",
  bar: "#e4e4e7",
  barMuted: "#d4d4d8",
  accent: "#3b82f6",
};

const DARK_COLORS: SchemeColors = {
  titleBar: "#333338",
  content: "#27272a",
  sidebar: "#1e1e21",
  bar: "#3f3f46",
  barMuted: "#52525b",
  accent: "#60a5fa",
};

const SCHEME_COLORS: Record<ThemeScheme, { light: SchemeColors; dark: SchemeColors }> = {
  zinc: { light: LIGHT_COLORS, dark: DARK_COLORS },
  morandi: {
    light: { titleBar: "#e8e0d8", content: "#f5f0eb", sidebar: "#efe8e0", bar: "#d8cfc4", barMuted: "#c8bfb4", accent: "#b07898" },
    dark: { titleBar: "#48403a", content: "#38302a", sidebar: "#3e3630", bar: "#58504a", barMuted: "#68605a", accent: "#c88aaa" },
  },
  ocean: {
    light: { titleBar: "#e0eaf0", content: "#f5f9fc", sidebar: "#eaf0f5", bar: "#c8d8e4", barMuted: "#b8c8d4", accent: "#2888c8" },
    dark: { titleBar: "#283848", content: "#1e3040", sidebar: "#243545", bar: "#3a5060", barMuted: "#4a6070", accent: "#4aa8e8" },
  },
  rose: {
    light: { titleBar: "#f0e4e8", content: "#fcf5f7", sidebar: "#f5eaee", bar: "#e4c8d0", barMuted: "#d4b8c0", accent: "#d04870" },
    dark: { titleBar: "#482838", content: "#3a2030", sidebar: "#422838", bar: "#5a3a4a", barMuted: "#6a4a5a", accent: "#e86888" },
  },
};

function WindowMockup({
  colors,
  className,
}: {
  colors: SchemeColors;
  className?: string;
}) {
  return (
    <div className={cn("flex h-full w-full flex-col", className)}>
      {/* Title bar */}
      <div
        className="flex items-center gap-[3px] px-2 py-1.5"
        style={{ backgroundColor: colors.titleBar }}
      >
        <span className="size-[6px] rounded-full bg-[#ff5f57]" aria-hidden="true" />
        <span className="size-[6px] rounded-full bg-[#febc2e]" aria-hidden="true" />
        <span className="size-[6px] rounded-full bg-[#28c840]" aria-hidden="true" />
      </div>
      {/* Content area */}
      <div
        className="flex flex-1"
        style={{ backgroundColor: colors.content }}
      >
        {/* Sidebar */}
        <div
          className="w-[30%] space-y-1 p-2"
          style={{ backgroundColor: colors.sidebar }}
        >
          <div
            className="h-1 w-3/4 rounded-full"
            style={{ backgroundColor: colors.bar }}
          />
          <div
            className="h-1 w-1/2 rounded-full"
            style={{ backgroundColor: colors.accent }}
          />
        </div>
        {/* Main */}
        <div className="flex-1 space-y-1.5 p-2">
          <div
            className="h-1.5 w-4/5 rounded-full"
            style={{ backgroundColor: colors.accent }}
          />
          <div
            className="h-1 w-full rounded-full"
            style={{ backgroundColor: colors.barMuted }}
          />
          <div
            className="h-1 w-3/5 rounded-full"
            style={{ backgroundColor: colors.barMuted }}
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
                        colors={SCHEME_COLORS[scheme].light}
                        className="absolute inset-0"
                      />
                      <WindowMockup
                        colors={SCHEME_COLORS[scheme].dark}
                        className="absolute inset-0 [clip-path:inset(0_0_0_50%)]"
                      />
                    </div>
                  ) : (
                    <WindowMockup colors={SCHEME_COLORS[scheme][opt.value]} />
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
            const colors = SCHEME_COLORS[opt.value];
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
                      colors={colors.light}
                      className="absolute inset-0"
                    />
                    <WindowMockup
                      colors={colors.dark}
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
