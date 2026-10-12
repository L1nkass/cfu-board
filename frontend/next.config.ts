import type { NextConfig } from "next";

// Прокси API на бэкенд: фронтенд ходит в /api/* того же origin,
// Next переписывает запросы на бэкенд (без CORS-проблем).
// В Docker-композите адрес задаётся через API_PROXY_TARGET=http://app:8080.
const API_PROXY_TARGET = process.env.API_PROXY_TARGET ?? "http://localhost:8080";

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${API_PROXY_TARGET}/:path*`,
      },
    ];
  },
  images: {
    remotePatterns: [
      { protocol: "http", hostname: "localhost", port: "9000" },
      { protocol: "http", hostname: "localhost", port: "8080" },
      { protocol: "http", hostname: "minio", port: "9000" },
      { protocol: "http", hostname: "app", port: "8080" },
    ],
  },
};

export default nextConfig;
