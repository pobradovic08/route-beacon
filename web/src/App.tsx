import { useState } from "react";
import {
  AppShell,
  Group,
  Text,
  Tabs,
  Box,
} from "@mantine/core";
import {
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
    <AppShell
      header={selectedTarget ? { height: 64 } : undefined}
      footer={selectedTarget ? { height: 40 } : undefined}
      padding={0}
      style={{ maxWidth: 960, margin: "0 auto" }}
    >
      {selectedTarget && (
        <AppShell.Header
          style={{
            background: "rgba(255, 255, 255, 0.8)",
            backdropFilter: "saturate(180%) blur(20px)",
            WebkitBackdropFilter: "saturate(180%) blur(20px)",
          }}
        >
          <Group h="100%" px="lg" justify="space-between">
            <Group gap={10}>
              <svg
                width="28"
                height="28"
                viewBox="0 0 28 28"
                fill="none"
                xmlns="http://www.w3.org/2000/svg"
              >
                <circle cx="14" cy="14" r="6" fill="var(--rb-accent)" />
                <circle
                  cx="14"
                  cy="14"
                  r="10"
                  stroke="var(--rb-accent)"
                  strokeWidth="1.5"
                  strokeOpacity="0.3"
                />
                <circle
                  cx="14"
                  cy="14"
                  r="13"
                  stroke="var(--rb-accent)"
                  strokeWidth="1"
                  strokeOpacity="0.12"
                />
              </svg>
              <Text
                size="md"
                fw={700}
                style={{ color: "var(--rb-text)", letterSpacing: "-0.02em" }}
              >
                Route Beacon
              </Text>
            </Group>
          </Group>
        </AppShell.Header>
      )}

      <AppShell.Main style={{ background: "var(--rb-canvas)" }}>
        <Box py={56} px="xl">
          {!selectedTarget ? (
            <Box ta="center" style={{ minHeight: "calc(100vh - 152px)", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center" }}>
              <div style={{ position: "relative", width: 48, height: 48, margin: "0 auto" }}>
                <div
                  style={{
                    position: "absolute",
                    top: 14,
                    left: 14,
                    width: 20,
                    height: 20,
                    borderRadius: "50%",
                    background: "var(--rb-accent)",
                    animation: "beacon-pulse 2s ease-out infinite",
                  }}
                />
                <svg
                  width="48"
                  height="48"
                  viewBox="0 0 48 48"
                  fill="none"
                  xmlns="http://www.w3.org/2000/svg"
                  style={{ position: "relative", display: "block" }}
                >
                  <circle cx="24" cy="24" r="10" fill="var(--rb-accent)" />
                  <circle
                    cx="24"
                    cy="24"
                    r="17"
                    stroke="var(--rb-accent)"
                    strokeWidth="1.5"
                    strokeOpacity="0.3"
                  />
                  <circle
                    cx="24"
                    cy="24"
                    r="23"
                    stroke="var(--rb-accent)"
                    strokeWidth="1"
                    strokeOpacity="0.15"
                  />
                </svg>
              </div>
              <Text
                size="lg"
                fw={700}
                mt="lg"
                style={{ color: "var(--rb-text)" }}
              >
                Welcome to Route Beacon Looking Glass
              </Text>
              <Text
                size="sm"
                fw={400}
                mt="sm"
                maw={480}
                mx="auto"
                style={{ color: "var(--rb-text-secondary)", lineHeight: 1.6 }}
              >
                A read-only view into the routing table of the network.
                Inspect advertised prefixes, verify AS paths, troubleshoot
                reachability, and measure latency to remote destinations.
              </Text>
              <Box w={360} mx="auto" mt="md">
                <TargetSelector
                  targets={targets}
                  value={selectedTarget}
                  onChange={setSelectedTarget}
                  loading={targetsLoading}
                />
              </Box>
              <Text
                size="sm"
                fw={500}
                mt="sm"
                style={{ color: "var(--rb-muted)" }}
              >
                Select a target router to get started.
              </Text>
            </Box>
          ) : (
            <Tabs
              value={activeTab}
              onChange={setActiveTab}
              styles={{ panel: { paddingTop: 32 } }}
            >
              <Tabs.List>
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
          )}
        </Box>
      </AppShell.Main>

      {selectedTarget && (
        <AppShell.Footer
          style={{
            background: "rgba(255, 255, 255, 0.8)",
            backdropFilter: "saturate(180%) blur(20px)",
            WebkitBackdropFilter: "saturate(180%) blur(20px)",
          }}
        >
          <StatusBar health={health} error={healthError} />
        </AppShell.Footer>
      )}
    </AppShell>
  );
}
