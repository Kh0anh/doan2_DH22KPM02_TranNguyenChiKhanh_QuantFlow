"use client";

/**
 * [3.1.2] Dashboard Layout — AppShell: Sidebar + Top Bar wrapper.
 * Protected route: redirects to /login if not authenticated.
 * Full implementation: Task 3.1.2 (Sidebar + TopBar) + Task 3.1.3 (AuthContext)
 */
export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  // TODO [3.1.3]: Wrap with AuthProvider, check isAuthenticated, redirect to /login
  // TODO [3.1.2]: Render AppShell (TopBar + Sidebar + main)
  return (
    <div className="h-screen flex flex-col overflow-hidden bg-background">
      {/* TODO [3.1.2]: <TopBar /> */}
      <div className="flex flex-1 overflow-hidden">
        {/* TODO [3.1.2]: <Sidebar /> */}
        <main className="flex-1 overflow-hidden">{children}</main>
      </div>
    </div>
  );
}
