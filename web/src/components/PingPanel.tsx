import {
  Stack,
  Text,
  Card,
} from "@mantine/core";
import {
  IconAlertTriangle,
} from "@tabler/icons-react";

interface PingPanelProps {
  targetId: string | null;
}

const cardStyle = {
  border: "1px solid var(--rb-border)",
  boxShadow: "var(--rb-shadow-sm)",
  background: "var(--rb-surface)",
};

export function PingPanel(_props: PingPanelProps) {
  return (
    <Stack gap="lg">
      <Card padding="xl" style={cardStyle}>
        <Stack gap="md" align="center" py="xl">
          <IconAlertTriangle size={32} style={{ color: "var(--rb-muted)" }} />
          <Text size="sm" fw={600} style={{ color: "var(--rb-text-secondary)" }}>
            Unavailable
          </Text>
          <Text
            size="xs"
            fw={400}
            ta="center"
            maw={360}
            style={{ color: "var(--rb-muted)", lineHeight: 1.6 }}
          >
            Diagnostics require a future backend component.
            Ping functionality will be restored when a diagnostics service is available.
          </Text>
        </Stack>
      </Card>
    </Stack>
  );
}
