import { useState, useRef } from "react";
import {
  Stack,
  Group,
  TextInput,
  NumberInput,
  Button,
  Text,
  Card,
  Progress,
  Alert,
  Table,
  Badge,
  Box,
} from "@mantine/core";
import {
  IconPlayerPlay,
  IconPlayerStop,
  IconAlertTriangle,
} from "@tabler/icons-react";
import { api } from "../api/client";
import { useSSE } from "../hooks/useSSE";
import type { PingReply, PingSummary } from "../api/types";

interface PingPanelProps {
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

export function PingPanel({ targetId }: PingPanelProps) {
  const [destination, setDestination] = useState("");
  const [count, setCount] = useState<number>(5);
  const [running, setRunning] = useState(false);
  const [replies, setReplies] = useState<PingReply[]>([]);
  const [summary, setSummary] = useState<PingSummary | null>(null);
  const [error, setError] = useState<string | null>(null);
  const repliesRef = useRef<PingReply[]>([]);

  const { start, abort } = useSSE<{
    reply: PingReply;
    summary: PingSummary;
  }>();

  const handleStart = async () => {
    if (!targetId || !destination.trim()) return;
    setRunning(true);
    setReplies([]);
    setSummary(null);
    setError(null);
    repliesRef.current = [];

    await start(
      (signal) =>
        api
          .ping(targetId, { destination: destination.trim(), count })
          .then((res) => {
            if (signal.aborted)
              throw new DOMException("Aborted", "AbortError");
            return res;
          }),
      {
        onEvent: (type, data) => {
          if (type === "reply") {
            const reply = data as PingReply;
            repliesRef.current = [...repliesRef.current, reply];
            setReplies([...repliesRef.current]);
          } else if (type === "summary") {
            setSummary(data as PingSummary);
          }
        },
        onError: (msg) => {
          setError(msg);
          setRunning(false);
        },
        onComplete: () => {
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

  const progress = count > 0 ? (replies.length / count) * 100 : 0;

  return (
    <Stack gap="lg">
      <Text size="xs" fw={400} style={{ color: "var(--rb-muted)" }}>
        Send ICMP echo requests to test reachability and measure round-trip latency.
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
          label="Count"
          value={count}
          onChange={(v) => setCount(typeof v === "number" ? v : 5)}
          min={1}
          max={10}
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
            Ping
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

      {(replies.length > 0 || running) && (
        <Card padding="xl" style={cardStyle}>
          <Stack gap="md">
            {running && (
              <Progress value={progress} color="blue" size="xs" radius="xl" animated />
            )}

            <Table
              horizontalSpacing="sm"
              verticalSpacing={12}
              styles={tableStyles}
            >
              <Table.Thead>
                <Table.Tr>
                  <Table.Th w={60}>Seq</Table.Th>
                  <Table.Th w={120}>RTT</Table.Th>
                  <Table.Th w={60}>TTL</Table.Th>
                  <Table.Th>Status</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {replies.map((r) => (
                  <Table.Tr key={r.seq}>
                    <Table.Td>
                      <Text size="sm" fw={500} ff="monospace">{r.seq}</Text>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm" fw={500} ff="monospace">
                        {r.success ? `${r.rtt_ms.toFixed(2)} ms` : "—"}
                      </Text>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm" fw={500} ff="monospace">
                        {r.success ? r.ttl : "—"}
                      </Text>
                    </Table.Td>
                    <Table.Td>
                      <Badge
                        size="sm"
                        color={r.success ? "green" : "red"}
                        variant="light"
                      >
                        {r.success ? "OK" : "FAIL"}
                      </Badge>
                    </Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          </Stack>
        </Card>
      )}

      {summary && <PingSummaryCard summary={summary} />}
    </Stack>
  );
}

function PingSummaryCard({ summary }: { summary: PingSummary }) {
  const lossColor =
    summary.loss_pct === 0
      ? "var(--rb-success)"
      : summary.loss_pct < 50
        ? "var(--rb-warning)"
        : "var(--rb-danger)";

  return (
    <Card padding="lg" style={cardStyle}>
      <Text
        size="xs"
        fw={700}
        tt="uppercase"
        mb="md"
        style={{ color: "var(--rb-muted)", letterSpacing: "0.05em" }}
      >
        Summary
      </Text>
      <Group gap="xl">
        <Box>
          <Text size="xs" fw={600} style={{ color: "var(--rb-text-secondary)" }}>
            Sent / Received
          </Text>
          <Text size="sm" fw={600} ff="monospace" mt={2}>
            {summary.packets_sent} / {summary.packets_received}
          </Text>
        </Box>
        <Box>
          <Text size="xs" fw={600} style={{ color: "var(--rb-text-secondary)" }}>
            Loss
          </Text>
          <Text
            size="sm"
            fw={600}
            ff="monospace"
            mt={2}
            style={{ color: lossColor }}
          >
            {summary.loss_pct.toFixed(1)}%
          </Text>
        </Box>
        <Box>
          <Text size="xs" fw={600} style={{ color: "var(--rb-text-secondary)" }}>
            Min / Avg / Max
          </Text>
          <Text size="sm" fw={600} ff="monospace" mt={2}>
            {summary.rtt_min_ms.toFixed(2)} / {summary.rtt_avg_ms.toFixed(2)}{" "}
            / {summary.rtt_max_ms.toFixed(2)} ms
          </Text>
        </Box>
      </Group>
    </Card>
  );
}
