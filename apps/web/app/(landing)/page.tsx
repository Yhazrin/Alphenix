import type { Metadata } from "next";
import { AlphenixLanding } from "@/features/landing/components/alphenix-landing";

export const metadata: Metadata = {
  title: {
    absolute: "Alphenix — AI-Native Task Management",
  },
  description:
    "Open-source platform that turns coding agents into real teammates. Assign tasks, track progress, compound skills.",
  openGraph: {
    title: "Alphenix — AI-Native Task Management",
    description:
      "Manage your human + agent workforce in one place.",
    url: "/",
  },
  alternates: {
    canonical: "/",
  },
};

export default function LandingPage() {
  return <AlphenixLanding />;
}
