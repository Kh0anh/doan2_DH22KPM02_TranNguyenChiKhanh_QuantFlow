"use client";

/**
 * (dashboard)/layout.tsx — Protected dashboard shell (Task 3.1.2).
 *
 * Composes:
 *  - <TopBar>  — sticky h-12 top bar (hamburger on mobile + page title)
 *  - <Sidebar> — icon-strip on desktop; overlay drawer on mobile
 *  - <main>    — scrollable content area
 *
 * State:
 *  - sidebarExpanded  (desktop): persisted collapse toggle
 *  - sidebarOpen      (mobile): overlay open/close
 *
 * WBS 3.1.2 · project_structure.md §3
 */

import { useState } from "react";
import { Sidebar } from "@/components/layout/sidebar";
import { TopBar } from "@/components/layout/top-bar";

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  // Desktop: collapsed (w-14) ↔ expanded (w-56)
  const [sidebarExpanded, setSidebarExpanded] = useState(false);
  // Mobile: hidden ↔ overlay open
  const [sidebarOpen, setSidebarOpen] = useState(false);

  return (
    <div className="h-screen flex flex-col overflow-hidden bg-background">
      {/* ── Top Bar ── */}
      <TopBar onMenuClick={() => setSidebarOpen(true)} />

      {/* ── Body ── */}
      <div className="flex flex-1 overflow-hidden">
        {/* ── Sidebar ── */}
        <Sidebar
          expanded={sidebarExpanded}
          isOpen={sidebarOpen}
          onToggleExpanded={() => setSidebarExpanded((v) => !v)}
          onClose={() => setSidebarOpen(false)}
        />

        {/* ── Main content ── */}
        <main className="flex-1 overflow-auto">{children}</main>
      </div>
    </div>
  );
}
