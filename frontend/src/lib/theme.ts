/**
 * 主题引擎：把 ConfigData 翻译成 CSS 变量 + naive-ui 的 themeOverrides。
 *
 * 设计取向：像一本私人藏书的目录册 —— 墨色/纸色的底、发丝般的分隔线、
 * 衬线标题、等宽数字。主色只用在一处：当前项、线脚、印章似的强调。
 * 预设是完整的"整套纸墨"，不是给同一个界面换色号。
 */
import type { GlobalThemeOverrides } from "naive-ui";
import type { ConfigData } from "../../bindings/gWvljBt/config/models";
import { hexToRgb, isHex, shiftLightness, withAlpha } from "./color";

export interface Palette {
  bg: string; // 底
  deep: string; // 更深一层的底（滚动区/凹陷）
  surface: string; // 纸面
  surface2: string; // 抬起一级的纸面
  surface3: string; // 输入框/代码块底
  text: string;
  dim: string;
  faint: string;
  line: string; // 发丝线
  lineStrong: string;
  accent: string;
}

export interface Preset {
  key: string;
  label: string;
  note: string;
  dark: boolean;
  palette: Palette;
}

export const PRESETS: Preset[] = [
  {
    key: "sumi",
    label: "墨",
    note: "冷灰墨底，朱砂点缀",
    dark: true,
    palette: {
      bg: "#14161a",
      deep: "#0f1114",
      surface: "#1a1d22",
      surface2: "#20242a",
      surface3: "#262b32",
      text: "#e9e5db",
      dim: "#a5a199",
      faint: "#73716b",
      line: "rgba(233, 229, 219, 0.10)",
      lineStrong: "rgba(233, 229, 219, 0.20)",
      accent: "#c05a4e",
    },
  },
  {
    key: "yoru",
    label: "夜",
    note: "蓝墨夜底，月白青",
    dark: true,
    palette: {
      bg: "#0f1518",
      deep: "#0a0f11",
      surface: "#151d21",
      surface2: "#1b252a",
      surface3: "#222d33",
      text: "#dfe9ea",
      dim: "#93a5a8",
      faint: "#63767a",
      line: "rgba(223, 233, 234, 0.10)",
      lineStrong: "rgba(223, 233, 234, 0.20)",
      accent: "#7fb0b7",
    },
  },
  {
    key: "bengara",
    label: "弁柄",
    note: "暖褐底，若竹绿",
    dark: true,
    palette: {
      bg: "#191512",
      deep: "#12100e",
      surface: "#201b18",
      surface2: "#272220",
      surface3: "#2f2925",
      text: "#efe6da",
      dim: "#b1a394",
      faint: "#837568",
      line: "rgba(239, 230, 218, 0.10)",
      lineStrong: "rgba(239, 230, 218, 0.20)",
      accent: "#7fa88a",
    },
  },
  {
    key: "washi",
    label: "和纸",
    note: "米灰纸面，藏青",
    dark: false,
    palette: {
      bg: "#e8e4d9",
      deep: "#ded9cc",
      surface: "#f3f0e7",
      surface2: "#faf8f1",
      surface3: "#efece2",
      text: "#23211d",
      dim: "#5b564d",
      faint: "#6f6a5e",
      line: "rgba(35, 33, 29, 0.13)",
      lineStrong: "rgba(35, 33, 29, 0.26)",
      accent: "#33506b",
    },
  },
  {
    key: "sakura",
    label: "樱",
    note: "藕粉纸面，蘇芳",
    dark: false,
    palette: {
      bg: "#efe9e9",
      deep: "#e5dcdc",
      surface: "#f8f4f4",
      surface2: "#fdfafa",
      surface3: "#f3eded",
      text: "#241f21",
      dim: "#645a5e",
      faint: "#7c7075",
      line: "rgba(36, 31, 33, 0.13)",
      lineStrong: "rgba(36, 31, 33, 0.26)",
      accent: "#b8495f",
    },
  },
  {
    key: "matcha",
    label: "抹茶",
    note: "灰绿纸面，苔绿",
    dark: false,
    palette: {
      bg: "#e6e8df",
      deep: "#dbded2",
      surface: "#f1f2ea",
      surface2: "#f8f9f3",
      surface3: "#e9ebe1",
      text: "#22251d",
      dim: "#5d6352",
      faint: "#6f7666",
      line: "rgba(34, 37, 29, 0.13)",
      lineStrong: "rgba(34, 37, 29, 0.26)",
      accent: "#4f6b45",
    },
  },
];

