<script setup lang="ts">
/**
 * 补丁总览：把所有作品登记的补丁拉到一张表上，方便集中清理。
 */
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { NButton, NEmpty, NIcon, NInput, NPopconfirm, NSpin, NTooltip, useMessage } from "naive-ui";
import {
  CloudDownloadOutline,
  ExtensionPuzzleOutline,
  RefreshOutline,
  SearchOutline,
  TrashOutline,
} from "@vicons/ionicons5";
import PosterImage from "../components/PosterImage.vue";
import { api, errorText, onPatchesChanged } from "../lib/api";
import { PATCH_KINDS, kindMeta, type PatchRow } from "../lib/types";
import { configStore } from "../state/config";
import { openPatchEditor } from "../state/ui";

const router = useRouter();
const message = useMessage();

const rows = ref<PatchRow[]>([]);
const loading = ref(true);
const error = ref("");
const keyword = ref("");
const kindFilter = ref("");

async function load(): Promise<void> {
  loading.value = true;
  error.value = "";
  try {
    rows.value = (await api.patches.all()) ?? [];
  } catch (err) {
    error.value = errorText(err);
    rows.value = [];
  } finally {
    loading.value = false;
  }
}

const filtered = computed(() => {
  const query = keyword.value.trim().toLowerCase();
  return rows.value.filter((row) => {
    if (kindFilter.value && row.patch.kind !== kindFilter.value) return false;
    if (!query) return true;
    return (
      (row.patch.name || "").toLowerCase().includes(query) ||
      (row.gameTitle || "").toLowerCase().includes(query) ||
      (row.patch.source || "").toLowerCase().includes(query)
    );
  });
});

const kindCounts = computed(() => {
  const counts = new Map<string, number>();
  for (const row of rows.value) {
    const key = row.patch.kind || "other";
    counts.set(key, (counts.get(key) ?? 0) + 1);
  }
  return counts;
});

const totalFiles = computed(() =>
  rows.value.reduce((sum, row) => sum + fileCount(row), 0),
);

const installedCount = computed(() => rows.value.filter((row) => row.patch.installed).length);

function fileCount(row: PatchRow): number {
  return row.patch.files?.length ?? 0;
}

async function uninstall(row: PatchRow): Promise<void> {
  try {
    const result = await api.patches.uninstall(row.patch.id, true);
    const failed = result.failed ?? [];
    const parts = [`删除 ${result.deleted} 个文件`];
    if (result.missing > 0) parts.push(`${result.missing} 个已不存在`);
    if (failed.length > 0) parts.push(`${failed.length} 个失败`);
    message.success(parts.join("，"));
    await load();
  } catch (err) {
    message.error(errorText(err));
  }
}

async function removeRecord(row: PatchRow): Promise<void> {
  try {
    await api.patches.remove(row.patch.id);
    message.success("已删除补丁记录");
    await load();
  } catch (err) {
    message.error(errorText(err));
  }
}

async function verify(row: PatchRow): Promise<void> {
  try {
    const missing = await api.patches.verify(row.patch.id);
    if (!missing || missing.length === 0) {
      message.success("清单里的文件都还在");
      return;
    }
    message.warning(`有 ${missing.length} 个文件已不在磁盘上：${missing.slice(0, 3).join("、")}${missing.length > 3 ? " …" : ""}`);
  } catch (err) {
    message.error(errorText(err));
  }
}

function openGame(row: PatchRow): void {
  router.push(`/game/${row.patch.gameId}`);
}

function openTwoDFan(): void {
  void api.games.openUrl(configStore.data.twoDFanUrl || "https://www.2dfan.com/");
}

onMounted(load);
onPatchesChanged(() => {
  void load();
});
</script>

