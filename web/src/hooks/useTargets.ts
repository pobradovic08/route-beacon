import { useState, useEffect, useCallback } from "react";
import { api } from "../api/client";
import type { Target } from "../api/types";

const MOCK_TARGETS: Target[] = [
  { id: "mock-fra1", collector: { id: "col-fra", location: "Frankfurt, DE" }, display_name: "DE-CIX Frankfurt", asn: 64513, status: "up", last_update: new Date().toISOString() },
  { id: "mock-ams1", collector: { id: "col-ams", location: "Amsterdam, NL" }, display_name: "AMS-IX Amsterdam", asn: 64514, status: "up", last_update: new Date().toISOString() },
  { id: "mock-lhr1", collector: { id: "col-lhr", location: "London, GB" }, display_name: "LINX London", asn: 64515, status: "up", last_update: new Date().toISOString() },
  { id: "mock-cdg1", collector: { id: "col-cdg", location: "Paris, FR" }, display_name: "France-IX Paris", asn: 64516, status: "down", last_update: new Date().toISOString() },
  { id: "mock-waw1", collector: { id: "col-waw", location: "Warsaw, PL" }, display_name: "PLIX Warsaw", asn: 64517, status: "up", last_update: new Date().toISOString() },
  { id: "mock-vie1", collector: { id: "col-vie", location: "Vienna, AT" }, display_name: "VIX Vienna", asn: 64518, status: "up", last_update: new Date().toISOString() },
  { id: "mock-mil1", collector: { id: "col-mil", location: "Milan, IT" }, display_name: "MIX Milan", asn: 64519, status: "unknown", last_update: new Date().toISOString() },
  { id: "mock-mad1", collector: { id: "col-mad", location: "Madrid, ES" }, display_name: "ESPANIX Madrid", asn: 64520, status: "up", last_update: new Date().toISOString() },
  { id: "mock-sto1", collector: { id: "col-sto", location: "Stockholm, SE" }, display_name: "Netnod Stockholm", asn: 64521, status: "up", last_update: new Date().toISOString() },
  { id: "mock-prg1", collector: { id: "col-prg", location: "Prague, CZ" }, display_name: "NIX.CZ Prague", asn: 64522, status: "down", last_update: new Date().toISOString() },
];

export function useTargets() {
  const [targets, setTargets] = useState<Target[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setLoading(true);
      const data = await api.targets();
      setTargets([...(data.data || []), ...MOCK_TARGETS]);
      setError(null);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to load targets");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
    const id = setInterval(refresh, 30_000);
    return () => clearInterval(id);
  }, [refresh]);

  return { targets, loading, error, refresh };
}
