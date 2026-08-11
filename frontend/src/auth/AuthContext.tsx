import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { api, ApiError, type User } from "../api/client";

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

  useEffect(() => {
    const run = async () => {
      try {
        let token = localStorage.getItem(ACCESS_KEY);
        const refresh = localStorage.getItem(REFRESH_KEY);
        if (!token && !refresh) return;

        if (token) {
          try {
            const me = await api.me(token);
            setAccessToken(token);
            setUser(me);
            return;
          } catch {
            // try refresh below
          }
        }

        if (refresh) {
          const pair = await api.refresh(refresh);
          persistPair(pair.accessToken, pair.refreshToken);
          setAccessToken(pair.accessToken);
          setUser(pair.user);
        }
      } catch {
        clearPair();
      } finally {
        setBootstrapping(false);
      }
    };
    void run();
  }, []);

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
