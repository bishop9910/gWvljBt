<script setup lang="ts">
/**
 * 扫描器：选一个总目录 → 自动找出下面的 galgame 安装目录 → 勾选批量入库。
 *
 * 判定逻辑在后端（文件指纹，不看目录名），这里只负责展示与选择。
 */
import { computed, ref, watch } from "vue";
import { NButton, NIcon, NInput, NModal, NSelect, useMessage } from "naive-ui";
import {
  AlertCircleOutline,
  CheckmarkOutline,
  FolderOpenOutline,
  ScanOutline,
} from "@vicons/ionicons5";
import { api, errorText } from "../lib/api";
import { formatBytes } from "../lib/format";
import { configStore } from "../state/config";
import { refreshLibrary } from "../state/library";
import { closeScan, uiStore } from "../state/ui";
import type { ScanCandidate } from "../../bindings/gWvljBt/data/models";

const message = useMessage();

const root = ref("");
const depth = ref(4);
const query = ref("");
const scanning = ref(false);
const importing = ref(false);
const candidates = ref<ScanCandidate[]>([]);
const scanned = ref(false);
const elapsed = ref(0);
const truncated = ref(false);
const dirs = ref(0);
const picked = ref<Set<string>>(new Set());

// 元数据抓取：默认打开，跟着设置里的默认数据源走。
const source = ref(configStore.data.defaultSource || "vndb");
const autoFetch = ref(true);
const matching = ref(0);
const matchTotal = ref(0);
/** 用户改过的标题，key 是目录路径。没改过就用抓到的标题，再没有就用目录名。 */
const titles = ref<Record<string, string>>({});
/** 每次扫描/取消都 +1，旧的匹配循环看到 token 变了就自己退出。 */
let matchToken = 0;

const sourceOptions = [
  { label: "VNDB", value: "vndb" },
  { label: "Bangumi", value: "bangumi" },
];

const depthOptions = [
  { label: "2 层", value: 2 },
  { label: "3 层", value: 3 },
  { label: "4 层", value: 4 },
  { label: "5 层", value: 5 },
  { label: "6 层", value: 6 },
];

const visible = computed(() => {
  const keyword = query.value.trim().toLowerCase();
  const list = [...candidates.value].sort((a, b) => b.confidence - a.confidence);
  if (!keyword) return list;
  return list.filter(
    (item) =>
      item.name.toLowerCase().includes(keyword) || item.path.toLowerCase().includes(keyword),
  );
});

const selectable = computed(() => visible.value.filter((item) => !item.matchedId));
const pickedCount = computed(() => picked.value.size);
const allPicked = computed(
  () => selectable.value.length > 0 && selectable.value.every((item) => picked.value.has(item.path)),
);

watch(
  () => uiStore.scanOpen,
  (open) => {
    if (open) {
      // 每次打开都从干净状态开始，但保留上次的目录，方便连扫第二个盘。
      candidates.value = [];
      picked.value = new Set();
      scanned.value = false;
      query.value = "";
      titles.value = {};
      matchToken += 1;
      matchTotal.value = 0;
      matching.value = 0;
    }
  },
);

/** 这一行最终要用的标题：用户改过 > 抓到的 > 目录名清洗结果。 */
function titleOf(item: ScanCandidate): string {
  return titles.value[item.path] ?? item.meta?.title ?? item.name;
}

function onTitleInput(item: ScanCandidate, event: Event): void {
  const value = (event.target as HTMLInputElement).value;
  titles.value = { ...titles.value, [item.path]: value };
}

/**
 * 扫描完之后逐个去数据源抓基础信息。
 *
 * 串行是为了照顾 VNDB / Bangumi 的速率限制（一个 500ms 一个 1.2s），
 * 抓不到的行留空，标题继续用目录名，用户可以在列表里直接改。
 */
