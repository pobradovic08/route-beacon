import { Group, Text, Box } from "@mantine/core";
import type { HealthResponse } from "../api/types";

interface StatusBarProps {
  health: HealthResponse | null;
  error: string | null;
}

function Dot({ color }: { color: string }) {
  return (
    <Box
      style={{
        width: "var(--rb-dot)",
        height: "var(--rb-dot)",
        borderRadius: "50%",
        background: color,
        flexShrink: 0,
      }}
    />
  );
}

export function StatusBar({ health, error }: StatusBarProps) {
  if (error) {
    return (
      <Group h="100%" px="lg" gap="sm">
        <Dot color="var(--rb-danger)" />
        <Text size="xs" fw={500} ff="monospace" style={{ color: "var(--rb-danger)" }}>
          API unreachable
        </Text>
      </Group>
    );
  }

  if (!health) return null;

  const statusColor =
    health.status === "healthy"
      ? "var(--rb-success)"
      : health.status === "degraded"
        ? "var(--rb-warning)"
        : "var(--rb-danger)";

  const uptimeHours = Math.floor(health.uptime_seconds / 3600);
  const uptimeMin = Math.floor((health.uptime_seconds % 3600) / 60);

  return (
    <Group h="100%" px="lg" justify="space-between">
      <Group gap="lg">
        <Group gap={6}>
          <Dot color={statusColor} />
          <Text
            size="xs"
            fw={500}
            ff="monospace"
            style={{ color: "var(--rb-text-secondary)" }}
          >
            {health.status}
          </Text>
        </Group>

        <Text
          size="xs"
          fw={500}
          ff="monospace"
          style={{ color: "var(--rb-text-secondary)" }}
        >
          {health.connected_collectors}/{health.collector_count} collectors
        </Text>

        <Text
          size="xs"
          fw={500}
          ff="monospace"
          style={{ color: "var(--rb-text-secondary)" }}
        >
          {health.total_routes.toLocaleString()} routes
        </Text>
      </Group>

      <Text
        size="xs"
        fw={500}
        ff="monospace"
        style={{ color: "var(--rb-muted)" }}
      >
        {uptimeHours}h {uptimeMin}m
      </Text>
    </Group>
  );
}
