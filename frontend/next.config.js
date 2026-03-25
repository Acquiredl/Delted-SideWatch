/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
  async rewrites() {
    // API_PROXY_URL is the server-side rewrite destination (Docker-internal).
    // NEXT_PUBLIC_API_URL is the client-side prefix (browser-accessible).
    // When NEXT_PUBLIC_API_URL is empty, the client fetches /api/* from same
    // origin and this rewrite proxies it to the backend.
    const proxyTarget = process.env.API_PROXY_URL || 'http://localhost:8080'
    return [
      {
        source: '/api/:path*',
        destination: `${proxyTarget}/api/:path*`,
      },
      {
        source: '/ws/:path*',
        destination: `${proxyTarget}/ws/:path*`,
      },
    ]
  },
}
module.exports = nextConfig
