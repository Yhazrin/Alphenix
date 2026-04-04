import { useState, useEffect } from "react";
import { api } from "@/shared/api";
import type { TaskDependency } from "@/shared/types";

export function useDependencyStatuses(dependencies: TaskDependency[]) {
  const [depStatuses, setDepStatuses] = useState<Record<string, string>>({});

  useEffect(() => {
    if (dependencies.length === 0) return;
    const missing = dependencies.filter((d) => !(d.depends_on_id in depStatuses));
    if (missing.length === 0) return;
    Promise.allSettled(
      missing.map(async (dep) => {
        const task = await api.getTask(dep.depends_on_id);
        return { id: dep.depends_on_id, status: task.status };
      }),
    ).then((results) => {
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
  }, [dependencies]);

  return depStatuses;
}
