<script setup lang="ts">
/**
 * 游戏库：状态筛选 + 海报图版。
 * 数据全部来自 libraryStore，筛选条件一改就自动去后端查。
 */
import { computed } from "vue";
import { useRouter } from "vue-router";
import { NButton, NIcon, NSelect, NTooltip, useMessage } from "naive-ui";
import {
  AlbumsOutline,
  FunnelOutline,
  HeartOutline,
  HeartDislikeOutline,
  AddOutline,
  RefreshOutline,
} from "@vicons/ionicons5";
import GameCard from "../components/GameCard.vue";
import { api, errorText } from "../lib/api";
import {
  PLAY_STATUSES,
  SORT_OPTIONS,
  STATUS_META,
  type Game,
  type PlayStatus,
} from "../lib/types";
import {
  activeFilterCount,
  libraryStore,
  loadGames,
  loadFacets,
  resetFilter,
} from "../state/library";
import { openGameEditor } from "../state/ui";

const router = useRouter();
const message = useMessage();

const statusFilters = computed(() => {
  const counts = new Map<string, number>();
  for (const game of libraryStore.games) {
    const key = game.playStatus || "unplayed";
    counts.set(key, (counts.get(key) ?? 0) + 1);
  }
  return [
    { key: "all", label: "全部", total: libraryStore.games.length },
    ...PLAY_STATUSES.map((status) => ({
      key: status as string,
      label: STATUS_META[status].label,
      total: counts.get(status) ?? 0,
    })),
  ];
});

const developerOptions = computed(() => [
  { label: "全部会社", value: "" },
  ...libraryStore.developers.map((item) => ({
    label: `${item.name} (${item.count})`,
    value: item.name,
  })),
]);

const tagOptions = computed(() => [
  { label: "全部标签", value: "" },
  ...libraryStore.tags.map((item) => ({ label: `${item.name} (${item.count})`, value: item.name })),
]);

const sortOptions = computed(() =>
  SORT_OPTIONS.map((item) => ({ label: item.label, value: item.value })),
);

function setStatus(key: string): void {
  libraryStore.filter.status = key === "all" ? "" : (key as PlayStatus);
}

function openGame(game: Game): void {
  router.push(`/game/${game.id}`);
}

async function launch(game: Game): Promise<void> {
  try {
    await api.games.launch(game.id);
    message.success(`正在启动「${game.title}」`);
  } catch (err) {
    message.error(errorText(err));
  }
}

async function toggleFavorite(game: Game): Promise<void> {
  try {
    await api.games.setFavorite(game.id, !game.favorite);
  } catch (err) {
    message.error(errorText(err));
  }
}

async function reload(): Promise<void> {
  await Promise.all([loadGames(), loadFacets()]);
}

function edit(game: Game): void {
  openGameEditor(game.id);
}

const hasGames = computed(() => libraryStore.games.length > 0);
const filtering = computed(() => activeFilterCount.value > 0 || !!libraryStore.filter.query);
</script>

<template>
  <div class="library gv-scroll">
    <div class="filters">
      <div class="filters__status">
        <button
          v-for="item in statusFilters"
          :key="item.key"
          type="button"
          class="tab"
          :class="{
            'is-active':
              (item.key === 'all' && !libraryStore.filter.status) ||
              libraryStore.filter.status === item.key,
          }"
          :style="
            item.key !== 'all'
              ? { '--tab-mark': STATUS_META[item.key as PlayStatus].color }
              : undefined
          "
          @click="setStatus(item.key)"
        >
          <span v-if="item.key !== 'all'" class="tab__mark" />
          {{ item.label }}
          <em>{{ item.total }}</em>
        </button>
      </div>

      <div class="filters__row">
        <n-tooltip trigger="hover">
          <template #trigger>
            <button
              type="button"
              class="toggle"
              :class="{ 'is-active': libraryStore.filter.favorite }"
              @click="libraryStore.filter.favorite = !libraryStore.filter.favorite"
            >
              <n-icon
                :size="14"
                :component="libraryStore.filter.favorite ? HeartOutline : HeartDislikeOutline"
              />
              只看收藏
            </button>
          </template>
          只显示加入收藏的作品
        </n-tooltip>

        <n-select
          v-model:value="libraryStore.filter.developer"
          :options="developerOptions"
          size="small"
          style="width: 168px"
        />

        <n-select
          v-model:value="libraryStore.filter.tag"
          :options="tagOptions"
          size="small"
          style="width: 150px"
        />

        <n-select
          v-model:value="libraryStore.filter.sort"
          :options="sortOptions"
          size="small"
          style="width: 124px"
        />

        <button v-if="filtering" type="button" class="clear" @click="resetFilter">
          <n-icon :size="12" :component="FunnelOutline" />
          清除筛选
        </button>

        <div class="gv-spacer" />

        <n-tooltip trigger="hover">
          <template #trigger>
            <button type="button" class="iconbtn" @click="reload">
              <n-icon :size="14" :component="RefreshOutline" />
            </button>
          </template>
          重新读取
        </n-tooltip>
      </div>

      <hr class="gv-rule" />
    </div>

    <div v-if="hasGames" class="grid">
      <GameCard
        v-for="game in libraryStore.games"
        :key="game.id"
        :game="game"
        @open="openGame(game)"
        @launch="launch(game)"
        @favorite="toggleFavorite(game)"
        @edit="edit(game)"
      />
    </div>

    <div v-else-if="!libraryStore.loading" class="empty">
      <template v-if="filtering">
        <h3 class="empty__title">这里没有东西</h3>
        <p class="empty__text">换个筛选条件，或者把搜索框清空。</p>
        <n-button text @click="resetFilter">清除全部筛选</n-button>
      </template>
      <template v-else>
        <n-icon :size="26" :component="AlbumsOutline" class="empty__mark" />
        <h3 class="empty__title">书架还是空的</h3>
        <p class="empty__text">
          填个标题就能收进来；其余信息可以按关联 ID 从 VNDB / Bangumi 直接抓。
        </p>
        <n-button type="primary" @click="openGameEditor()">
          <template #icon>
            <n-icon :component="AddOutline" />
          </template>
          添加第一部作品
        </n-button>
      </template>
    </div>

    <div v-else class="loading-hint gv-faint">正在读取…</div>

    <p v-if="libraryStore.error" class="error">{{ libraryStore.error }}</p>
  </div>
