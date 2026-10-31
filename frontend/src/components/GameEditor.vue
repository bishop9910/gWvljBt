<script setup lang="ts">
/**
 * 作品编辑器：新建 / 编辑共用。
 * 右侧抽屉，顶部带 VNDB / Bangumi 元数据检索，命中后一键把字段填进来。
 */
import { computed, reactive, ref, watch } from "vue";
import { useRouter } from "vue-router";
import {
  NButton,
  NDrawer,
  NDrawerContent,
  NIcon,
  NInput,
  NInputNumber,
  NPopconfirm,
  NSelect,
  NSlider,
  NSpin,
  NSwitch,
  NTooltip,
  useMessage,
} from "naive-ui";
import {
  CheckmarkOutline,
  CloseOutline,
  DocumentTextOutline,
  FolderOpenOutline,
  ImageOutline,
  LinkOutline,
  PlayOutline,
  RefreshOutline,
  SearchOutline,
  SparklesOutline,
  TrashOutline,
} from "@vicons/ionicons5";
import PosterImage from "./PosterImage.vue";
import { api, errorText } from "../lib/api";
import { formatPlaytime, splitList } from "../lib/format";
import {
  KIND_META,
  PLAY_STATUSES,
  SORT_OPTIONS,
  STATUS_META,
  type FolderStats,
  type GameInput,
  type MetaDraft,
  type MetaSource,
  type SearchHit,
} from "../lib/types";
import { closeGameEditor, uiStore } from "../state/ui";

const message = useMessage();
const router = useRouter();

function blankInput(): GameInput {
  return {
    title: "",
    originalTitle: "",
    alias: "",
    poster: "",
    banner: "",
    developer: "",
    publisher: "",
    releaseDate: "",
    description: "",
    tags: "",
    genres: "",
    rating: 0,
    playStatus: "unplayed",
    playtime: 0,
    favorite: false,
    folderPath: "",
    exePath: "",
    savePath: "",
    vndbId: "",
    bangumiId: 0,
    source: "",
    notes: "",
  };
}

const form = reactive<GameInput>(blankInput());
const loading = ref(false);
const saving = ref(false);
const removing = ref(false);
const folderInfo = ref<FolderStats | null>(null);

const isEdit = computed(() => uiStore.gameEditorId > 0);
const drawerTitle = computed(() => (isEdit.value ? "编辑作品" : "添加作品"));

const statusOptions = PLAY_STATUSES.map((status) => ({
  label: STATUS_META[status].label,
  value: status,
}));

/* ------------------------------ 元数据检索 ------------------------------ */

const metaSource = ref<MetaSource>("vndb");
const metaQuery = ref("");
const metaHits = ref<SearchHit[]>([]);
const searching = ref(false);
const searchingId = ref("");
const metaError = ref("");
const sourceOptions = [
  { label: "VNDB", value: "vndb" },
  { label: "Bangumi", value: "bangumi" },
];

async function runSearch(): Promise<void> {
  const query = metaQuery.value.trim();
  if (!query) return;
  searching.value = true;
  metaError.value = "";
  try {
    metaHits.value = (await api.meta.search(metaSource.value, query)) ?? [];
    if (metaHits.value.length === 0) metaError.value = "没有找到结果，换个关键词试试";
  } catch (err) {
    metaHits.value = [];
    metaError.value = errorText(err);
  } finally {
    searching.value = false;
  }
}

async function applyHit(hit: SearchHit): Promise<void> {
  searchingId.value = hit.id;
  metaError.value = "";
  try {
    const draft = await api.meta.detail(hit.source, hit.id);
    mergeDraft(draft);
    message.success(`已填入「${draft.title || hit.title}」的元数据`);
  } catch (err) {
    metaError.value = errorText(err);
  } finally {
    searchingId.value = "";
  }
}

/** 草稿 → 表单：只覆盖有值的字段，避免把手工填的东西冲掉。 */
function mergeDraft(draft: MetaDraft): void {
  const assign = <K extends keyof GameInput>(key: K, value: GameInput[K] | undefined | null) => {
    if (value === undefined || value === null || value === "") return;
    form[key] = value as GameInput[K];
  };
  assign("title", draft.title);
  assign("originalTitle", draft.originalTitle);
  assign("alias", draft.alias);
  assign("poster", draft.poster);
  assign("banner", draft.banner);
  assign("developer", draft.developer);
  assign("publisher", draft.publisher);
  assign("releaseDate", draft.releaseDate);
  assign("description", draft.description);
  assign("tags", draft.tags);
  assign("genres", draft.genres);
  assign("vndbId", draft.vndbId);
  assign("source", draft.source);
  if (draft.bangumiId > 0) form.bangumiId = draft.bangumiId;
  if (draft.rating > 0) form.rating = Math.min(10, Math.round(draft.rating * 10) / 10);
  if (draft.lengthMinutes > 0) form.playtime = draft.lengthMinutes;
}