/** 可选的强调色：都不太亮，避免在纸面上跳出来。 */
export const ACCENTS = [
  { label: "朱", value: "#c05a4e" },
  { label: "蘇芳", value: "#b8495f" },
  { label: "紫", value: "#7b6aa8" },
  { label: "藍", value: "#4a6b8a" },
  { label: "青", value: "#4d8089" },
  { label: "若竹", value: "#5c8a63" },
  { label: "黄土", value: "#a8873f" },
  { label: "鼠", value: "#7c8288" },
];

export const POSTER_ASPECTS = [
  { label: "2 : 3", value: "2/3" },
  { label: "3 : 4", value: "3/4" },
  { label: "1 : 1", value: "1/1" },
  { label: "4 : 5", value: "4/5" },
];

export function presetOf(cfg: Pick<ConfigData, "preset">): Preset {
  return PRESETS.find((p) => p.key === cfg.preset) ?? PRESETS[0];
}

/** 自定义色优先，其次预设主色。 */
export function accentOf(cfg: Pick<ConfigData, "preset" | "accent">): string {
  const custom = (cfg.accent ?? "").trim();
  if (isHex(custom)) return custom.startsWith("#") ? custom : `#${custom}`;
  return presetOf(cfg).palette.accent;
}

export function isDark(cfg: Pick<ConfigData, "preset">): boolean {
  return presetOf(cfg).dark;
}

function clamp(value: unknown, min: number, max: number, fallback: number): number {
  const n = Number(value);
  if (!Number.isFinite(n)) return fallback;
  return Math.min(max, Math.max(min, Math.round(n)));
}

/** 把配置落到 CSS 变量上。纯副作用。 */
export function applyTheme(cfg: ConfigData): void {
  if (typeof document === "undefined") return;
  const root = document.documentElement;
  const preset = presetOf(cfg);
  const p = preset.palette;
  const dark = preset.dark;
  const accent = accentOf(cfg);
  const radius = clamp(cfg.radius, 0, 24, 6);
  const compact = cfg.density === "compact";

  root.dataset.preset = preset.key;
  root.dataset.scheme = dark ? "dark" : "light";
  root.dataset.motion = cfg.motion ? "on" : "off";
  root.dataset.density = compact ? "compact" : "cozy";
  root.style.colorScheme = dark ? "dark" : "light";

  const vars: Record<string, string> = {
    "--gv-bg": p.bg,
    "--gv-bg-deep": p.deep,
    "--gv-surface": p.surface,
    "--gv-surface-2": p.surface2,
    "--gv-surface-3": p.surface3,
    // 侧栏要"退后"：深色系往上提一档（surface），浅色系往下压一档（deep）。
    "--gv-rail": dark ? p.surface : p.deep,
    "--gv-text": p.text,
    "--gv-text-dim": p.dim,
    "--gv-text-faint": p.faint,
    "--gv-border": p.line,
    "--gv-border-strong": p.lineStrong,

    "--gv-accent": accent,
    "--gv-accent-soft": withAlpha(accent, dark ? 0.14 : 0.1),
    "--gv-accent-line": withAlpha(accent, dark ? 0.45 : 0.4),
    "--gv-accent-ink": dark ? p.text : "#ffffff",
    "--gv-shadow-accent": `0 10px 30px -18px ${withAlpha(accent, 0.9)}`,

    "--gv-danger": dark ? "#d9736b" : "#a83a34",
    "--gv-success": dark ? "#7ba17a" : "#3f6b45",
    "--gv-warn": dark ? "#c9a227" : "#8a6a1c",

    "--gv-radius": `${radius}px`,
    "--gv-radius-sm": `${Math.max(2, Math.round(radius * 0.55))}px`,
    // 密度靠"卡片最小宽度 + 卡间距"一起决定每行放几张，只改宽度在宽屏上
    // 经常还是同样的列数，看起来就像没生效。
    "--gv-card-min": compact ? "164px" : "206px",
    "--gv-grid-row-gap": compact ? "14px" : "28px",
    "--gv-grid-col-gap": compact ? "12px" : "20px",
    "--gv-poster-ratio": (cfg.posterAspect || "2/3").split("/").join(" / "),
  };
  for (const [key, value] of Object.entries(vars)) root.style.setProperty(key, value);
}

