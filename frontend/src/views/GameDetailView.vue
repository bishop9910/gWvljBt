<script setup lang="ts">
/**
 * 作品详情：左侧主区放补丁与简介，右侧放游玩记录和链接。
 */
import { computed, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import {
  NButton,
  NEmpty,
  NIcon,
  NModal,
  NPopconfirm,
  NSelect,
  NSlider,
  NSpin,
  NTooltip,
  useMessage,
} from "naive-ui";
import {
  ArrowBackOutline,
  CloudDownloadOutline,
  CreateOutline,
  ExtensionPuzzleOutline,
  FolderOpenOutline,
  HeartDislikeOutline,
  HeartOutline,
  LinkOutline,
  PlayOutline,
  SaveOutline,
  StarOutline,
  TimeOutline,
  TrashOutline,
} from "@vicons/ionicons5";
import PosterImage from "../components/PosterImage.vue";
import { api, errorText } from "../lib/api";
import { formatPlaytime, splitList, timeAgo } from "../lib/format";
import {
  PLAY_STATUSES,
  STATUS_META,
  kindMeta,
  statusMeta,
  type Game,
  type Patch,
} from "../lib/types";
import { configStore } from "../state/config";
import { openGameEditor, openPatchEditor } from "../state/ui";

const route = useRoute();
const router = useRouter();
const message = useMessage();

const game = ref<Game | null>(null);
const patches = ref<Patch[]>([]);
const loading = ref(true);
const error = ref("");

const missingModal = ref(false);
const missingFiles = ref<string[]>([]);

const gameId = computed(() => Number(route.params.id));
const statusOptions = PLAY_STATUSES.map((status) => ({
  label: STATUS_META[status].label,
  value: status,
}));

async function load(): Promise<void> {
  const id = gameId.value;
  if (!Number.isFinite(id) || id <= 0) {
    error.value = "无效的作品编号";
    loading.value = false;
    return;
  }
  loading.value = true;
  error.value = "";
  try {
    game.value = await api.games.get(id);
    patches.value = (await api.patches.listByGame(id)) ?? [];
  } catch (err) {
    error.value = errorText(err);
    game.value = null;
    patches.value = [];
  } finally {
    loading.value = false;
  }
}

async function launch(): Promise<void> {
  if (!game.value) return;
  try {
    await api.games.launch(game.value.id);
    message.success("正在启动");
    setTimeout(load, 600);
  } catch (err) {
    message.error(errorText(err));
  }
}

async function toggleFavorite(): Promise<void> {
  if (!game.value) return;
  try {
    await api.games.setFavorite(game.value.id, !game.value.favorite);
    await load();
  } catch (err) {
    message.error(errorText(err));
  }
}

async function changeStatus(value: string): Promise<void> {
  if (!game.value) return;
  try {
    await api.games.setStatus(game.value.id, value);
    await load();
  } catch (err) {
    message.error(errorText(err));
  }
}

async function changeRating(value: number): Promise<void> {
  if (!game.value) return;
  try {
    await api.games.setRating(game.value.id, value);
    game.value.rating = value;
  } catch (err) {
    message.error(errorText(err));
  }
}

async function addTime(minutes: number): Promise<void> {
  if (!game.value) return;
  try {
    await api.games.addPlaytime(game.value.id, minutes);
    await load();
    message.success(`+${minutes} 分钟`);
  } catch (err) {
    message.error(errorText(err));
  }
}

async function openPath(path: string | undefined): Promise<void> {
  if (!path) {
    message.warning("这个作品还没设置对应路径");
    return;
  }
  try {
    await api.games.openPath(path);
  } catch (err) {
    message.error(errorText(err));
  }
}

async function openUrl(url: string): Promise<void> {
  if (!url) return;
  try {
    await api.games.openUrl(url);
  } catch (err) {
    message.error(errorText(err));
  }
}

async function removeGame(): Promise<void> {
  if (!game.value) return;
  try {
    await api.games.remove(game.value.id);
    message.success("已删除作品记录");
    router.push("/");
  } catch (err) {
    message.error(errorText(err));
  }
}

async function verifyPatch(patch: Patch): Promise<void> {
  try {
    const missing = await api.patches.verify(patch.id);
    if (!missing || missing.length === 0) {
      message.success("清单里的文件都还在");
      return;
    }
    missingFiles.value = missing;
    missingModal.value = true;
  } catch (err) {
    message.error(errorText(err));
  }
}

async function uninstallPatch(patch: Patch): Promise<void> {
  try {
    const result = await api.patches.uninstall(patch.id, true);
    const failed = result.failed ?? [];
    const parts = [`删除 ${result.deleted} 个文件`];
    if (result.missing > 0) parts.push(`${result.missing} 个本来就没了`);
    if (failed.length > 0) parts.push(`${failed.length} 个失败`);
    message.success(parts.join("，"));
    await load();
  } catch (err) {
    message.error(errorText(err));
  }
}

async function removePatchRecord(patch: Patch): Promise<void> {
  try {
    await api.patches.remove(patch.id);
    message.success("已删除补丁记录");
    await load();
  } catch (err) {
    message.error(errorText(err));
  }
}

const patchFileCount = computed(() =>
  patches.value.reduce((sum, patch) => sum + (patch.files?.length ?? 0), 0),
);

const installedCount = computed(() => patches.value.filter((patch) => patch.installed).length);

const tagList = computed(() => splitList(game.value?.tags ?? ""));
const genreList = computed(() => splitList(game.value?.genres ?? ""));
const aliasList = computed(() => splitList(game.value?.alias ?? ""));

const vndbUrl = computed(() =>
  game.value?.vndbId ? `https://vndb.org/${game.value.vndbId}` : "",
);
const bangumiUrl = computed(() =>
  game.value?.bangumiId ? `https://bgm.tv/subject/${game.value.bangumiId}` : "",
);

const heroStyle = computed(() => {
  const src = game.value?.banner || game.value?.poster || "";
  return src ? { backgroundImage: `url("${src}")` } : undefined;
});

watch(
  () => route.params.id,
  () => {
    if (route.name === "game") void load();
  },
  { immediate: true },
);
</script>

<template>
  <div class="detail gv-scroll">
    <n-spin :show="loading">
      <div v-if="game" class="detail__inner">
        <!-- 头图 -->
        <header class="hero">
          <div class="hero__bg" :style="heroStyle" />
          <div class="hero__veil" />
          <div class="hero__inner">
            <PosterImage class="hero__poster" :src="game.poster" :title="game.title" />
            <div class="hero__info">
              <div class="hero__crumb">
                <button type="button" class="crumb" @click="router.push('/')">
                  <n-icon :size="13" :component="ArrowBackOutline" />
                  游戏库
                </button>
              </div>

              <h1 class="hero__title">{{ game.title }}</h1>
              <p v-if="game.originalTitle || aliasList.length" class="hero__sub">
                {{ game.originalTitle }}
                <span v-if="aliasList.length" class="gv-faint">· {{ aliasList.join(" / ") }}</span>
              </p>

              <div class="hero__chips">
                <span
                  class="pill"
                  :style="{ '--pill': statusMeta(game.playStatus).color }"
                >
                  {{ statusMeta(game.playStatus).label }}
                </span>
                <span v-if="game.year" class="gv-chip">{{ game.year }}</span>
                <span v-if="game.developer" class="gv-chip">{{ game.developer }}</span>
                <span v-if="game.rating > 0" class="gv-chip gv-chip--accent">
                  <n-icon :size="12" :component="StarOutline" />
                  {{ Number(game.rating).toFixed(1) }}
                </span>
                <span v-if="game.playtime > 0" class="gv-chip">
                  <n-icon :size="12" :component="TimeOutline" />
                  {{ formatPlaytime(game.playtime) }}
                </span>
                <span v-if="installedCount > 0" class="gv-chip">
                  <n-icon :size="12" :component="ExtensionPuzzleOutline" />
                  {{ installedCount }} 个补丁
                </span>
              </div>

              <div class="hero__actions">
                <n-button type="primary" size="medium" @click="launch">
                  <template #icon>
                    <n-icon :component="PlayOutline" />
                  </template>
                  启动游戏
                </n-button>
                <n-button :secondary="game.favorite" @click="toggleFavorite">
                  <template #icon>
                    <n-icon :component="game.favorite ? HeartOutline : HeartDislikeOutline" />
                  </template>
                  {{ game.favorite ? "已收藏" : "收藏" }}
                </n-button>
                <n-button @click="openGameEditor(game.id)">
                  <template #icon>
                    <n-icon :component="CreateOutline" />
                  </template>
                  编辑
                </n-button>
                <n-popconfirm @positive-click="removeGame">
                  <template #trigger>
                    <n-button quaternary type="error">
                      <template #icon>
                        <n-icon :component="TrashOutline" />
                      </template>
                    </n-button>
                  </template>
                  只删除数据库里的记录，磁盘上的文件不会动。
                </n-popconfirm>
              </div>
            </div>
          </div>
        </header>

        <!-- 主体 -->
        <div class="cols">
          <div class="col-main">
            <!-- 补丁 -->
            <section class="panel">
              <div class="panel__head">
                <h3>
                  <n-icon :size="15" :component="ExtensionPuzzleOutline" />
                  补丁
                  <em v-if="patches.length">{{ patches.length }} 个 · {{ patchFileCount }} 文件</em>
                </h3>
                <div class="gv-spacer" />
                <n-button size="small" @click="openPatchEditor({ gameId: game.id })">
                  登记补丁
                </n-button>
                <n-button
                  size="small"
                  quaternary
                  @click="openUrl(configStore.data.twoDFanUrl || 'https://www.2dfan.com/')"
                >
                  <template #icon>
                    <n-icon :component="CloudDownloadOutline" />
                  </template>
                  去 2DFan 下载
                </n-button>
              </div>

              <div v-if="patches.length === 0" class="panel__empty">
                <p>还没登记补丁。装好汉化/解锁补丁后回来登记它改了哪些文件，以后一键就能清干净。</p>
              </div>

              <div v-else class="patches">
                <article v-for="patch in patches" :key="patch.id" class="patch">
                  <span
                    class="patch__kind"
                    :style="{ '--kind': kindMeta(patch.kind).color }"
                  >
                    {{ kindMeta(patch.kind).label }}
                  </span>

                  <div class="patch__body">
                    <div class="patch__title">
                      <strong>{{ patch.name }}</strong>
                      <span v-if="patch.version" class="gv-chip">{{ patch.version }}</span>
                      <span class="patch__state" :class="{ 'is-on': patch.installed }">
                        {{ patch.installed ? "已安装" : "未安装" }}
                      </span>
                    </div>
                    <div class="patch__meta">
                      <span>{{ patch.files?.length ?? 0 }} 个文件</span>
                      <span v-if="patch.source">· {{ patch.source }}</span>
                      <span v-if="patch.installedAt">· {{ timeAgo(patch.installedAt) }}</span>
                    </div>
                    <p v-if="patch.note" class="patch__note">{{ patch.note }}</p>
                  </div>

                  <div class="patch__actions">
                    <n-tooltip trigger="hover">
                      <template #trigger>
                        <n-button size="tiny" quaternary @click="verifyPatch(patch)">
                          校验
                        </n-button>
                      </template>
                      检查清单里的文件还在不在
                    </n-tooltip>
                    <n-button size="tiny" quaternary @click="openPatchEditor({ patchId: patch.id })">
                      编辑
                    </n-button>
                    <n-popconfirm @positive-click="uninstallPatch(patch)">
                      <template #trigger>
                        <n-button size="tiny" quaternary type="warning">卸载</n-button>
                      </template>
                      会把 {{ patch.files?.length ?? 0 }} 个文件从磁盘删掉，确认？
                    </n-popconfirm>
                    <n-popconfirm @positive-click="removePatchRecord(patch)">
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
            </section>

            <!-- 简介 -->
            <section v-if="game.description" class="panel">
              <div class="panel__head">
                <h3>简介</h3>
              </div>
              <p class="prose">{{ game.description }}</p>
            </section>

            <!-- 标签 -->
            <section v-if="tagList.length || genreList.length" class="panel">
              <div class="panel__head">
                <h3>标签与分类</h3>
              </div>
              <div class="tag-cloud">
                <span v-for="genre in genreList" :key="'g' + genre" class="gv-chip gv-chip--accent">
                  {{ genre }}
                </span>
                <span v-for="tag in tagList" :key="'t' + tag" class="gv-chip">{{ tag }}</span>
              </div>
            </section>

            <!-- 备注 -->
            <section v-if="game.notes" class="panel">
              <div class="panel__head">
                <h3>备注</h3>
              </div>
              <p class="prose">{{ game.notes }}</p>
            </section>
          </div>

          <!-- 侧栏 -->
          <aside class="col-side">
            <section class="panel">
              <div class="panel__head">
                <h3>游玩记录</h3>
              </div>

              <label class="side-field">
                <span>状态</span>
                <n-select
                  :value="game.playStatus"
                  :options="statusOptions"
                  size="small"
                  @update:value="changeStatus"
                />
              </label>

              <label class="side-field">
                <span>评分 {{ Number(game.rating || 0).toFixed(1) }}</span>
                <n-slider
                  :value="game.rating"
                  :min="0"
                  :max="10"
                  :step="0.5"
                  @update:value="changeRating"
                />
              </label>

              <div class="side-field">
                <span>时长 {{ formatPlaytime(game.playtime) }}</span>
                <div class="row">
                  <n-button size="tiny" @click="addTime(30)">+30 分</n-button>
                  <n-button size="tiny" @click="addTime(60)">+1 小时</n-button>
                </div>
              </div>

              <p v-if="game.lastPlayedAt" class="side-note">
                上次游玩：{{ timeAgo(game.lastPlayedAt) }}
              </p>
            </section>

            <section class="panel">
              <div class="panel__head">
                <h3>文件</h3>
              </div>

              <button type="button" class="path-row" @click="openPath(game.folderPath)">
                <n-icon :size="14" :component="FolderOpenOutline" />
                <div>
                  <strong>安装目录</strong>
                  <small class="gv-mono">{{ game.folderPath || "未设置" }}</small>
                </div>
              </button>

              <button
                type="button"
                class="path-row"
                :disabled="!game.exePath"
                @click="openPath(game.exePath)"
              >
                <n-icon :size="14" :component="PlayOutline" />
                <div>
                  <strong>启动程序</strong>
                  <small class="gv-mono">{{ game.exePath || "自动查找" }}</small>
                </div>
              </button>

              <button
                type="button"
                class="path-row"
                :disabled="!game.savePath"
                @click="openPath(game.savePath)"
              >
                <n-icon :size="14" :component="SaveOutline" />
                <div>
                  <strong>存档目录</strong>
                  <small class="gv-mono">{{ game.savePath || "未设置" }}</small>
                </div>
              </button>
            </section>

            <section v-if="vndbUrl || bangumiUrl" class="panel">
              <div class="panel__head">
                <h3>外部链接</h3>
              </div>
              <button v-if="vndbUrl" type="button" class="path-row" @click="openUrl(vndbUrl)">
                <n-icon :size="14" :component="LinkOutline" />
                <div>
                  <strong>VNDB</strong>
                  <small class="gv-mono">{{ game.vndbId }}</small>
                </div>
              </button>
              <button v-if="bangumiUrl" type="button" class="path-row" @click="openUrl(bangumiUrl)">
                <n-icon :size="14" :component="LinkOutline" />
                <div>
                  <strong>Bangumi</strong>
                  <small class="gv-mono">{{ game.bangumiId }}</small>
                </div>
              </button>
            </section>

            <section class="panel panel--quiet">
              <div class="panel__head">
                <h3>记录</h3>
              </div>
              <p class="side-note">加入：{{ timeAgo(game.createdAt) }}</p>
              <p class="side-note">更新：{{ timeAgo(game.updatedAt) }}</p>
            </section>
          </aside>
        </div>
      </div>

      <div v-else-if="!loading" class="not-found">
        <n-empty :description="error || '找不到这个作品'">
          <template #extra>
            <n-button @click="router.push('/')">回到游戏库</n-button>
          </template>
        </n-empty>
      </div>
    </n-spin>

    <n-modal
      v-model:show="missingModal"
      preset="card"
      title="这些文件已经不在磁盘上了"
      style="max-width: 620px"
    >
      <div class="missing-list gv-scroll">
        <code v-for="file in missingFiles" :key="file">{{ file }}</code>
      </div>
      <p class="gv-faint missing-note">可能是你手动删了，或者游戏更新覆盖了。卸载时这些会被跳过。</p>
    </n-modal>
  </div>
</template>

<style scoped>
.detail {
  height: 100%;
  overflow: auto;
}

.detail__inner {
  padding-bottom: 48px;
}

.hero {
  position: relative;
  overflow: hidden;
  border-bottom: 1px solid var(--gv-border);
}

.hero__bg {
  position: absolute;
  inset: 0;
  background-size: cover;
  background-position: center 22%;
  filter: blur(7px) saturate(108%);
  opacity: 0.2;
  transform: scale(1.06);
}

.hero__veil {
  position: absolute;
  inset: 0;
  background: linear-gradient(
    180deg,
    color-mix(in srgb, var(--gv-bg) 40%, transparent) 0%,
    color-mix(in srgb, var(--gv-bg) 88%, transparent) 60%,
    var(--gv-bg) 100%
  );
}

.hero__inner {
  position: relative;
  display: flex;
  gap: 26px;
  padding: 26px 32px 28px;
  align-items: flex-end;
}

.hero__poster {
  flex: 0 0 168px;
  width: 168px;
  border: 1px solid var(--gv-border-strong);
}

.hero__info {
  flex: 1 1 auto;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 9px;
  padding-bottom: 4px;
}

.hero__crumb {
  margin-bottom: 2px;
}

.crumb {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border: none;
  background: transparent;
  color: var(--gv-text-dim);
  font-size: var(--gv-fs-label);
  cursor: pointer;
  padding: 0;
}

.crumb:hover {
  color: var(--gv-accent);
}

.hero__title {
  margin: 0;
  font-family: var(--gv-font-display);
  font-size: var(--gv-fs-hero);
  line-height: 1.2;
  font-weight: 600;
  letter-spacing: 0.004em;
}

.hero__sub {
  margin: 0;
  font-size: var(--gv-fs-body);
  color: var(--gv-text-dim);
}

.hero__chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}

