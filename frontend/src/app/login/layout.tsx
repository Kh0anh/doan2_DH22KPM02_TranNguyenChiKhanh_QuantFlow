import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Sign In — QuantFlow",
  description: "Sign in to your QuantFlow account to manage your automated crypto trading bots.",
};

export default function LoginLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="min-h-screen bg-background flex items-center justify-center p-4">
      {children}
    </div>
  );
}
