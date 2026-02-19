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
          result.paths.map((path, i) => (
            <Box key={i}>
              {i > 0 && (
                <Box
                  mb="md"
                  style={{ borderTop: "1px solid var(--rb-border)" }}
                />
              )}
              <PathCard path={path} index={i} />
            </Box>
          ))
        )}
      </Stack>
    </Card>
  );
}

function PathCard({ path, index }: { path: RoutePath; index: number }) {
  const allCommunities = [
    ...path.communities.map((c) => c.value),
    ...path.extended_communities.map((c) => c.value),
    ...path.large_communities.map((c) => c.value),
  ];

  return (
    <Stack gap={10}>
        {/* Header */}
        <Group gap="sm" justify="space-between">
          <Group gap="sm">
            <Text
              size="xs"
              fw={700}
              tt="uppercase"
              style={{ color: "var(--rb-muted)", letterSpacing: "0.05em" }}
            >
              Path {index + 1}
            </Text>
            {path.best && (
              <Badge size="sm" color="blue" variant="filled">
                Best
              </Badge>
            )}
            <Badge size="sm" variant="light" color="gray" tt="uppercase">
              {path.origin}
            </Badge>
          </Group>
          <CopyButton value={formatPathText(path)}>
            {({ copied, copy }) => (
              <Tooltip label={copied ? "Copied" : "Copy path"}>
                <ActionIcon
                  variant="subtle"
                  color={copied ? "blue" : "gray"}
                  size="sm"
                  onClick={copy}
                >
                  {copied ? <IconCheck size={14} /> : <IconCopy size={14} />}
                </ActionIcon>
              </Tooltip>
            )}
          </CopyButton>
        </Group>

        {/* Attributes line */}
        <Group gap={6} wrap="wrap">
          <Attr label="Next Hop" value={path.next_hop} />
          {path.med != null && <Attr label="MED" value={String(path.med)} />}
          {path.local_pref != null && (
            <Attr label="LP" value={String(path.local_pref)} />
          )}
        </Group>

        {/* AS Path */}
        <Group gap={6} align="center" wrap="wrap">
          <Text size="xs" fw={600} style={{ color: "var(--rb-text-secondary)" }}>
            AS Path
          </Text>
          {path.as_path.length === 0 ? (
            <Text size="xs" fw={500} ff="monospace" style={{ color: "var(--rb-muted)" }}>
              (empty)
            </Text>
          ) : (
            path.as_path.map((asn, i) => (
              <Box
                key={i}
                style={{ display: "inline-flex", alignItems: "center", gap: 4 }}
              >
                <Badge
                  size="sm"
                  variant="filled"
                  color="dark"
                  ff="monospace"
                  styles={{ label: { textTransform: "none", fontWeight: 600 } }}
                >
                  {asn}
                </Badge>
                {i < path.as_path.length - 1 && (
                  <Text size="xs" span style={{ color: "var(--rb-muted)" }}>
                    →
                  </Text>
                )}
              </Box>
            ))
          )}
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
    </Stack>
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