async function enrichAll(list: ScanCandidate[]): Promise<void> {
  const token = ++matchToken;
  const targets = list.filter((item) => !item.matchedId).slice(0, 80);
  if (!autoFetch.value || targets.length === 0) {
    matchTotal.value = 0;
    return;
  }

  matchTotal.value = targets.length;
  matching.value = 0;
  for (const item of targets) {
    if (token !== matchToken) break;
    matching.value += 1;
    try {
      const meta = await api.scan.lookup(item.name, source.value);
      if (token !== matchToken) break;
      if (meta?.title) {
        item.meta = meta;
        if (titles.value[item.path] === undefined) {
          titles.value = { ...titles.value, [item.path]: meta.title };
        }
      }
    } catch {
      // 单条失败（限流、超时）不影响后面的
    }
  }
  if (token === matchToken) {
    matchTotal.value = 0;
    matching.value = 0;
  }
}

function stopMatch(): void {
  matchToken += 1;
  matchTotal.value = 0;
  matching.value = 0;
}

function toggle(path: string): void {
  const next = new Set(picked.value);
  if (next.has(path)) next.delete(path);
  else next.add(path);
  picked.value = next;
}

function toggleAll(): void {
  if (allPicked.value) {
    picked.value = new Set();
    return;
  }
  picked.value = new Set(selectable.value.map((item) => item.path));
}

function preselect(list: ScanCandidate[]): void {
  picked.value = new Set(list.filter((item) => !item.matchedId).map((item) => item.path));
}

async function chooseFolder(): Promise<void> {
  try {
    const pickedRoot = await api.scan.pickRoot();
    if (!pickedRoot) return;
    root.value = pickedRoot;
    await runScan();
  } catch (err) {
    message.error(errorText(err));
  }
}

async function runScan(): Promise<void> {
  if (!root.value) {
    message.warning("先选一个要扫描的目录");
    return;
  }
  scanning.value = true;
  try {
    const result = await api.scan.scan(root.value, depth.value);
    candidates.value = result.candidates ?? [];
    elapsed.value = result.elapsedMs ?? 0;
    truncated.value = result.truncated ?? false;
    dirs.value = result.dirs ?? 0;
    scanned.value = true;
    titles.value = {};
    preselect(candidates.value);
    if (candidates.value.length === 0) {
      message.info("没找到像 galgame 的目录，试试选更上一层或者加大层级");
    } else {
      void enrichAll(candidates.value);
    }
  } catch (err) {
    message.error(errorText(err));
  } finally {
    scanning.value = false;
  }
}

async function importPicked(): Promise<void> {
  const items = candidates.value
    .filter((item) => picked.value.has(item.path))
    .map((item) => ({
      path: item.path,
      name: titleOf(item),
      exe: item.exe,
      engine: item.engine,
      // 抓到的元数据原样带上；没抓到的字段留空，后端会回退到目录名那条路。
      source: item.meta?.source ?? "",
      sourceId: item.meta?.sourceId ?? "",
      title: titleOf(item),
      originalTitle: item.meta?.originalTitle ?? "",
      poster: item.meta?.poster ?? "",
      banner: item.meta?.banner ?? "",
      developer: item.meta?.developer ?? "",
      publisher: item.meta?.publisher ?? "",
      releaseDate: item.meta?.released ?? "",
      description: item.meta?.description ?? "",
      tags: item.meta?.tags ?? "",
      genres: item.meta?.genres ?? "",
      rating: item.meta?.rating ?? 0,
    }));
  if (items.length === 0) return;

  importing.value = true;
  try {
    const result = await api.scan.import(items);
    const skipped = result.skipped ?? [];
    const failed = result.failed ?? [];

    if (result.created > 0) {
      message.success(`已收录 ${result.created} 部作品`);
      await refreshLibrary();
      // 导入成功的从列表里撤掉，避免重复勾选。
      const done = new Set(items.map((item) => item.path));
      candidates.value = candidates.value.filter(
        (item) => !(done.has(item.path) && !skipped.includes(item.name)),
      );
      for (const item of candidates.value) {
        if (done.has(item.path)) item.matchedId = item.matchedId || -1;
      }
      picked.value = new Set();
    }
    if (skipped.length > 0) {
      message.info(`${skipped.length} 个已经在库里，跳过了`);
    }
    if (failed.length > 0) {
      message.warning(`${failed.length} 个没能收录：${failed[0]}`);
    }
    if (result.created === 0 && skipped.length === 0 && failed.length === 0) {
      message.info("没有需要导入的目录");
    }
  } catch (err) {
    message.error(errorText(err));
  } finally {
    importing.value = false;
  }
}
</script>

