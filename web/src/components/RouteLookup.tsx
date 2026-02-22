import { useState } from "react";
import {
  Stack,
  Group,
  TextInput,
  Button,
  Text,
  Title,
  Card,
  Badge,
  Alert,
  Tooltip,
  CopyButton,
  ActionIcon,
  Loader,
  Box,
  SimpleGrid,
  Table,
} from "@mantine/core";
import {
  IconSearch,
  IconCopy,
  IconCheck,
  IconAlertTriangle,
  IconChevronDown,
  IconChevronUp,
  IconShieldCheck,
  IconShieldQuestion,
  IconShieldX,
} from "@tabler/icons-react";
import { api, ApiError } from "../api/client";
import type { RouteLookupResponse, RoutePath } from "../api/types";

interface RouteLookupProps {
  targetId: string | null;
}

const cardStyle: React.CSSProperties = {
  border: "none",
  borderRadius: "var(--rb-radius)",
  boxShadow: "0 2px 12px rgba(0,0,0,0.08)",
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
      <Stack gap={12}>
        <Title order={4}>Prefix / IP</Title>
        <Group gap="sm" align="flex-end">
          <TextInput
            placeholder="192.0.2.0/24 or 2001:db8::1"
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
      </Stack>

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

      {result && (
        <Stack gap={12}>
          <Group justify="space-between" align="center">
            <Title order={4}>
              {result.paths.length === 1 ? "Selected route" : "Selected routes"}
            </Title>
            <Text size="xs" fw={500} style={{ color: "var(--rb-muted)" }}>
              {result.paths.length} path{result.paths.length !== 1 ? "s" : ""}
              {" · "}updated{" "}
              {new Date(result.meta.data_updated_at).toLocaleTimeString()}
            </Text>
          </Group>
          <RouteResult result={result} />
        </Stack>
      )}
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
                <Table.Th style={{ width: 16, paddingLeft: 0, paddingRight: 0 }}>ROA</Table.Th>
                <Table.Th>Prefix</Table.Th>
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
                    prefix={result.prefix}
                    target={result.target}
                    path={path}
                    expanded={isExpanded}
                    onToggle={() => toggleRow(i)}
                  />
                );
              })}
            </Table.Tbody>
          </Table>
          <AsPathLegend />
          <CommunityLegend />
          </>
        )}
      </Stack>
    </Card>
  );
}


function collapseAsPath(segments: (number | number[])[]) {
  const result: { label: string; count: number }[] = [];
  for (const seg of segments) {
    const label = Array.isArray(seg) ? `{${seg.join(",")}}` : String(seg);
    const last = result[result.length - 1];
    if (last && last.label === label) {
      last.count++;
    } else {
      result.push({ label, count: 1 });
    }
  }
  return result;
}

function AsPathLegend() {
  const items = [
    { label: "Transit ASN", bg: "rgba(0, 113, 227, 0.08)", border: "var(--rb-accent)", text: "#1d4ed8" },
    { label: "Origin ASN", bg: "rgba(52, 199, 89, 0.12)", border: "#34c759", text: "#15803d" },
    { label: "Prepended ASNs (prepend count)", bg: "rgba(255, 159, 10, 0.12)", border: "#ff9f0a", text: "#c2410c" },
    { label: "Private ASN", bg: "rgba(139, 92, 246, 0.1)", border: "#8b5cf6", text: "#6d28d9" },
  ];
  return (
    <Stack gap={4}>
      <Text size="xs" fw={600} style={{ color: "var(--rb-muted)", letterSpacing: "0.03em" }}>AS Path Legend</Text>
      <Group gap={8} wrap="wrap">
        {items.map((item) => (
          <Group key={item.label} gap={6} wrap="nowrap">
            <Box style={{
              width: 20,
              height: 20,
              borderRadius: 4,
              background: item.bg,
              borderLeft: item.border ? `2px solid ${item.border}` : undefined,
            }} />
            <Text size="xs" fw={500} style={{ color: "var(--rb-text-secondary)" }}>{item.label}</Text>
          </Group>
        ))}
      </Group>
    </Stack>
  );
}

