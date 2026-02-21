import type {
  HealthResponse,
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

// API Router shape from the backend
interface ApiRouter {
  id: string;
  router_ip: string | null;
  hostname: string | null;
  display_name: string;
  as_number: number | null;
  description: string | null;
  status: "up" | "down";
  eor_received: boolean;
  first_seen: string;
  last_seen: string;
}

interface ApiRouterListResponse {
  data: ApiRouter[];
}

// API Route shape from the backend
interface ApiRoute {
  prefix: string;
  path_id: number;
  next_hop: string | null;
  as_path: (number | number[])[];
  origin: "igp" | "egp" | "incomplete" | null;
  local_pref: number | null;
  med: number | null;
  origin_asn: number | null;
  communities: { type: string; value: string }[];
  extended_communities: { type: string; value: string }[];
  large_communities: { type: string; value: string }[];
  attrs: Record<string, unknown> | null;
  first_seen: string;
  updated_at: string;
}

interface ApiRouteLookupResponse {
  prefix: string;
  router: { id: string; display_name: string; as_number: number | null };
  routes: ApiRoute[];
  plain_text: string;
  meta: { match_type: "exact" | "longest"; router_status: "up" | "down" };
}

// API Health shape from the backend
interface ApiHealthResponse {
  status: "healthy" | "degraded" | "unhealthy";
  router_count: number;
  online_routers: number;
  total_routes: number;
  uptime_seconds: number;
}

function mapHealthResponse(api: ApiHealthResponse): HealthResponse {
  return {
    status: api.status,
    collector_count: api.router_count,
    connected_collectors: api.online_routers,
    total_routes: api.total_routes,
    uptime_seconds: api.uptime_seconds,
  };
}

function mapRoutersToTargets(apiRouters: ApiRouter[]): TargetsResponse {
  return {
    data: apiRouters.map((r) => ({
      id: r.id,
      collector: { id: r.id, location: r.description || "" },
      display_name: r.display_name,
      asn: r.as_number,
      status: r.status === "up" ? ("up" as const) : ("down" as const),
      last_update: r.last_seen,
    })),
  };
}

function mapRouteLookupResponse(api: ApiRouteLookupResponse): RouteLookupResponse {
  const latestUpdate = api.routes.length > 0
    ? api.routes.reduce((latest, r) =>
        r.updated_at > latest ? r.updated_at : latest, api.routes[0].updated_at)
    : new Date().toISOString();

  return {
    prefix: api.prefix,
    target: {
      id: api.router.id,
      display_name: api.router.display_name,
      asn: api.router.as_number,
    },
    paths: api.routes.map((route) => ({
      best: route.path_id === 0,
      filtered: false,
      stale: false,
      as_path: route.as_path,
      next_hop: route.next_hop ?? "",
      origin: route.origin ?? "incomplete",
      local_pref: route.local_pref,
      med: route.med,
      communities: route.communities.map((c) => ({
        type: c.type as "standard" | "extended" | "large",
        value: c.value,
      })),
      extended_communities: route.extended_communities.map((c) => ({
        type: c.type as "standard" | "extended" | "large",
        value: c.value,
      })),
      large_communities: route.large_communities.map((c) => ({
        type: c.type as "standard" | "extended" | "large",
        value: c.value,
      })),
      aggregator: route.attrs?.aggregator
        ? (route.attrs.aggregator as { asn: number; address: string })
        : null,
      atomic_aggregate: route.attrs?.atomic_aggregate === true,
    })),
    plain_text: api.plain_text,
    meta: {
      match_type: api.meta.match_type,
      data_updated_at: latestUpdate,
      stale: false,
      collector_status: api.meta.router_status === "up" ? "online" : "offline",
    },
    pagination: null,
  };
}

export const api = {
  health: async (): Promise<HealthResponse> => {
    const data = await request<ApiHealthResponse>("/health");
    return mapHealthResponse(data);
  },

  routers: () => request<ApiRouterListResponse>("/routers"),

  targets: async (): Promise<TargetsResponse> => {
    const data = await request<ApiRouterListResponse>("/routers");
    return mapRoutersToTargets(data.data);
  },

  lookupRoutes: async (
    targetId: string,
    prefix: string,
    matchType?: "exact" | "longest",
  ): Promise<RouteLookupResponse> => {
    const params = new URLSearchParams({ prefix });
    if (matchType) params.set("match_type", matchType);
    const data = await request<ApiRouteLookupResponse>(
      `/routers/${encodeURIComponent(targetId)}/routes/lookup?${params}`,
    );
    return mapRouteLookupResponse(data);
  },

  ping: (
    _targetId: string,
    _body: { destination: string; count?: number; timeout_ms?: number },
  ) => Promise.reject(new Error("Diagnostics unavailable")),

  traceroute: (
    _targetId: string,
    _body: { destination: string; max_hops?: number; timeout_ms?: number },
  ) => Promise.reject(new Error("Diagnostics unavailable")),
};