<template>
  <n-modal
    :show="uiStore.scanOpen"
    :mask-closable="false"
    :auto-focus="false"
    transform-origin="center"
    @update:show="(value: boolean) => !value && closeScan()"
  >
    <div class="scan">
      <header class="scan__head">
        <div>
          <h2 class="scan__title">扫描游戏目录</h2>
          <p class="scan__note">
            指一个总目录，会自动找出下面像 galgame 的安装目录（看引擎文件和素材目录，不看名字）
          </p>
        </div>
        <n-button quaternary size="small" @click="closeScan">关闭</n-button>
      </header>

      <div class="scan__bar">
        <div class="root" :class="{ 'root--empty': !root }" :title="root">
          {{ root || "还没有选目录" }}
        </div>
        <n-button size="small" :loading="scanning" @click="chooseFolder">
          <template #icon>
            <n-icon :component="FolderOpenOutline" />
          </template>
          选目录
        </n-button>
        <n-select
          v-model:value="depth"
          :options="depthOptions"
          size="small"
          style="width: 96px"
        />
        <n-select
          v-model:value="source"
          :options="sourceOptions"
          size="small"
          style="width: 108px"
        />
        <n-button size="small" type="primary" :loading="scanning" @click="runScan">
          <template #icon>
            <n-icon :component="ScanOutline" />
          </template>
          扫描
        </n-button>
      </div>

      <div v-if="scanned" class="scan__meta">
        <span class="scan__count">
          扫了 {{ dirs }} 个目录，找到 <em>{{ candidates.length }}</em> 个候选
        </span>
        <span class="scan__dots">·</span>
        <span>用时 {{ elapsed }} ms</span>
        <span v-if="truncated" class="scan__warn">
          <n-icon :component="AlertCircleOutline" /> 目录太多，已提前收手
        </span>
        <span class="gv-spacer" />
        <n-input
          v-model:value="query"
          size="small"
          clearable
          placeholder="过滤名字或路径"
          style="width: 180px"
        />
      </div>

      <div v-if="candidates.length > 0" class="scan__tools">
        <button type="button" class="tick" :class="{ 'is-on': allPicked }" @click="toggleAll">
          <n-icon :size="12" :component="allPicked ? CheckmarkOutline : undefined" />
          <span>{{ allPicked ? "取消全选" : "全选未入库" }}</span>
        </button>
        <button type="button" class="tick" :class="{ 'is-on': autoFetch }" @click="autoFetch = !autoFetch">
          <n-icon :size="12" :component="autoFetch ? CheckmarkOutline : undefined" />
          <span>自动抓取元数据</span>
        </button>
        <span class="scan__picked">已选 {{ pickedCount }} 个</span>
        <span class="gv-spacer" />
        <span v-if="matchTotal > 0" class="scan__match">
          匹配中 {{ matching }}/{{ matchTotal }}
          <button type="button" class="link" @click="stopMatch">停止</button>
        </span>
      </div>

      <div v-if="candidates.length > 0" class="list gv-scroll">
        <div
          v-for="item in visible"
          :key="item.path"
          class="row"
          role="button"
          tabindex="0"
          :class="{ 'is-picked': picked.has(item.path), 'is-known': !!item.matchedId }"
          @click="!item.matchedId && toggle(item.path)"
          @keydown.enter.prevent="!item.matchedId && toggle(item.path)"
          @keydown.space.prevent="!item.matchedId && toggle(item.path)"
        >
          <span class="row__box">
            <n-icon v-if="item.matchedId" :size="12" :component="CheckmarkOutline" />
            <n-icon v-else-if="picked.has(item.path)" :size="12" :component="CheckmarkOutline" />
          </span>
          <span v-if="item.meta?.poster" class="row__thumb">
            <img :src="item.meta.poster" alt="" loading="lazy" />
          </span>
          <span class="row__body">
            <input
              class="row__name"
              :value="titleOf(item)"
              :disabled="!!item.matchedId"
              spellcheck="false"
              @click.stop
              @input="onTitleInput(item, $event)"
            />
            <span class="row__path">{{ item.path }}</span>
          </span>
          <span class="row__tags">
            <span v-if="item.meta?.source" class="tag tag--meta">
              {{ item.meta.source === "bangumi" ? "Bangumi" : "VNDB" }}
              <em>{{ Math.round((item.meta.match ?? 0) * 100) }}%</em>
            </span>
            <span v-else-if="autoFetch && matchTotal > 0" class="tag tag--quiet">匹配中…</span>
            <span v-if="item.engine" class="tag">{{ item.engine }}</span>
            <span class="tag tag--quiet">{{ item.files }} 文件</span>
            <span class="tag tag--quiet">{{ formatBytes(item.size) }}</span>
            <span v-if="item.matchedId" class="tag tag--known">已在库</span>
            <span class="conf" :title="`置信度 ${item.confidence}`">{{ item.confidence }}</span>
          </span>
        </div>
      </div>

      <div v-else-if="scanned && !scanning" class="empty">
        <p class="empty__title">这里没有找到 galgame</p>
        <p class="empty__note">
          试试把层级加大一点，或者选到真正的游戏根目录。判定看的是
          <code>.xp3 / .ypf / .pfs / .npa / .rpa / nscript.dat</code> 这类引擎文件。
        </p>
      </div>

      <div v-else-if="!scanning" class="empty">
        <p class="empty__title">还没有开始扫描</p>
        <p class="empty__note">
          选一个装了游戏的盘符或文件夹。扫描只看文件名与目录结构，不会改动任何文件。
        </p>
      </div>

      <footer class="scan__foot">
        <span class="hint">导入后可以再逐个补封面和信息</span>
        <n-button quaternary size="small" @click="closeScan">取消</n-button>
        <n-button
          type="primary"
          size="small"
          :disabled="pickedCount === 0"
          :loading="importing"
          @click="importPicked"
        >
          导入选中（{{ pickedCount }}）
        </n-button>
      </footer>
    </div>
  </n-modal>
