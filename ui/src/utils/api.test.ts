import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { api, ApiError, setOnUnauthorized } from './api';

function mockFetchResponse(
  body: string,
  init: { status: number; statusText?: string },
) {
  return Promise.resolve({
    ok: init.status >= 200 && init.status < 300,
    status: init.status,
    statusText: init.statusText ?? '',
    text: async () => body,
  } as Response);
}

describe('api request helper', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    setOnUnauthorized(null);
  });

  it('parses JSON bodies on success', async () => {
    vi.mocked(fetch).mockReturnValue(mockFetchResponse('{"a":1}', { status: 200 }));
    await expect(api.get('/api/v1/x')).resolves.toEqual({ a: 1 });
    expect(fetch).toHaveBeenCalledWith(
      expect.stringContaining('/api/v1/x'),
      expect.objectContaining({ method: 'GET', credentials: 'include' }),
    );
  });

  it('sends JSON for POST bodies', async () => {
    vi.mocked(fetch).mockReturnValue(mockFetchResponse('{}', { status: 200 }));
    await api.post('/api/v1/y', { foo: 'bar' });
    expect(fetch).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ foo: 'bar' }),
      }),
    );
  });

  it('throws ApiError with message from JSON body', async () => {
    vi.mocked(fetch).mockReturnValue(
      mockFetchResponse(JSON.stringify({ message: 'bad thing' }), { status: 400 }),
    );
    await expect(api.get('/api/v1/z')).rejects.toMatchObject({
      message: 'bad thing',
      status: 400,
    });
  });

  it('invokes onUnauthorized for 401 on normal API paths', async () => {
    const on401 = vi.fn();
    setOnUnauthorized(on401);
    vi.mocked(fetch).mockReturnValue(
      mockFetchResponse(JSON.stringify({ message: 'nope' }), { status: 401 }),
    );
    await expect(api.get('/api/v1/me')).rejects.toBeInstanceOf(ApiError);
    expect(on401).toHaveBeenCalledTimes(1);
  });

  it('does not invoke onUnauthorized for 401 on admin paths', async () => {
    const on401 = vi.fn();
    setOnUnauthorized(on401);
    vi.mocked(fetch).mockReturnValue(
      mockFetchResponse(JSON.stringify({ message: 'nope' }), { status: 401 }),
    );
    await expect(api.get('/api/v1/admin/users')).rejects.toBeInstanceOf(ApiError);
    expect(on401).not.toHaveBeenCalled();
  });

  it('falls back to statusText when JSON body has no message field', async () => {
    vi.mocked(fetch).mockReturnValue(
      mockFetchResponse('{}', { status: 500, statusText: 'Server Error' }),
    );
    await expect(api.get('/api/v1/x')).rejects.toMatchObject({
      message: 'Server Error',
      status: 500,
    });
  });
});