.pill {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 3px;
  font-size: var(--gv-fs-label);
  font-weight: 600;
  color: var(--pill);
  background: color-mix(in srgb, var(--pill) 13%, transparent);
  border-left: 2px solid var(--pill);
}

.hero__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  margin-top: 6px;
}

.cols {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 306px;
  gap: 20px;
  padding: 22px 32px 0;
  align-items: start;
}

.col-main,
.col-side {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}

.panel {
  border-top: 1px solid var(--gv-border);
  padding: 15px 0 0;
}

.panel--quiet {
  color: var(--gv-text-dim);
}

.panel__head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 11px;
}

.panel__head h3 {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  margin: 0;
  font-family: var(--gv-font-display);
  font-size: var(--gv-fs-title);
  font-weight: 600;
  letter-spacing: 0.04em;
}

.panel__head em {
  font-style: normal;
  font-size: var(--gv-fs-label);
  font-weight: 500;
  color: var(--gv-text-faint);
}

.panel__empty p {
  margin: 0;
  font-size: var(--gv-fs-label);
  line-height: 1.65;
  color: var(--gv-text-dim);
}

.patches {
  display: flex;
  flex-direction: column;
}

.patch {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 11px 2px;
  border-bottom: 1px solid var(--gv-border);
  transition: background 0.18s var(--gv-ease);
}

