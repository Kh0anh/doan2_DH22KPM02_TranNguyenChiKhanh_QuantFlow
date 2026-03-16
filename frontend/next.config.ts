import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
<<<<<<< Updated upstream
=======

  // Turbopack is the default dev bundler in Next.js 16+.
  // An empty config silences the "no turbopack config" error while
  // keeping the webpack block below for production builds.
  turbopack: {},

  // Prevent constant HMR reloads when running inside Docker on Windows.
  // Docker volume mounts trigger spurious file-change events; disabling
  // polling and ignoring heavy directories stops the dev server from
  // reloading the page continuously.
  webpack: (config, { dev }) => {
    if (dev) {
      config.watchOptions = {
        ...config.watchOptions,
        poll: 1000,
        aggregateTimeout: 300,
        ignored: ["**/node_modules/**", "**/.next/**"],
      };
    }
    return config;
  },
>>>>>>> Stashed changes
};

export default nextConfig;
