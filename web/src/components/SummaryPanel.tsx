import { useEffect, useState } from "react";
import {
  Stack,
  Group,
  Text,
  Card,
  SimpleGrid,
  Box,
  Loader,
  Alert,
} from "@mantine/core";
import { IconAlertTriangle } from "@tabler/icons-react";
import { api } from "../api/client";
import type { RouterDetail } from "../api/types";

interface SummaryPanelProps {
  targetId: string | null;
}

const cardStyle = {
  border: "1px solid var(--rb-border)",
  boxShadow: "var(--rb-shadow-sm)",
  background: "var(--rb-surface)",
};

function Stat({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <Box>
      <Text size="xs" fw={600} tt="uppercase" lts="0.05em" style={{ color: "var(--rb-muted)" }}>
        {label}
      </Text>
      <Text size="lg" fw={700} ff={mono ? "monospace" : undefined} style={{ color: "var(--rb-text)" }}>
        {value}
      </Text>
    </Box>
  );
}

function Dot({ color }: { color: string }) {
  return (
    <Box
      style={{
        width: 10,
        height: 10,
        borderRadius: "50%",
        background: color,
        flexShrink: 0,
      }}
    />
  );
}

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

  const statusColor = detail.status === "up" ? "var(--rb-success)" : "var(--rb-danger)";
  const statusLabel = detail.status === "up" ? "Online" : "Offline";
  const syncLabel = detail.eor_received ? "Complete" : "In progress";
  const infoLine = [
    detail.as_number != null ? `AS${detail.as_number}` : null,
    detail.location,
  ].filter(Boolean).join(" · ");

  return (
    <Stack gap="lg">
      {/* Router identity */}
      <Card padding="md" style={cardStyle}>
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
          </Box>
          <Group gap={6}>
            <Dot color={statusColor} />
            <Text size="sm" fw={600} style={{ color: statusColor }}>
              {statusLabel}
            </Text>
          </Group>
        </Group>
      </Card>

      {/* Stats grid */}
      <SimpleGrid cols={{ base: 2, sm: 3 }} spacing="md">
        <Card padding="md" style={cardStyle}>
          <Stat label="Total Routes" value={detail.route_count.toLocaleString()} mono />
        </Card>
        <Card padding="md" style={cardStyle}>
          <Stat label="Unique Prefixes" value={detail.unique_prefixes.toLocaleString()} mono />
        </Card>
        <Card padding="md" style={cardStyle}>
          <Stat label="Peers" value={detail.peer_count.toLocaleString()} mono />
        </Card>
        <Card padding="md" style={cardStyle}>
          <Stat label="IPv4 Routes" value={detail.ipv4_routes.toLocaleString()} mono />
        </Card>
        <Card padding="md" style={cardStyle}>
          <Stat label="IPv6 Routes" value={detail.ipv6_routes.toLocaleString()} mono />
        </Card>
        <Card padding="md" style={cardStyle}>
          <Stat
            label="Avg AS Path"
            value={detail.avg_as_path_len != null ? detail.avg_as_path_len.toFixed(1) : "—"}
            mono
          />
        </Card>
      </SimpleGrid>

      {/* Sync status */}
      <Card padding="md" style={cardStyle}>
        <Group gap="xl">
          <Group gap={6}>
            <Text size="xs" fw={600} tt="uppercase" lts="0.05em" style={{ color: "var(--rb-muted)" }}>
              RIB Sync
            </Text>
            <Text size="sm" fw={600} style={{ color: detail.eor_received ? "var(--rb-success)" : "var(--rb-warning)" }}>
              {syncLabel}
            </Text>
          </Group>
          {detail.first_seen && (
            <Group gap={6}>
              <Text size="xs" fw={600} tt="uppercase" lts="0.05em" style={{ color: "var(--rb-muted)" }}>
                First Seen
              </Text>
              <Text size="sm" fw={500} ff="monospace" style={{ color: "var(--rb-text-secondary)" }}>
                {new Date(detail.first_seen).toLocaleDateString()}
              </Text>
            </Group>
          )}
          {detail.sync_updated_at && (
            <Group gap={6}>
              <Text size="xs" fw={600} tt="uppercase" lts="0.05em" style={{ color: "var(--rb-muted)" }}>
                Last Update
              </Text>
              <Text size="sm" fw={500} ff="monospace" style={{ color: "var(--rb-text-secondary)" }}>
                {new Date(detail.sync_updated_at).toLocaleString()}
              </Text>
            </Group>
          )}
        </Group>
      </Card>
    </Stack>
  );
}
