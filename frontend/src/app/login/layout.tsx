/**
 * [3.1.1] Login layout — centered auth layout, no sidebar.
 * Full implementation: Task 3.1.1
 */
export default function LoginLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      {children}
    </div>
  );
}
