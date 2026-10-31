<script setup lang="ts">
/**
 * 补丁编辑器：登记一个补丁装了哪些文件，方便以后一键卸掉。
 * 文件的来源有两条路：
 *   1. 快照对比 —— 装补丁前先建基线，装完点「检测变化」，自动列出新增/改动的文件；
 *   2. 手动浏览 —— 直接翻游戏目录勾选。
 */
import { computed, reactive, ref, watch } from "vue";
import {
  NButton,
  NCheckbox,
  NDrawer,
  NDrawerContent,
  NIcon,
  NInput,
  NPopconfirm,
  NSelect,
  NSpin,
  NSwitch,
  NTooltip,
  useMessage,
} from "naive-ui";
import {
  ArrowBackOutline,
  ArrowUpOutline,
  CheckmarkOutline,
  CloseOutline,
  CloudDownloadOutline,
  DocumentOutline,
  FolderOpenOutline,
  RefreshOutline,
  ScanOutline,
  TrashOutline,
} from "@vicons/ionicons5";
import { api, errorText } from "../lib/api";
import { formatBytes } from "../lib/format";
import {
  PATCH_KINDS,
  type DirEntry,
  type DetectResult,
  type FileChange,
  type PatchInput,
} from "../lib/types";
import { configStore } from "../state/config";
import { libraryStore } from "../state/library";
import { closePatchEditor, uiStore } from "../state/ui";

const message = useMessage();

function blankInput(): PatchInput {
  return {
    gameId: 0,
    name: "",
    kind: "translation",
    version: "",
    source: "",
    sourceUrl: "",
    note: "",
    installed: true,
    relPaths: [],
  };
}

const form = reactive<PatchInput>(blankInput());
const selected = ref<string[]>([]);
const loading = ref(false);
const saving = ref(false);
const detecting = ref(false);
const detail = ref<DetectResult | null>(null);

const isEdit = computed(() => uiStore.patchEditorId > 0);
const drawerTitle = computed(() => (isEdit.value ? "编辑补丁" : "登记补丁"));

const kindOptions = PATCH_KINDS.map((kind) => ({ label: kindLabel(kind), value: kind }));
const gameOptions = computed(() =>
  libraryStore.games.map((game) => ({ label: game.title, value: game.id })),
);
const currentGame = computed(() =>
  libraryStore.games.find((game) => game.id === Number(form.gameId)),
);

function kindLabel(kind: string): string {
  const map: Record<string, string> = {
    translation: "汉化补丁",
    unlock: "解锁补丁",
    fix: "修复补丁",
    mod: "MOD",
    bypass: "破解 / 免认证",
    other: "其他",
  };
  return map[kind] ?? kind;
}

/* ------------------------------ 文件选择 ------------------------------ */

const selectedSet = computed(() => new Set(selected.value));

function toggle(relPath: string, checked: boolean): void {
  if (checked) {
    if (!selectedSet.value.has(relPath)) selected.value = [...selected.value, relPath];
  } else {
    selected.value = selected.value.filter((item) => item !== relPath);
  }
}

/** 已勾选但本次检测里没出现的（手工选的），也要列出来 */
const manualOnly = computed(() => {
  const changed = new Set((detail.value?.changes ?? []).map((item) => item.relPath));
  return selected.value.filter((relPath) => !changed.has(relPath));
});

const changes = computed<FileChange[]>(() => detail.value?.changes ?? []);

async function detect(): Promise<void> {
  if (!form.gameId) {
    message.warning("先选一个作品");
    return;
  }
  detecting.value = true;
  try {
    const result = await api.patches.detect(Number(form.gameId));
    detail.value = result;
    if (!result.hadBaseline) {
      message.info("已为这个目录建立基线快照。装好补丁后再点一次检测，就能看到变化了。");
      return;
    }
    const list = result.changes ?? [];
    const fresh = list.map((item) => item.relPath);
    const merged = Array.from(new Set([...selected.value, ...fresh]));
    selected.value = merged;
    if (result.truncated) {
      message.warning(`文件太多，只列出前 ${list.length} 个变化`);
    } else if (list.length === 0) {
      message.info("和基线一致，没检测到变化");
    }
  } catch (err) {
    message.error(errorText(err));
  } finally {
    detecting.value = false;
  }
}

async function rebuildBaseline(): Promise<void> {
  if (!form.gameId) return;
  try {
    const count = await api.patches.rebuild(Number(form.gameId));
    message.success(`基线已重建（${count} 个文件）`);
    detail.value = null;
  } catch (err) {
    message.error(errorText(err));
  }
}

