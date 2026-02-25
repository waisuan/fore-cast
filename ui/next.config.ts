import type { NextConfig } from 'next';

const apiTarget =
  process.env.NEXT_PUBLIC_API_BASE_URL || 'http://localhost:8080';

const nextConfig: NextConfig = {
  poweredByHeader: false,
  async rewrites() {
    return [
      { source: '/api/:path*', destination: `${apiTarget}/api/:path*` },
    ];
  },
};

export default nextConfig;