/* -------------------------------- 图片 -------------------------------- */

async function pickPoster(): Promise<void> {
  try {
    const picked = await api.games.pickImage();
    if (picked) form.poster = picked;
  } catch (err) {
    message.error(errorText(err));
  }
}

async function pickBanner(): Promise<void> {
  try {
    const picked = await api.games.pickImage();
    if (picked) form.banner = picked;
  } catch (err) {
    message.error(errorText(err));
  }
}

/* ------------------------------ 目录与启动 ------------------------------ */

async function pickFolder(): Promise<void> {
  try {
    const picked = await api.games.pickFolder();
    if (picked) {
      form.folderPath = picked;
      await inspectFolder();
    }
  } catch (err) {
    message.error(errorText(err));
  }
}

async function pickExe(): Promise<void> {
  try {
    const picked = await api.games.pickExecutable(form.folderPath);
    if (picked) form.exePath = picked;
  } catch (err) {
    message.error(errorText(err));
  }
}

async function pickSaveFolder(): Promise<void> {
  try {
    const picked = await api.games.pickFolder();
    if (picked) form.savePath = picked;
  } catch (err) {
    message.error(errorText(err));
  }
}

async function inspectFolder(): Promise<void> {
  const path = form.folderPath.trim();
  if (!path) {
    folderInfo.value = null;
    return;
  }
  try {
    folderInfo.value = await api.games.folderStats(path);
  } catch {
    folderInfo.value = null;
  }
}

/* -------------------------------- 读写 -------------------------------- */

async function loadGame(id: number): Promise<void> {
  loading.value = true;
  try {
    const game = await api.games.get(id);
    Object.assign(form, {
      title: game.title ?? "",
      originalTitle: game.originalTitle ?? "",
      alias: game.alias ?? "",
      poster: game.poster ?? "",
      banner: game.banner ?? "",
      developer: game.developer ?? "",
      publisher: game.publisher ?? "",
      releaseDate: game.releaseDate ?? "",
      description: game.description ?? "",
      tags: game.tags ?? "",
      genres: game.genres ?? "",
      rating: Number(game.rating ?? 0),
      playStatus: game.playStatus || "unplayed",
      playtime: Number(game.playtime ?? 0),
      favorite: !!game.favorite,
      folderPath: game.folderPath ?? "",
      exePath: game.exePath ?? "",
      savePath: game.savePath ?? "",
      vndbId: game.vndbId ?? "",
      bangumiId: Number(game.bangumiId ?? 0),
      source: game.source ?? "",
      notes: game.notes ?? "",
    } satisfies GameInput);
    await inspectFolder();
  } catch (err) {
    message.error(errorText(err));
  } finally {
    loading.value = false;
  }
}

async function save(): Promise<void> {
  if (!form.title.trim()) {
    message.warning("至少要填个标题");
    return;
  }
  saving.value = true;
  try {
    const payload: GameInput = { ...form, title: form.title.trim() };
    if (isEdit.value) {
      await api.games.update(uiStore.gameEditorId, payload);
      message.success("已保存");
    } else {
      const created = await api.games.create(payload);
      message.success(`已添加「${created.title}」`);
      router.push(`/game/${created.id}`);
    }
    closeGameEditor();
  } catch (err) {
    message.error(errorText(err));
  } finally {
    saving.value = false;
  }
}

async function remove(): Promise<void> {
  if (!isEdit.value) return;
  removing.value = true;
  try {
    await api.games.remove(uiStore.gameEditorId);
    message.success("已删除作品记录");
    closeGameEditor();
    router.push("/");
  } catch (err) {
    message.error(errorText(err));
  } finally {
    removing.value = false;
  }
}

function resetSearch(): void {
  metaHits.value = [];
  metaError.value = "";
  metaQuery.value = "";
}

watch(
  () => uiStore.gameEditorOpen,
  async (open) => {
    if (!open) return;
    resetSearch();
    folderInfo.value = null;
    Object.assign(form, blankInput());
    if (uiStore.gameEditorId > 0) {
      await loadGame(uiStore.gameEditorId);
    } else {
      metaQuery.value = "";
    }
  },
  { immediate: true },
);

