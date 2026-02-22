import { useEffect, useState } from "react";
import {
  Stack,
  Group,
  Text,
  SimpleGrid,
  Box,
  Badge,
  Tooltip,
  Loader,
  Alert,
} from "@mantine/core";
import { IconAlertTriangle } from "@tabler/icons-react";
import { api } from "../api/client";
import type { RouterDetail } from "../api/types";

interface SummaryPanelProps {
  targetId: string | null;
}

/* ── card styles ───────────────────────────────────────── */
const cardStyle: React.CSSProperties = {
  border: "none",
  borderRadius: "var(--rb-radius)",
  boxShadow: "0 2px 12px rgba(0,0,0,0.08)",
  padding: 16,
};

const identityStyle: React.CSSProperties = {
  ...cardStyle,
  padding: 20,
};

function timeAgo(date: Date): string {
  const seconds = Math.floor((Date.now() - date.getTime()) / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

/* ── sub-components ────────────────────────────────────── */
function Stat({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <Box>
      <Text size="xs" fw={600} lts="0.03em" style={{ color: "var(--rb-muted)" }}>
        {label}
      </Text>
      <Text size="xl" fw={700} ff={mono ? "monospace" : undefined} style={{ color: "var(--rb-text)" }}>
        {value}
      </Text>
    </Box>
  );
}

/* ── main component ────────────────────────────────────── */
export function SummaryPanel({ targetId }: SummaryPanelProps) {
  const [detail, setDetail] = useState<RouterDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!targetId) return;
    let cancelled = false;
    setLoading(true);
    setError(null);
    api.routerDetail(targetId).then((d) => {
      if (!cancelled) setDetail(d);
    }).catch((err) => {
      if (!cancelled) setError(err instanceof Error ? err.message : "Failed to load router details");
    }).finally(() => {
      if (!cancelled) setLoading(false);
    });
    return () => { cancelled = true; };
  }, [targetId]);

  if (loading) {
    return (
      <Group justify="center" py={40}>
        <Loader size="sm" color="blue" />
        <Text size="sm" fw={500} style={{ color: "var(--rb-text-secondary)" }}>
          Loading router details...
        </Text>
      </Group>
    );
  }

  if (error) {
    return (
      <Alert color="red" variant="light" icon={<IconAlertTriangle size={16} />} radius="lg">
        <Text size="sm" fw={500} ff="monospace">{error}</Text>
      </Alert>
    );
  }

  if (!detail) return null;

  const statusLabel = detail.status === "up" ? "Online" : "Offline";
  const syncLabel = detail.eor_received ? "RIB Synchronized" : "RIB Synchronizing";
  const infoLine = [
    detail.as_number != null ? `AS${detail.as_number}` : null,
    detail.location,
  ].filter(Boolean).join(" · ");

  return (
    <Stack gap="lg">
      {/* Router identity */}
      <Box style={identityStyle}>
        <Group justify="space-between" align="center">
          <Box>
            <Text size="lg" fw={700} style={{ color: "var(--rb-text)" }}>
              {detail.display_name}
            </Text>
            {infoLine && (
              <Text size="sm" fw={500} style={{ color: "var(--rb-text-secondary)" }}>
                {infoLine}
              </Text>
            )}
            {detail.sync_updated_at && (
              <Text size="xs" fw={500} mt={12} style={{ color: "var(--rb-muted)" }}>
                Last Update:{" "}
                <Tooltip label={new Date(detail.sync_updated_at).toLocaleString()} withArrow position="bottom" fz="xs">
                  <Text span size="xs" fw={500} ff="monospace" td="underline" style={{ color: "var(--rb-text-secondary)", cursor: "default" }}>
                    {timeAgo(new Date(detail.sync_updated_at))}
                  </Text>
                </Tooltip>
              </Text>
            )}
          </Box>
          <Stack gap={6} align="flex-end">
            <Badge variant="light" color={detail.status === "up" ? "green" : "red"} size="sm">
              {statusLabel}
            </Badge>
            <Badge variant="light" color={detail.eor_received ? "green" : "yellow"} size="sm">
              {syncLabel}
            </Badge>
          </Stack>
        </Group>
      </Box>

      {/* Stats grid */}
      <SimpleGrid cols={{ base: 2, sm: 3 }} spacing="md">
        <Box style={cardStyle}>
          <Stat label="Total Routes" value={detail.route_count.toLocaleString()} mono />
        </Box>
        <Box style={cardStyle}>
          <Stat label="Unique Prefixes" value={detail.unique_prefixes.toLocaleString()} mono />
        </Box>
        <Box style={cardStyle}>
          <Stat label="Peers" value={detail.peer_count.toLocaleString()} mono />
        </Box>
        <Box style={cardStyle}>
          <Stat label="IPv4 Routes" value={detail.ipv4_routes.toLocaleString()} mono />
        </Box>
        <Box style={cardStyle}>
          <Stat label="IPv6 Routes" value={detail.ipv6_routes.toLocaleString()} mono />
        </Box>
        <Box style={cardStyle}>
          <Stat
            label="Avg AS Path"
            value={detail.avg_as_path_len != null ? detail.avg_as_path_len.toFixed(1) : "—"}
            mono
          />
        </Box>
      </SimpleGrid>
    </Stack>
  );
}