/* 手动浏览 */
const browseDir = ref("");
const entries = ref<DirEntry[]>([]);
const browsing = ref(false);

async function browse(relDir: string): Promise<void> {
  if (!form.gameId) {
    message.warning("先选一个作品");
    return;
  }
  browsing.value = true;
  try {
    entries.value = await api.patches.browse(Number(form.gameId), relDir);
    browseDir.value = relDir;
  } catch (err) {
    message.error(errorText(err));
    entries.value = [];
  } finally {
    browsing.value = false;
  }
}

function joinRel(base: string, name: string): string {
  const trimmed = base.replace(/\\/g, "/").replace(/\/+$/, "");
  return trimmed ? `${trimmed}/${name}` : name;
}

function parentOf(rel: string): string {
  const parts = rel.replace(/\\/g, "/").split("/").filter(Boolean);
  parts.pop();
  return parts.join("/");
}

async function toggleAll(checked: boolean): Promise<void> {
  if (checked) {
    selected.value = Array.from(new Set([...selected.value, ...changes.value.map((c) => c.relPath)]));
  } else {
    const changed = new Set(changes.value.map((c) => c.relPath));
    selected.value = selected.value.filter((relPath) => !changed.has(relPath));
  }
}

/* -------------------------------- 读写 -------------------------------- */

async function loadPatch(id: number): Promise<void> {
  loading.value = true;
  try {
    const patch = await api.patches.get(id);
    Object.assign(form, {
      gameId: Number(patch.gameId),
      name: patch.name ?? "",
      kind: patch.kind || "translation",
      version: patch.version ?? "",
      source: patch.source ?? "",
      sourceUrl: patch.sourceUrl ?? "",
      note: patch.note ?? "",
      installed: !!patch.installed,
      relPaths: [],
    } satisfies PatchInput);
    selected.value = (patch.files ?? []).map((file) => file.relPath);
    await api.patches.hasSnapshot(Number(patch.gameId));
    browseDir.value = "";
    entries.value = [];
    detail.value = null;
  } catch (err) {
    message.error(errorText(err));
  } finally {
    loading.value = false;
  }
}

async function save(): Promise<void> {
  if (!form.gameId) {
    message.warning("先选一个作品");
    return;
  }
  if (!form.name.trim()) {
    message.warning("补丁得有个名字");
    return;
  }
  saving.value = true;
  try {
    const payload: PatchInput = {
      ...form,
      gameId: Number(form.gameId),
      name: form.name.trim(),
      relPaths: selected.value,
    };
    if (isEdit.value) {
      await api.patches.update(uiStore.patchEditorId, payload);
      message.success("已保存");
    } else {
      await api.patches.create(payload);
      message.success(`已登记「${payload.name}」，共 ${selected.value.length} 个文件`);
    }
    closePatchEditor();
  } catch (err) {
    message.error(errorText(err));
  } finally {
    saving.value = false;
  }
}

async function uninstall(deleteFiles: boolean): Promise<void> {
  if (!isEdit.value) return;
  try {
    const result = await api.patches.uninstall(uiStore.patchEditorId, deleteFiles);
    const failed = result.failed ?? [];
    const parts = [`删除 ${result.deleted} 个文件`];
    if (result.missing > 0) parts.push(`${result.missing} 个已不存在`);
    if (failed.length > 0) parts.push(`${failed.length} 个失败`);
    message.success(parts.join("，"));
    closePatchEditor();
  } catch (err) {
    message.error(errorText(err));
  }
}

async function removeRecord(): Promise<void> {
  if (!isEdit.value) return;
  try {
    await api.patches.remove(uiStore.patchEditorId);
    message.success("已删除补丁记录");
    closePatchEditor();
  } catch (err) {
    message.error(errorText(err));
  }
}

async function openTwoDFan(): Promise<void> {
  try {
    await api.games.openUrl(configStore.data.twoDFanUrl || "https://www.2dfan.com/");
  } catch (err) {
    message.error(errorText(err));
  }
}

async function openGameFolder(): Promise<void> {
  const game = currentGame.value;
  if (!game?.folderPath) {
    message.warning("这个作品还没设置安装目录");
    return;
  }
  try {
    await api.games.openPath(game.folderPath);
  } catch (err) {
    message.error(errorText(err));
  }
}

