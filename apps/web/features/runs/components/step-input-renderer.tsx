"use client";

import { Copy, Check } from "lucide-react";
import { useState } from "react";

interface StepInputRendererProps {
  toolName: string;
  toolInput: Record<string, unknown>;
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <button
      type="button"
      onClick={handleCopy}
      className="absolute top-1.5 right-1.5 rounded p-1 opacity-0 group-hover:opacity-100 hover:bg-muted transition-opacity"
      aria-label="Copy"
    >
      {copied ? <Check className="h-3 w-3 text-green-500" /> : <Copy className="h-3 w-3 text-muted-foreground" />}
    </button>
  );
}

function BashInput({ command }: { command: string }) {
  return (
    <div className="relative group">
      <code className="block text-[11px] bg-muted rounded p-2 pr-8 font-mono whitespace-pre-wrap">
        {command}
      </code>
      <CopyButton text={command} />
    </div>
  );
}

function ReadInput({ path, startLine, endLine }: { path: string; startLine?: number; endLine?: number }) {
  return (
    <div className="text-[11px] text-muted-foreground">
      <span className="font-mono">{path}</span>
      {startLine != null && endLine != null && (
        <span className="ml-2 text-[10px]">L{startLine}–L{endLine}</span>
      )}
    </div>
  );
}

function EditInput({ path, action }: { path: string; action?: string }) {
  return (
    <div className="text-[11px] text-muted-foreground">
      {action && <span className="mr-1">{action}</span>}
      <span className="font-mono">{path}</span>
    </div>
  );
}

function GenericInput({ toolInput }: { toolInput: Record<string, unknown> }) {
  const json = JSON.stringify(toolInput, null, 2);
  return (
    <div className="relative group">
      <pre className="text-[11px] bg-muted rounded p-2 pr-8 overflow-x-auto">
        {json}
      </pre>
      <CopyButton text={json} />
    </div>
  );
}

/** Renders step input based on tool type — per UX spec §2 tool dispatch. */
export function StepInputRenderer({ toolName, toolInput }: StepInputRendererProps) {
  if (toolName === "thinking") return null;

  if (toolName === "bash" || toolName === "Bash") {
    const command = (toolInput.command as string) ?? (toolInput.cmd as string) ?? "";
    return <BashInput command={command} />;
  }

  if (toolName === "read_file" || toolName === "Read") {
    return (
      <ReadInput
        path={(toolInput.path as string) ?? ""}
        startLine={toolInput.startLine as number | undefined}
        endLine={toolInput.endLine as number | undefined}
      />
    );
  }

  if (toolName === "edit_file" || toolName === "Edit") {
    return (
      <EditInput
        path={(toolInput.path as string) ?? ""}
        action={toolInput.action as string | undefined}
      />
    );
  }

  return <GenericInput toolInput={toolInput} />;
}
