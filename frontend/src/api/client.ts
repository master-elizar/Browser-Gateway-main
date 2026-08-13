export type Role = "SUPER_ADMIN" | "USER";

export type User = {
  id: string;
  email: string;
  displayName: string;
  role: Role;
  active?: boolean;
};

export type AppSettings = {
  instanceName?: string;
  maxConcurrentSessionsGlobal: number;
  maxConcurrentSessionsPerUser: number;
  idleTimeoutSec: number;
  maxSessionDurationSec: number;
  retentionBytes: number;
  logSessionLifecycle: boolean;
  logControlActions: boolean;
  logVisitedUrls: boolean;
  logDownloads: boolean;
  logNetworkDns: boolean;
  logNetworkHttp: boolean;
  logKeystrokes: boolean;
  allowRegistration: boolean;
  passwordMinLength: number;
  passwordRequireComplexity: boolean;
  dnsMode: "docker" | "custom" | "doh" | "custom_doh" | string;
  dnsServers: string;
  dnsDohUrl: string;
  tiEnabled?: boolean;
  tiProvider?: string;
  tiAutoEnrich?: boolean;
  tiVirusTotalEnabled?: boolean;
  tiApiKey?: string;
  tiApiKeySet?: boolean;
  tiUrlhausEnabled?: boolean;
  tiThreatfoxEnabled?: boolean;
  tiThreatfoxApiKey?: string;
  tiThreatfoxApiKeySet?: boolean;
  tiAbuseipdbEnabled?: boolean;
  tiAbuseipdbApiKey?: string;
  tiAbuseipdbApiKeySet?: boolean;
  tiOtxEnabled?: boolean;
  tiOtxApiKey?: string;
  tiOtxApiKeySet?: boolean;
  tiSpamhausEnabled?: boolean;
  tiShodanEnabled?: boolean;
  tiShodanApiKey?: string;
  tiShodanApiKeySet?: boolean;
  tiSafebrowsingEnabled?: boolean;
  tiSafebrowsingApiKey?: string;
  tiSafebrowsingApiKeySet?: boolean;
  tiCrtshEnabled?: boolean;
  tiFeodoEnabled?: boolean;
  tiMalwarebazaarEnabled?: boolean;
  tiMalwarebazaarApiKey?: string;
  tiMalwarebazaarApiKeySet?: boolean;
  viewerWebrtcEnabled?: boolean;
  viewerNovncEnabled?: boolean;
  viewerFitEnabled?: boolean;
  viewerStretchEnabled?: boolean;
  viewerClipboardEnabled?: boolean;
  viewerUploadEnabled?: boolean;
  viewerDownloadsEnabled?: boolean;
  viewerNetworkEnabled?: boolean;
  historyRetentionDays?: number;
  downloadZipPasswordDefault?: string;
  downloadZipPasswordDefaultSet?: boolean;
};

export type ViewerFeatures = {
  instanceName?: string;
  viewerWebrtcEnabled: boolean;
  viewerNovncEnabled: boolean;
  viewerFitEnabled: boolean;
  viewerStretchEnabled: boolean;
  viewerClipboardEnabled: boolean;
  viewerUploadEnabled: boolean;
  viewerDownloadsEnabled: boolean;
  viewerNetworkEnabled: boolean;
  downloadZipPasswordDefaultSet?: boolean;
};

export type TIResult = {
  provider: string;
  kind: string;
  indicator: string;
  verdict: string;
  malicious: number;
  suspicious: number;
  harmless: number;
  undetected: number;
  permalink?: string;
  cached?: boolean;
  checkedAt?: string;
  error?: string;
  providers?: TIResult[];
};

export type AuditEvent = {
  id: string;
  userId?: string;
  sessionId?: string;
  type: string;
  message: string;
  summary?: string;
  createdAt: string;
  userEmail?: string;
  userDisplayName?: string;
  userRole?: string;
};

export type TLSStatus = {
  configured: boolean;
  subject?: string;
  notBefore?: string;
  notAfter?: string;
  dnsNames?: string[];
  hasChain: boolean;
  pendingRestart: boolean;
  certsDir: string;
};

export type SystemHealth = {
  status: string;
  checkedAt: string;
  checks: {
    postgres: { ok: boolean };
    redis: { ok: boolean };
    docker: { ok: boolean };
    traefik: { ok: boolean; status?: string; error?: string; name?: string };
    tls: TLSStatus;
  };
};

