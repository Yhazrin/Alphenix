import { redirect } from "next/navigation";

/** Legacy URL — personal views live under Issues → Focus. */
export default function MyIssuesRedirectPage() {
  redirect("/issues");
}
