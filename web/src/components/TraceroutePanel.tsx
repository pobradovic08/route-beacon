import { useState, useRef } from "react";
import {
  Stack,
  Group,
  TextInput,
  NumberInput,
  Button,
  Text,
  Card,
  Alert,
  Table,
  Badge,
  Loader,
} from "@mantine/core";
import {
  IconPlayerPlay,
  IconPlayerStop,
  IconAlertTriangle,
} from "@tabler/icons-react";
import { api } from "../api/client";
import { useSSE } from "../hooks/useSSE";
import type { TracerouteHop, TracerouteComplete } from "../api/types";

interface TraceroutePanelProps {
  targetId: string | null;
}

const tableStyles = {
  td: {
    fontFamily: "var(--mantine-font-family-monospace)",
    fontSize: "var(--mantine-font-size-sm)",
  },
  th: {
    fontSize: "var(--mantine-font-size-xs)",
    fontWeight: 700,
    color: "var(--rb-muted)",
    textTransform: "uppercase" as const,
    letterSpacing: "0.05em",
    borderBottomColor: "var(--rb-border)",
  },
};

const cardStyle = {
  border: "1px solid var(--rb-border)",
  boxShadow: "var(--rb-shadow-sm)",
  background: "var(--rb-surface)",
};

export function TraceroutePanel({ targetId }: TraceroutePanelProps) {
  const [destination, setDestination] = useState("");
  const [maxHops, setMaxHops] = useState<number>(30);
  const [running, setRunning] = useState(false);
  const [hops, setHops] = useState<TracerouteHop[]>([]);
  const [complete, setComplete] = useState<TracerouteComplete | null>(null);
  const [error, setError] = useState<string | null>(null);
  const hopsRef = useRef<TracerouteHop[]>([]);

  const { start, abort } = useSSE<{
    hop: TracerouteHop;
    complete: TracerouteComplete;
  }>();

  const handleStart = async () => {
    if (!targetId || !destination.trim()) return;
    setRunning(true);
    setHops([]);
    setComplete(null);
    setError(null);
    hopsRef.current = [];

    await start(
      (signal) =>
        api
          .traceroute(targetId, {
            destination: destination.trim(),
            max_hops: maxHops,
          })
          .then((res) => {
            if (signal.aborted)
              throw new DOMException("Aborted", "AbortError");
            return res;
          }),
      {
        onEvent: (type, data) => {
          if (type === "hop") {
            const hop = data as TracerouteHop;
            hopsRef.current = [...hopsRef.current, hop];
            setHops([...hopsRef.current]);
          }
        },
        onError: (msg) => {
          setError(msg);
          setRunning(false);
        },
        onComplete: () => {
          setComplete({ reached_destination: true });
          setRunning(false);
        },
      },
    );
    setRunning(false);
  };

  const handleStop = () => {
    abort();
    setRunning(false);
  };

  return (
    <Stack gap="lg">
      <Text size="xs" fw={400} style={{ color: "var(--rb-muted)" }}>
        Trace the network path to a destination, showing each hop along the way.
      </Text>
      <Group gap="sm" align="flex-end">
        <TextInput
          placeholder="192.0.2.1 or 2001:db8::1"
          label="Destination"
          value={destination}
          onChange={(e) => setDestination(e.currentTarget.value)}
          onKeyDown={(e) => e.key === "Enter" && !running && handleStart()}
          disabled={!targetId || running}
          style={{ flex: 1 }}
          styles={{
            input: { fontFamily: "var(--mantine-font-family-monospace)" },
          }}
        />
        <NumberInput
          label="Max Hops"
          value={maxHops}
          onChange={(v) => setMaxHops(typeof v === "number" ? v : 30)}
          min={1}
          max={64}
          w={100}
          disabled={running}
        />
        {running ? (
          <Button
            color="red"
            variant="light"
            onClick={handleStop}
            leftSection={<IconPlayerStop size={16} />}
            w={120}
          >
            Stop
          </Button>
        ) : (
          <Button
            onClick={handleStart}
            disabled={!targetId || !destination.trim()}
            leftSection={<IconPlayerPlay size={16} />}
            w={120}
          >
            Trace
          </Button>
        )}
      </Group>

      {error && (
        <Alert
          color="red"
          variant="light"
          icon={<IconAlertTriangle size={16} />}
          radius="lg"
        >
          <Text size="sm" fw={500} ff="monospace">
            {error}
          </Text>
        </Alert>
      )}

      {(hops.length > 0 || running) && (
        <Card padding="xl" style={cardStyle}>
          <Table
            horizontalSpacing="sm"
            verticalSpacing={12}
            styles={tableStyles}
          >
            <Table.Thead>
              <Table.Tr>
                <Table.Th w={60}>Hop</Table.Th>
                <Table.Th>Address</Table.Th>
                <Table.Th>RTT (ms)</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {hops.map((hop) => (
                <Table.Tr key={hop.hop_number}>
                  <Table.Td>
                    <Text size="sm" fw={600} ff="monospace">
                      {hop.hop_number}
                    </Text>
                  </Table.Td>
                  <Table.Td>
                    <Text
                      size="sm"
                      fw={500}
                      ff="monospace"
                      style={
                        hop.address === "*"
                          ? { color: "var(--rb-muted)" }
                          : undefined
                      }
                    >
                      {hop.address === "*" ? "* * *" : hop.address}
                    </Text>
                  </Table.Td>
                  <Table.Td>
                    {hop.rtt_ms.length > 0 ? (
                      <Group gap="sm">
                        {hop.rtt_ms.map((rtt, i) => (
                          <Text key={i} size="sm" fw={500} ff="monospace">
                            {rtt.toFixed(2)}
                          </Text>
                        ))}
                      </Group>
                    ) : (
                      <Text
                        size="sm"
                        fw={500}
                        ff="monospace"
                        style={{ color: "var(--rb-muted)" }}
                      >
                        *
                      </Text>
                    )}
                  </Table.Td>
                </Table.Tr>
              ))}
              {running && (
                <Table.Tr>
                  <Table.Td colSpan={3}>
                    <Group gap="sm" py="xs">
                      <Loader size="xs" color="blue" />
                      <Text
                        size="sm"
                        fw={500}
                        style={{ color: "var(--rb-text-secondary)" }}
                      >
                        Tracing...
                      </Text>
                    </Group>
                  </Table.Td>
                </Table.Tr>
              )}
            </Table.Tbody>
          </Table>

          {complete && !running && (
            <Badge
              size="sm"
              color={complete.reached_destination ? "green" : "yellow"}
              variant="light"
              mt="md"
            >
              {complete.reached_destination
                ? "Destination reached"
                : "Destination not reached"}
            </Badge>
          )}
        </Card>
      )}
    </Stack>
  );
}
