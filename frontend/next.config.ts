import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  /**
   * API proxy rewrites for local development (without Docker).
   * In production, Nginx handles routing — these rewrites are ignored.
   * Backend URL can be overridden via NEXT_PUBLIC_API_URL env var.
   */
  async rewrites() {
    const backendUrl = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
    return [
      {
        source: "/api/v1/:path*",
        destination: `${backendUrl}/api/v1/:path*`,
      },
    ];
  },
};

export default nextConfig;
