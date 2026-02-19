import type {
  HealthResponse,
  CollectorsResponse,
  TargetsResponse,
  RouteLookupResponse,
  ProblemDetail,
} from "./types";

const BASE = "/api/v1";

export class ApiError extends Error {
  status: number;
  problem: ProblemDetail;

  constructor(status: number, problem: ProblemDetail) {
    super(problem.detail || problem.title);
    this.name = "ApiError";
    this.status = status;
    this.problem = problem;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, init);
  if (!res.ok) {
    let problem: ProblemDetail;
    try {
      problem = await res.json();
    } catch {
      problem = {
        type: "about:blank",
        title: res.statusText,
        status: res.status,
        detail: `HTTP ${res.status}`,
      };
    }
    throw new ApiError(res.status, problem);
  }
  return res.json();
}

export const api = {
  health: () => request<HealthResponse>("/health"),
  collectors: () => request<CollectorsResponse>("/collectors"),
  targets: () => request<TargetsResponse>("/targets"),

  lookupRoutes: (
    targetId: string,
    prefix: string,
    matchType?: "exact" | "longest",
  ) => {
    const params = new URLSearchParams({ prefix });
    if (matchType) params.set("match_type", matchType);
    return request<RouteLookupResponse>(
      `/targets/${encodeURIComponent(targetId)}/routes/lookup?${params}`,
    );
  },

  ping: (
    targetId: string,
    body: { destination: string; count?: number; timeout_ms?: number },
  ) =>
    fetch(`${BASE}/targets/${encodeURIComponent(targetId)}/ping`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    }),

  traceroute: (
    targetId: string,
    body: { destination: string; max_hops?: number; timeout_ms?: number },
  ) =>
    fetch(`${BASE}/targets/${encodeURIComponent(targetId)}/traceroute`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    }),
};
