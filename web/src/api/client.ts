import type {
  RunConfig,
  Snapshot,
  RunSummary,
  Preset,
  ServerSettings,
  Package,
  ComparisonResponse,
  MetricValue,
  GrafanaSettings,
  AuthUser,
  Tenant,
  TenantMember,
  TenantAPIToken,
  DatabaseKind,
  ProbeRequest,
  ProbeResponse,
} from "./types";

const API_BASE = "/api/v1";

// Module-level access token — never stored in localStorage.
let _accessToken: string | null = null;

export function setAccessToken(token: string | null) {
  _accessToken = token;
}

export function getAccessToken(): string | null {
  return _accessToken;
}

// Flag to avoid concurrent refresh attempts.
let _refreshPromise: Promise<{ access_token: string }> | null = null;

async function request<T>(
  path: string,
  options: RequestInit = {},
  _isRetry = false
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string>),
  };

  if (_accessToken) {
    headers["Authorization"] = `Bearer ${_accessToken}`;
  }

  const res = await fetch(path, { ...options, headers });

  if (res.status === 401 && !_isRetry && _accessToken) {
    // Try a single silent refresh (only if we had a token — skip for login/public calls).
    try {
      const r = await refreshToken();
      _accessToken = r.access_token;
      headers["Authorization"] = `Bearer ${_accessToken}`;
      const retry = await fetch(path, { ...options, headers });
      if (!retry.ok) {
        const text = await retry.text();
        throw new SessionExpiredError(`${retry.status}: ${text}`);
      }
      return retry.json();
    } catch (e) {
      // Refresh failed — clear token, throw SessionExpiredError so UI can redirect.
      _accessToken = null;
      if (e instanceof SessionExpiredError) throw e;
      throw new SessionExpiredError("session expired");
    }
  }

  if (!res.ok) {
    const text = await res.text();
    throw new Error(`${res.status}: ${text}`);
  }

  return res.json();
}

// Thrown when auth refresh fails — UI should redirect to login.
export class SessionExpiredError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "SessionExpiredError";
  }
}

// ---------- Auth ----------

export async function loginAPI(
  username: string,
  password: string
): Promise<{ access_token: string }> {
  return request(`${API_BASE}/auth/login`, {
    method: "POST",
    body: JSON.stringify({ username, password }),
  });
}

export async function refreshToken(): Promise<{ access_token: string }> {
  if (_refreshPromise) return _refreshPromise;
  _refreshPromise = (async () => {
    const res = await fetch(`${API_BASE}/auth/refresh`, {
      method: "POST",
      credentials: "include",
    });
    if (!res.ok) throw new Error("refresh failed");
    return res.json();
  })();
  try {
    return await _refreshPromise;
  } finally {
    _refreshPromise = null;
  }
}

export async function meAPI(): Promise<AuthUser> {
  return request(`${API_BASE}/auth/me`);
}

export async function logoutAPI(): Promise<void> {
  await fetch(`${API_BASE}/auth/logout`, {
    method: "POST",
    credentials: "include",
    headers: _accessToken
      ? { Authorization: `Bearer ${_accessToken}` }
      : undefined,
  });
}

export async function selectTenantAPI(
  tenantId: string
): Promise<{ access_token: string }> {
  return request(`${API_BASE}/auth/select-tenant`, {
    method: "POST",
    body: JSON.stringify({ tenant_id: tenantId }),
  });
}

export async function changePasswordAPI(
  currentPassword: string,
  newPassword: string
): Promise<{ status: string }> {
  return request(`${API_BASE}/auth/password`, {
    method: "PUT",
    body: JSON.stringify({
      current_password: currentPassword,
      new_password: newPassword,
    }),
  });
}

// ---------- Tenant members ----------

export async function listMembers(): Promise<TenantMember[]> {
  return request(`${API_BASE}/tenant/members`);
}

export async function addMember(
  userId: string,
  role: string
): Promise<TenantMember> {
  return request(`${API_BASE}/tenant/members`, {
    method: "POST",
    body: JSON.stringify({ user_id: userId, role }),
  });
}

export async function updateMemberRole(
  userId: string,
  role: string
): Promise<{ status: string }> {
  return request(`${API_BASE}/tenant/members/${userId}`, {
    method: "PUT",
    body: JSON.stringify({ role }),
  });
}

export async function removeMember(
  userId: string
): Promise<{ status: string }> {
  return request(`${API_BASE}/tenant/members/${userId}`, {
    method: "DELETE",
  });
}

// ---------- Tenant API tokens ----------

export async function listAPITokens(): Promise<TenantAPIToken[]> {
  return request(`${API_BASE}/tenant/tokens`);
}