/** naive-ui 的组件级覆盖：让控件和纸面同一套色。 */
export function buildOverrides(cfg: ConfigData): GlobalThemeOverrides {
  const preset = presetOf(cfg);
  const p = preset.palette;
  const dark = preset.dark;
  const accent = accentOf(cfg);
  const radius = clamp(cfg.radius, 0, 24, 6);
  const r = `${radius}px`;
  const rsm = `${Math.max(2, Math.round(radius * 0.55))}px`;
  // 控件一律不铺底色：纸面上只用一条发丝线表达边界，
  // 免得 naive 默认的浅灰填充在浅色预设里显得像另一套组件。
  const field = "transparent";
  const pop = p.surface2;
  // 浮层（tooltip / popover / popconfirm）的投影：和纸面上要能"浮起来"，
  // 但不能用默认那层大黑雾。
  const popShadow = dark
    ? "0 10px 28px -14px rgba(0, 0, 0, 0.85)"
    : "0 10px 26px -16px rgba(40, 36, 30, 0.55)";

  return {
    common: {
      primaryColor: accent,
      primaryColorHover: shiftLightness(accent, dark ? 0.07 : -0.06),
      primaryColorPressed: shiftLightness(accent, dark ? -0.08 : -0.14),
      primaryColorSuppl: accent,
      borderRadius: r,
      borderRadiusSmall: rsm,
      fontFamily: "var(--gv-font)",
      fontFamilyMono: "var(--gv-mono)",
      fontSize: "13px",
      fontWeight: "400",
      bodyColor: "transparent",
      cardColor: "transparent",
      modalColor: pop,
      popoverColor: pop,
      tableColor: "transparent",
      inputColor: field,
      textColorBase: p.text,
      textColor1: p.text,
      textColor2: dark ? p.dim : "#2b2823",
      textColor3: p.faint,
      borderColor: p.lineStrong,
      dividerColor: p.line,
      hoverColor: withAlpha(accent, dark ? 0.12 : 0.08),
    },
    Layout: { color: "transparent", siderColor: "transparent" },
    Button: {
      borderRadiusTiny: rsm,
      borderRadiusSmall: rsm,
      borderRadiusMedium: r,
      borderRadiusLarge: r,
      fontWeight: "500",
      fontWeightStrong: "600",
    },
    Input: {
      borderRadius: rsm,
      color: field,
      colorFocus: field,
      border: `1px solid ${p.line}`,
      borderHover: `1px solid ${withAlpha(accent, 0.5)}`,
      borderFocus: `1px solid ${accent}`,
      boxShadowFocus: "none",
    },
    Select: {
      peers: {
        InternalSelection: {
          borderRadius: rsm,
          color: field,
          colorActive: field,
          border: `1px solid ${p.line}`,
          borderHover: `1px solid ${withAlpha(accent, 0.5)}`,
          borderActive: `1px solid ${accent}`,
          borderFocus: `1px solid ${accent}`,
          boxShadowFocus: "none",
          boxShadowActive: "none",
          boxShadowHover: "none",
          textColor: p.text,
          placeholderColor: p.faint,
          arrowColor: p.dim,
        },
        InternalSelectMenu: {
          color: pop,
          borderRadius: rsm,
          optionTextColor: p.dim,
          optionTextColorActive: accent,
          optionTextColorPressed: p.text,
          optionHeightMedium: "30px",
        },
      },
    },
    Switch: { railColorActive: accent },
    Slider: {
      fillColor: accent,
      fillColorHover: accent,
      handleColor: accent,
      railColor: p.lineStrong,
    },
    Progress: { fillColor: accent },
    Tag: { borderRadius: rsm },
    Tooltip: {
      borderRadius: rsm,
      color: pop,
      textColor: p.text,
      boxShadow: popShadow,
    },
    Popover: {
      borderRadius: rsm,
      color: pop,
      textColor: p.text,
      dividerColor: p.line,
      boxShadow: popShadow,
    },
    Modal: { borderRadius: r },
    Drawer: { borderRadius: r },
    Card: { borderRadius: r },
    Message: { borderRadius: rsm },
    Popconfirm: { peers: { Popover: { borderRadius: r } } },
    Pagination: { itemBorderRadius: rsm },
    Radio: { buttonBorderRadius: rsm },
    Scrollbar: {
      color: dark ? "rgba(255, 255, 255, 0.14)" : "rgba(35, 33, 29, 0.16)",
      colorHover: withAlpha(accent, 0.45),
    },
  };
}

/** 给颜色选择器用的当前生效色。 */
export function accentRgb(cfg: ConfigData): string {
  const { r, g, b } = hexToRgb(accentOf(cfg));
  return `${r}, ${g}, ${b}`;
}
