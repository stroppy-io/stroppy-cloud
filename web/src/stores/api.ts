import { $accessToken, refreshAccessToken, logout } from "./auth"

async function authRpc<T>(procedure: string, body: unknown): Promise<T> {
  const token = $accessToken.get()
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  }
  if (token) {
    headers["Authorization"] = `Bearer ${token}`
  }

  let resp = await fetch(procedure, {
    method: "POST",
    headers,
    body: JSON.stringify(body),
  })

  if (resp.status === 401) {
    const refreshed = await refreshAccessToken()
    if (refreshed) {
      headers["Authorization"] = `Bearer ${$accessToken.get()}`
      resp = await fetch(procedure, {
        method: "POST",
        headers,
        body: JSON.stringify(body),
      })
    } else {
      logout()
      throw new Error("Unauthenticated")
    }
  }

  if (!resp.ok) {
    const text = await resp.text()
    throw new Error(text || resp.statusText)
  }
  return resp.json() as Promise<T>
}

// Test API
export const testApi = {
  runTestSuite: (body: unknown) =>
    authRpc<{ runId: string }>("/api.v1.TestAPI/RunTestSuite", body),
  getTestStatus: (runId: string) =>
    authRpc<{ status: string; currentStep: string; progress: number }>("/api.v1.TestAPI/GetTestStatus", { runId }),
  getTestResult: (runId: string) =>
    authRpc<{ result: unknown }>("/api.v1.TestAPI/GetTestResult", { runId }),
  listTestRuns: (pageSize?: number, pageToken?: string) =>
    authRpc<{ runs: unknown[]; nextPageToken: string }>("/api.v1.TestAPI/ListTestRuns", { pageSize: pageSize ?? 20, pageToken: pageToken ?? "" }),
}

// Topology API
export const topologyApi = {
  validateTopology: (testSuite: unknown) =>
    authRpc<{ errors: Array<{ fieldPath: string; severity: string; message: string }> }>("/api.v1.TopologyAPI/ValidateTopology", { testSuite }),
}

// Settings API
export const settingsApi = {
  getSettings: () =>
    authRpc<{ settings: unknown }>("/api.v1.SettingsAPI/GetSettings", {}),
  updateSettings: (settings: unknown) =>
    authRpc<{ settings: unknown }>("/api.v1.SettingsAPI/UpdateSettings", { settings }),
}

// Execution API (streaming — use EventSource pattern)
export async function* streamWorkflowGraph(runId: string): AsyncGenerator<unknown> {
  const token = $accessToken.get()
  const resp = await fetch("/api.v1.ExecutionAPI/StreamWorkflowGraph", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "Authorization": `Bearer ${token}`,
    },
    body: JSON.stringify({ runId }),
  })

  if (!resp.ok || !resp.body) {
    throw new Error("Stream failed")
  }

  const reader = resp.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ""

  while (true) {
    const { done, value } = await reader.read()
    if (done) break

    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split("\n")
    buffer = lines.pop() ?? ""

    for (const line of lines) {
      if (line.trim()) {
        try {
          yield JSON.parse(line)
        } catch {
          // skip malformed lines
        }
      }
    }
  }
}
