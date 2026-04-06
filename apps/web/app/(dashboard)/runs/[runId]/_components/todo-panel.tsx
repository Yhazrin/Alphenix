"use client";

import type { RunTodo } from "@/shared/types";
import { CheckCircle2, Circle, Loader2, AlertCircle } from "lucide-react";

interface TodoPanelProps {
  todos: RunTodo[];
}

function todoIcon(status: RunTodo["status"]) {
  switch (status) {
    case "completed":
      return <CheckCircle2 className="h-4 w-4 text-green-500" />;
    case "in_progress":
      return <Loader2 className="h-4 w-4 text-blue-500 animate-spin" />;
    case "blocked":
      return <AlertCircle className="h-4 w-4 text-yellow-500" />;
    default:
      return <Circle className="h-4 w-4 text-muted-foreground" />;
  }
}

export function TodoPanel({ todos }: TodoPanelProps) {
  const completed = todos.filter((t) => t.status === "completed").length;
  const progress = todos.length > 0 ? Math.round((completed / todos.length) * 100) : 0;

  return (
    <div className="space-y-3" data-testid="todo-panel">
      <div className="flex items-center justify-between">
        <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">
          Progress
        </h3>
        <span className="text-xs text-muted-foreground">
          {completed}/{todos.length} ({progress}%)
        </span>
      </div>

      {/* Progress bar */}
      <div className="h-1.5 w-full rounded-full bg-muted overflow-hidden">
        <div
          className="h-full rounded-full bg-primary transition-all duration-500"
          style={{ width: `${progress}%` }}
        />
      </div>

      {/* Todo list */}
      <div className="space-y-1.5">
        {todos.map((todo) => (
          <div
            key={todo.id}
            className={`flex items-start gap-2 rounded-md px-2 py-1.5 text-xs ${
              todo.status === "in_progress" ? "bg-blue-50 dark:bg-blue-950/20" : ""
            }`}
          >
            <span className="mt-0.5 shrink-0">{todoIcon(todo.status)}</span>
            <div className="min-w-0 flex-1">
              <p className={`${todo.status === "completed" ? "line-through text-muted-foreground" : ""}`}>
                {todo.title}
              </p>
              {todo.status === "blocked" && todo.blocker && (
                <p className="mt-0.5 text-[10px] text-yellow-600">{todo.blocker}</p>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