<template>
  <div class="patches-view gv-scroll">
    <div class="summary">
      <div class="stat">
        <strong>{{ rows.length }}</strong>
        <span>个补丁</span>
      </div>
      <div class="stat">
        <strong>{{ installedCount }}</strong>
        <span>已安装</span>
      </div>
      <div class="stat">
        <strong>{{ totalFiles }}</strong>
        <span>个登记文件</span>
      </div>
      <div class="gv-spacer" />
      <n-button quaternary @click="openTwoDFan">
        <template #icon>
          <n-icon :component="CloudDownloadOutline" />
        </template>
        补丁下载去 2DFan
      </n-button>
      <n-button quaternary :loading="loading" @click="load">
        <template #icon>
          <n-icon :component="RefreshOutline" />
        </template>
        刷新
      </n-button>
    </div>

    <div class="toolbar">
      <div class="chips">
        <button
          type="button"
          class="chip"
          :class="{ 'is-active': !kindFilter }"
          @click="kindFilter = ''"
        >
          全部 <em>{{ rows.length }}</em>
        </button>
        <button
          v-for="kind in PATCH_KINDS"
          :key="kind"
          type="button"
          class="chip"
          :class="{ 'is-active': kindFilter === kind }"
          :style="{ '--dot': kindMeta(kind).color }"
          @click="kindFilter = kind"
        >
          <span class="dot" />
          {{ kindMeta(kind).label }}
          <em>{{ kindCounts.get(kind) ?? 0 }}</em>
        </button>
      </div>

      <n-input v-model:value="keyword" placeholder="搜索补丁名 / 作品 / 来源" clearable size="small">
        <template #prefix>
          <n-icon :component="SearchOutline" />
        </template>
      </n-input>
    </div>

    <n-spin :show="loading">
      <div v-if="filtered.length > 0" class="list">
        <article v-for="row in filtered" :key="row.patch.id" class="row">
          <PosterImage
            class="row__poster"
            :src="row.gamePoster"
            :title="row.gameTitle"
            rounded="8px"
          />

          <div class="row__main">
            <div class="row__title">
              <strong>{{ row.patch.name }}</strong>
              <span class="kind" :style="{ '--kind': kindMeta(row.patch.kind).color }">
                {{ kindMeta(row.patch.kind).label }}
              </span>
              <span v-if="row.patch.version" class="gv-chip">{{ row.patch.version }}</span>
              <span class="state" :class="{ 'is-on': row.patch.installed }">
                {{ row.patch.installed ? "已安装" : "未安装" }}
              </span>
            </div>

            <div class="row__meta">
              <button type="button" class="game-link" @click="openGame(row)">
                {{ row.gameTitle || "未知作品" }}
              </button>
              <span>· {{ fileCount(row) }} 个文件</span>
              <span v-if="row.patch.source">· {{ row.patch.source }}</span>
              <span v-if="row.patch.note">· {{ row.patch.note }}</span>
            </div>
          </div>

          <div class="row__actions">
            <n-tooltip trigger="hover">
              <template #trigger>
                <n-button size="tiny" quaternary @click="verify(row)">校验</n-button>
              </template>
              检查文件是否还在
            </n-tooltip>
            <n-button size="tiny" quaternary @click="openPatchEditor({ patchId: row.patch.id })">
              编辑
            </n-button>
            <n-popconfirm @positive-click="uninstall(row)">
              <template #trigger>
                <n-button size="tiny" quaternary type="warning">卸载</n-button>
              </template>
              会把 {{ fileCount(row) }} 个文件从磁盘删掉，确认？
            </n-popconfirm>
            <n-popconfirm @positive-click="removeRecord(row)">
              <template #trigger>
                <n-button size="tiny" quaternary type="error">
                  <template #icon>
                    <n-icon :component="TrashOutline" />
                  </template>
                </n-button>
              </template>
              只删记录，不碰磁盘
            </n-popconfirm>
          </div>
        </article>
      </div>

      <div v-else-if="!loading" class="empty">
        <n-empty :description="keyword || kindFilter ? '没有匹配的补丁' : '还没有登记任何补丁'">
          <template #extra>
            <p class="empty__hint">
              去作品详情页登记：装补丁前先建基线，装完点「检测变化」，它改了哪些文件一目了然。
            </p>
          </template>
        </n-empty>
      </div>
    </n-spin>

    <p v-if="error" class="error">{{ error }}</p>

    <div class="tip">
      <n-icon :size="16" :component="ExtensionPuzzleOutline" />
      <span>
        本工具只负责记录和清理，不提供补丁文件本身。汉化、解锁补丁请到
        <button type="button" class="link" @click="openTwoDFan">2DFan</button>
        等站点获取。
      </span>
    </div>
  </div>
