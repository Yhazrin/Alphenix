"use client";

import { useState, useEffect, useCallback } from "react";
import { Eye, Copy, Check, ChevronRight } from "lucide-react";
import type { Agent, PromptSection } from "@/shared/types";
import { agentsApi } from "@/shared/api/agents";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";

export function PromptPreviewTab({ agent }: { agent: Agent }) {
  const [sections, setSections] = useState<PromptSection[] | null>(null);
  const [fullPrompt, setFullPrompt] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [expandedSection, setExpandedSection] = useState<string | null>(null);

  const loadPreview = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await agentsApi.previewPrompt(agent.id);
      setSections(res.sections);
      setFullPrompt(res.full_prompt);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load prompt preview");
    } finally {
      setLoading(false);
    }
  }, [agent.id]);

  useEffect(() => {
    loadPreview();
  }, [loadPreview]);

  const handleCopy = useCallback(async () => {
    await navigator.clipboard.writeText(fullPrompt);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }, [fullPrompt]);

  if (loading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-24 w-full" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center gap-2 py-12 text-center">
        <Eye className="h-8 w-8 text-muted-foreground" />
        <p className="text-sm text-destructive">{error}</p>
        <Button variant="outline" size="sm" onClick={loadPreview}>
          Retry
        </Button>
      </div>
    );
  }

  if (!sections) return null;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold">System Prompt Preview</h3>
          <p className="text-xs text-muted-foreground">
            Preview of the assembled system prompt for this agent. Sections are composed in order.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" className="h-7 text-xs" onClick={handleCopy}>
            {copied ? <Check className="mr-1.5 h-3 w-3" /> : <Copy className="mr-1.5 h-3 w-3" />}
            {copied ? "Copied" : "Copy All"}
          </Button>
          <Button variant="outline" size="sm" className="h-7 text-xs" onClick={loadPreview}>
            Refresh
          </Button>
        </div>
      </div>

      {/* Full prompt summary */}
      <div className="rounded-lg border bg-muted/30 px-4 py-3">
        <div className="flex items-center justify-between text-xs">
          <span className="text-muted-foreground">
            {sections.length} sections &middot; {fullPrompt.length.toLocaleString()} characters
          </span>
          <Badge variant="outline" className="text-[10px]">
            {sections.filter((s) => s.phase === "static").length} static /{" "}
            {sections.filter((s) => s.phase === "dynamic").length} dynamic
          </Badge>
        </div>
      </div>

      {/* Section list */}
      <div className="space-y-2">
        {sections.map((section) => {
          const isExpanded = expandedSection === section.name;
          const preview =
            section.content.length > 120 ? section.content.slice(0, 120) + "..." : section.content;

          return (
            <div
              key={section.name}
              className="rounded-lg border transition-colors hover:bg-muted/20"
            >
              <button
                className="flex w-full items-center gap-3 px-4 py-3 text-left"
                onClick={() => setExpandedSection(isExpanded ? null : section.name)}
              >
                <ChevronRight
                  className={`h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform ${
                    isExpanded ? "rotate-90" : ""
                  }`}
                />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-xs font-medium">{section.name}</span>
                    <Badge
                      variant="outline"
                      className={`text-[10px] ${
                        section.phase === "static"
                          ? "border-blue-200 text-blue-600 dark:border-blue-800 dark:text-blue-400"
                          : "border-amber-200 text-amber-600 dark:border-amber-800 dark:text-amber-400"
                      }`}
                    >
                      {section.phase}
                    </Badge>
                    <span className="text-[10px] text-muted-foreground">order: {section.order}</span>
                  </div>
                  {!isExpanded && (
                    <p className="mt-0.5 text-[11px] text-muted-foreground truncate font-mono">
                      {preview}
                    </p>
                  )}
                </div>
                <span className="text-[10px] text-muted-foreground shrink-0">
                  {section.content.length} chars
                </span>
              </button>
              {isExpanded && (
                <div className="border-t px-4 py-3">
                  <pre className="text-[11px] leading-relaxed whitespace-pre-wrap font-mono text-muted-foreground max-h-80 overflow-y-auto">
                    {section.content}
                  </pre>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="mt-2 h-6 text-[10px]"
                    onClick={async (e) => {
                      e.stopPropagation();
                      await navigator.clipboard.writeText(section.content);
                    }}
                  >
                    <Copy className="mr-1 h-3 w-3" /> Copy section
                  </Button>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
