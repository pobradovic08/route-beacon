import { useState } from "react";
import {
  Stack,
  Group,
  TextInput,
  Button,
  SegmentedControl,
  Text,
  Card,
  Badge,
  Table,
  Alert,
  Code,
  Tooltip,
  CopyButton,
  ActionIcon,
  Loader,
  Box,
} from "@mantine/core";
import {
  IconSearch,
  IconCopy,
  IconCheck,
  IconAlertTriangle,
} from "@tabler/icons-react";
import { api, ApiError } from "../api/client";
import type { RouteLookupResponse, RoutePath } from "../api/types";

interface RouteLookupProps {
  targetId: string | null;
}

export function RouteLookup({ targetId }: RouteLookupProps) {
  const [prefix, setPrefix] = useState("");
  const [matchType, setMatchType] = useState<string>("auto");
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<RouteLookupResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handleSearch = async () => {
    if (!targetId || !prefix.trim()) return;
    setLoading(true);
    setError(null);
    setResult(null);
    try {
      const mt =
        matchType === "auto"
          ? undefined
          : (matchType as "exact" | "longest");
      const data = await api.lookupRoutes(targetId, prefix.trim(), mt);
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
    <Stack gap="md">
      <Group gap="sm" align="flex-end">
        <TextInput
          placeholder="8.8.8.0/24 or 2001:db8::1"
          label="Prefix / IP"
          value={prefix}
          onChange={(e) => setPrefix(e.currentTarget.value)}
          onKeyDown={(e) => e.key === "Enter" && handleSearch()}
          disabled={!targetId}
          style={{ flex: 1 }}
          styles={{ input: { fontFamily: "var(--mantine-font-family-monospace)" } }}
        />
        <SegmentedControl
          value={matchType}
          onChange={setMatchType}
          data={[
            { label: "Auto", value: "auto" },
            { label: "Exact", value: "exact" },
            { label: "Longest", value: "longest" },
          ]}
        />
        <Button
          onClick={handleSearch}
          loading={loading}
          disabled={!targetId || !prefix.trim()}
          leftSection={!loading && <IconSearch size={16} />}
        >
          Lookup
        </Button>
      </Group>

      {error && (
        <Alert
          color="red"
          variant="light"
          icon={<IconAlertTriangle size={16} />}
          title="Lookup failed"
        >
          <Text size="xs" ff="monospace">
            {error}
          </Text>
        </Alert>
      )}

      {loading && (
        <Group justify="center" py="xl">
          <Loader size="sm" color="teal" />
          <Text size="sm" c="dimmed">
            Querying routes...
          </Text>
        </Group>
      )}

      {result && <RouteResult result={result} />}
    </Stack>
  );
}

function RouteResult({ result }: { result: RouteLookupResponse }) {
  return (
    <Stack gap="sm">
      <Group gap="sm" justify="space-between">
        <Group gap="xs">
          <Code ff="monospace">{result.prefix}</Code>
          <Badge size="xs" variant="light" color="teal">
            {result.meta.match_type}
          </Badge>
          {result.meta.stale && (
            <Badge size="xs" variant="light" color="yellow">
              stale
            </Badge>
          )}
        </Group>
        <Text size="xs" c="dimmed">
          {result.paths.length} path{result.paths.length !== 1 ? "s" : ""}
          {" "}&middot; updated{" "}
          {new Date(result.meta.data_updated_at).toLocaleTimeString()}
        </Text>
      </Group>

      {result.paths.length === 0 ? (
        <Text size="sm" c="dimmed" ta="center" py="md">
          No paths found for this prefix.
        </Text>
      ) : (
        result.paths.map((path, i) => (
          <PathCard key={i} path={path} index={i} />
        ))
      )}
    </Stack>
  );
}

function PathCard({ path, index }: { path: RoutePath; index: number }) {
  return (
    <Card
      withBorder
      padding="md"
      style={{
        borderColor: path.best
          ? "var(--mantine-color-teal-3)"
          : "var(--rb-border)",
        background: path.best
          ? "var(--mantine-color-teal-0)"
          : "var(--rb-surface)",
      }}
    >
      <Stack gap="xs">
        <Group gap="xs" justify="space-between">
          <Group gap="xs">
            <Text size="xs" fw={600} c="dimmed">
              PATH {index + 1}
            </Text>
            {path.best && (
              <Badge size="xs" color="teal">
                BEST
              </Badge>
            )}
            <Badge size="xs" variant="light" color="gray" tt="uppercase">
              {path.origin}
            </Badge>
          </Group>
          <CopyButton value={formatPathText(path)}>
            {({ copied, copy }) => (
              <Tooltip label={copied ? "Copied" : "Copy path"}>
                <ActionIcon
                  variant="subtle"
                  color={copied ? "teal" : "gray"}
                  size="sm"
                  onClick={copy}
                >
                  {copied ? <IconCheck size={14} /> : <IconCopy size={14} />}
                </ActionIcon>
              </Tooltip>
            )}
          </CopyButton>
        </Group>

        <Table
          horizontalSpacing="sm"
          verticalSpacing="xs"
          styles={{
            td: {
              fontFamily: "var(--mantine-font-family-monospace)",
              fontSize: "var(--mantine-font-size-xs)",
            },
          }}
        >
          <Table.Tbody>
            <AttrRow label="Next Hop" value={path.next_hop} />
            <Table.Tr>
              <Table.Td w={110}>
                <Text size="xs" c="dimmed" fw={500}>
                  AS Path
                </Text>
              </Table.Td>
              <Table.Td>
                <ASPath asns={path.as_path} />
              </Table.Td>
            </Table.Tr>
            {path.med != null && (
              <AttrRow label="MED" value={String(path.med)} />
            )}
            {path.local_pref != null && (
              <AttrRow label="Local Pref" value={String(path.local_pref)} />
            )}
            {path.communities.length > 0 && (
              <CommunityRow
                label="Communities"
                values={path.communities.map((c) => c.value)}
              />
            )}
            {path.extended_communities.length > 0 && (
              <CommunityRow
                label="Ext Communities"
                values={path.extended_communities.map((c) => c.value)}
              />
            )}
            {path.large_communities.length > 0 && (
              <CommunityRow
                label="Large Communities"
                values={path.large_communities.map((c) => c.value)}
              />
            )}
          </Table.Tbody>
        </Table>
      </Stack>
    </Card>
  );
}

function AttrRow({ label, value }: { label: string; value: string }) {
  return (
    <Table.Tr>
      <Table.Td w={110}>
        <Text size="xs" c="dimmed" fw={500}>
          {label}
        </Text>
      </Table.Td>
      <Table.Td>
        <Text size="xs" ff="monospace">
          {value}
        </Text>
      </Table.Td>
    </Table.Tr>
  );
}

function CommunityRow({
  label,
  values,
}: {
  label: string;
  values: string[];
}) {
  return (
    <Table.Tr>
      <Table.Td w={110}>
        <Text size="xs" c="dimmed" fw={500}>
          {label}
        </Text>
      </Table.Td>
      <Table.Td>
        <Group gap={4} wrap="wrap">
          {values.map((v, i) => (
            <Badge
              key={i}
              size="xs"
              variant="outline"
              color="gray"
              ff="monospace"
              styles={{ label: { textTransform: "none" } }}
            >
              {v}
            </Badge>
          ))}
        </Group>
      </Table.Td>
    </Table.Tr>
  );
}

function ASPath({ asns }: { asns: number[] }) {
  if (asns.length === 0) {
    return (
      <Text size="xs" c="dimmed" ff="monospace">
        (empty)
      </Text>
    );
  }
  return (
    <Group gap={2} wrap="wrap">
      {asns.map((asn, i) => (
        <Box
          key={i}
          style={{ display: "inline-flex", alignItems: "center", gap: 2 }}
        >
          <Badge
            size="xs"
            variant="filled"
            color="gray.7"
            ff="monospace"
            styles={{ label: { textTransform: "none" } }}
          >
            {asn}
          </Badge>
          {i < asns.length - 1 && (
            <Text size="xs" c="dimmed" span>
              &rarr;
            </Text>
          )}
        </Box>
      ))}
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
