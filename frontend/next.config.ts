import type { NextConfig } from 'next'

const USER_SERVICE_URL = process.env.USER_SERVICE_URL || 'http://user-service:8080'
const TUTORING_SERVICE_URL = process.env.TUTORING_SERVICE_URL || 'http://tutoring-service:8080'

const config: NextConfig = {
  output: 'standalone',

  // GKE internal DNS rewrites — forward backend traffic to services.
  // Note: auth endpoints go through /app/api/user/auth/ (route handler) so we
  // can manage the tl_token cookie. Everything else can rewrite directly.
  async rewrites() {
    return [
      {
        source: '/api/user/users/:path*',
        destination: `${USER_SERVICE_URL}/users/:path*`,
      },
      {
        source: '/api/user/webhooks/:path*',
        destination: `${USER_SERVICE_URL}/webhooks/:path*`,
      },
      {
        source: '/api/tutor/:path*',
        destination: `${TUTORING_SERVICE_URL}/:path*`,
      },
    ]
  },
}

export default config