function CommunityLegend() {
  const items = [
    { label: "Own ASN", color: "blue" },
    { label: "Private ASN", color: "violet" },
    { label: "External", color: "red" },
  ];
  return (
    <Stack gap={4}>
      <Text size="xs" fw={600} style={{ color: "var(--rb-muted)", letterSpacing: "0.03em" }}>Communities Legend</Text>
      <Group gap={8} wrap="wrap">
        {items.map((item) => (
          <Group key={item.label} gap={6} wrap="nowrap">
            <Badge size="sm" variant="light" color={item.color} styles={{ label: { textTransform: "none" } }}>
              ···
            </Badge>
            <Text size="xs" fw={500} style={{ color: "var(--rb-text-secondary)" }}>{item.label}</Text>
          </Group>
        ))}
      </Group>
    </Stack>
  );
}

function isPrivateAsn(n: number): boolean {
  return (n >= 64512 && n <= 65534) || (n >= 4200000000 && n <= 4294967294);
}

function communityColor(value: string, routerAsn: number | null): string {
  const asn = Number(value.split(":")[0]);
  if (!isNaN(asn) && routerAsn != null && asn === routerAsn) return "blue";
  if (!isNaN(asn) && isPrivateAsn(asn)) return "violet";
  return "red";
}

function AsChevron({ label, first, last, count = 1 }: { label: string; first: boolean; last: boolean; count?: number }) {
  const priv = isPrivateAsn(Number(label));
  const notch = 6;
  const clipPath = first
    ? `polygon(0 0, calc(100% - ${notch}px) 0, 100% 50%, calc(100% - ${notch}px) 100%, 0 100%)`
    : last
      ? `polygon(0 0, 100% 0, 100% 100%, 0 100%, ${notch}px 50%)`
      : `polygon(0 0, calc(100% - ${notch}px) 0, 100% 50%, calc(100% - ${notch}px) 100%, 0 100%, ${notch}px 50%)`;

  return (
    <Box
      style={{
        display: "inline-flex",
        alignItems: "center",
        background: priv ? "rgba(139, 92, 246, 0.1)" : last ? "rgba(52, 199, 89, 0.12)" : count > 1 ? "rgba(255, 159, 10, 0.12)" : "rgba(0, 113, 227, 0.08)",
        clipPath,
        marginLeft: first ? 0 : -1,
        borderLeft: first ? `2px solid ${priv ? "#8b5cf6" : "var(--rb-accent)"}` : undefined,
        borderRight: last ? `2px solid ${priv ? "#8b5cf6" : "#34c759"}` : undefined,
      }}
    >
      <Box style={{
        paddingLeft: first ? 6 : notch + 4,
        paddingRight: count > 1 ? 4 : (last ? 6 : notch + 4),
        paddingTop: 2,
        paddingBottom: 2,
      }}>
        <Text size="xs" fw={600} ff="monospace" style={{ color: priv ? "#6d28d9" : last ? "#15803d" : first ? "#1d4ed8" : count > 1 ? "#c2410c" : "#1e40af", lineHeight: 1 }}>
          {label}
        </Text>
      </Box>
      {count > 1 && (
        <Box style={{
          background: "rgba(255, 159, 10, 0.2)",
          paddingLeft: 4,
          paddingRight: last ? 6 : notch + 4,
          paddingTop: 2,
          paddingBottom: 2,
        }}>
          <Text size="xs" fw={600} ff="monospace" style={{ color: "#9a3412", lineHeight: 1 }}>
            {count}x
          </Text>
        </Box>
      )}
    </Box>
  );
}

function RpkiIcon({ status }: { status?: string }) {
  if (status === "valid") return <IconShieldCheck size={16} style={{ color: "var(--rb-success)" }} />;
  if (status === "invalid") return <IconShieldX size={16} style={{ color: "var(--rb-danger)" }} />;
  return <IconShieldQuestion size={16} style={{ color: "var(--rb-muted)" }} />;
}

