import type { Metadata } from "next";
import { JetBrains_Mono } from "next/font/google";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Toaster } from "@/components/ui/sonner";
import "./globals.css";

const jetbrainsMono = JetBrains_Mono({
  variable: "--font-jetbrains-mono",
  subsets: ["latin", "vietnamese"],
});

export const metadata: Metadata = {
  title: "QuantFlow — Low-code Crypto Trading",
  description:
    "Nền tảng Low-code hỗ trợ xây dựng và vận hành chiến lược giao dịch tiền mã hóa",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="vi"
      className={`${jetbrainsMono.variable} dark`}
    >
      <head>
        <link
          href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500;600;700&display=swap"
          rel="stylesheet"
        />
      </head>
      <body className="antialiased">
        <TooltipProvider>
          {children}
        </TooltipProvider>
        <Toaster richColors position="top-right" />
      </body>
    </html>
  );
}
