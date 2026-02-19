import type {
  HealthResponse,
  CollectorsResponse,
  TargetsResponse,
  RouteLookupResponse,
  RoutePath,
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
  ): Promise<RouteLookupResponse> => {
    if (prefix === "10.1.0.0/24") return Promise.resolve(MOCK_MULTI_PATH);
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

const mockPath = (
  best: boolean,
  asPath: number[],
  nextHop: string,
  origin: RoutePath["origin"],
  localPref: number | null,
  med: number | null,
  communities: string[],
): RoutePath => ({
  best,
  filtered: false,
  stale: false,
  as_path: asPath,
  next_hop: nextHop,
  origin,
  local_pref: localPref,
  med,
  communities: communities.map((v) => ({ type: "standard", value: v })),
  extended_communities: [],
  large_communities: [],
  aggregator: null,
  atomic_aggregate: false,
});

const MOCK_MULTI_PATH: RouteLookupResponse = {
  prefix: "10.1.0.0/24",
  target: { id: "mock-fra1", display_name: "DE-CIX Frankfurt", asn: 64513 },
  paths: [
    mockPath(true, [64513, 174, 13335], "172.28.0.10", "igp", 200, null, [
      "174:100", "174:3000", "13335:10",
    ]),
    mockPath(false, [64514, 6939, 13335], "172.28.0.11", "igp", 150, 50, [
      "6939:100", "6939:6939", "13335:10",
    ]),
    mockPath(false, [64515, 3356, 1299, 13335], "172.28.0.12", "igp", 100, 120, [
      "3356:3", "3356:22", "1299:20000",
    ]),
    mockPath(false, [64516, 2914, 13335], "172.28.0.13", "egp", 100, null, []),
    mockPath(false, [64517, 9002, 6939, 13335], "172.28.0.14", "incomplete", 80, 200, [
      "9002:64800", "6939:100", "13335:10", "65000:100",
    ]),
  ],
  plain_text: "",
  meta: {
    match_type: "exact",
    data_updated_at: new Date().toISOString(),
    stale: false,
    collector_status: "online",
  },
  pagination: null,
};