watch(
  () => uiStore.patchEditorOpen,
  async (open) => {
    if (!open) return;
    Object.assign(form, blankInput());
    selected.value = [];
    entries.value = [];
    browseDir.value = "";
    detail.value = null;
    if (uiStore.patchEditorId > 0) {
      await loadPatch(uiStore.patchEditorId);
    } else {
      form.gameId = uiStore.patchEditorGameId || libraryStore.games[0]?.id || 0;
      loading.value = false;
    }
  },
  { immediate: true },
);

const totalSelectedSize = computed(() => {
  const sizes = new Map(changes.value.map((change) => [change.relPath, change.size]));
  return selected.value.reduce((sum, relPath) => sum + (sizes.get(relPath) ?? 0), 0);
});
</script>

<template>
  <n-drawer
    v-model:show="uiStore.patchEditorOpen"
    :width="720"
    placement="right"
    :mask-closable="!saving"
    @after-leave="closePatchEditor"
  >
    <n-drawer-content :title="drawerTitle" closable :native-scrollbar="false">
      <n-spin :show="loading">
        <div class="editor">
          <!-- 基本信息 -->
          <section class="block">
            <div class="block__head">
              <h4>基本信息</h4>
              <span v-if="currentGame" class="gv-faint">{{ currentGame.title }}</span>
            </div>

            <div class="grid">
              <label class="field span-2">
                <span class="field__label">所属作品 <i>*</i></span>
                <n-select
                  v-model:value="form.gameId"
                  :options="gameOptions"
                  filterable
                  placeholder="选择作品"
                  :disabled="isEdit"
                />
              </label>

              <label class="field">
                <span class="field__label">补丁名称 <i>*</i></span>
                <n-input v-model:value="form.name" placeholder="如：简体中文汉化补丁 v1.2" />
              </label>

              <label class="field">
                <span class="field__label">类型</span>
                <n-select v-model:value="form.kind" :options="kindOptions" />
              </label>

              <label class="field">
                <span class="field__label">版本</span>
                <n-input v-model:value="form.version" placeholder="v1.0" />
              </label>

              <label class="field">
                <span class="field__label">来源</span>
                <n-input v-model:value="form.source" placeholder="汉化组 / 发布者" />
              </label>

              <label class="field span-2">
                <span class="field__label">来源链接</span>
                <n-input v-model:value="form.sourceUrl" placeholder="发布页地址（可选）" />
              </label>

              <label class="field span-2">
                <span class="field__label">备注</span>
                <n-input
                  v-model:value="form.note"
                  type="textarea"
                  :autosize="{ minRows: 2, maxRows: 5 }"
                  placeholder="安装方式、注意事项…"
                />
              </label>

              <label class="field">
                <span class="field__label">当前已安装</span>
                <div class="inline">
                  <n-switch v-model:value="form.installed" />
                  <span class="gv-faint">卸载后会标记为未安装</span>
                </div>
              </label>
            </div>
          </section>

          <!-- 快照检测 -->
          <section class="block block--detect">
            <div class="block__head">
              <h4>
                <n-icon :size="15" :component="ScanOutline" />
                快照对比
              </h4>
              <span class="gv-faint">先在干净状态下建基线，装完补丁再检测</span>
            </div>

            <div class="detect-bar">
              <n-button type="primary" :loading="detecting" :disabled="!form.gameId" @click="detect">
                <template #icon>
                  <n-icon :component="ScanOutline" />
                </template>
                检测变化
              </n-button>
              <n-button :disabled="!form.gameId" @click="rebuildBaseline">
                <template #icon>
                  <n-icon :component="RefreshOutline" />
                </template>
                重建基线
              </n-button>
              <n-button :disabled="!currentGame?.folderPath" @click="openGameFolder">
                <template #icon>
                  <n-icon :component="FolderOpenOutline" />
                </template>
                打开目录
              </n-button>
            </div>

            <div v-if="detail" class="detect-summary">
              <template v-if="!detail.hadBaseline">
                <n-icon :size="15" :component="CheckmarkOutline" />
                基线已建立，装着补丁进游戏跑一圈，回来再点一次「检测变化」。
              </template>
              <template v-else>
                共 {{ detail.total }} 个文件，
                <b>{{ changes.length }}</b> 个变化
                <span v-if="detail.truncated" class="gv-warn">（已截断）</span>
                <span v-if="changes.length > 0" class="gv-faint">
                  · 已选 {{ selected.length }} 项 / 约 {{ formatBytes(totalSelectedSize) }}
                </span>
              </template>
            </div>

            <div v-if="changes.length > 0" class="changes">
              <div class="changes__head">
                <n-checkbox
                  :checked="changes.every((c) => selectedSet.has(c.relPath))"
                  @update:checked="toggleAll"
                >
                  全选
                </n-checkbox>
              </div>
              <label v-for="change in changes" :key="change.relPath" class="file-row">
                <n-checkbox
                  :checked="selectedSet.has(change.relPath)"
                  @update:checked="(checked: boolean) => toggle(change.relPath, checked)"
                />
                <span class="file-row__op" :class="`is-${change.op}`">
                  {{ change.op === "added" ? "新增" : "改动" }}
                </span>
                <span class="file-row__path gv-mono" :title="change.relPath">
                  {{ change.relPath }}
                </span>
                <span class="file-row__size">{{ formatBytes(change.size) }}</span>
              </label>
            </div>
          </section>

          <!-- 手动浏览 -->
          <section class="block">
            <div class="block__head">
              <h4>
                <n-icon :size="15" :component="DocumentOutline" />
                手动挑选文件
              </h4>
              <span class="gv-faint">快照没覆盖到的，可以在这里直接勾</span>
            </div>

            <div class="detect-bar">
              <n-button :disabled="!form.gameId" @click="browse(browseDir)">
                <template #icon>
                  <n-icon :component="FolderOpenOutline" />
                </template>
                浏览目录
              </n-button>
              <n-button v-if="browseDir" quaternary @click="browse(parentOf(browseDir))">
                <template #icon>
                  <n-icon :component="ArrowUpOutline" />
                </template>
                上一级
              </n-button>
              <span v-if="browseDir" class="gv-faint gv-mono">/{{ browseDir }}</span>
              <span v-else class="gv-faint gv-mono">/</span>
            </div>

            <n-spin :show="browsing">
              <div v-if="entries.length > 0" class="browser">
                <button
                  v-for="entry in entries"
                  :key="entry.relPath"
                  type="button"
                  class="file-row file-row--browse"
                  @click="entry.isDir ? browse(entry.relPath) : toggle(entry.relPath, !selectedSet.has(entry.relPath))"
                >
                  <n-checkbox
                    v-if="!entry.isDir"
                    :checked="selectedSet.has(entry.relPath)"
                    @update:checked="(checked: boolean) => toggle(entry.relPath, checked)"
                    @click.stop
                  />
                  <n-icon v-else :size="15" :component="FolderOpenOutline" />
                  <span class="file-row__path gv-mono" :title="entry.relPath">
                    {{ entry.name }}
                  </span>
                  <span v-if="entry.inPatch" class="file-row__tag">已登记</span>
                  <span class="file-row__size">{{ entry.isDir ? "" : formatBytes(entry.size) }}</span>
                </button>
              </div>
            </n-spin>
          </section>

          <!-- 已选清单 -->
          <section class="block">
            <div class="block__head">
              <h4>已选文件（{{ selected.length }}）</h4>
              <n-button v-if="selected.length" quaternary size="small" @click="selected = []">
                清空
              </n-button>
            </div>

            <p v-if="selected.length === 0" class="gv-faint empty">
              还没有选择任何文件。没有文件的补丁也能保存，只是卸载时无文件可删。
            </p>

            <div v-else class="selected">
              <span v-for="relPath in selected" :key="relPath" class="selected__item gv-mono">
                {{ relPath }}
                <button type="button" class="selected__drop" @click="toggle(relPath, false)">
                  <n-icon :size="12" :component="CloseOutline" />
                </button>
              </span>
            </div>

            <div v-if="manualOnly.length" class="gv-faint hint">
              其中 {{ manualOnly.length }} 个是手动挑选的，不在本次检测结果里。
            </div>
          </section>

          <!-- 下载来源提示 -->
          <section class="block block--hint">
            <n-icon :size="18" :component="CloudDownloadOutline" />
            <div>
              <strong>本工具不提供补丁下载</strong>
              <p>
                补丁请自行到 2DFan 等站点获取。装好之后回来登记它动了哪些文件，以后就能一键清干净。
              </p>
              <n-button text type="primary" @click="openTwoDFan">打开 2DFan →</n-button>
            </div>
          </section>
        </div>
      </n-spin>

      <template #footer>
        <div class="footer">
          <template v-if="isEdit">
            <n-popconfirm @positive-click="() => uninstall(true)">
              <template #trigger>
                <n-button type="warning" quaternary>
                  <template #icon>
                    <n-icon :component="TrashOutline" />
                  </template>
                  卸载并删文件
                </n-button>
              </template>
              会把这些文件从磁盘上真的删掉，确认吗？
            </n-popconfirm>

            <n-popconfirm @positive-click="() => uninstall(false)">
              <template #trigger>
                <n-button quaternary>只标记未安装</n-button>
              </template>
              不动文件，只把补丁标记为未安装。
            </n-popconfirm>

            <n-tooltip trigger="hover">
              <template #trigger>
                <n-button quaternary @click="removeRecord">
                  <template #icon>
                    <n-icon :component="ArrowBackOutline" />
                  </template>
                  删记录
                </n-button>
              </template>
              只删数据库记录，不碰磁盘
            </n-tooltip>
          </template>

          <div class="gv-spacer" />

          <n-button :disabled="saving" @click="closePatchEditor">取消</n-button>
          <n-button type="primary" :loading="saving" @click="save">
            <template #icon>
              <n-icon :component="CheckmarkOutline" />
            </template>
            保存
          </n-button>
        </div>
      </template>
    </n-drawer-content>
  </n-drawer>
