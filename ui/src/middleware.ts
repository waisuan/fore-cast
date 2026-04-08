import type { NextRequest } from 'next/server';
import { NextResponse } from 'next/server';
import { handleDevMockRequest } from '@/mocks/dev-mock-handler';

/**
 * When NEXT_PUBLIC_USE_MOCK_API=true (see `npm run dev:mock`), answers `/api/v1/*` locally
 * without the Go server. Omit that variable in production builds.
 */
export async function middleware(request: NextRequest) {
  if (process.env.NEXT_PUBLIC_USE_MOCK_API !== 'true') {
    return NextResponse.next();
  }
  return handleDevMockRequest(request);
}

export const config = {
  matcher: '/api/v1/:path*',
};