.patch:last-child {
  border-bottom: 0;
}

.patch__kind {
  flex: 0 0 auto;
  padding: 2px 7px;
  border-radius: 2px;
  font-size: var(--gv-fs-caption);
  font-weight: 600;
  color: var(--kind);
  background: color-mix(in srgb, var(--kind) 14%, transparent);
}

.patch__body {
  flex: 1 1 auto;
  min-width: 0;
}

.patch__title {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.patch__title strong {
  font-size: var(--gv-fs-body);
  font-weight: 600;
}

.patch__state {
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-faint);
}

.patch__state.is-on {
  color: var(--gv-success);
}

.patch__meta {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
  margin-top: 2px;
  font-size: var(--gv-fs-label);
  color: var(--gv-text-faint);
}

.patch__note {
  margin: 5px 0 0;
  font-size: var(--gv-fs-label);
  color: var(--gv-text-dim);
  line-height: 1.55;
}

.patch__actions {
  display: flex;
  align-items: center;
  gap: 1px;
  flex: 0 0 auto;
}

.prose {
  margin: 0;
  font-size: var(--gv-fs-body);
  line-height: 1.78;
  color: var(--gv-text-dim);
  white-space: pre-wrap;
}

.tag-cloud {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
}

.side-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 12px;
  font-size: var(--gv-fs-label);
  color: var(--gv-text-dim);
}

