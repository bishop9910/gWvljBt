<script setup lang="ts">
import { computed } from "vue";
import { NIcon, NTooltip } from "naive-ui";
import {
  ExtensionPuzzleOutline,
  Heart,
  HeartOutline,
  PencilOutline,
  PlayOutline,
  Star,
} from "@vicons/ionicons5";
import PosterImage from "./PosterImage.vue";
import { statusMeta, type Game } from "../lib/types";
import { yearOf } from "../lib/format";

const props = defineProps<{ game: Game }>();

const emit = defineEmits<{
  (event: "open"): void;
  (event: "launch"): void;
  (event: "favorite"): void;
  (event: "edit"): void;
}>();

const status = computed(() => statusMeta(props.game.playStatus));
const patchCount = computed(() => props.game.patches?.length ?? 0);
const installedPatches = computed(
  () => props.game.patches?.filter((p) => p.installed).length ?? 0,
);
const year = computed(() => yearOf(props.game.releaseDate, props.game.year));
const canLaunch = computed(() => !!(props.game.exePath || props.game.folderPath));
const rating = computed(() => Number(props.game.rating ?? 0));

function stop(event: MouseEvent, action: () => void): void {
  event.stopPropagation();
  action();
}
</script>

<template>
  <article
    class="plate"
    tabindex="0"
    role="button"
    @click="emit('open')"
    @keydown.enter="emit('open')"
    @keydown.space.prevent="emit('open')"
  >
    <div class="plate__frame">
      <PosterImage class="plate__poster" :src="game.poster" :title="game.title" rounded="0" />

      <button
        v-if="game.favorite"
        class="plate__fav"
        type="button"
        title="取消收藏"
        @click="stop($event, () => emit('favorite'))"
      >
        <n-icon :size="13" :component="Heart" />
      </button>

      <span v-if="patchCount > 0" class="plate__patch" :title="`已安装 ${installedPatches} / 共 ${patchCount} 个补丁`">
        <n-icon :size="10" :component="ExtensionPuzzleOutline" />
        <span>{{ installedPatches }}/{{ patchCount }}</span>
      </span>
    </div>

    <div class="plate__cap" :style="{ '--mark': status.color }">
      <h3 class="plate__title" :title="game.title">{{ game.title || "未命名作品" }}</h3>

      <div class="plate__meta">
        <span v-if="year" class="plate__year">{{ year }}</span>
        <span v-if="game.developer" class="plate__dev" :title="game.developer">
          {{ game.developer }}
        </span>
        <span v-if="rating > 0" class="plate__rating">
          <n-icon :size="10" :component="Star" />
          <span>{{ rating.toFixed(1) }}</span>
        </span>

        <span class="plate__acts">
          <n-tooltip v-if="canLaunch" placement="top" :delay="220">
            <template #trigger>
              <button class="plate__act" type="button" @click="stop($event, () => emit('launch'))">
                <n-icon :size="13" :component="PlayOutline" />
              </button>
            </template>
            启动游戏
          </n-tooltip>
          <n-tooltip placement="top" :delay="220">
            <template #trigger>
              <button class="plate__act" type="button" @click="stop($event, () => emit('edit'))">
                <n-icon :size="13" :component="PencilOutline" />
              </button>
            </template>
            编辑信息
          </n-tooltip>
          <n-tooltip placement="top" :delay="220">
            <template #trigger>
              <button class="plate__act" type="button" @click="stop($event, () => emit('favorite'))">
                <n-icon :size="13" :component="game.favorite ? Heart : HeartOutline" />
              </button>
            </template>
            {{ game.favorite ? "取消收藏" : "加入收藏" }}
          </n-tooltip>
        </span>
      </div>
    </div>
  </article>
</template>

<style scoped>
.plate {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 9px;
  padding: 0;
  background: transparent;
  cursor: pointer;
  outline: none;
}