export type DownloadItem = {
  id: string;
  name: string;
  size: number;
  modifiedAt?: string;
};

export type NetworkEventItem = {
  id?: string;
  type: string;
  ts?: string;
  query?: string;
  qtype?: string;
  answers?: string[];
  remoteIP?: string;
  method?: string;
  url?: string;
  status?: number;
  requestHeaders?: Record<string, string>;
  responseHeaders?: Record<string, string>;
  documentURL?: string;
  resourceType?: string;
  initiator?: { type?: string; url?: string };
  // Threat intelligence (type === "ti" or enrich overlay)
  provider?: string;
  kind?: string;
  indicator?: string;
  verdict?: string;
  malicious?: number;
  suspicious?: number;
  harmless?: number;
  undetected?: number;
  permalink?: string;
  cached?: boolean;
  providers?: TIResult[];
};

export type BrowserSession = {
  id: string;
  name: string;
  status: string;
  ownerId?: string;
  containerId?: string;
  startUrl?: string;
  browser?: string;
  dnsMode?: string;
  dnsServers?: string;
  dnsDohUrl?: string;
  memoryMb?: number;
  cpus?: number;
  resolution?: string;
  /** -1 = unlimited, 0/absent = backend default (500). */
  networkEventLimit?: number;
  errorReason?: string;
  startedAt?: string;
  stoppedAt?: string;
  durationSec?: number;
  createdAt?: string;
  signalingUrl?: string;
  netmonUrl?: string;
  controlUrl?: string;
  streamUrl?: string;
  streamType?: string;
};

export type CreateSessionInput = {
  name?: string;
  startUrl?: string;
  browser?: string;
  dnsMode?: string;
  dnsServers?: string;
  dnsDohUrl?: string;
  memoryMb?: number;
  cpus?: number;
  resolution?: string;
  /** -1 = unlimited, 0/absent = backend default (500). */
  networkEventLimit?: number;
};

export type LaunchOptions = {
  browsers: { id: string; name: string; description: string }[];
  defaults: {
    browser: string;
    startUrl: string;
    dnsMode: string;
    dnsServers: string;
    dnsDohUrl: string;
    memoryMb: number;
    cpus: number;
    resolution: string;
    networkEventLimit: number;
  };
  limits: {
    memoryMbMin: number;
    memoryMbMax: number;
    cpusMin: number;
    cpusMax: number;
    resolutions: string[];
    /** Preset choices for the launch wizard; -1 means unlimited. */
    networkEventLimits: number[];
  };
};

export type AuthPair = {
  accessToken: string;
  refreshToken: string;
  user: User;
};

export type ApiErrorBody = {
  error: {
    code: string;
    message: string;
  };
};

export class ApiError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

async function parse<T>(res: Response): Promise<T> {
  const text = await res.text();
  const data = text ? (JSON.parse(text) as unknown) : null;
  if (!res.ok) {
    const body = data as ApiErrorBody | null;
    throw new ApiError(
      res.status,
      body?.error?.code ?? "HTTP_ERROR",
      body?.error?.message ?? res.statusText,
    );
  }
  return data as T;
}

function authHeaders(token?: string | null): HeadersInit {
  const h: Record<string, string> = { "Content-Type": "application/json" };
  if (token) h.Authorization = `Bearer ${token}`;
  return h;
}