</template>

<style scoped>
.library {
  height: 100%;
  overflow: auto;
  padding: 22px 30px 46px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.filters {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.filters__status {
  display: flex;
  flex-wrap: wrap;
  gap: 18px;
}

/* 状态筛选：一行文字 + 数字，选中时下面一道线 */
.tab {
  position: relative;
  display: inline-flex;
  align-items: baseline;
  gap: 5px;
  padding: 0 0 6px;
  border: 0;
  background: transparent;
  color: var(--gv-text-faint);
  font-family: inherit;
  font-size: var(--gv-fs-body);
  cursor: pointer;
  transition: color 0.18s var(--gv-ease);
}

.tab:hover {
  color: var(--gv-text-dim);
}

.tab.is-active {
  color: var(--gv-text);
}

.tab.is-active::after {
  content: "";
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 1px;
  background: var(--gv-accent);
}

.tab__mark {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--tab-mark, var(--gv-text-faint));
  transform: translateY(-1px);
}

.tab em {
  font-style: normal;
  font-family: var(--gv-mono);
  font-size: var(--gv-fs-caption);
  font-variant-numeric: tabular-nums;
  color: var(--gv-text-faint);
}

.filters__row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.toggle {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 28px;
  padding: 0 9px;
  border: 1px solid var(--gv-border);
  border-radius: var(--gv-radius-sm);
  background: transparent;
  color: var(--gv-text-dim);
  font-family: inherit;
  font-size: var(--gv-fs-label);
  cursor: pointer;
  transition: color 0.18s var(--gv-ease), border-color 0.18s var(--gv-ease);
}

.toggle:hover {
  color: var(--gv-text);
  border-color: var(--gv-border-strong);
}

.toggle.is-active {
  color: var(--gv-accent);
  border-color: var(--gv-accent-line);
  background: var(--gv-accent-soft);
}

.clear {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 2px;
  border: 0;
  background: transparent;
  color: var(--gv-text-faint);
  font-family: inherit;
  font-size: var(--gv-fs-label);
  cursor: pointer;
  transition: color 0.18s var(--gv-ease);
}

.clear:hover {
  color: var(--gv-accent);
}

.iconbtn {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  border: 1px solid transparent;
  border-radius: var(--gv-radius-sm);
  background: transparent;
  color: var(--gv-text-faint);
  cursor: pointer;
  transition: color 0.18s var(--gv-ease), border-color 0.18s var(--gv-ease);
}

.iconbtn:hover {
  color: var(--gv-text);
  border-color: var(--gv-border);
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(var(--gv-card-min), 1fr));
  gap: 26px 20px;
  padding-bottom: 8px;
}

.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  flex: 1 1 auto;
  min-height: 300px;
  padding-bottom: 60px;
  text-align: center;
}

.empty__mark {
  color: var(--gv-text-faint);
  margin-bottom: 4px;
}

.empty__title {
  margin: 0;
  font-size: var(--gv-fs-subhead);
  font-weight: 600;
  letter-spacing: 0.02em;
}

.empty__text {
  margin: 0 0 6px;
  max-width: 320px;
  font-size: var(--gv-fs-body);
  line-height: 1.7;
  color: var(--gv-text-dim);
}

.loading-hint {
  padding: 60px 0;
  text-align: center;
  font-size: var(--gv-fs-body);
}

.error {
  margin: 0;
  font-size: var(--gv-fs-label);
  color: var(--gv-danger);
}
</style>