</template>

<style scoped>
.editor {
  display: flex;
  flex-direction: column;
  gap: 18px;
  padding-bottom: 8px;
}

.block {
  border: 1px solid var(--gv-border);
  border-radius: var(--gv-radius);
  background: var(--gv-surface);
  padding: 16px;
}

.block--detect {
  background: var(--gv-surface-2);
}

.block--hint {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  border-color: var(--gv-accent-line);
  background: color-mix(in srgb, var(--gv-accent) 5%, transparent);
}

.block--hint p {
  margin: 4px 0 8px;
  font-size: var(--gv-fs-label);
  color: var(--gv-text-dim);
  line-height: 1.6;
}

.block__head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 14px;
}

.block__head h4 {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  margin: 0;
  font-size: var(--gv-fs-body);
  font-weight: 600;
}

.block__head .gv-faint {
  margin-left: auto;
  font-size: var(--gv-fs-label);
}

.grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.span-2 {
  grid-column: span 2;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.field__label {
  font-size: var(--gv-fs-label);
  font-weight: 500;
  color: var(--gv-text-dim);
}

.field__label i {
  color: var(--gv-danger);
  font-style: normal;
}

.inline {
  display: flex;
  align-items: center;
  gap: 8px;
}

.detect-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.detect-summary {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 12px;
  padding: 9px 12px;
  border-radius: var(--gv-radius-sm);
  background: var(--gv-surface-3);
  font-size: var(--gv-fs-label);
  color: var(--gv-text-dim);
}

.changes,
.browser {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin-top: 12px;
  max-height: 260px;
  overflow: auto;
}

.changes__head {
  position: sticky;
  top: 0;
  padding: 4px 6px;
  background: var(--gv-surface-2);
  border-bottom: 1px solid var(--gv-border);
  z-index: 1;
}

.file-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 6px;
  border-radius: var(--gv-radius-sm);
  font-size: var(--gv-fs-label);
  cursor: pointer;
}

