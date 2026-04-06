"use client";

import Link from "next/link";
import { AlphenixIcon } from "@/components/alphenix-icon";
import { cn } from "@/lib/utils";
import { useAuthStore } from "@/features/auth";
import { useLocale, locales, localeLabels } from "../i18n";

export function LandingFooter() {
  const { t, locale, setLocale } = useLocale();
  const user = useAuthStore((s) => s.user);
  const groups = Object.values(t.footer.groups);

  return (
    <footer className="bg-landing-dark text-landing-dark-foreground">
      <div className="mx-auto max-w-[1320px] px-4 sm:px-6 lg:px-8">
        {/* Top: CTA + link columns */}
        <div className="flex flex-col gap-12 border-b border-landing-dark-foreground/10 py-16 sm:py-20 lg:flex-row lg:gap-20">
          {/* Left — newsletter / CTA */}
          <div className="lg:w-[340px] lg:shrink-0">
            <Link href="#product" className="flex items-center gap-3">
              <AlphenixIcon className="size-5 text-landing-dark-foreground" noSpin aria-hidden="true" />
              <span className="text-[18px] font-semibold tracking-[0.04em] lowercase">
                alphenix
              </span>
            </Link>
            <p className="mt-4 max-w-[300px] text-[14px] leading-[1.7] text-landing-dark-foreground/50 sm:text-[15px]">
              {t.footer.tagline}
            </p>
            <div className="mt-6">
              <Link
                href={user ? "/issues" : "/login"}
                className="inline-flex items-center justify-center rounded-xl bg-landing-dark-foreground px-5 py-2.5 text-[13px] font-semibold text-landing-dark transition-colors hover:bg-landing-dark-foreground/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                {user ? t.header.dashboard : t.footer.cta}
              </Link>
            </div>
          </div>

          {/* Right — link columns */}
          <nav className="grid flex-1 grid-cols-2 gap-8 sm:grid-cols-4" aria-label="Footer navigation">
            {groups.map((group) => (
              <div key={group.label}>
                <h4 className="text-[12px] font-semibold uppercase tracking-[0.1em] text-landing-dark-foreground/40">
                  {group.label}
                </h4>
                <ul className="mt-4 flex flex-col gap-2.5">
                  {group.links.map((link) => (
                    <li key={link.label}>
                      <Link
                        href={link.href}
                        {...(link.href.startsWith("http")
                          ? { target: "_blank", rel: "noopener noreferrer" }
                          : {})}
                        className="text-[14px] text-landing-dark-foreground/50 transition-colors hover:text-landing-dark-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                      >
                        {link.label}
                      </Link>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </nav>
        </div>

        {/* Bottom: copyright + language switcher */}
        <div className="flex items-center justify-between py-6">
          <p className="text-[13px] text-landing-dark-foreground/50">
            {t.footer.copyright.replace(
              "{year}",
              String(new Date().getFullYear()),
            )}
          </p>
          <div className="flex items-center" role="group" aria-label="Language">
            {locales.map((l, i) => (
              <button
                key={l}
                onClick={() => setLocale(l)}
                aria-label={`Switch to ${localeLabels[l]}`}
                aria-current={l === locale ? "true" : undefined}
                className={cn(
                  "px-3 py-2 text-[12px] font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded",
                  l === locale
                    ? "text-landing-dark-foreground/70"
                    : "text-landing-dark-foreground/30 hover:text-landing-dark-foreground/50",
                  i > 0 && "border-l border-landing-dark-foreground/15",
                )}
              >
                {localeLabels[l]}
              </button>
            ))}
          </div>
        </div>

        {/* Giant logo */}
        <div className="relative overflow-hidden pb-4">
          <div className="flex items-end gap-6 sm:gap-8">
            <AlphenixIcon
              className="size-[clamp(4rem,12vw,10rem)] shrink-0 text-landing-dark-foreground"
              noSpin
              aria-hidden="true"
            />
            <span className="font-[family-name:var(--font-serif)] text-[clamp(6rem,22vw,16rem)] font-normal leading-[0.82] tracking-[-0.04em] text-landing-dark-foreground lowercase">
                alphenix
            </span>
          </div>
        </div>
      </div>
    </footer>
  );
}
