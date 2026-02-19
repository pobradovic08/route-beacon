import { useState } from "react";
import {
  Stack,
  Group,
  TextInput,
  Button,
  Text,
  Card,
  Badge,
  Alert,
  Tooltip,
  CopyButton,
  ActionIcon,
  Loader,
  Box,
  Table,
} from "@mantine/core";
import {
  IconSearch,
  IconCopy,
  IconCheck,
  IconAlertTriangle,
  IconChevronDown,
  IconChevronUp,
} from "@tabler/icons-react";
import { api, ApiError } from "../api/client";
import type { RouteLookupResponse, RoutePath } from "../api/types";

interface RouteLookupProps {
  targetId: string | null;
}

const cardStyle = {
  border: "1px solid var(--rb-border)",
  boxShadow: "var(--rb-shadow-sm)",
  background: "var(--rb-surface)",
};

export function RouteLookup({ targetId }: RouteLookupProps) {
  const [prefix, setPrefix] = useState("");
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<RouteLookupResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handleSearch = async () => {
    if (!targetId || !prefix.trim()) return;
    setLoading(true);
    setError(null);
    setResult(null);
    try {
      const trimmed = prefix.trim();
      const mt = trimmed.includes("/") ? "exact" : "longest";
      const data = await api.lookupRoutes(targetId, trimmed, mt);
      setResult(data);
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        if (err.problem.invalid_params?.length) {
          setError(
            err.problem.invalid_params
              .map((p) => `${p.name}: ${p.reason}`)
              .join("; "),
          );
        } else {
          setError(err.problem.detail);
        }
      } else {
        setError(err instanceof Error ? err.message : "Lookup failed");
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <Stack gap="lg">
      <Text size="xs" fw={400} style={{ color: "var(--rb-muted)" }}>
        Enter a CIDR prefix (e.g. 192.0.2.0/24) for an exact match, or a bare IP address (e.g. 198.51.100.1) for the longest prefix match.
      </Text>
      <Group gap="sm" align="flex-end">
        <TextInput
          placeholder="192.0.2.0/24 or 2001:db8::1"
          label="Prefix / IP"
          value={prefix}
          onChange={(e) => setPrefix(e.currentTarget.value)}
          onKeyDown={(e) => e.key === "Enter" && handleSearch()}
          disabled={!targetId}
          style={{ flex: 1 }}
          styles={{
            input: { fontFamily: "var(--mantine-font-family-monospace)" },
          }}
        />
        <Button
          onClick={handleSearch}
          loading={loading}
          disabled={!targetId || !prefix.trim()}
          leftSection={!loading && <IconSearch size={16} />}
          w={120}
        >
          Lookup
        </Button>
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

      {loading && (
        <Group justify="center" py={40}>
          <Loader size="sm" color="blue" />
          <Text size="sm" fw={500} style={{ color: "var(--rb-text-secondary)" }}>
            Querying routes...
          </Text>
        </Group>
      )}

      {result && <RouteResult result={result} />}
    </Stack>
  );
}

function RouteResult({ result }: { result: RouteLookupResponse }) {
  const [expandedIndex, setExpandedIndex] = useState<number | null>(null);

  const toggleRow = (index: number) => {
    setExpandedIndex(expandedIndex === index ? null : index);
  };

  return (
    <Card padding="md" style={cardStyle}>
      <Stack gap="md">
        <Group gap="sm" justify="space-between" align="center">
          <Group gap="sm">
            <Text size="sm" fw={600} ff="monospace">
              {result.prefix}
            </Text>
            <Badge size="sm" variant="light" color="blue">
              {result.meta.match_type}
            </Badge>
            {result.meta.stale && (
              <Badge size="sm" variant="light" color="yellow">
                stale
              </Badge>
            )}
          </Group>
          <Text size="xs" fw={500} style={{ color: "var(--rb-muted)" }}>
            {result.paths.length} path{result.paths.length !== 1 ? "s" : ""}
            {" · "}updated{" "}
            {new Date(result.meta.data_updated_at).toLocaleTimeString()}
          </Text>
        </Group>

        {result.paths.length === 0 ? (
          <Text
            size="sm"
            fw={500}
            ta="center"
            py="lg"
            style={{ color: "var(--rb-text-secondary)" }}
          >
            No paths found for this prefix.
          </Text>
        ) : (
          <>
          <Table
            horizontalSpacing="sm"
            verticalSpacing={8}
            styles={{
              table: { borderCollapse: "collapse" },
              th: {
                color: "var(--rb-muted)",
                fontSize: "var(--mantine-font-size-xs)",
                fontWeight: 600,
                textTransform: "uppercase" as const,
                letterSpacing: "0.05em",
                borderBottom: "1px solid var(--rb-border)",
              },
              td: {
                borderBottom: "1px solid var(--rb-border)",
              },
            }}
          >
            <Table.Thead>
              <Table.Tr>
                <Table.Th style={{ width: 48 }}>Status</Table.Th>
                <Table.Th>Next Hop</Table.Th>
                <Table.Th>AS Path</Table.Th>
                <Table.Th style={{ width: 36 }} />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {result.paths.map((path, i) => {
                const isExpanded = expandedIndex === i;
                return (
                  <PathRows
                    key={i}
                    path={path}
                    expanded={isExpanded}
                    onToggle={() => toggleRow(i)}
                  />
                );
              })}
            </Table.Tbody>
          </Table>
          <StatusLegend />
          </>
        )}
      </Stack>
    </Card>
  );
}

const STATUS_CODES = [
  { code: "B", label: "Best", desc: "Selected as the preferred path to the destination", color: "green", variant: "filled" as const },
  { code: "F", label: "Filtered", desc: "Denied by an inbound or outbound route policy", color: "red", variant: "filled" as const },
  { code: "s", label: "Stale", desc: "Not refreshed after a graceful restart", color: "yellow", variant: "filled" as const },
];

function StatusLegend() {
  return (
    <Stack gap={4}>
      {STATUS_CODES.map(({ code, label, desc, color, variant }) => (
        <Group key={code} gap={6} wrap="nowrap">
          <Badge
            size="sm"
            variant={variant}
            color={color}
            ff="monospace"
            styles={{ label: { textTransform: "none", fontWeight: 700, minWidth: 14, textAlign: "center" } }}
          >
            {code}
          </Badge>
          <Text size="xs" fw={600} style={{ color: "var(--rb-text-secondary)" }}>
            {label}
          </Text>
          <Text size="xs" fw={400} style={{ color: "var(--rb-muted)" }}>
            — {desc}
          </Text>
        </Group>
      ))}
    </Stack>
  );
}

function getStatusCodes(path: RoutePath): string {
  const codes: string[] = [];
  if (path.best) codes.push("B");
  if (path.filtered) codes.push("F");
  if (path.stale) codes.push("s");
  return codes.length > 0 ? codes.join("") : "·";
}

function PathRows({
  path,
  expanded,
  onToggle,
}: {
  path: RoutePath;
  expanded: boolean;
  onToggle: () => void;
}) {
  const allCommunities = [
    ...path.communities.map((c) => c.value),
    ...path.extended_communities.map((c) => c.value),
    ...path.large_communities.map((c) => c.value),
  ];

  const asPathStr = path.as_path.length > 0 ? path.as_path.join(" → ") : "(empty)";

  return (
    <>
      {/* Summary row */}
      <Table.Tr
        onClick={onToggle}
        style={{
          cursor: "pointer",
          transition: "background 0.1s",
        }}
        onMouseEnter={(e) => {
          e.currentTarget.style.background = "var(--rb-hover, rgba(0,0,0,0.02))";
        }}
        onMouseLeave={(e) => {
          e.currentTarget.style.background = "";
        }}
      >
        <Table.Td>
          <Text size="sm" fw={700} ff="monospace" ta="center" style={{ color: path.filtered ? "var(--mantine-color-red-6)" : path.stale ? "var(--mantine-color-yellow-6)" : path.best ? "var(--mantine-color-green-7)" : "var(--rb-muted)" }}>
            {getStatusCodes(path)}
          </Text>
        </Table.Td>
        <Table.Td>
          <Text size="sm" fw={500} ff="monospace">
            {path.next_hop}
          </Text>
        </Table.Td>
        <Table.Td>
          <Text size="sm" fw={500} ff="monospace" style={{ color: path.as_path.length === 0 ? "var(--rb-muted)" : undefined }}>
            {asPathStr}
          </Text>
        </Table.Td>
        <Table.Td>
          {expanded ? (
            <IconChevronUp size={16} style={{ color: "var(--rb-muted)" }} />
          ) : (
            <IconChevronDown size={16} style={{ color: "var(--rb-muted)" }} />
          )}
        </Table.Td>
      </Table.Tr>

      {/* Expanded detail row */}
      {expanded && (
        <Table.Tr>
          <Table.Td
            colSpan={4}
            style={{
              background: "var(--rb-surface-raised, rgba(0,0,0,0.01))",
              borderBottom: "1px solid var(--rb-border)",
            }}
          >
            <Box px="sm" py="md">
              <Stack gap="md">
                {/* Detail attributes */}
                <Group gap="xl" wrap="wrap">
                  <Attr label="Metric (MED)" value={path.med != null ? String(path.med) : "—"} />
                  <Attr label="Local Pref" value={path.local_pref != null ? String(path.local_pref) : "—"} />
                  <Attr label="Weight" value="—" />
                  <Attr label="Origin" value={path.origin.toUpperCase()} />
                  <Attr label="RPKI" value="Not available" />
                </Group>

                {/* Communities */}
                {allCommunities.length > 0 && (
                  <Group gap={4} wrap="wrap" align="center">
                    <Text size="xs" fw={600} style={{ color: "var(--rb-text-secondary)" }}>
                      Communities
                    </Text>
                    {allCommunities.map((v, i) => (
                      <Badge
                        key={i}
                        size="sm"
                        variant="light"
                        color="gray"
                        ff="monospace"
                        styles={{ label: { textTransform: "none", fontWeight: 500 } }}
                      >
                        {v}
                      </Badge>
                    ))}
                  </Group>
                )}

                {/* Copy button */}
                <Group justify="flex-end">
                  <CopyButton value={formatPathText(path)}>
                    {({ copied, copy }) => (
                      <Tooltip label={copied ? "Copied" : "Copy path"}>
                        <ActionIcon
                          variant="subtle"
                          color={copied ? "blue" : "gray"}
                          size="sm"
                          onClick={(e) => {
                            e.stopPropagation();
                            copy();
                          }}
                        >
                          {copied ? <IconCheck size={14} /> : <IconCopy size={14} />}
                        </ActionIcon>
                      </Tooltip>
                    )}
                  </CopyButton>
                </Group>
              </Stack>
            </Box>
          </Table.Td>
        </Table.Tr>
      )}
    </>
  );
}

function Attr({ label, value }: { label: string; value: string }) {
  return (
    <Group gap={4} wrap="nowrap">
      <Text size="xs" fw={600} style={{ color: "var(--rb-text-secondary)" }}>
        {label}
      </Text>
      <Text size="sm" fw={500} ff="monospace">
        {value}
      </Text>
    </Group>
  );
}

function formatPathText(path: RoutePath): string {
  const lines: string[] = [];
  lines.push(`Next Hop: ${path.next_hop}`);
  lines.push(`AS Path: ${path.as_path.join(" ")}`);
  lines.push(`Origin: ${path.origin}`);
  if (path.med != null) lines.push(`MED: ${path.med}`);
  if (path.local_pref != null) lines.push(`Local Pref: ${path.local_pref}`);
  if (path.communities.length > 0)
    lines.push(
      `Communities: ${path.communities.map((c) => c.value).join(" ")}`,
    );
  return lines.join("\n");
}