/* 图版：只有一圈发丝边，没有卡片壳 */
.plate__frame {
  position: relative;
  border: 1px solid var(--gv-border);
  background: var(--gv-surface-2);
  overflow: hidden;
  transition: border-color 0.22s var(--gv-ease);
}

.plate__poster {
  display: block;
  width: 100%;
  transition: transform 0.5s var(--gv-ease), filter 0.3s var(--gv-ease);
}

/* 海报自己带一圈边，外层图版就不再画第二条线 */
.plate__frame :deep(.poster) {
  border: 0;
}

.plate:hover .plate__frame,
.plate:focus-visible .plate__frame {
  border-color: var(--gv-accent-line);
}

.plate:hover .plate__poster {
  transform: scale(1.025);
}

.plate:focus-visible .plate__frame {
  outline: 2px solid var(--gv-accent-line);
  outline-offset: 3px;
}

.plate__fav {
  position: absolute;
  top: 6px;
  right: 6px;
  display: grid;
  place-items: center;
  width: 20px;
  height: 20px;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--gv-accent);
  cursor: pointer;
  filter: drop-shadow(0 1px 2px rgba(0, 0, 0, 0.5));
}

.plate__patch {
  position: absolute;
  left: 6px;
  bottom: 6px;
  display: inline-flex;
  align-items: center;
  gap: 3px;
  height: 17px;
  padding: 0 5px;
  border: 1px solid rgba(255, 255, 255, 0.22);
  border-radius: 2px;
  background: rgba(14, 16, 20, 0.55);
  color: #f2f0e8;
  font-family: var(--gv-font);
  font-size: var(--gv-fs-caption);
  font-variant-numeric: tabular-nums;
  line-height: 1;
}

/* 说明文字：左边一道状态色的记号 */
.plate__cap {
  position: relative;
  padding-left: 8px;
  min-width: 0;
}

.plate__cap::before {
  content: "";
  position: absolute;
  left: 0;
  top: 3px;
  bottom: 2px;
  width: 2px;
  background: var(--mark);
  opacity: 0.75;
  transition: opacity 0.2s var(--gv-ease);
}

.plate:hover .plate__cap::before {
  opacity: 1;
}

.plate__title {
  margin: 0;
  font-family: var(--gv-font-display);
  font-size: var(--gv-fs-title);
  font-weight: 600;
  line-height: 1.42;
  letter-spacing: var(--gv-track-display);
  color: var(--gv-text);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 2.9em;
  border-bottom: 1px solid transparent;
  transition: border-color 0.22s var(--gv-ease);
}

.plate:hover .plate__title {
  border-bottom-color: var(--gv-accent-line);
}

.plate__meta {
  display: flex;
  align-items: center;
  gap: 7px;
  margin-top: 2px;
  font-family: var(--gv-font);
  font-size: var(--gv-fs-caption);
  font-variant-numeric: tabular-nums;
  color: var(--gv-text-faint);
  min-width: 0;
}

.plate__year {
  flex: 0 0 auto;
}

.plate__dev {
  flex: 1 1 auto;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--gv-font);
  font-size: var(--gv-fs-caption);
}

.plate__rating {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  gap: 2px;
  color: var(--gv-warn);
}

.plate__acts {
  flex: 0 0 auto;
  display: flex;
  gap: 1px;
  margin-left: auto;
  opacity: 0;
  transition: opacity 0.2s var(--gv-ease);
}

.plate:hover .plate__acts,
.plate:focus-within .plate__acts {
  opacity: 1;
}

.plate__act {
  display: grid;
  place-items: center;
  width: 21px;
  height: 21px;
  padding: 0;
  border: 0;
  border-radius: var(--gv-radius-sm);
  background: transparent;
  color: var(--gv-text-faint);
  cursor: pointer;
  transition: color 0.16s var(--gv-ease), background 0.16s var(--gv-ease);
}

.plate__act:hover {
  color: var(--gv-accent);
  background: var(--gv-surface-3);
}
</style>