</template>

<style scoped>
.scan {
  display: flex;
  flex-direction: column;
  width: min(880px, 92vw);
  max-height: 82vh;
  padding: 20px 22px 16px;
  background: var(--gv-surface);
  border: 1px solid var(--gv-border-strong);
  border-radius: var(--gv-radius);
  box-shadow: var(--gv-shadow-2);
}

.scan__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--gv-border);
}

.scan__title {
  margin: 0;
  font-family: var(--gv-font-display);
  font-size: var(--gv-fs-subhead);
  font-weight: 600;
  letter-spacing: var(--gv-track-display);
}

.scan__note {
  margin: 4px 0 0;
  max-width: 560px;
  font-size: var(--gv-fs-caption);
  line-height: 1.6;
  color: var(--gv-text-faint);
}

.scan__bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 0;
}

.root {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  padding: 5px 9px;
  border: 1px solid var(--gv-border);
  font-family: var(--gv-mono);
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-dim);
}

.root--empty {
  color: var(--gv-text-faint);
}

.scan__meta {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-bottom: 10px;
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-faint);
}

.scan__count em {
  font-style: normal;
  font-family: var(--gv-mono);
  font-size: var(--gv-fs-label);
  color: var(--gv-accent);
}

.scan__dots {
  color: var(--gv-border-strong);
}

.scan__warn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--gv-warn);
}

.scan__tools {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 6px 0 8px;
  border-top: 1px solid var(--gv-border);
}

.tick {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 8px;
  border: 1px solid var(--gv-border);
  background: transparent;
  color: var(--gv-text-dim);
  font-family: inherit;
  font-size: var(--gv-fs-caption);
  cursor: pointer;
}

.tick:hover {
  border-color: var(--gv-border-strong);
  color: var(--gv-text);
}

.tick.is-on {
  border-color: var(--gv-accent-line);
  color: var(--gv-accent);
}

.scan__picked {
  font-family: var(--gv-mono);
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-faint);
}

