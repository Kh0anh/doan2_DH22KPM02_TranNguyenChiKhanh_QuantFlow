/**
 * [3.1.2] Dashboard Layout — AppShell: TopBar + Sidebar + Content.
 * [3.1.3] Wrapped with AuthProvider — calls /auth/me on mount, redirects to /login if
 *         unauthenticated, and schedules automatic JWT refresh.
 * [3.1.4] Mounts <SettingsDialog /> — controlled by useUIStore.settingsOpen.
 *
 * Structure:
 *   <AuthProvider>      ← session check + token refresh
 *     <TopBar />        ← 48px fixed header
 *     <Sidebar />       ← 60px fixed side nav
 *     <main />          ← flex-1, scrollable content area per page
 *     <SettingsDialog/> ← modal overlay, rendered once at shell level
 *   </AuthProvider>
 */
import { AuthProvider } from "@/lib/auth";
import { Sidebar } from "@/components/layout/sidebar";
import { TopBar } from "@/components/layout/top-bar";
import { SettingsDialog } from "@/components/settings/settings-dialog";

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <AuthProvider>
      <div className="flex h-screen flex-col overflow-hidden bg-background">
        <TopBar />
        <div className="flex flex-1 overflow-hidden">
          <Sidebar />
          <main className="flex-1 overflow-hidden">{children}</main>
        </div>
      </div>
      <SettingsDialog />
    </AuthProvider>
  );
}
