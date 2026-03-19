/**
 * [3.2.x] Multi-tab Strategy Editor — single page, no :id route.
 * Tab state managed by GlobalEditorStore (Zustand).
 * openTab() called from /strategies or Sidebar [+] button.
 * Full implementation: Tasks 3.2.1 – 3.2.12
 */
export default function EditorPage() {
  return (
    <div className="h-full flex items-center justify-center">
      <p className="text-sm text-muted-foreground">
        Multi-tab Strategy Editor — Task 3.2.x
      </p>
    </div>
  );
}
