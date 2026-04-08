import type { NextRequest } from 'next/server';
import { NextResponse } from 'next/server';
import {
  MOCK_SESSION_COOKIE,
  mockAdminUsers,
  mockBookingResponse,
  mockHistoryResponse,
  mockPresetFull,
  mockSlotsResponse,
  mockUser,
} from '@/mocks/api-fixtures';

function json(data: unknown, init?: ResponseInit) {
  return NextResponse.json(data, init);
}

function mockRole(): 'ADMIN' | 'NON_ADMIN' {
  return process.env.NEXT_PUBLIC_MOCK_ROLE === 'ADMIN' ? 'ADMIN' : 'NON_ADMIN';
}

function isLoggedIn(req: NextRequest) {
  return req.cookies.get(MOCK_SESSION_COOKIE)?.value === '1';
}

function segments(pathname: string) {
  return pathname
    .replace(/^\/api\/v1\/?/, '')
    .split('/')
    .filter(Boolean);
}

/** Handles `/api/v1/*` when NEXT_PUBLIC_USE_MOCK_API=true (local preview only). */
export async function handleDevMockRequest(req: NextRequest): Promise<Response> {
  const segs = segments(req.nextUrl.pathname);
  const method = req.method;

  if (segs[0] === 'auth' && segs[1] === 'me' && method === 'GET') {
    if (!isLoggedIn(req)) {
      return json({ message: 'Unauthorized' }, { status: 401 });
    }
    return json({ user: mockUser(mockRole()) });
  }

  if (segs[0] === 'auth' && segs[1] === 'login' && method === 'POST') {
    let body: { username?: string; password?: string } = {};
    try {
      body = (await req.json()) as { username?: string; password?: string };
    } catch {
      /* empty */
    }
    const username = typeof body.username === 'string' ? body.username : 'mockuser';
    const res = json({
      user: mockUser(mockRole(), username),
    });
    res.cookies.set(MOCK_SESSION_COOKIE, '1', {
      path: '/',
      sameSite: 'lax',
      maxAge: 60 * 60 * 24,
    });
    return res;
  }

  if (segs[0] === 'auth' && segs[1] === 'logout' && method === 'POST') {
    const res = json({});
    res.cookies.set(MOCK_SESSION_COOKIE, '', { path: '/', maxAge: 0 });
    return res;
  }

  if (!isLoggedIn(req)) {
    return json({ message: 'Unauthorized' }, { status: 401 });
  }

  if (segs[0] === 'preset' && segs.length === 1 && method === 'GET') {
    return json(mockPresetFull());
  }

  if (segs[0] === 'preset' && segs.length === 1 && method === 'PUT') {
    return json({});
  }

  if (segs[0] === 'preset' && segs[1] === 'cancel' && method === 'POST') {
    return json({});
  }

  if (segs[0] === 'slots' && method === 'GET') {
    return json(mockSlotsResponse);
  }

  if (segs[0] === 'booking' && segs.length === 1 && method === 'GET') {
    return json(mockBookingResponse);
  }

  if (segs[0] === 'booking' && segs[1] === 'book' && method === 'POST') {
    return json({ bookingID: 'MOCK-NEW' });
  }

  if (segs[0] === 'booking' && segs[1] === 'cancel' && method === 'POST') {
    return json({});
  }

  if (segs[0] === 'booking' && segs[1] === 'check-status' && method === 'GET') {
    return json({ ok: true });
  }

  if (segs[0] === 'history' && method === 'GET') {
    return json(mockHistoryResponse);
  }

  if (segs[0] === 'admin' && segs[1] === 'users' && segs.length === 2 && method === 'GET') {
    return json(mockAdminUsers);
  }

  if (segs[0] === 'admin' && segs[1] === 'register' && method === 'POST') {
    return json({ ok: true });
  }

  if (segs[0] === 'admin' && segs[1] === 'users' && segs.length === 3 && method === 'DELETE') {
    return json({});
  }

  if (
    segs[0] === 'admin' &&
    segs[1] === 'users' &&
    segs.length === 4 &&
    segs[3] === 'role' &&
    method === 'PUT'
  ) {
    return json({});
  }

  if (segs[0] === 'admin' && segs[1] === 'presets' && segs.length === 3 && method === 'DELETE') {
    return json({});
  }

  return json(
    { message: `Dev mock: unhandled ${method} /api/v1/${segs.join('/')}` },
    { status: 501 },
  );
}
