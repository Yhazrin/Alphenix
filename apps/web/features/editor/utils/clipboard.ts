/**
 * Copy markdown content to the clipboard.
 */
export async function copyMarkdown(markdown: string): Promise<void> {
  try { await navigator.clipboard.writeText(markdown); } catch {}
}
