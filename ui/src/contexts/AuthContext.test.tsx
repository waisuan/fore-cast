import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { AuthProvider, useAuth } from './AuthContext';
import { api, API_ENDPOINTS } from '@/utils/api';

vi.mock('@/utils/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/utils/api')>();
  return {
    ...actual,
    api: {
      get: vi.fn(),
      post: vi.fn(),
      put: vi.fn(),
      delete: vi.fn(),
    },
  };
});

function Probe() {
  const a = useAuth();
  return (
    <div>
      <span data-testid="loading">{String(a.isLoading)}</span>
      <span data-testid="auth">{String(a.isAuthenticated)}</span>
      <span data-testid="admin">{String(a.isAdmin)}</span>
      <span data-testid="user">{a.user?.username ?? 'none'}</span>
      <button type="button" onClick={() => void a.login('u1', 'p1')}>
        do-login
      </button>
      <button type="button" onClick={() => void a.logout()}>
        do-logout
      </button>
    </div>
  );
}

describe('AuthProvider', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('loads session from GET /me and exposes the user', async () => {
    vi.mocked(api.get).mockResolvedValue({
      user: { username: 'alice', role: 'NON_ADMIN' },
    });

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );

    await waitFor(() => expect(screen.getByTestId('loading')).toHaveTextContent('false'));
    expect(api.get).toHaveBeenCalledWith(API_ENDPOINTS.me);
    expect(screen.getByTestId('auth')).toHaveTextContent('true');
    expect(screen.getByTestId('user')).toHaveTextContent('alice');
    expect(screen.getByTestId('admin')).toHaveTextContent('false');
  });

  it('treats failed session load as logged out', async () => {
    vi.mocked(api.get).mockRejectedValue(new Error('401'));

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );

    await waitFor(() => expect(screen.getByTestId('loading')).toHaveTextContent('false'));
    expect(screen.getByTestId('auth')).toHaveTextContent('false');
    expect(screen.getByTestId('user')).toHaveTextContent('none');
  });

  it('login posts credentials and updates user', async () => {
    vi.mocked(api.get).mockRejectedValue(new Error('no session'));
    vi.mocked(api.post).mockResolvedValue({
      user: { username: 'bob', role: 'ADMIN' },
    });

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );

    await waitFor(() => expect(screen.getByTestId('loading')).toHaveTextContent('false'));

    fireEvent.click(screen.getByRole('button', { name: 'do-login' }));

    await waitFor(() => {
      expect(api.post).toHaveBeenCalledWith(API_ENDPOINTS.login, {
        username: 'u1',
        password: 'p1',
      });
    });
    expect(screen.getByTestId('user')).toHaveTextContent('bob');
    expect(screen.getByTestId('admin')).toHaveTextContent('true');
  });

  it('logout posts logout and clears user', async () => {
    vi.mocked(api.get).mockResolvedValue({
      user: { username: 'alice', role: 'NON_ADMIN' },
    });
    vi.mocked(api.post).mockResolvedValue(undefined);

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    );

    await waitFor(() => expect(screen.getByTestId('auth')).toHaveTextContent('true'));

    fireEvent.click(screen.getByRole('button', { name: 'do-logout' }));

    await waitFor(() => {
      expect(api.post).toHaveBeenCalledWith(API_ENDPOINTS.logout);
    });
    expect(screen.getByTestId('auth')).toHaveTextContent('false');
  });
});
