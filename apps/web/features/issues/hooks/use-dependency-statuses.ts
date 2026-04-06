import { useState, useEffect, useRef } from "react";
import { api } from "@/shared/api";
import type { TaskDependency } from "@/shared/types";

export function useDependencyStatuses(dependencies: TaskDependency[]) {
  const [depStatuses, setDepStatuses] = useState<Record<string, string>>({});
  const statusesRef = useRef(depStatuses);
  statusesRef.current = depStatuses;

  useEffect(() => {
    if (dependencies.length === 0) return;
    let cancelled = false;
    const missing = dependencies.filter((d) => !(d.depends_on_id in statusesRef.current));
    if (missing.length === 0) return;
    Promise.allSettled(
      missing.map(async (dep) => {
        const task = await api.getTask(dep.depends_on_id);
        return { id: dep.depends_on_id, status: task.status };
      }),
    ).then((results) => {
      if (cancelled) return;
      const updates: Record<string, string> = {};
      for (const r of results) {
        if (r.status === "fulfilled") {
          updates[r.value.id] = r.value.status;
        }
      }
      if (Object.keys(updates).length > 0) {
        setDepStatuses((prev) => ({ ...prev, ...updates }));
      }
    });
    return () => { cancelled = true; };
  }, [dependencies]);

  return depStatuses;
}