export async function createAPIToken(
  name: string,
  role: string,
  expiresAt?: string
): Promise<{ token: TenantAPIToken; plaintext: string }> {
  return request(`${API_BASE}/tenant/tokens`, {
    method: "POST",
    body: JSON.stringify({ name, role, expires_at: expiresAt || null }),
  });
}

export async function revokeAPIToken(
  id: string
): Promise<{ status: string }> {
  return request(`${API_BASE}/tenant/tokens/${id}`, {
    method: "DELETE",
  });
}

// ---------- Admin ----------

export async function listTenantsAdmin(): Promise<Tenant[]> {
  return request(`${API_BASE}/admin/tenants`);
}

export async function createTenantAdmin(
  name: string
): Promise<Tenant> {
  return request(`${API_BASE}/admin/tenants`, {
    method: "POST",
    body: JSON.stringify({ name }),
  });
}

export async function deleteTenantAdmin(
  id: string
): Promise<{ status: string }> {
  return request(`${API_BASE}/admin/tenants/${id}`, {
    method: "DELETE",
  });
}

export async function listUsersAdmin(): Promise<
  { id: string; username: string; is_root: boolean; created_at: string }[]
> {
  return request(`${API_BASE}/admin/users`);
}

export async function createUserAdmin(
  username: string,
  password: string,
  isRoot: boolean
): Promise<{ id: string; username: string }> {
  return request(`${API_BASE}/admin/users`, {
    method: "POST",
    body: JSON.stringify({ username, password, is_root: isRoot }),
  });
}

export async function deleteUserAdmin(
  id: string
): Promise<{ status: string }> {
  return request(`${API_BASE}/admin/users/${id}`, {
    method: "DELETE",
  });
}

export async function resetPasswordAdmin(
  userId: string,
  password: string
): Promise<{ status: string }> {
  return request(`${API_BASE}/admin/users/${userId}/password`, {
    method: "PUT",
    body: JSON.stringify({ password }),
  });
}

// ---------- Health ----------

export async function getHealth(): Promise<{ status: string }> {
  return request("/health");
}

// ---------- Runs ----------

export async function startRun(
  config: RunConfig
): Promise<{ run_id: string; status: string }> {
  return request(`${API_BASE}/run`, {
    method: "POST",
    body: JSON.stringify(config),
  });
}

export async function listRuns(): Promise<RunSummary[]> {
  return request(`${API_BASE}/runs`);
}

export async function deleteRun(
  runID: string
): Promise<{ status: string }> {
  return request(`${API_BASE}/run/${runID}`, { method: "DELETE" });
}

export async function cancelRun(
  runID: string
): Promise<{ status: string }> {
  return request(`${API_BASE}/run/${runID}/cancel`, { method: "POST" });
}

export async function validateRun(
  config: RunConfig
): Promise<{ status: string; error?: string }> {
  return request(`${API_BASE}/validate`, {
    method: "POST",
    body: JSON.stringify(config),
  });
}

export async function dryRun(config: RunConfig): Promise<unknown> {
  return request(`${API_BASE}/dry-run`, {
    method: "POST",
    body: JSON.stringify(config),
  });
}

export async function getRunStatus(runID: string): Promise<Snapshot> {
  return request(`${API_BASE}/run/${runID}/status`);
}

// ---------- Presets ----------

export async function listPresets(params?: {
  db_kind?: string;
}): Promise<Preset[]> {
  const qs = new URLSearchParams();
  if (params?.db_kind) qs.set("db_kind", params.db_kind);
  const suffix = qs.toString() ? `?${qs}` : "";
  return request(`${API_BASE}/presets${suffix}`);
}

export async function getPreset(id: string): Promise<Preset> {
  return request(`${API_BASE}/presets/${id}`);
}

