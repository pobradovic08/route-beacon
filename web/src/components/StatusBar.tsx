import { Group, Text, Badge, Tooltip, Box } from "@mantine/core";
import { IconCircleFilled, IconRoute, IconServer } from "@tabler/icons-react";
import type { HealthResponse } from "../api/types";

interface StatusBarProps {
  health: HealthResponse | null;
  error: string | null;
}

export function StatusBar({ health, error }: StatusBarProps) {
  if (error) {
    return (
      <Box
        py="xs"
        px="md"
        style={{
          borderTop: "1px solid var(--mantine-color-red-2)",
          background: "var(--mantine-color-red-0)",
        }}
      >
        <Group gap="xs">
          <IconCircleFilled size={8} color="var(--mantine-color-red-6)" />
          <Text size="xs" c="red.7" ff="monospace">
            API unreachable
          </Text>
        </Group>
      </Box>
    );
  }

  if (!health) return null;

  const statusColor =
    health.status === "healthy"
      ? "teal"
      : health.status === "degraded"
        ? "yellow"
        : "red";

  const uptimeHours = Math.floor(health.uptime_seconds / 3600);
  const uptimeMin = Math.floor((health.uptime_seconds % 3600) / 60);

  return (
    <Box
      py="xs"
      px="md"
      style={{
        borderTop: "1px solid var(--rb-border)",
        background: "var(--rb-canvas)",
      }}
    >
      <Group gap="md" justify="space-between">
        <Group gap="md">
          <Tooltip label={`Status: ${health.status}`}>
            <Badge
              size="xs"
              variant="dot"
              color={statusColor}
              styles={{ root: { textTransform: "none" } }}
            >
              {health.status}
            </Badge>
          </Tooltip>

          <Group gap="xs">
            <IconServer size={14} color="var(--rb-muted)" />
            <Text size="xs" c="dimmed" ff="monospace">
              {health.connected_collectors}/{health.collector_count} collectors
            </Text>
          </Group>

          <Group gap="xs">
            <IconRoute size={14} color="var(--rb-muted)" />
            <Text size="xs" c="dimmed" ff="monospace">
              {health.total_routes.toLocaleString()} routes
            </Text>
          </Group>
        </Group>

        <Text size="xs" c="dimmed" ff="monospace">
          uptime {uptimeHours}h {uptimeMin}m
        </Text>
      </Group>
    </Box>
  );
}