.file-row:hover {
  background: var(--gv-surface-3);
}

.file-row--browse {
  border: none;
  background: transparent;
  color: inherit;
  text-align: left;
  font-family: inherit;
}

.file-row__op {
  flex: 0 0 auto;
  padding: 1px 5px;
  border-radius: 4px;
  font-size: var(--gv-fs-caption);
  font-weight: 600;
}

.file-row__op.is-added {
  background: color-mix(in srgb, var(--gv-success) 20%, transparent);
  color: var(--gv-success);
}

.file-row__op.is-modified {
  background: color-mix(in srgb, var(--gv-warn) 20%, transparent);
  color: var(--gv-warn);
}

.file-row__path {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--gv-fs-label);
}

.file-row__size {
  flex: 0 0 auto;
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-faint);
}

.file-row__tag {
  flex: 0 0 auto;
  padding: 0 5px;
  border-radius: 4px;
  background: var(--gv-accent-soft);
  color: var(--gv-accent);
  font-size: var(--gv-fs-caption);
}

.selected {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  max-height: 180px;
  overflow: auto;
}

.selected__item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 6px;
  border: 1px solid var(--gv-border);
  border-radius: 6px;
  font-size: var(--gv-fs-caption);
  background: var(--gv-surface-2);
}

.selected__drop {
  display: inline-flex;
  border: none;
  background: transparent;
  color: var(--gv-text-faint);
  cursor: pointer;
  padding: 0;
}

.selected__drop:hover {
  color: var(--gv-danger);
}

.empty {
  margin: 0;
  font-size: var(--gv-fs-label);
  line-height: 1.6;
}

.hint {
  margin-top: 8px;
  font-size: var(--gv-fs-label);
}

.footer {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.gv-warn {
  color: var(--gv-warn);
}
</style>