.list {
  flex: 1 1 auto;
  min-height: 120px;
  overflow-y: auto;
  border-top: 1px solid var(--gv-border);
}

.row {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 9px 4px;
  border: 0;
  border-bottom: 1px solid var(--gv-border);
  background: transparent;
  text-align: left;
  cursor: pointer;
  transition: background 0.14s var(--gv-ease);
}

.row:hover {
  background: var(--gv-surface-2);
}

.row.is-picked {
  background: var(--gv-accent-soft);
}

.row.is-known {
  cursor: default;
  opacity: 0.62;
}

.row__box {
  flex: 0 0 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 14px;
  height: 14px;
  border: 1px solid var(--gv-border-strong);
  color: var(--gv-accent);
}

.row.is-picked .row__box {
  border-color: var(--gv-accent);
}

.row.is-known .row__box {
  border-style: dashed;
  color: var(--gv-text-faint);
}

.row__body {
  flex: 1 1 auto;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

/* 缩略图：抓不到封面时这一格直接不渲染 */
.row__thumb {
  flex: 0 0 auto;
  width: 30px;
  height: 40px;
  border: 1px solid var(--gv-border);
  overflow: hidden;
}

.row__thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.row__name {
  width: 100%;
  padding: 0;
  border: 0;
  border-bottom: 1px solid transparent;
  background: transparent;
  font-family: var(--gv-font-display);
  font-size: var(--gv-fs-title);
  font-weight: 600;
  color: var(--gv-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: text;
  transition: border-color 0.16s var(--gv-ease), color 0.16s var(--gv-ease);
}

.row__name:hover:not(:disabled) {
  border-bottom-color: var(--gv-border-strong);
}

.row__name:focus {
  outline: none;
  border-bottom-color: var(--gv-accent);
}

.row__name:disabled {
  cursor: default;
  color: var(--gv-text-dim);
}

.row__path {
  font-family: var(--gv-mono);
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-faint);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.row__tags {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  gap: 6px;
}

.tag {
  padding: 1px 6px;
  border: 1px solid var(--gv-border);
  font-family: var(--gv-mono);
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-dim);
  white-space: nowrap;
}

.tag--quiet {
  color: var(--gv-text-faint);
}

.tag--known {
  border-color: var(--gv-accent-line);
  color: var(--gv-accent);
}

.tag--meta {
  border-color: var(--gv-accent-line);
  color: var(--gv-accent);
  font-family: var(--gv-font);
  letter-spacing: 0.01em;
}

.tag--meta em {
  margin-left: 4px;
  font-style: normal;
  font-family: var(--gv-mono);
  font-variant-numeric: tabular-nums;
  letter-spacing: 0;
  color: var(--gv-text-faint);
}

.scan__match {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-family: var(--gv-mono);
  font-size: var(--gv-fs-caption);
  font-variant-numeric: tabular-nums;
  color: var(--gv-text-faint);
}

.link {
  padding: 0;
  border: 0;
  border-bottom: 1px solid var(--gv-border-strong);
  background: transparent;
  font-family: var(--gv-font);
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-dim);
  cursor: pointer;
}

.link:hover {
  color: var(--gv-accent);
  border-bottom-color: var(--gv-accent-line);
}

.conf {
  width: 24px;
  text-align: right;
  font-family: var(--gv-mono);
  font-size: var(--gv-fs-caption);
  font-variant-numeric: tabular-nums;
  letter-spacing: 0;
  color: var(--gv-text-faint);
}

.empty {
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 36px 20px;
  border-top: 1px solid var(--gv-border);
}

.empty__title {
  margin: 0;
  font-family: var(--gv-font-display);
  font-size: var(--gv-fs-title);
  color: var(--gv-text);
}

.empty__note {
  margin: 0;
  max-width: 460px;
  text-align: center;
  font-size: var(--gv-fs-caption);
  line-height: 1.7;
  color: var(--gv-text-faint);
}

.empty__note code {
  font-family: var(--gv-mono);
  color: var(--gv-text-dim);
}

.scan__foot {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-top: 12px;
  border-top: 1px solid var(--gv-border);
}

.hint {
  flex: 1 1 auto;
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-faint);
}
</style>
