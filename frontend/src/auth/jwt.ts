// Client-side JWT payload decode -- no signature verification (the server always
// re-validates that on every request), just reading `exp` so AuthContext knows when to
// proactively refresh the access token instead of waiting for API calls/WebSockets to
// start failing with 401 "invalid access token" once it expires mid-session.
export function decodeJwtExpiryMs(token: string): number | null {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;
    const payload = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const padded = payload + "=".repeat((4 - (payload.length % 4)) % 4);
    const json = atob(padded);
    const claims = JSON.parse(json) as { exp?: number };
    if (typeof claims.exp !== "number") return null;
    return claims.exp * 1000;
  } catch {
    return null;
  }
}
