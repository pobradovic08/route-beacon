export interface HealthResponse {
  status: "healthy" | "degraded" | "unhealthy";
  collector_count: number;
  connected_collectors: number;
  total_routes: number;
  uptime_seconds: number;
}

export interface Collector {
  id: string;
  location: string;
  status: "online" | "offline";
  router_count: number;
}

export interface CollectorsResponse {
  data: Collector[];
}

export interface TargetCollector {
  id: string;
  location: string;
}

export interface Target {
  id: string;
  collector: TargetCollector;
  display_name: string;
  asn: number;
  status: "up" | "down" | "unknown";
  last_update: string;
}

export interface TargetsResponse {
  data: Target[];
}

export interface Community {
  type: "standard" | "extended" | "large";
  value: string;
}

export interface Aggregator {
  asn: number;
  address: string;
}

export interface RoutePath {
  best: boolean;
  as_path: number[];
  next_hop: string;
  origin: "igp" | "egp" | "incomplete";
  med: number | null;
  local_pref: number | null;
  communities: Community[];
  extended_communities: Community[];
  large_communities: Community[];
  aggregator: Aggregator | null;
  atomic_aggregate: boolean;
}

export interface RouteLookupMeta {
  match_type: "exact" | "longest";
  data_updated_at: string;
  stale: boolean;
  collector_status: "online" | "offline";
}

export interface Pagination {
  next_cursor: string | null;
  has_more: boolean;
}

export interface RouteLookupResponse {
  prefix: string;
  target: {
    id: string;
    display_name: string;
    asn: number;
  };
  paths: RoutePath[];
  plain_text: string;
  meta: RouteLookupMeta;
  pagination: Pagination | null;
}

export interface PingReply {
  seq: number;
  rtt_ms: number;
  ttl: number;
  success: boolean;
}

export interface PingSummary {
  packets_sent: number;
  packets_received: number;
  loss_pct: number;
  rtt_min_ms: number;
  rtt_avg_ms: number;
  rtt_max_ms: number;
}

export interface TracerouteHop {
  hop_number: number;
  address: string;
  rtt_ms: number[];
}

export interface TracerouteComplete {
  reached_destination: boolean;
}

export interface SSEError {
  code: string;
  message: string;
}

export interface ProblemDetail {
  type: string;
  title: string;
  status: number;
  detail: string;
  instance?: string;
  retry_after?: number;
  invalid_params?: { name: string; reason: string }[];
}
