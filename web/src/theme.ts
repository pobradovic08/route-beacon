import { createTheme, rem } from "@mantine/core";

export const theme = createTheme({
  fontFamily: "'IBM Plex Sans', sans-serif",
  fontFamilyMonospace: "'IBM Plex Mono', monospace",
  headings: {
    fontFamily: "'IBM Plex Sans', sans-serif",
    fontWeight: "700",
  },
  primaryColor: "blue",
  colors: {
    blue: [
      "#f5f9ff",
      "#e8f1ff",
      "#c7dbff",
      "#a3c4ff",
      "#6da3ff",
      "#3d85ff",
      "#0071e3",
      "#005ec4",
      "#004da3",
      "#003d82",
    ],
  },
  radius: {
    xs: rem(6),
    sm: rem(8),
    md: rem(12),
    lg: rem(16),
    xl: rem(20),
  },
  fontSizes: {
    xs: rem(13),
    sm: rem(15),
    md: rem(17),
    lg: rem(19),
    xl: rem(22),
  },
  defaultRadius: "md",
  spacing: {
    xs: rem(12),
    sm: rem(18),
    md: rem(28),
    lg: rem(40),
    xl: rem(56),
  },
  components: {
    Button: { defaultProps: { size: "sm", radius: "md" } },
    TextInput: {
      defaultProps: { size: "sm", radius: "md" },
      styles: {
        label: { fontWeight: 600, color: "var(--rb-text)", marginBottom: 6 },
      },
    },
    NumberInput: {
      defaultProps: { size: "sm", radius: "md" },
      styles: {
        label: { fontWeight: 600, color: "var(--rb-text)", marginBottom: 6 },
      },
    },
    Select: {
      defaultProps: { size: "sm", radius: "md" },
      styles: {
        label: { fontWeight: 600, color: "var(--rb-text)", marginBottom: 6 },
      },
    },
    SegmentedControl: { defaultProps: { size: "sm", radius: "md" } },
    Card: { defaultProps: { radius: "lg" } },
    Badge: { defaultProps: { radius: "sm" } },
  },
});
