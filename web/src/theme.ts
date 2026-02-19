import { createTheme, rem } from "@mantine/core";

/**
 * Design tokens (referenced via CSS variables in globals.css):
 *
 *  --rb-border:    #e2e8f0    consistent border color everywhere
 *  --rb-canvas:    #f8fafc    page / panel background
 *  --rb-surface:   #ffffff    card / elevated surface
 *  --rb-muted:     #64748b    secondary text
 *  --rb-mono-sm:   0.8125rem  (13px) monospace data size
 *
 * Component size contract:
 *  - All inputs, selects, number inputs:  size="sm"
 *  - All buttons:                          size="sm"
 *  - All badges:                           size="xs"
 *  - Card padding:                         "md"
 *  - Section gaps (Stack):                 "md"
 *  - Inline gaps (Group):                  "sm"
 *  - Data table monospace text:            size="xs", ff="monospace"
 *  - Label text:                           size="xs", fw={500}, c="dimmed"
 */
export const theme = createTheme({
  fontFamily: "'IBM Plex Sans', sans-serif",
  fontFamilyMonospace: "'IBM Plex Mono', monospace",
  headings: {
    fontFamily: "'IBM Plex Sans', sans-serif",
    fontWeight: "600",
  },
  primaryColor: "teal",
  colors: {
    teal: [
      "#f0fdfa",
      "#ccfbf1",
      "#99f6e4",
      "#5eead4",
      "#2dd4bf",
      "#14b8a6",
      "#0d9488",
      "#0f766e",
      "#115e59",
      "#134e4a",
    ],
  },
  radius: {
    xs: rem(4),
    sm: rem(6),
    md: rem(8),
    lg: rem(12),
    xl: rem(16),
  },
  defaultRadius: "md",
  spacing: {
    xs: rem(8),
    sm: rem(12),
    md: rem(16),
    lg: rem(24),
    xl: rem(32),
  },
  components: {
    Button: { defaultProps: { size: "sm" } },
    TextInput: { defaultProps: { size: "sm" } },
    NumberInput: { defaultProps: { size: "sm" } },
    Select: { defaultProps: { size: "sm" } },
    SegmentedControl: { defaultProps: { size: "sm" } },
  },
});
