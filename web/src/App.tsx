import { useState } from "react";
import {
  AppShell,
  Group,
  Text,
  Tabs,
  Container,
  Box,
} from "@mantine/core";
import {
  IconRoute,
  IconWorldSearch,
  IconPingPong,
  IconArrowsShuffle,
} from "@tabler/icons-react";
import { TargetSelector } from "./components/TargetSelector";
import { RouteLookup } from "./components/RouteLookup";
import { PingPanel } from "./components/PingPanel";
import { TraceroutePanel } from "./components/TraceroutePanel";
import { StatusBar } from "./components/StatusBar";
import { useHealth } from "./hooks/useHealth";
import { useTargets } from "./hooks/useTargets";

export default function App() {
  const { health, error: healthError } = useHealth();
  const { targets, loading: targetsLoading } = useTargets();
  const [selectedTarget, setSelectedTarget] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<string | null>("routes");

  return (
    <AppShell header={{ height: 56 }} footer={{ height: 36 }} padding={0}>
      <AppShell.Header
        style={{
          borderBottom: "1px solid var(--rb-border)",
          background: "var(--rb-surface)",
        }}
      >
        <Group h="100%" px="md" justify="space-between">
          <Group gap="sm">
            <Box
              style={{
                width: 32,
                height: 32,
                borderRadius: "var(--mantine-radius-md)",
                background:
                  "linear-gradient(135deg, #0d9488 0%, #14b8a6 100%)",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                flexShrink: 0,
              }}
            >
              <IconRoute size={18} color="#fff" stroke={2.5} />
            </Box>
            <div>
              <Text size="sm" fw={700} lh={1.2}>
                Route Beacon
              </Text>
              <Text size="xs" c="dimmed" lh={1}>
                BGP Looking Glass
              </Text>
            </div>
          </Group>

          <Box w={320}>
            <TargetSelector
              targets={targets}
              value={selectedTarget}
              onChange={setSelectedTarget}
              loading={targetsLoading}
            />
          </Box>
        </Group>
      </AppShell.Header>

      <AppShell.Main
        style={{
          background: "var(--rb-canvas)",
          minHeight: "calc(100vh - 56px - 36px)",
        }}
      >
        <Container size="lg" py="md">
          <Tabs
            value={activeTab}
            onChange={setActiveTab}
            variant="default"
            styles={{
              tab: {
                fontWeight: 500,
                fontSize: "var(--mantine-font-size-sm)",
              },
              panel: {
                paddingTop: "var(--mantine-spacing-md)",
              },
            }}
          >
            <Tabs.List
              style={{ borderBottom: "1px solid var(--rb-border)" }}
            >
              <Tabs.Tab
                value="routes"
                leftSection={<IconWorldSearch size={16} />}
              >
                Route Lookup
              </Tabs.Tab>
              <Tabs.Tab
                value="ping"
                leftSection={<IconPingPong size={16} />}
              >
                Ping
              </Tabs.Tab>
              <Tabs.Tab
                value="traceroute"
                leftSection={<IconArrowsShuffle size={16} />}
              >
                Traceroute
              </Tabs.Tab>
            </Tabs.List>

            <Tabs.Panel value="routes">
              <RouteLookup targetId={selectedTarget} />
            </Tabs.Panel>

            <Tabs.Panel value="ping">
              <PingPanel targetId={selectedTarget} />
            </Tabs.Panel>

            <Tabs.Panel value="traceroute">
              <TraceroutePanel targetId={selectedTarget} />
            </Tabs.Panel>
          </Tabs>

          {!selectedTarget && (
            <Box ta="center" py="xl">
              <IconRoute size={48} color="var(--rb-muted)" stroke={1.5} />
              <Text size="sm" fw={500} c="dimmed" mt="sm">
                Select a target router to begin
              </Text>
              <Text size="xs" c="dimmed" mt="xs">
                Choose a BGP peer from the dropdown above to query routes,
                run ping, or traceroute.
              </Text>
            </Box>
          )}
        </Container>
      </AppShell.Main>

      <AppShell.Footer>
        <StatusBar health={health} error={healthError} />
      </AppShell.Footer>
    </AppShell>
  );
}
