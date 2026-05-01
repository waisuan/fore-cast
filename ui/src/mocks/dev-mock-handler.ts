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

// Mutable in-memory mock so PUT /preset toggles (enable, override, etc.) survive
// across requests within a single `next dev` process and the homepage / settings
// reflect them immediately. Reset when the dev server restarts.
type MockPreset = ReturnType<typeof mockPresetFull>;
type PresetPatch = Partial<
  Pick<
    MockPreset,
    | 'enabled'
    | 'enable_notifications'
    | 'cutoff'
    | 'retry_interval'
    | 'timeout'
    | 'ntfy_topic'
    | 'course'
    | 'override_course'
    | 'override_until'
  >
>;
const presetState: MockPreset = mockPresetFull();
if (process.env.NEXT_PUBLIC_MOCK_PRESET_ENABLED === 'true') {
  presetState.enabled = true;
}

function mergePresetUpdate(body: PresetPatch) {
  if (typeof body.enabled === 'boolean') presetState.enabled = body.enabled;
  if (typeof body.enable_notifications === 'boolean') {
    presetState.enable_notifications = body.enable_notifications;
  }
  if (typeof body.cutoff === 'string') presetState.cutoff = body.cutoff;
  if (typeof body.retry_interval === 'string') presetState.retry_interval = body.retry_interval;
  if (typeof body.timeout === 'string') presetState.timeout = body.timeout;
  if (typeof body.ntfy_topic === 'string') presetState.ntfy_topic = body.ntfy_topic;
  if (typeof body.course === 'string') presetState.course = body.course;
  if (typeof body.override_course === 'string') presetState.override_course = body.override_course;
  if (body.override_until === null || typeof body.override_until === 'string') {
    presetState.override_until = body.override_until ?? null;
  }
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
    return json(presetState);
  }

  if (segs[0] === 'preset' && segs.length === 1 && method === 'PUT') {
    let body: PresetPatch = {};
    try {
      body = (await req.json()) as PresetPatch;
    } catch {
      // Invalid JSON: fall through with empty patch (mock is permissive).
    }
    mergePresetUpdate(body);
    return json({});
  }

  if (segs[0] === 'preset' && segs[1] === 'cancel' && method === 'POST') {
    return json({});
  }

  if (segs[0] === 'preset' && segs[1] === 'skip-next') {
    if (method === 'POST') {
      if (!presetState.enabled) {
        return json({ message: 'auto-booker is not enabled for this account' }, { status: 409 });
      }
      presetState.skip_next_run = true;
      return json({ status: 'skip_requested' });
    }
    if (method === 'DELETE') {
      presetState.skip_next_run = false;
      return json({ status: 'skip_cleared' });
    }
    return json({ message: 'method not allowed' }, { status: 405 });
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
