"use client";

import { useEffect, useState, useCallback } from "react";
import { Plus, Trash2, GitBranch, Star, Pencil, Check, X } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Empty, EmptyHeader, EmptyMedia, EmptyTitle, EmptyDescription } from "@/components/ui/empty";
import { toast } from "sonner";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore } from "@/features/workspace";
import { api } from "@/shared/api";
import type { WorkspaceRepo, CreateWorkspaceRepoRequest } from "@/shared/types";

export function RepositoriesTab() {
  const user = useAuthStore((s) => s.user);
  const workspace = useWorkspaceStore((s) => s.workspace);
  const members = useWorkspaceStore((s) => s.members);

  const [repos, setRepos] = useState<WorkspaceRepo[]>([]);
  const [loading, setLoading] = useState(true);
  const [adding, setAdding] = useState(false);
  const [newRepo, setNewRepo] = useState<CreateWorkspaceRepoRequest>({ name: "", url: "" });
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editName, setEditName] = useState("");
  const [editUrl, setEditUrl] = useState("");
  const [editBranch, setEditBranch] = useState("");
  const [editDesc, setEditDesc] = useState("");

  const currentMember = members.find((m) => m.user_id === user?.id) ?? null;
  const canManage = currentMember?.role === "owner" || currentMember?.role === "admin";

  const loadRepos = useCallback(async () => {
    if (!workspace) return;
    try {
      const data = await api.listWorkspaceRepos(workspace.id);
      setRepos(data);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to load repositories");
    } finally {
      setLoading(false);
    }
  }, [workspace]);

  useEffect(() => { loadRepos(); }, [loadRepos]);

  const handleAdd = async () => {
    if (!workspace || !newRepo.name.trim() || !newRepo.url.trim()) return;
    try {
      const created = await api.createWorkspaceRepo(workspace.id, newRepo);
      setRepos((prev) => [...prev, created].sort((a, b) => a.name.localeCompare(b.name)));
      setNewRepo({ name: "", url: "" });
      setAdding(false);
      toast.success("Repository added");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to add repository");
    }
  };

  const handleDelete = async (repoId: string) => {
    if (!workspace) return;
    try {
      await api.deleteWorkspaceRepo(workspace.id, repoId);
      setRepos((prev) => prev.filter((r) => r.id !== repoId));
      toast.success("Repository removed");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to remove repository");
    }
  };

  const handleSetDefault = async (repoId: string) => {
    if (!workspace) return;
    try {
      const updated = await api.updateWorkspaceRepo(workspace.id, repoId, { is_default: true });
      setRepos((prev) =>
        prev.map((r) => ({
          ...r,
          is_default: r.id === repoId ? true : (r.is_default && r.id !== repoId ? false : r.is_default),
        }))
      );
      toast.success("Default repository updated");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to set default");
    }
  };

  const startEdit = (repo: WorkspaceRepo) => {
    setEditingId(repo.id);
    setEditName(repo.name);
    setEditUrl(repo.url);
    setEditBranch(repo.default_branch);
    setEditDesc(repo.description ?? "");
  };

  const handleSaveEdit = async () => {
    if (!workspace || !editingId) return;
    try {
      const updated = await api.updateWorkspaceRepo(workspace.id, editingId, {
        name: editName,
        url: editUrl,
        default_branch: editBranch,
        description: editDesc || undefined,
      });
      setRepos((prev) => prev.map((r) => (r.id === editingId ? updated : r)));
      setEditingId(null);
      toast.success("Repository updated");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to update repository");
    }
  };

  if (!workspace) return null;

  return (
    <div className="space-y-8">
      <section className="space-y-4">
        <h2 className="text-sm font-semibold">Repositories</h2>

        <Card>
          <CardContent className="space-y-3">
            <p className="text-xs text-muted-foreground">
              GitHub repositories associated with this workspace. Agents use these to clone and work on code.
            </p>

            {loading ? (
              <p className="text-xs text-muted-foreground py-4 text-center">Loading...</p>
            ) : repos.length === 0 && !adding ? (
              <Empty className="border-0 py-4">
                <EmptyHeader>
                  <EmptyMedia variant="icon">
                    <GitBranch aria-hidden="true" />
                  </EmptyMedia>
                  <EmptyTitle>No repositories</EmptyTitle>
                  <EmptyDescription>
                    Add a GitHub repository so agents can clone and work on code.
                  </EmptyDescription>
                </EmptyHeader>
              </Empty>
            ) : (
              <div className="space-y-2">
                {/* Header row */}
                <div className="flex items-center gap-2 px-2 text-[10px] font-medium text-muted-foreground uppercase tracking-wider">
                  <span className="w-8" />
                  <span className="flex-1">Name</span>
                  <span className="w-32 hidden sm:block">Branch</span>
                  <span className="w-20 text-right">Actions</span>
                </div>

                {repos.map((repo) => (
                  <div
                    key={repo.id}
                    className="flex items-center gap-2 rounded-md border px-2 py-1.5 text-sm hover:bg-accent/30 transition-colors"
                  >
                    {editingId === repo.id ? (
                      /* ── Edit mode ── */
                      <>
                        <div className="flex-1 space-y-1">
                          <div className="flex gap-1.5">
                            <Input
                              value={editName}
                              onChange={(e) => setEditName(e.target.value)}
                              placeholder="name"
                              className="h-7 text-xs flex-1"
                            />
                            <Input
                              value={editUrl}
                              onChange={(e) => setEditUrl(e.target.value)}
                              placeholder="url"
                              className="h-7 text-xs flex-[2]"
                            />
                          </div>
                          <div className="flex gap-1.5">
                            <Input
                              value={editBranch}
                              onChange={(e) => setEditBranch(e.target.value)}
                              placeholder="branch"
                              className="h-7 text-xs w-28"
                            />
                            <Input
                              value={editDesc}
                              onChange={(e) => setEditDesc(e.target.value)}
                              placeholder="description (optional)"
                              className="h-7 text-xs flex-1"
                            />
                          </div>
                        </div>
                        <div className="flex shrink-0 gap-0.5">
                          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={handleSaveEdit} aria-label="Save">
                            <Check className="h-3.5 w-3.5 text-success" />
                          </Button>
                          <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => setEditingId(null)} aria-label="Cancel">
                            <X className="h-3.5 w-3.5" />
                          </Button>
                        </div>
                      </>
                    ) : (
                      /* ── View mode ── */
                      <>
                        <button
                          onClick={() => canManage && handleSetDefault(repo.id)}
                          className={`shrink-0 ${repo.is_default ? "text-warning" : "text-muted-foreground/30 hover:text-muted-foreground"} transition-colors`}
                          aria-label={repo.is_default ? "Default repository" : "Set as default"}
                          title={repo.is_default ? "Default repository" : "Set as default"}
                          disabled={!canManage}
                        >
                          <Star className="h-4 w-4" fill={repo.is_default ? "currentColor" : "none"} />
                        </button>
                        <div className="flex-1 min-w-0">
                          <p className="text-xs font-medium truncate">{repo.name}</p>
                          <p className="text-[11px] text-muted-foreground truncate">{repo.url}</p>
                          {repo.description && (
                            <p className="text-[11px] text-muted-foreground/70 truncate">{repo.description}</p>
                          )}
                        </div>
                        <span className="w-32 hidden sm:block text-[11px] text-muted-foreground truncate shrink-0">
                          {repo.default_branch}
                        </span>
                        {canManage && (
                          <div className="flex shrink-0 gap-0.5">
                            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => startEdit(repo)} aria-label="Edit">
                              <Pencil className="h-3 w-3" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="icon"
                              className="h-7 w-7 text-muted-foreground hover:text-destructive"
                              onClick={() => handleDelete(repo.id)}
                              aria-label="Remove"
                            >
                              <Trash2 className="h-3 w-3" />
                            </Button>
                          </div>
                        )}
                      </>
                    )}
                  </div>
                ))}
              </div>
            )}

            {/* Add new repo form */}
            {canManage && adding && (
              <div className="rounded-md border border-dashed p-2 space-y-1.5">
                <div className="flex gap-1.5">
                  <Input
                    value={newRepo.name}
                    onChange={(e) => setNewRepo((p) => ({ ...p, name: e.target.value }))}
                    placeholder="my-repo"
                    className="h-7 text-xs flex-1"
                    autoFocus
                  />
                  <Input
                    value={newRepo.url}
                    onChange={(e) => setNewRepo((p) => ({ ...p, url: e.target.value }))}
                    placeholder="https://github.com/org/repo"
                    className="h-7 text-xs flex-[2]"
                  />
                </div>
                <div className="flex items-center justify-between">
                  <Input
                    value={newRepo.default_branch ?? ""}
                    onChange={(e) => setNewRepo((p) => ({ ...p, default_branch: e.target.value }))}
                    placeholder="main (default)"
                    className="h-7 text-xs w-32"
                  />
                  <div className="flex gap-1">
                    <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={() => { setAdding(false); setNewRepo({ name: "", url: "" }); }}>
                      Cancel
                    </Button>
                    <Button size="sm" className="h-7 text-xs" onClick={handleAdd} disabled={!newRepo.name.trim() || !newRepo.url.trim()}>
                      Add
                    </Button>
                  </div>
                </div>
              </div>
            )}

            {canManage && (
              <div className="flex items-center justify-between pt-1">
                {!adding && (
                  <Button variant="outline" size="sm" onClick={() => setAdding(true)}>
                    <Plus className="h-3 w-3" aria-hidden="true" />
                    Add repository
                  </Button>
                )}
              </div>
            )}

            {!canManage && (
              <p className="text-xs text-muted-foreground">
                Only admins and owners can manage repositories.
              </p>
            )}
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
