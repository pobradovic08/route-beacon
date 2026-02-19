import { Select, Group, Text } from "@mantine/core";
import { IconCircleFilled } from "@tabler/icons-react";
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
      leftSection={
        value ? (
          <IconCircleFilled
            size={8}
            color={
              targets.find((t) => t.id === value)?.status === "up"
                ? "var(--mantine-color-teal-6)"
                : "var(--mantine-color-red-6)"
            }
          />
        ) : undefined
      }
      renderOption={({ option }) => {
        const target = targets.find((t) => t.id === option.value);
        return (
          <Group gap="sm" wrap="nowrap">
            <IconCircleFilled
              size={8}
              color={
                target?.status === "up"
                  ? "var(--mantine-color-teal-6)"
                  : target?.status === "down"
                    ? "var(--mantine-color-red-6)"
                    : "var(--mantine-color-gray-4)"
              }
            />
            <div>
              <Text size="sm">{target?.display_name}</Text>
              <Text size="xs" c="dimmed" ff="monospace">
                AS{target?.asn} &middot; {target?.collector.location}
              </Text>
            </div>
          </Group>
        );
      }}
    />
  );
}