</template>

<style scoped>
.patches-view {
  height: 100%;
  overflow: auto;
  padding: 20px 28px 40px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.summary {
  display: flex;
  align-items: center;
  gap: 22px;
}

.stat {
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.stat strong {
  font-family: var(--gv-font-display);
  font-size: 22px;
  font-weight: 700;
}

.stat span {
  font-size: var(--gv-fs-label);
  color: var(--gv-text-faint);
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.toolbar .n-input {
  width: 260px;
  margin-left: auto;
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 9px;
  border: 1px solid var(--gv-border);
  border-radius: 2px;
  background: transparent;
  color: var(--gv-text-dim);
  font-size: var(--gv-fs-label);
  cursor: pointer;
  transition: color 0.2s var(--gv-ease), border-color 0.2s var(--gv-ease), background 0.2s var(--gv-ease);
}

.chip:hover {
  border-color: var(--gv-border-strong);
  color: var(--gv-text);
}

.chip.is-active {
  border-color: var(--gv-accent-line);
  background: var(--gv-accent-soft);
  color: var(--gv-accent);
}

.chip .dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--dot, var(--gv-text-faint));
}

.chip em {
  font-style: normal;
  font-family: var(--gv-mono);
  font-size: var(--gv-fs-caption);
  opacity: 0.7;
}

.list {
  display: flex;
  flex-direction: column;
  border-top: 1px solid var(--gv-border);
}

.row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 11px 2px;
  border-bottom: 1px solid var(--gv-border);
  transition: background 0.2s var(--gv-ease);
}

.row:hover {
  background: var(--gv-surface-2);
}

.row__poster {
  flex: 0 0 auto;
  width: 40px;
}

.row__main {
  flex: 1 1 auto;
  min-width: 0;
}

.row__title {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.row__title strong {
  font-size: var(--gv-fs-body);
  font-weight: 600;
}

.kind {
  padding: 1px 6px;
  border-radius: 2px;
  font-size: var(--gv-fs-caption);
  font-weight: 600;
  color: var(--kind);
  background: color-mix(in srgb, var(--kind) 16%, transparent);
}

.state {
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-faint);
}

.state.is-on {
  color: var(--gv-success);
}

.row__meta {
  display: flex;
  gap: 5px;
  flex-wrap: wrap;
  margin-top: 2px;
  font-size: var(--gv-fs-label);
  color: var(--gv-text-faint);
  overflow: hidden;
}

.game-link {
  border: none;
  background: transparent;
  padding: 0;
  color: var(--gv-accent);
  font-size: var(--gv-fs-label);
  cursor: pointer;
}

.game-link:hover {
  text-decoration: underline;
}

.row__actions {
  display: flex;
  align-items: center;
  gap: 1px;
  flex: 0 0 auto;
}

.empty {
  padding: 50px 0;
}

.empty__hint {
  margin: 0;
  max-width: 380px;
  font-size: var(--gv-fs-label);
  line-height: 1.65;
  color: var(--gv-text-faint);
}

.error {
  margin: 0;
  font-size: var(--gv-fs-label);
  color: var(--gv-danger);
}

.tip {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: auto;
  padding: 10px 14px;
  border: 1px solid var(--gv-border);
  border-radius: var(--gv-radius-sm);
  background: var(--gv-surface-2);
  font-size: var(--gv-fs-label);
  color: var(--gv-text-dim);
}

.link {
  border: none;
  background: transparent;
  padding: 0;
  color: var(--gv-accent);
  font-size: var(--gv-fs-label);
  cursor: pointer;
}

.link:hover {
  text-decoration: underline;
}
</style>