const tagPreview = computed(() => splitList(form.tags).slice(0, 12));
</script>

<template>
  <n-drawer
    v-model:show="uiStore.gameEditorOpen"
    :width="760"
    placement="right"
    :mask-closable="!saving"
    @after-leave="closeGameEditor"
  >
    <n-drawer-content :title="drawerTitle" closable :native-scrollbar="false">
      <n-spin :show="loading">
        <div class="editor">
          <!-- 元数据检索 -->
          <section class="editor__block editor__block--meta">
            <div class="editor__block-head">
              <h4>
                <n-icon :size="15" :component="SparklesOutline" />
                从数据源抓取元数据
              </h4>
              <span class="gv-faint">搜索后点击结果即可填充</span>
            </div>

            <div class="meta-bar">
              <n-select v-model:value="metaSource" :options="sourceOptions" style="width: 122px" />
              <n-input
                v-model:value="metaQuery"
                placeholder="作品名 / 别名 / VNDB ID（如 v17449）"
                clearable
                @keyup.enter="runSearch"
              >
                <template #prefix>
                  <n-icon :component="SearchOutline" />
                </template>
              </n-input>
              <n-button type="primary" :loading="searching" @click="runSearch">搜索</n-button>
            </div>

            <p v-if="metaError" class="meta-error">{{ metaError }}</p>

            <div v-if="metaHits.length > 0" class="meta-list">
              <button
                v-for="hit in metaHits"
                :key="hit.source + hit.id"
                type="button"
                class="meta-hit"
                :disabled="searchingId !== ''"
                @click="applyHit(hit)"
              >
                <PosterImage
                  class="meta-hit__poster"
                  :src="hit.poster"
                  :title="hit.title"
                  rounded="8px"
                />
                <div class="meta-hit__body">
                  <strong>{{ hit.title || hit.titleCn || hit.id }}</strong>
                  <span v-if="hit.titleCn && hit.titleCn !== hit.title" class="gv-faint">
                    {{ hit.titleCn }}
                  </span>
                  <small>
                    <span>{{ hit.released || "—" }}</span>
                    <span v-if="hit.developer">· {{ hit.developer }}</span>
                    <span v-if="hit.score > 0">· ★ {{ hit.score.toFixed(1) }}</span>
                    <span v-if="hit.nsfw" class="meta-hit__nsfw">18+</span>
                  </small>
                </div>
                <n-spin v-if="searchingId === hit.id" :size="14" />
                <n-icon v-else :size="16" :component="CheckmarkOutline" class="meta-hit__apply" />
              </button>
            </div>
          </section>

          <!-- 基础信息 -->
          <section class="editor__block">
            <div class="editor__block-head">
              <h4>基础信息</h4>
            </div>

            <div class="grid">
              <label class="field span-2">
                <span class="field__label">标题 <i>*</i></span>
                <n-input v-model:value="form.title" placeholder="作品名" />
              </label>

              <label class="field">
                <span class="field__label">原名</span>
                <n-input v-model:value="form.originalTitle" placeholder="日文原名" />
              </label>

              <label class="field">
                <span class="field__label">别名</span>
                <n-input v-model:value="form.alias" placeholder="多个用逗号分隔" />
              </label>

              <label class="field">
                <span class="field__label">开发商 / 会社</span>
                <n-input v-model:value="form.developer" placeholder="Maker" />
              </label>

              <label class="field">
                <span class="field__label">发行商</span>
                <n-input v-model:value="form.publisher" />
              </label>

              <label class="field">
                <span class="field__label">发行日期</span>
                <n-input v-model:value="form.releaseDate" placeholder="YYYY-MM-DD" />
              </label>

              <label class="field">
                <span class="field__label">游玩状态</span>
                <n-select v-model:value="form.playStatus" :options="statusOptions" />
              </label>

              <label class="field">
                <span class="field__label">
                  评分
                  <em>{{ Number(form.rating).toFixed(1) }}</em>
                </span>
                <n-slider
                  v-model:value="form.rating"
                  :min="0"
                  :max="10"
                  :step="0.5"
                  :format-tooltip="(v: number) => `${Number(v).toFixed(1)} 分`"
                />
              </label>

              <label class="field">
                <span class="field__label">游玩时长（分钟）</span>
                <div class="inline">
                  <n-input-number
                    v-model:value="form.playtime"
                    :min="0"
                    :max="99999"
                    :step="30"
                    style="width: 100%"
                  />
                  <n-button quaternary @click="form.playtime = Number(form.playtime || 0) + 30">
                    +30
                  </n-button>
                </div>
                <small class="field__hint">{{ formatPlaytime(form.playtime) }}</small>
              </label>

              <label class="field">
                <span class="field__label">收藏</span>
                <div class="inline">
                  <n-switch v-model:value="form.favorite" />
                  <span class="gv-faint">加入「我的收藏」</span>
                </div>
              </label>
            </div>
          </section>

          <!-- 图片 -->
          <section class="editor__block">
            <div class="editor__block-head">
              <h4>
                <n-icon :size="15" :component="ImageOutline" />
                图片
              </h4>
              <span class="gv-faint">支持本地路径或网络地址，保存时会自动复制到数据目录</span>
            </div>

            <div class="art">
              <div class="art__preview">
                <PosterImage :src="form.poster" :title="form.title" />
              </div>
              <div class="art__fields">
                <label class="field">
                  <span class="field__label">封面</span>
                  <div class="inline">
                    <n-input v-model:value="form.poster" placeholder="本地路径 / URL" />
                    <n-button @click="pickPoster">选择</n-button>
                  </div>
                </label>
                <label class="field">
                  <span class="field__label">横幅（详情页顶图）</span>
                  <div class="inline">
                    <n-input v-model:value="form.banner" placeholder="本地路径 / URL" />
                    <n-button @click="pickBanner">选择</n-button>
                  </div>
                </label>
                <small class="field__hint">
                  也可以直接把 VNDB 的图片地址粘进来，保存时会下载到本地，断网也能看。
                </small>
              </div>
            </div>
          </section>

          <!-- 文件与启动 -->
          <section class="editor__block">
            <div class="editor__block-head">
              <h4>
                <n-icon :size="15" :component="FolderOpenOutline" />
                文件与启动
              </h4>
              <span v-if="folderInfo" class="gv-faint">
                {{ folderInfo.exists ? `${folderInfo.files} 个文件` : "目录不存在" }}
              </span>
            </div>

            <div class="grid">
              <label class="field span-2">
                <span class="field__label">安装目录</span>
                <div class="inline">
                  <n-input
                    v-model:value="form.folderPath"
                    placeholder="游戏所在文件夹，补丁检测也基于这里"
                    @blur="inspectFolder"
                  />
                  <n-button @click="pickFolder">浏览</n-button>
                </div>
              </label>

              <label class="field span-2">
                <span class="field__label">启动程序</span>
                <div class="inline">
                  <n-input v-model:value="form.exePath" placeholder="留空会自动在目录里找 .exe" />
                  <n-button @click="pickExe">浏览</n-button>
                </div>
              </label>

              <label class="field span-2">
                <span class="field__label">存档目录（可选）</span>
                <div class="inline">
                  <n-input v-model:value="form.savePath" placeholder="方便一键打开存档位置" />
                  <n-button @click="pickSaveFolder">浏览</n-button>
                </div>
              </label>
            </div>
          </section>

          <!-- 描述与标签 -->
          <section class="editor__block">
            <div class="editor__block-head">
              <h4>
                <n-icon :size="15" :component="DocumentTextOutline" />
                简介与标签
              </h4>
            </div>

            <div class="grid">
              <label class="field span-2">
                <span class="field__label">简介</span>
                <n-input
                  v-model:value="form.description"
                  type="textarea"
                  :autosize="{ minRows: 3, maxRows: 8 }"
                  placeholder="故事简介"
                />
              </label>

              <label class="field">
                <span class="field__label">标签</span>
                <n-input v-model:value="form.tags" placeholder="用逗号分隔" />
                <div v-if="tagPreview.length" class="tag-row">
                  <span v-for="tag in tagPreview" :key="tag" class="gv-chip">{{ tag }}</span>
                </div>
              </label>

              <label class="field">
                <span class="field__label">分类 / 类型</span>
                <n-input v-model:value="form.genres" placeholder="ADV、NVL、SLG…" />
              </label>

              <label class="field span-2">
                <span class="field__label">备注</span>
                <n-input
                  v-model:value="form.notes"
                  type="textarea"
                  :autosize="{ minRows: 2, maxRows: 6 }"
                  placeholder="安装注意事项、汉化来源、通关感想…"
                />
              </label>
            </div>
          </section>

          <!-- 关联 ID -->
          <section class="editor__block editor__block--quiet">
            <div class="grid grid--3">
              <label class="field">
                <span class="field__label">VNDB ID</span>
                <n-input v-model:value="form.vndbId" placeholder="v1234" />
              </label>
              <label class="field">
                <span class="field__label">Bangumi ID</span>
                <n-input-number
                  v-model:value="form.bangumiId"
                  :min="0"
                  :show-button="false"
                  style="width: 100%"
                />
              </label>
              <label class="field">
                <span class="field__label">数据来源</span>
                <n-input v-model:value="form.source" placeholder="vndb / bangumi" />
              </label>
            </div>
          </section>
        </div>
      </n-spin>

      <template #footer>
        <div class="footer">
          <n-popconfirm v-if="isEdit" @positive-click="remove">
            <template #trigger>
              <n-button type="error" quaternary :loading="removing">
                <template #icon>
                  <n-icon :component="TrashOutline" />
                </template>
                删除
              </n-button>
            </template>
            只删除数据库里的记录，磁盘上的游戏文件不会动。
          </n-popconfirm>

          <div class="gv-spacer" />

          <n-button :disabled="saving" @click="closeGameEditor">
            <template #icon>
              <n-icon :component="CloseOutline" />
            </template>
            取消
          </n-button>
          <n-button type="primary" :loading="saving" @click="save">
            <template #icon>
              <n-icon :component="CheckmarkOutline" />
            </template>
            {{ isEdit ? "保存" : "添加" }}
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

