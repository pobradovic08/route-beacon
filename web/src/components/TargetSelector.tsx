import { Select, Group, Text, Box } from "@mantine/core";
import type { Target } from "../api/types";

interface TargetSelectorProps {
  targets: Target[];
  value: string | null;
  onChange: (value: string | null) => void;
  loading?: boolean;
}

export function TargetSelector({
  targets,
  value,
  onChange,
  loading,
}: TargetSelectorProps) {
  const data = targets.map((t) => ({
    value: t.id,
    label: `${t.display_name} — AS${t.asn}`,
  }));

  return (
    <Select
      placeholder="Select a target router"
      data={data}
      value={value}
      onChange={onChange}
      searchable
      clearable
      disabled={loading}
      renderOption={({ option }) => {
        const target = targets.find((t) => t.id === option.value);
        const dotColor =
          target?.status === "up"
            ? "var(--rb-success)"
            : target?.status === "down"
              ? "var(--rb-danger)"
              : "var(--rb-muted)";
        return (
          <Group gap="sm" wrap="nowrap" py={2}>
            <Box
              style={{
                width: "var(--rb-dot)",
                height: "var(--rb-dot)",
                borderRadius: "50%",
                background: dotColor,
                flexShrink: 0,
              }}
            />
            <div>
              <Text size="sm" fw={500} style={{ color: "var(--rb-text)" }}>
                {target?.display_name}
              </Text>
              <Text
                size="xs"
                fw={500}
                ff="monospace"
                style={{ color: "var(--rb-text-secondary)" }}
              >
                AS{target?.asn} · {target?.collector.location}
              </Text>
            </div>
          </Group>
        );
      }}
    />
  );
}
