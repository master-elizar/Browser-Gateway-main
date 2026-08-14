import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { api, ApiError, type User } from "../api/client";
import { decodeJwtExpiryMs } from "./jwt";

type AuthState = {
  user: User | null;
  accessToken: string | null;
  bootstrapping: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string, displayName?: string) => Promise<void>;
  logout: () => Promise<void>;
};

const AuthContext = createContext<AuthState | null>(null);

const ACCESS_KEY = "bg.accessToken";
const REFRESH_KEY = "bg.refreshToken";

function persistPair(accessToken: string, refreshToken: string) {
  localStorage.setItem(ACCESS_KEY, accessToken);
  localStorage.setItem(REFRESH_KEY, refreshToken);
}

function clearPair() {
  localStorage.removeItem(ACCESS_KEY);
  localStorage.removeItem(REFRESH_KEY);
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [accessToken, setAccessToken] = useState<string | null>(null);
  const [bootstrapping, setBootstrapping] = useState(true);

  // Single-flight guard: if the scheduled refresh, a visibility-regain check, and (in the
  // future) a reactive 401 handler all fire around the same moment, they share one in-flight
  // call instead of each independently consuming/rotating the stored refresh token.
  const refreshInFlight = useRef<Promise<string | null> | null>(null);

  const doRefresh = useCallback((): Promise<string | null> => {
    if (refreshInFlight.current) return refreshInFlight.current;
    const p = (async () => {
      const refresh = localStorage.getItem(REFRESH_KEY);
      if (!refresh) return null;
      try {
        const pair = await api.refresh(refresh);
        persistPair(pair.accessToken, pair.refreshToken);
        setAccessToken(pair.accessToken);
        setUser(pair.user);
        return pair.accessToken;
      } catch {
        clearPair();
        setAccessToken(null);
        setUser(null);
        return null;
      } finally {
        refreshInFlight.current = null;
      }
    })();
    refreshInFlight.current = p;
    return p;
  }, []);

  useEffect(() => {
    const run = async () => {
      const token = localStorage.getItem(ACCESS_KEY);
      const refresh = localStorage.getItem(REFRESH_KEY);
      if (!token && !refresh) {
        setBootstrapping(false);
        return;
      }
      if (token) {
        try {
          const me = await api.me(token);
          setAccessToken(token);
          setUser(me);
          setBootstrapping(false);
          return;
        } catch {
          // stored access token didn't validate (expired or otherwise) -- fall through
          // to the refresh token below instead of leaving the user logged out for that
          // alone.
        }
      }
      await doRefresh();
      setBootstrapping(false);
    };
    void run();
  }, [doRefresh]);

  // Access tokens are short-lived (backend ACCESS_TOKEN_TTL, 15m by default) so that a
  // stolen one has a small blast radius -- but nothing previously renewed it while a page
  // stayed open past that window. Every API call and WebSocket (the session viewer's netmon
  // panel in particular) would then start failing with 401 "invalid access token" and stay
  // dead until a manual reload, since the only place that ever refreshed was this
  // component's mount-time bootstrap above. Decode the token's own `exp` (no server round
  // trip needed) and proactively refresh a minute before it actually expires; every
  // WebSocket-opening effect elsewhere already depends on `accessToken`, so it reconnects
  // with the new one automatically once this updates it.
  useEffect(() => {
    if (!accessToken) return;
    const expMs = decodeJwtExpiryMs(accessToken);
    if (!expMs) return;
    const delay = Math.max(expMs - Date.now() - 60_000, 2_000);
    const timer = window.setTimeout(() => {
      void doRefresh();
    }, delay);
    // Background tabs throttle/pause setTimeout, so a session left open in an inactive tab
    // can sail past the scheduled refresh entirely -- catch up as soon as the tab is
    // visible again instead of waiting for whatever's left of the (possibly already-missed)
    // timer.
    const onVisible = () => {
      if (document.visibilityState === "visible" && Date.now() >= expMs - 60_000) {
        void doRefresh();
      }
    };
    document.addEventListener("visibilitychange", onVisible);
    return () => {
      window.clearTimeout(timer);
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, [accessToken, doRefresh]);

  const applyPair = useCallback((pair: { accessToken: string; refreshToken: string; user: User }) => {
    persistPair(pair.accessToken, pair.refreshToken);
    setAccessToken(pair.accessToken);
    setUser(pair.user);
  }, []);

  const login = useCallback(
    async (email: string, password: string) => {
      const pair = await api.login(email, password);
      applyPair(pair);
    },
    [applyPair],
  );

  const register = useCallback(
    async (email: string, password: string, displayName?: string) => {
      const pair = await api.register(email, password, displayName);
      applyPair(pair);
    },
    [applyPair],
  );

  const logout = useCallback(async () => {
    const refresh = localStorage.getItem(REFRESH_KEY);
    try {
      await api.logout(refresh);
    } catch {
      // ignore network/logout errors
    }
    clearPair();
    setAccessToken(null);
    setUser(null);
  }, []);

  const value = useMemo(
    () => ({
      user,
      accessToken,
      bootstrapping,
      login,
      register,
      logout,
    }),
    [user, accessToken, bootstrapping, login, register, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth outside provider");
  return ctx;
}

export function isNotImplemented(err: unknown) {
  return err instanceof ApiError && (err.status === 501 || err.code === "NOT_IMPLEMENTED");
}