function PathRows({
  prefix,
  target,
  path,
  expanded,
  onToggle,
}: {
  prefix: string;
  target: { id: string; display_name: string; asn: number | null };
  path: RoutePath;
  expanded: boolean;
  onToggle: () => void;
}) {
  const allCommunities = [
    ...path.communities.map((c) => c.value),
    ...path.extended_communities.map((c) => c.value),
    ...path.large_communities.map((c) => c.value),
  ];

  const asPathStr = path.as_path.length > 0
    ? path.as_path.map((seg) => Array.isArray(seg) ? `{${seg.join(",")}}` : String(seg)).join(" → ")
    : "(empty)";

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
        <Table.Td style={{ paddingLeft: 0, paddingRight: 0 }}>
          <Group justify="center" align="center">
            <RpkiIcon status={(path as RoutePath & { rpki_status?: string }).rpki_status} />
          </Group>
        </Table.Td>
        <Table.Td>
          <Text size="xs" fw={600} ff="monospace">
            {prefix}
          </Text>
        </Table.Td>
        <Table.Td>
          <Text size="xs" fw={500} ff="monospace">
            {path.next_hop}
          </Text>
        </Table.Td>
        <Table.Td>
          {path.as_path.length === 0 ? (
            <Text size="xs" fw={500} ff="monospace" style={{ color: "var(--rb-muted)" }}>(empty)</Text>
          ) : (
            <Group gap={0} wrap="wrap" style={{ rowGap: 4 }}>
              {collapseAsPath(path.as_path).map((seg, i, arr) => (
                <AsChevron key={i} label={seg.label} count={seg.count} first={i === 0} last={i === arr.length - 1} />
              ))}
            </Group>
          )}
        </Table.Td>
        <Table.Td>
          <Group justify="center" align="center">
            {expanded ? (
              <IconChevronUp size={16} style={{ color: "var(--rb-muted)" }} />
            ) : (
              <IconChevronDown size={16} style={{ color: "var(--rb-muted)" }} />
            )}
          </Group>
        </Table.Td>
      </Table.Tr>

      {/* Expanded detail row */}
      {expanded && (
        <Table.Tr>
          <Table.Td
            colSpan={5}
            style={{
              background: "rgba(0, 0, 0, 0.015)",
              borderBottom: "1px solid var(--rb-border)",
              padding: 0,
            }}
          >
            <Box py={12} px={12}>
              {/* Attribute grid */}
              <SimpleGrid cols={5} spacing={12}>
                <AttrCell label="MED" value={path.med != null ? String(path.med) : "—"} />
                <AttrCell label="Local Pref" value={path.local_pref != null ? String(path.local_pref) : "—"} />
                <AttrCell label="Weight" value="—" />
                <AttrCell label="Origin" value={path.origin.toUpperCase()} />
                <AttrCell label="RPKI" value="N/A" muted />
              </SimpleGrid>

              {/* Communities */}
              {allCommunities.length > 0 && (
                <Box mt={12}>
                  <Text size="xs" fw={600} mb={6} style={{ color: "var(--rb-muted)", letterSpacing: "0.03em" }}>
                    Communities
                  </Text>
                  <Group gap={4} wrap="wrap">
                    {allCommunities.map((v, i) => (
                      <Badge
                        key={i}
                        size="md"
                        variant="light"
                        color={communityColor(v, target.asn)}
                        ff="monospace"
                        styles={{ label: { textTransform: "none", fontWeight: 700 } }}
                      >
                        {v}
                      </Badge>
                    ))}
                  </Group>
                </Box>
              )}

              {/* Copy */}
              <Group justify="flex-end" mt={8}>
                <CopyButton value={formatPathText(prefix, target, path)}>
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
            </Box>
          </Table.Td>
        </Table.Tr>
      )}
    </>
  );
}

function AttrCell({ label, value, muted }: { label: string; value: string; muted?: boolean }) {
  return (
    <Box style={{
      background: "rgba(0, 0, 0, 0.03)",
      borderRadius: 6,
      padding: "6px 10px",
    }}>
      <Text size="xs" fw={600} style={{ color: "var(--rb-muted)", lineHeight: 1, letterSpacing: "0.03em" }}>
        {label}
      </Text>
      <Text size="sm" fw={700} ff="monospace" mt={2} style={{ color: muted ? "var(--rb-muted)" : "var(--rb-text)", lineHeight: 1 }}>
        {value}
      </Text>
    </Box>
  );
}

function formatPathText(prefix: string, target: { display_name: string; asn: number | null }, path: RoutePath): string {
  const lines: string[] = [];
  const router = target.asn != null ? `${target.display_name} (AS${target.asn})` : target.display_name;
  lines.push(`Router: ${router}`);
  lines.push(`Prefix: ${prefix}`);
  lines.push(`Next Hop: ${path.next_hop}`);
  lines.push(`AS Path: ${path.as_path.map((seg) => Array.isArray(seg) ? `{${seg.join(",")}}` : String(seg)).join(" ")}`);
  lines.push(`Origin: ${path.origin}`);
  if (path.med != null) lines.push(`MED: ${path.med}`);
  if (path.local_pref != null) lines.push(`Local Pref: ${path.local_pref}`);
  if (path.communities.length > 0)
    lines.push(
      `Communities: ${path.communities.map((c) => c.value).join(" ")}`,
    );
  return lines.join("\n");
}