.row {
  display: flex;
  gap: 6px;
}

.path-row {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  width: 100%;
  padding: 6px 4px;
  border: 0;
  border-radius: 2px;
  background: transparent;
  color: var(--gv-text-dim);
  text-align: left;
  cursor: pointer;
  transition: background 0.18s var(--gv-ease), color 0.18s var(--gv-ease);
}

.path-row:hover:not(:disabled) {
  background: var(--gv-surface-2);
  color: var(--gv-text);
}

.path-row:disabled {
  opacity: 0.5;
  cursor: default;
}

.path-row div {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
}

.path-row strong {
  font-size: var(--gv-fs-label);
  font-weight: 500;
  color: var(--gv-text);
}

.path-row small {
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-faint);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 230px;
}

.side-note {
  margin: 0;
  font-size: var(--gv-fs-label);
  color: var(--gv-text-faint);
}

.not-found {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 60vh;
}

.missing-list {
  display: flex;
  flex-direction: column;
  gap: 3px;
  max-height: 320px;
  overflow: auto;
}

.missing-list code {
  font-family: var(--gv-mono);
  font-size: var(--gv-fs-label);
  color: var(--gv-text-dim);
  padding: 2px 6px;
  border-radius: 4px;
  background: var(--gv-surface-2);
}

.missing-note {
  margin: 12px 0 0;
  font-size: var(--gv-fs-label);
}
</style>