.editor__block {
  border: 1px solid var(--gv-border);
  border-radius: var(--gv-radius);
  background: var(--gv-surface);
  padding: 16px;
}

.editor__block--meta {
  background: color-mix(in srgb, var(--gv-accent) 5%, transparent);
  border-color: var(--gv-accent-line);
}

.editor__block--quiet {
  background: var(--gv-surface-2);
}

.editor__block-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 14px;
}

.editor__block-head h4 {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  margin: 0;
  font-size: var(--gv-fs-body);
  font-weight: 600;
  letter-spacing: 0.01em;
}

.editor__block-head .gv-faint {
  margin-left: auto;
  font-size: var(--gv-fs-label);
}

.grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.grid--3 {
  grid-template-columns: repeat(3, minmax(0, 1fr));
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
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: var(--gv-fs-label);
  font-weight: 500;
  color: var(--gv-text-dim);
}

.field__label i {
  color: var(--gv-danger);
  font-style: normal;
}

.field__label em {
  margin-left: auto;
  font-style: normal;
  font-family: var(--gv-mono);
  color: var(--gv-accent);
}

.field__hint {
  font-size: var(--gv-fs-label);
  color: var(--gv-text-faint);
}

.inline {
  display: flex;
  align-items: center;
  gap: 8px;
}

