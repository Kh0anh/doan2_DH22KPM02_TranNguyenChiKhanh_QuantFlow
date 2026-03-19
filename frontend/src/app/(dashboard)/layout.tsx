/**
 * [3.1.2] Dashboard Layout — AppShell: TopBar + Sidebar + Content.
 *
 * Structure:
 *   <TopBar />          ← 48px fixed header
 *   <Sidebar />         ← 60px fixed side nav
 *   <main />            ← flex-1, scrollable content area per page
 *
 * Auth protection (redirect to /login if unauthenticated):
 *   TODO [3.1.3] — Wrap with AuthProvider / check session here.
 */
import { Sidebar } from "@/components/layout/sidebar";
import { TopBar } from "@/components/layout/top-bar";

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  // TODO [3.1.3]: Add AuthProvider wrapper + session check → redirect /login if unauth
  return (
    <div className="flex h-screen flex-col overflow-hidden bg-background">
      <TopBar />
      <div className="flex flex-1 overflow-hidden">
        <Sidebar />
        <main className="flex-1 overflow-hidden">{children}</main>
      </div>
    </div>
  );
}