export async function createPreset(data: {
  name: string;
  description?: string;
  db_kind: DatabaseKind;
  topology: unknown;
}): Promise<{ id: string }> {
  return request(`${API_BASE}/presets`, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function updatePreset(
  id: string,
  data: {
    name?: string;
    description?: string;
    topology?: unknown;
  },
): Promise<{ status: string }> {
  return request(`${API_BASE}/presets/${id}`, {
    method: "PUT",
    body: JSON.stringify(data),
  });
}

export async function deletePreset(id: string): Promise<{ status: string }> {
  return request(`${API_BASE}/presets/${id}`, { method: "DELETE" });
}

export async function clonePreset(id: string): Promise<{ id: string; name: string }> {
  return request(`${API_BASE}/presets/${id}/clone`, { method: "POST" });
}

// ---------- Probe ----------

export async function probeScript(req: ProbeRequest): Promise<ProbeResponse> {
  return request(`${API_BASE}/probe`, {
    method: "POST",
    body: JSON.stringify(req),
  });
}

// ---------- Metrics ----------

export async function getRunMetrics(
  runID: string,
  start: string,
  end: string
): Promise<MetricValue[]> {
  return request(
    `${API_BASE}/run/${runID}/metrics?start=${encodeURIComponent(start)}&end=${encodeURIComponent(end)}`
  );
}

export async function getRunLogs(
  runID: string,
  opts?: { end?: string; start?: string; limit?: number; desc?: boolean; search?: string },
): Promise<string[]> {
  const headers: Record<string, string> = {};
  if (_accessToken) headers["Authorization"] = `Bearer ${_accessToken}`;

  const params = new URLSearchParams();
  if (opts?.end) params.set("end", opts.end);
  if (opts?.start) params.set("start", opts.start);
  if (opts?.desc) params.set("dir", "desc");
  if (opts?.search) params.set("search", opts.search);
  params.set("limit", String(opts?.limit ?? 500));
  const url = `${API_BASE}/run/${runID}/logs?${params.toString()}`;

  const res = await fetch(url, { headers });
  if (res.status === 503) return []; // monitoring not configured — no logs, not an error
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`logs: ${res.status}: ${text}`);
  }

  const text = await res.text();
  if (!text.trim()) return [];

  // Return raw JSON lines — LogStream parses all fields (action, _time, etc.).
  return text.trim().split("\n").filter(Boolean);
}

export async function compareRuns(
  a: string,
  b: string,
  start?: string,
  end?: string
): Promise<ComparisonResponse> {
  const params = new URLSearchParams({ a, b });
  if (start) params.set("start", start);
  if (end) params.set("end", end);
  return request(`${API_BASE}/compare?${params.toString()}`);
}

// ---------- Settings ----------

export async function getSettings(): Promise<ServerSettings> {
  return request(`${API_BASE}/settings`);
}

export async function updateSettings(
  settings: ServerSettings
): Promise<{ status: string }> {
  return request(`${API_BASE}/settings`, {
    method: "PUT",
    body: JSON.stringify(settings),
  });
}

export async function getDBDefaults(
  kind: string
): Promise<Record<string, unknown>> {
  const presets = await listPresets({ db_kind: kind });
  const result: Record<string, unknown> = {};
  for (const p of presets) {
    result[p.name] = p.topology;
  }
  return result;
}

// ---------- Grafana ----------

export async function getGrafanaSettings(): Promise<GrafanaSettings> {
  return request(`${API_BASE}/grafana`);
}

// ---------- Packages ----------

export async function listPackages(params?: {
  db_kind?: string;
  db_version?: string;
}): Promise<Package[]> {
  const qs = new URLSearchParams();
  if (params?.db_kind) qs.set("db_kind", params.db_kind);
  if (params?.db_version) qs.set("db_version", params.db_version);
  const suffix = qs.toString() ? `?${qs}` : "";
  return request(`${API_BASE}/packages${suffix}`);
}

export async function getPackage(id: string): Promise<Package> {
  return request(`${API_BASE}/packages/${id}`);
}

export async function createPackage(data: {
  name: string;
  description?: string;
  db_kind: string;
  db_version?: string;
  apt_packages?: string[];
  pre_install?: string[];
  custom_repo?: string;
  custom_repo_key?: string;
}): Promise<{ id: string }> {
  return request(`${API_BASE}/packages`, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function updatePackage(
  id: string,
  data: {
    name: string;
    description?: string;
    db_kind: string;
    db_version?: string;
    apt_packages?: string[];
    pre_install?: string[];
    custom_repo?: string;
    custom_repo_key?: string;
  },
): Promise<{ status: string }> {
  return request(`${API_BASE}/packages/${id}`, {
    method: "PUT",
    body: JSON.stringify(data),
  });
}

export async function deletePackage(id: string): Promise<{ status: string }> {
  return request(`${API_BASE}/packages/${id}`, { method: "DELETE" });
}

export async function clonePackage(id: string): Promise<{ id: string; name: string }> {
  return request(`${API_BASE}/packages/${id}/clone`, { method: "POST" });
}

export async function uploadPackageDeb(
  packageId: string,
  file: File,
): Promise<{ status: string; filename: string; size: string }> {
  const formData = new FormData();
  formData.append("file", file);

  const headers: Record<string, string> = {};
  if (_accessToken) {
    headers["Authorization"] = `Bearer ${_accessToken}`;
  }

  const res = await fetch(`${API_BASE}/packages/${packageId}/deb`, {
    method: "POST",
    headers,
    body: formData,
  });

  if (!res.ok) {
    const text = await res.text();
    throw new Error(`${res.status}: ${text}`);
  }

  return res.json();
}
