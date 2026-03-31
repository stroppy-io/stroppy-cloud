import type {
  RunConfig,
  Snapshot,
  RunSummary,
  PresetsResponse,
  ServerSettings,
  PackageDefaults,
  ComparisonRow,
  MetricValue,
  GrafanaSettings,
} from "./types";

const API_BASE = "/api/v1";

function getToken(): string | null {
  return localStorage.getItem("token");
}

async function request<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string>),
  };

  const token = getToken();
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const res = await fetch(path, { ...options, headers });

  if (!res.ok) {
    const text = await res.text();
    throw new Error(`${res.status}: ${text}`);
  }

  return res.json();
}

// --- Auth ---

export async function login(
  username: string,
  password: string
): Promise<{ token: string; username: string }> {
  return request(`${API_BASE}/auth/login`, {
    method: "POST",
    body: JSON.stringify({ username, password }),
  });
}

export async function me(): Promise<{ username: string; role: string }> {
  return request(`${API_BASE}/auth/me`);
}

// --- Health ---

export async function getHealth(): Promise<{ status: string }> {
  return request("/health");
}

// --- Runs ---

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

// --- Presets ---

export async function getPresets(): Promise<PresetsResponse> {
  return request(`${API_BASE}/presets`);
}

// --- Metrics ---

export async function getRunMetrics(
  runID: string,
  start: string,
  end: string
): Promise<MetricValue[]> {
  return request(
    `${API_BASE}/run/${runID}/metrics?start=${encodeURIComponent(start)}&end=${encodeURIComponent(end)}`
  );
}

// Fetch historical logs from VictoriaLogs (NDJSON format).
export async function getRunLogs(runID: string): Promise<string[]> {
  const token = getToken();
  const headers: Record<string, string> = {};
  if (token) headers["Authorization"] = `Bearer ${token}`;

  const res = await fetch(`${API_BASE}/run/${runID}/logs`, { headers });
  if (!res.ok) return []; // 503 = vlogs not configured

  const text = await res.text();
  if (!text.trim()) return [];

  // NDJSON: each line is a JSON object with _msg, machine_id, etc.
  return text
    .trim()
    .split("\n")
    .filter(Boolean)
    .map((line) => {
      try {
        const obj = JSON.parse(line);
        const mid = obj.machine_id || "system";
        const msg = obj._msg || obj.message || "";
        return JSON.stringify({ machine_id: mid, line: msg });
      } catch {
        return line;
      }
    });
}

export async function compareRuns(
  a: string,
  b: string,
  start: string,
  end: string
): Promise<ComparisonRow[]> {
  return request(
    `${API_BASE}/compare?a=${encodeURIComponent(a)}&b=${encodeURIComponent(b)}&start=${encodeURIComponent(start)}&end=${encodeURIComponent(end)}`
  );
}

// --- Admin ---

export async function getSettings(): Promise<ServerSettings> {
  return request(`${API_BASE}/admin/settings`);
}

export async function updateSettings(
  settings: ServerSettings
): Promise<{ status: string }> {
  return request(`${API_BASE}/admin/settings`, {
    method: "PUT",
    body: JSON.stringify(settings),
  });
}

export async function getPackages(): Promise<PackageDefaults> {
  return request(`${API_BASE}/admin/packages`);
}

export async function updatePackages(
  packages: PackageDefaults
): Promise<{ status: string }> {
  return request(`${API_BASE}/admin/packages`, {
    method: "PUT",
    body: JSON.stringify(packages),
  });
}

export async function getDBDefaults(
  kind: string
): Promise<Record<string, unknown>> {
  return request(`${API_BASE}/admin/db-defaults/${kind}`);
}

// --- Grafana ---

export async function getGrafanaSettings(): Promise<GrafanaSettings> {
  return request(`${API_BASE}/admin/grafana`);
}

// --- Upload ---

export async function uploadDeb(
  file: File
): Promise<{ filename: string; url: string; size: string }> {
  const formData = new FormData();
  formData.append("file", file);

  const headers: Record<string, string> = {};
  const token = getToken();
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}/upload/deb`, {
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
