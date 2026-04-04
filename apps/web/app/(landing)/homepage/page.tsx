import type { Metadata } from "next";
import { MulticodeLanding } from "@/features/landing/components/multicode-landing";

export const metadata: Metadata = {
  title: "Homepage",
  description:
    "Multicode — open-source platform that turns coding agents into real teammates. Assign tasks, track progress, compound skills.",
  openGraph: {
    title: "Multicode — AI-Native Task Management",
    description:
      "Manage your human + agent workforce in one place.",
    url: "/homepage",
  },
  alternates: {
    canonical: "/homepage",
  },
};

export default function HomepagePage() {
  return <MulticodeLanding />;
}