export const api = {
  async version() {
    return parse<{ name: string; stage: number; version: string }>(
      await fetch("/api/version"),
    );
  },

  async setupStatus() {
    return parse<{ needsSetup: boolean; keyPresent: boolean }>(await fetch("/api/setup/status"));
  },

  async setupComplete(body: {
    setupKey: string;
    email: string;
    password: string;
    displayName?: string;
  }) {
    return parse<{ accessToken: string; refreshToken: string; user: User }>(
      await fetch("/api/setup/complete", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      }),
    );
  },

  async checkUpdates(token: string) {
    return parse<{
      currentVersion: string;
      latestTag?: string;
      latestName?: string;
      latestCommit?: string;
      installedCommit?: string;
      updateAvailable: boolean;
      checkedAt: string;
      htmlUrl?: string;
      updatePending: boolean;
      pendingStale?: boolean;
      error?: string;
      source?: string;
      progress?: {
        percent: number;
        phase: string;
        message: string;
        updatedAt?: string;
        done: boolean;
        error?: string;
      };
    }>(await fetch("/api/admin/updates", { headers: authHeaders(token) }));
  },

  async applyUpdate(token: string, opts?: { force?: boolean }) {
    const q = opts?.force ? "?force=1" : "";
    return parse<{ ok: boolean; message: string }>(
      await fetch(`/api/admin/updates/apply${q}`, {
        method: "POST",
        headers: authHeaders(token),
      }),
    );
  },

  async clearUpdate(token: string) {
    return parse<{ ok: boolean; message: string }>(
      await fetch("/api/admin/updates/clear", {
        method: "POST",
        headers: authHeaders(token),
      }),
    );
  },

  async applyNetwork(token: string, turnUrls: string) {
    return parse<{ ok: boolean; message: string }>(
      await fetch("/api/admin/network/apply", {
        method: "POST",
        headers: authHeaders(token),
        body: JSON.stringify({ turnUrls }),
      }),
    );
  },

  async networkStatus(token: string) {
    return parse<{
      pending: boolean;
      progress?: {
        percent: number;
        phase: string;
        message: string;
        updatedAt?: string;
        done: boolean;
        error?: string;
      };
    }>(await fetch("/api/admin/network/status", { headers: authHeaders(token) }));
  },

  async register(email: string, password: string, displayName?: string) {
    return parse<AuthPair>(
      await fetch("/api/auth/register", {
        method: "POST",
        headers: authHeaders(),
        body: JSON.stringify({ email, password, displayName }),
      }),
    );
  },

  async login(email: string, password: string) {
    return parse<AuthPair>(
      await fetch("/api/auth/login", {
        method: "POST",
        headers: authHeaders(),
        body: JSON.stringify({ email, password }),
      }),
    );
  },

  async refresh(refreshToken: string) {
    return parse<AuthPair>(
      await fetch("/api/auth/refresh", {
        method: "POST",
        headers: authHeaders(),
        body: JSON.stringify({ refreshToken }),
      }),
    );
  },

  async logout(refreshToken: string | null) {
    return parse<{ ok: boolean }>(
      await fetch("/api/auth/logout", {
        method: "POST",
        headers: authHeaders(),
        body: JSON.stringify({ refreshToken: refreshToken ?? "" }),
      }),
    );
  },

  async changePassword(token: string, currentPassword: string, newPassword: string) {
    return parse<{ ok: boolean }>(
      await fetch("/api/auth/password", {
        method: "POST",
        headers: authHeaders(token),
        body: JSON.stringify({ currentPassword, newPassword }),
      }),
    );
  },

  async me(token: string) {
    return parse<User>(
      await fetch("/api/auth/me", { headers: authHeaders(token) }),
    );
  },

  async listUsers(token: string) {
    return parse<{ items: User[]; total: number }>(
      await fetch("/api/admin/users", { headers: authHeaders(token) }),
    );
  },

  async createUser(
    token: string,
    body: { email: string; password: string; displayName?: string; role?: Role },
  ) {
    return parse<User>(
      await fetch("/api/admin/users", {
        method: "POST",
        headers: authHeaders(token),
        body: JSON.stringify(body),
      }),
    );
  },

  async patchUser(
    token: string,
    id: string,
    body: { role?: Role; active?: boolean; password?: string },
  ) {
    return parse<User>(
      await fetch(`/api/admin/users/${id}`, {
        method: "PATCH",
        headers: authHeaders(token),
        body: JSON.stringify(body),
      }),
    );
  },

  async deleteUser(token: string, id: string) {
    const res = await fetch(`/api/admin/users/${id}`, {
      method: "DELETE",
      headers: authHeaders(token),
    });
    if (!res.ok) {
      return parse<never>(res);
    }
  },

  async listAdminSessions(token: string) {
    return parse<{ items: BrowserSession[]; total: number }>(
      await fetch("/api/admin/sessions", { headers: authHeaders(token) }),
    );
  },

  async adminStopSession(token: string, id: string) {
    return parse<BrowserSession>(
      await fetch(`/api/admin/sessions/${id}/stop`, {
        method: "POST",
        headers: authHeaders(token),
      }),
    );
  },

  async getSettings(token: string) {
    return parse<AppSettings>(
      await fetch("/api/admin/settings", { headers: authHeaders(token) }),
    );
  },

  async getViewerFeatures(token: string) {
    return parse<ViewerFeatures>(
      await fetch("/api/viewer/features", { headers: authHeaders(token) }),
    );
  },

  async putSettings(token: string, body: AppSettings) {
    return parse<AppSettings>(
      await fetch("/api/admin/settings", {
        method: "PUT",
        headers: authHeaders(token),
        body: JSON.stringify(body),
      }),
    );
  },

  async listAudit(
    token: string,
    params?: { type?: string; userId?: string; sessionId?: string; limit?: number; offset?: number },
  ) {
    const q = new URLSearchParams();
    if (params?.type) q.set("type", params.type);
    if (params?.userId) q.set("userId", params.userId);
    if (params?.sessionId) q.set("sessionId", params.sessionId);
    if (params?.limit) q.set("limit", String(params.limit));
    if (params?.offset) q.set("offset", String(params.offset));
    const qs = q.toString();
    return parse<{ items: AuditEvent[]; total: number }>(
      await fetch(`/api/admin/audit${qs ? `?${qs}` : ""}`, { headers: authHeaders(token) }),
    );
  },

  async exportAudit(token: string, format: "csv" | "json" | "rfc5424" | "syslog" = "csv") {
    const res = await fetch(`/api/admin/audit/export?format=${format}`, {
      headers: authHeaders(token),
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body?.error?.message || res.statusText);
    }
    return res.blob();
  },

  async getTLS(token: string) {
    return parse<TLSStatus>(await fetch("/api/admin/tls", { headers: authHeaders(token) }));
  },

  async putTLS(
    token: string,
    body: {
      format: "pem" | "pkcs12";
      certificatePem?: string;
      privateKeyPem?: string;
      chainPem?: string;
      pkcs12Base64?: string;
      pkcs12Password?: string;
      applyNow?: boolean;
    },
  ) {
    return parse<{ ok: boolean; pendingRestart: boolean; applyError?: string; tls?: TLSStatus }>(
      await fetch("/api/admin/tls", {
        method: "PUT",
        headers: authHeaders(token),
        body: JSON.stringify(body),
      }),
    );
  },

  async applyTLS(token: string) {
    return parse<{ ok: boolean; pendingRestart: boolean; tls?: TLSStatus }>(
      await fetch("/api/admin/tls/apply", {
        method: "POST",
        headers: authHeaders(token),
      }),
    );
  },

  async systemHealth(token: string) {
    return parse<SystemHealth>(
      await fetch("/api/admin/health", { headers: authHeaders(token) }),
    );
  },

  async getIceServers(token: string) {
    return parse<{ iceServers: RTCIceServer[] }>(
      await fetch("/api/webrtc/ice", { headers: authHeaders(token) }),
    );
  },

  async getSession(token: string, id: string) {
    return parse<BrowserSession>(
      await fetch(`/api/browser/${id}`, { headers: authHeaders(token) }),
    );
  },

  async listSessions(token: string) {
    return parse<{ items: BrowserSession[]; total: number }>(
      await fetch("/api/browser/list", { headers: authHeaders(token) }),
    );
  },

  async launchOptions(token: string) {
    return parse<LaunchOptions>(
      await fetch("/api/browser/launch-options", { headers: authHeaders(token) }),
    );
  },

  async createSession(token: string, input?: CreateSessionInput | string) {
    const body: CreateSessionInput =
      typeof input === "string" || input === undefined
        ? { name: (input as string) || "Session" }
        : input;
    return parse<BrowserSession>(
      await fetch("/api/browser/create", {
        method: "POST",
        headers: authHeaders(token),
        body: JSON.stringify(body),
      }),
    );
  },

  async stopSession(token: string, id: string) {
    return parse<unknown>(
      await fetch(`/api/browser/${id}/stop`, {
        method: "POST",
        headers: authHeaders(token),
      }),
    );
  },

  async clipboard(
    token: string,
    id: string,
    direction: "toRemote" | "fromRemote",
    text?: string,
  ) {
    return parse<{ text?: string; ok?: boolean }>(
      await fetch(`/api/browser/${id}/clipboard`, {
        method: "POST",
        headers: authHeaders(token),
        body: JSON.stringify({ direction, text: text ?? "" }),
      }),
    );
  },

  async upload(token: string, id: string, file: File) {
    const fd = new FormData();
    fd.append("file", file);
    const res = await fetch(`/api/browser/${id}/upload`, {
      method: "POST",
      headers: token ? { Authorization: `Bearer ${token}` } : undefined,
      body: fd,
    });
    return parse<{ ok?: boolean; name?: string; size?: number }>(res);
  },

  async listDownloads(token: string, id: string) {
    return parse<{ items: DownloadItem[] }>(
      await fetch(`/api/browser/${id}/downloads`, { headers: authHeaders(token) }),
    );
  },

  downloadUrl(id: string, fileId: string, opts?: { format?: "file" | "zip"; password?: string }) {
    const q = new URLSearchParams();
    if (opts?.format) q.set("format", opts.format);
    if (opts?.password) q.set("password", opts.password);
    const qs = q.toString();
    return `/api/browser/${id}/downloads/${encodeURIComponent(fileId)}${qs ? `?${qs}` : ""}`;
  },

  async listHistory(
    token: string,
    query?: { name?: string; browser?: string; from?: string; to?: string; tiVerdict?: string },
  ) {
    const q = new URLSearchParams();
    if (query?.name) q.set("name", query.name);
    if (query?.browser) q.set("browser", query.browser);
    if (query?.from) q.set("from", query.from);
    if (query?.to) q.set("to", query.to);
    if (query?.tiVerdict) q.set("tiVerdict", query.tiVerdict);
    const qs = q.toString();
    return parse<{ items: HistoryListItem[]; total: number }>(
      await fetch(`/api/history${qs ? `?${qs}` : ""}`, { headers: authHeaders(token) }),
    );
  },

  async getHistory(token: string, id: string) {
    return parse<HistoryDetail>(
      await fetch(`/api/history/${id}`, { headers: authHeaders(token) }),
    );
  },

  historyFrameUrl(id: string, eventId: string) {
    return `/api/history/${id}/frames/${eventId}`;
  },

  async deleteHistory(token: string, id: string) {
    return parse<{ ok: boolean }>(
      await fetch(`/api/history/${id}`, {
        method: "DELETE",
        headers: authHeaders(token),
      }),
    );
  },

  async listNetworkEvents(token: string, id: string, type?: string) {
    const q = type ? `?type=${encodeURIComponent(type)}` : "";
    return parse<{ items: NetworkEventItem[]; total: number }>(
      await fetch(`/api/browser/${id}/network/events${q}`, { headers: authHeaders(token) }),
    );
  },

  async clearNetworkEvents(token: string, id: string, type: "all" | "dns" | "http" = "all") {
    const q = `?type=${encodeURIComponent(type)}`;
    return parse<{ ok: boolean; deleted: number }>(
      await fetch(`/api/browser/${id}/network/events${q}`, {
        method: "DELETE",
        headers: authHeaders(token),
      }),
    );
  },

  async tiLookup(token: string, body: { kind?: string; value: string; sessionId?: string }) {
    return parse<TIResult>(
      await fetch("/api/ti/lookup", {
        method: "POST",
        headers: authHeaders(token),
        body: JSON.stringify(body),
      }),
    );
  },

  async enrichNetwork(token: string, sessionId: string, values?: string[]) {
    return parse<{ items: TIResult[]; total: number }>(
      await fetch(`/api/browser/${sessionId}/network/enrich`, {
        method: "POST",
        headers: authHeaders(token),
        body: JSON.stringify({ values: values || [] }),
      }),
    );
  },
};

export type HistoryListItem = {
  id: string;
  name: string;
  browser: string;
  status: string;
  startUrl?: string;
  ownerId: string;
  startedAt?: string;
  stoppedAt?: string;
  createdAt: string;
  frameCount: number;
  tiVerdict?: string;
  durationSec?: number;
};

export type HistoryDetail = {
  session: {
    id: string;
    name: string;
    browser: string;
    status: string;
    startUrl?: string;
    ownerId: string;
    startedAt?: string;
    stoppedAt?: string;
    createdAt: string;
    dnsMode?: string;
    memoryMb?: number;
    cpus?: number;
    resolution?: string;
    errorReason?: string;
  };
  frames: Array<{
    id: string;
    kind: string;
    url?: string;
    createdAt: string;
    hasImage?: boolean;
    type: string;
    meta?: Record<string, unknown>;
  }>;
  network: Array<Record<string, unknown>>;
  audit: Array<Record<string, unknown>>;
};
