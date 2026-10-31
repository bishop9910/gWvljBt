/**
 * 领域类型与常量。
 *
 * 结构体字段跟 Go 侧 json tag 一一对应；这里手写而不是直接引用生成的
 * bindings，是因为 main 包里的结构体生成路径会随打包方式变化，而 JSON
 * 形状是稳定的（Wails 走 JSON RPC）。ConfigData 例外——它就在
 * gWvljBt/config 包里，直接引用生成的模型最省事。
 */
export type { ConfigData } from "../../bindings/gWvljBt/config/models";
import type { ConfigData } from "../../bindings/gWvljBt/config/models";

export type PlayStatus = "unplayed" | "playing" | "cleared" | "onhold" | "dropped";
export type PatchKind = "translation" | "unlock" | "fix" | "mod" | "bypass" | "other";
export type MetaSource = "vndb" | "bangumi";

export const PLAY_STATUSES: PlayStatus[] = ["unplayed", "playing", "cleared", "onhold", "dropped"];
export const PATCH_KINDS: PatchKind[] = ["translation", "unlock", "fix", "mod", "bypass", "other"];

export interface StatusMeta {
  label: string;
  color: string;
  short: string;
}

export const STATUS_META: Record<PlayStatus, StatusMeta> = {
  unplayed: { label: "未开始", short: "未开始", color: "#8b93a7" },
  playing: { label: "在玩", short: "在玩", color: "#38bdf8" },
  cleared: { label: "已通关", short: "通关", color: "#4ade80" },
  onhold: { label: "搁置", short: "搁置", color: "#ffb454" },
  dropped: { label: "弃坑", short: "弃坑", color: "#ff6b81" },
};

export interface KindMeta {
  label: string;
  hint: string;
  color: string;
}

export const KIND_META: Record<PatchKind, KindMeta> = {
  translation: { label: "汉化", hint: "汉化补丁", color: "#8b7cff" },
  unlock: { label: "解锁", hint: "解锁补丁（全 CG / 全路线）", color: "#ff7eb6" },
  fix: { label: "修复", hint: "兼容性 / 崩溃修复", color: "#4ade80" },
  mod: { label: "MOD", hint: "游戏性改动", color: "#ffb454" },
  bypass: { label: "破解", hint: "DRM / 运行时绕过", color: "#ff6b81" },
  other: { label: "其他", hint: "其他类型的补丁", color: "#8b93a7" },
};

export const SORT_OPTIONS: { label: string; value: string }[] = [
  { label: "最近更新", value: "updated" },
  { label: "最近加入", value: "created" },
  { label: "标题", value: "title" },
  { label: "发行年份", value: "year" },
  { label: "评分", value: "rating" },
  { label: "游玩时长", value: "playtime" },
];

export interface PatchFile {
  id: number;
  patchId: number;
  relPath: string;
  size: number;
  op: string;
  isDir: boolean;
}

export interface Patch {
  id: number;
  createdAt: string;
  updatedAt: string;
  gameId: number;
  name: string;
  kind: PatchKind | string;
  version: string;
  source: string;
  sourceUrl: string;
  note: string;
  installed: boolean;
  installedAt: string | null;
  files: PatchFile[] | null;
}

export interface Game {
  id: number;
  createdAt: string;
  updatedAt: string;
  title: string;
  originalTitle: string;
  alias: string;
  poster: string;
  banner: string;
  developer: string;
  publisher: string;
  releaseDate: string;
  year: number;
  description: string;
  tags: string;
  genres: string;
  rating: number;
  playStatus: PlayStatus | string;
  playtime: number;
  favorite: boolean;
  folderPath: string;
  exePath: string;
  savePath: string;
  vndbId: string;
  bangumiId: number;
  source: string;
  notes: string;
  lastPlayedAt: string | null;
  patches: Patch[] | null;
}

export interface GameInput {
  title: string;
  originalTitle: string;
  alias: string;
  poster: string;
  banner: string;
  developer: string;
  publisher: string;
  releaseDate: string;
  description: string;
  tags: string;
  genres: string;
  rating: number;
  playStatus: string;
  playtime: number;
  favorite: boolean;
  folderPath: string;
  exePath: string;
  savePath: string;
  vndbId: string;
  bangumiId: number;
  source: string;
  notes: string;
}

export interface GameFilter {
  query: string;
  status: string;
  developer: string;
  tag: string;
  sort: string;
  favorite: boolean;
}

export interface CountItem {
  name: string;
  count: number;
}

export interface TimelinePoint {
  year: number;
  total: number;
  cleared: number;
}

export interface Stats {
  total: number;
  favorite: number;
  rated: number;
  avgRating: number;
  totalPlaytime: number;
  clearedCount: number;
  byStatus: CountItem[] | null;
  byYear: CountItem[] | null;
  byDeveloper: CountItem[] | null;
  byPatchKind: CountItem[] | null;
  topTags: CountItem[] | null;
  timeline: TimelinePoint[] | null;
  missingFolder: number;
  noPoster: number;
  recent: Game[] | null;
}

export interface PatchInput {
  gameId: number;
  name: string;
  kind: string;
  version: string;
  source: string;
  sourceUrl: string;
  note: string;
  installed: boolean;
  relPaths: string[];
}

export interface PatchRow {
  patch: Patch;
  gameTitle: string;
  gamePoster: string;
}

export interface DetectResult {
  hadBaseline: boolean;
  changes: FileChange[] | null;
  total: number;
  truncated: boolean;
  scannedAt: number;
}

export interface UninstallResult {
  deleted: number;
  missing: number;
  failed: string[] | null;
}

export interface FileChange {
  relPath: string;
  op: string;
  size: number;
  modTime: number;
}

export interface DirEntry {
  relPath: string;
  name: string;
  isDir: boolean;
  size: number;
  modTime: number;
  inPatch: boolean;
}

export interface FolderStats {
  exists: boolean;
  files: number;
  size: number;
  truncated: boolean;
}

export interface SearchHit {
  source: string;
  id: string;
  title: string;
  titleCn: string;
  originalTitle: string;
  poster: string;
  released: string;
  developer: string;
  score: number;
  ratingCount: number;
  description: string;
  tags: string[] | null;
  nsfw: boolean;
}

export interface MetaDraft {
  source: string;
  vndbId: string;
  bangumiId: number;
  title: string;
  originalTitle: string;
  alias: string;
  poster: string;
  banner: string;
  developer: string;
  publisher: string;
  releaseDate: string;
  description: string;
  tags: string;
  genres: string;
  rating: number;
  lengthMinutes: number;
  nsfw: boolean;
  sourceUrl: string;
}

/** 与 Go 侧 config.DefaultConfig() 保持一致的兜底值。 */
export const DEFAULT_CONFIG: ConfigData = {
  preset: "sumi",
  accent: "",
  wallpaper: "",
  wallpaperBlur: 8,
  wallpaperVeil: 72,
  radius: 6,
  motion: true,
  density: "cozy",
  posterAspect: "2/3",
  sortMode: "updated",
  defaultSource: "vndb",
  vndbToken: "",
  bangumiToken: "",
  twoDFanUrl: "https://www.2dfan.com/",
} as ConfigData;

export function statusMeta(status?: string | null): StatusMeta {
  return STATUS_META[(status ?? "unplayed") as PlayStatus] ?? STATUS_META.unplayed;
}

export function kindMeta(kind?: string | null): KindMeta {
  return KIND_META[(kind ?? "other") as PatchKind] ?? KIND_META.other;
}
