/**
 * [3.1.1] Login layout — full-screen centered, no sidebar.
 * Dark gradient background matching design spec (frontend_flows.md §3.1.1).
 */
export default function LoginLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="relative min-h-screen flex items-center justify-center overflow-hidden bg-[#0D1117]">
      {/* Decorative radial gradient — accent glow at top */}
      <div
        className="pointer-events-none absolute inset-0"
        style={{
          background:
            "radial-gradient(ellipse 80% 50% at 50% -20%, rgba(88,166,255,0.10) 0%, transparent 70%)",
        }}
        aria-hidden="true"
      />
      {children}
    </div>
  );
}