.inline > :first-child {
  flex: 1 1 auto;
  min-width: 0;
}

.meta-bar {
  display: flex;
  gap: 8px;
}

.meta-error {
  margin: 10px 0 0;
  font-size: var(--gv-fs-label);
  color: var(--gv-danger);
}

.meta-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-top: 12px;
  max-height: 280px;
  overflow: auto;
}

.meta-hit {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px;
  border: 1px solid var(--gv-border);
  border-radius: var(--gv-radius-sm);
  background: var(--gv-surface);
  color: inherit;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.2s var(--gv-ease), transform 0.2s var(--gv-ease);
}

.meta-hit:hover:not(:disabled) {
  border-color: var(--gv-accent-line);
  transform: translateX(2px);
}

.meta-hit__poster {
  flex: 0 0 auto;
  width: 42px;
}

.meta-hit__body {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  flex: 1 1 auto;
}

.meta-hit__body strong {
  font-size: var(--gv-fs-body);
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.meta-hit__body small {
  display: flex;
  gap: 5px;
  font-size: var(--gv-fs-label);
  color: var(--gv-text-faint);
}

.meta-hit__nsfw {
  padding: 0 5px;
  border-radius: 4px;
  background: var(--gv-danger);
  color: #fff;
  font-size: var(--gv-fs-caption);
}

.meta-hit__apply {
  color: var(--gv-accent);
  opacity: 0;
  transition: opacity 0.2s var(--gv-ease);
}

.meta-hit:hover .meta-hit__apply {
  opacity: 1;
}

.art {
  display: flex;
  gap: 16px;
  align-items: flex-start;
}

.art__preview {
  flex: 0 0 110px;
}

.art__fields {
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
}

.tag-row {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  margin-top: 4px;
}

.footer {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
