export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="h-screen flex flex-col overflow-hidden bg-background">
      {/* TopBar — implemented in Task 3.1.2 */}
      <header className="h-12 border-b border-border flex items-center px-4 shrink-0">
        <span className="text-sm font-medium text-foreground">QuantFlow</span>
      </header>

      <div className="flex flex-1 overflow-hidden">
        {/* Sidebar — implemented in Task 3.1.2 */}
        <aside className="w-14 border-r border-border shrink-0" />

        <main className="flex-1 overflow-hidden">{children}</main>
      </div>
    </div>
  );
}
