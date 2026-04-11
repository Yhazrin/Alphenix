"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Bot, Users } from "lucide-react";
import { cn } from "@/lib/utils";

const tabs = [
  { href: "/agents", label: "Agents", icon: Bot },
  { href: "/teams", label: "Teams", icon: Users },
] as const;

export function AutomationSubnav() {
  const pathname = usePathname();

  return (
    <nav
      className="flex shrink-0 items-center gap-1 border-b border-border/60 bg-background/80 px-3 py-2 backdrop-blur-sm"
      aria-label="Agents and teams"
    >
      {tabs.map(({ href, label, icon: Icon }) => {
        const active =
          pathname === href || pathname.startsWith(`${href}/`);
        return (
          <Link
            key={href}
            href={href}
            className={cn(
              "inline-flex items-center gap-1.5 rounded-full px-3 py-1.5 text-xs font-medium transition-colors",
              active
                ? "bg-primary/12 text-primary"
                : "text-muted-foreground hover:bg-muted/80 hover:text-foreground",
            )}
          >
            <Icon className="size-3.5 shrink-0 opacity-80" aria-hidden="true" />
            {label}
          </Link>
        );
      })}
    </nav>
  );
}
