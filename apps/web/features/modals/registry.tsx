"use client";

import dynamic from "next/dynamic";
import { useModalStore } from "./store";

const CreateWorkspaceModal = dynamic(
  () => import("./create-workspace").then((m) => m.CreateWorkspaceModal),
  { ssr: false, loading: () => null },
);

const CreateIssueModal = dynamic(
  () => import("./create-issue").then((m) => m.CreateIssueModal),
  { ssr: false, loading: () => null },
);

const SearchModal = dynamic(
  () => import("./search-modal").then((m) => m.SearchModal),
  { ssr: false, loading: () => null },
);

export function ModalRegistry() {
  const modal = useModalStore((s) => s.modal);
  const data = useModalStore((s) => s.data);
  const close = useModalStore((s) => s.close);

  switch (modal) {
    case "create-workspace":
      return <CreateWorkspaceModal onClose={close} />;
    case "create-issue":
      return <CreateIssueModal onClose={close} data={data} />;
    case "search":
      return <SearchModal onClose={close} />;
    default:
      return null;
  }
}
