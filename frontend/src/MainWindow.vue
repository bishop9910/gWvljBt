<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, onUnmounted, ref } from "vue";
import { RouterView, useRoute, useRouter } from "vue-router";
import { NButton, NIcon, NInput, NTooltip } from "naive-ui";
import type { Component } from "vue";
import {
  AddOutline,
  AlbumsOutline,
  BarChartOutline,
  ChevronBackOutline,
  ChevronForwardOutline,
  CompassOutline,
  ExtensionPuzzleOutline,
  ScanOutline,
  SearchOutline,
  SettingsOutline,
} from "@vicons/ionicons5";
// 三个抽屉/弹窗只在真正打开时才需要，做成异步组件，主 chunk 里就不带它们了。
const GameEditor = defineAsyncComponent(() => import("./components/GameEditor.vue"));
const PatchEditor = defineAsyncComponent(() => import("./components/PatchEditor.vue"));
const ScanDialog = defineAsyncComponent(() => import("./components/ScanDialog.vue"));
const TouchgalDialog = defineAsyncComponent(() => import("./components/TouchgalDialog.vue"));
import { libraryStore, loadGames } from "./state/library";
import { openGameEditor, openScan, openTouchgal, uiStore } from "./state/ui";

const route = useRoute();
const router = useRouter();

interface NavItem {
  key: string;
  label: string;
  icon: Component;
}

const navItems: NavItem[] = [
  { key: "/", label: "游戏库", icon: AlbumsOutline },
  { key: "/patches", label: "补丁", icon: ExtensionPuzzleOutline },
  { key: "/stats", label: "统计", icon: BarChartOutline },
  { key: "/settings", label: "设置", icon: SettingsOutline },
];

const title = computed(() => (route.meta.title as string) ?? "游戏库");
const subtitle = computed(() => {
  if (route.name === "library") {
    return libraryStore.loaded ? `${libraryStore.games.length}` : "";
  }
  return (route.meta.subtitle as string) ?? "";
});

function isActive(key: string): boolean {
  if (key === "/") return route.path === "/" || route.path.startsWith("/game");
  return route.path.startsWith(key);
}

function go(key: string): void {
  if (route.path !== key) void router.push(key);
}

/* ---------------- 搜索 ---------------- */

const search = computed({
  get: () => libraryStore.filter.query,
  set: (value: string) => {
    libraryStore.filter.query = value;
  },
});

const searchInput = ref<{ focus: () => void } | null>(null);

async function submitSearch(): Promise<void> {
  if (route.path !== "/") await router.push("/");
  await loadGames();
}

function onKeydown(event: KeyboardEvent): void {
  const target = event.target as HTMLElement | null;
  const typing = !!target && /^(input|textarea|select)$/i.test(target.tagName);
  if (event.key === "/" && !typing) {
    event.preventDefault();
    searchInput.value?.focus();
  }
}

/* ---------------- 时钟 ---------------- */

const now = ref(new Date());
let timer = 0;

const clock = computed(() => {
  const d = now.value;
  return `${`${d.getHours()}`.padStart(2, "0")}:${`${d.getMinutes()}`.padStart(2, "0")}`;
});

const today = computed(() => {
  const d = now.value;
  const week = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"][d.getDay()];
  return `${d.getMonth() + 1} 月 ${d.getDate()} 日　${week}`;
});

onMounted(() => {
  window.addEventListener("keydown", onKeydown);
  now.value = new Date();
  timer = window.setInterval(() => {
    now.value = new Date();
  }, 5000);
});

onUnmounted(() => {
  window.removeEventListener("keydown", onKeydown);
  window.clearInterval(timer);
});
</script>

<template>
  <div class="shell" :class="{ 'shell--collapsed': uiStore.railCollapsed }">
    <aside class="rail">
      <nav class="rail__nav">
        <n-tooltip
          v-for="item in navItems"
          :key="item.key"
          :disabled="!uiStore.railCollapsed"
          placement="right"
          :delay="160"
        >
          <template #trigger>
            <button
              class="rail__item"
              :class="{ 'rail__item--active': isActive(item.key) }"
              type="button"
              @click="go(item.key)"
            >
              <n-icon :size="18" :component="item.icon" />
              <span class="rail__label">{{ item.label }}</span>
            </button>
          </template>
          {{ item.label }}
        </n-tooltip>
      </nav>

      <div class="rail__foot">
        <div class="clock" :title="today">
          <span class="clock__time">{{ clock }}</span>
          <span v-if="!uiStore.railCollapsed" class="clock__date">{{ today }}</span>
        </div>

        <button
          class="rail__collapse"
          type="button"
          :title="uiStore.railCollapsed ? '展开侧栏' : '收起侧栏'"
          @click="uiStore.railCollapsed = !uiStore.railCollapsed"
        >
          <n-icon
            :size="15"
            :component="uiStore.railCollapsed ? ChevronForwardOutline : ChevronBackOutline"
          />
        </button>
      </div>
    </aside>

    <section class="stage">
      <header class="topbar">
        <div class="topbar__heading">
          <h1>{{ title }}</h1>
          <span v-if="subtitle" class="topbar__count">{{ subtitle }}</span>
        </div>

        <div class="searchline">
          <n-icon class="searchline__icon" :size="15" :component="SearchOutline" />
          <n-input
            ref="searchInput"
            v-model:value="search"
            clearable
            placeholder="标题 / 原名 / 会社 / 标签"
            @keyup.enter="submitSearch"
          />
        </div>

        <n-button class="scanbtn" quaternary @click="openScan">
          <template #icon>
            <n-icon :component="ScanOutline" />
          </template>
          扫描
        </n-button>

        <n-button class="scanbtn" quaternary @click="openTouchgal()">
          <template #icon>
            <n-icon :component="CompassOutline" />
          </template>
          找资源
        </n-button>

        <n-button class="addbtn" type="primary" @click="openGameEditor()">
          <template #icon>
            <n-icon :component="AddOutline" />
          </template>
          添加
        </n-button>
      </header>

      <div class="viewport">
        <RouterView v-slot="{ Component }">
          <transition name="gv-view" mode="out-in">
            <component :is="Component" :key="route.fullPath" />
          </transition>
        </RouterView>
      </div>
    </section>

    <GameEditor />
    <PatchEditor />
    <ScanDialog />
    <TouchgalDialog />
  </div>
</template>

<style scoped>
.shell {
  position: relative;
  z-index: 1;
  display: flex;
  height: 100%;
  overflow: hidden;
}

/* ---------------- 侧栏：一条安静的边 ---------------- */

.rail {
  flex: 0 0 196px;
  width: 196px;
  display: flex;
  flex-direction: column;
  padding: 22px 14px 16px;
  border-right: 1px solid var(--gv-border);
  background: var(--gv-rail, var(--gv-surface));
  transition: flex-basis 0.32s var(--gv-ease), width 0.32s var(--gv-ease),
    padding 0.32s var(--gv-ease);
}

.shell--collapsed .rail {
  flex-basis: 68px;
  width: 68px;
  padding: 22px 10px 16px;
}

.rail__nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.rail__foot {
  margin-top: auto;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding-top: 14px;
  border-top: 1px solid var(--gv-border);
}

.rail__item {
  position: relative;
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  height: 36px;
  padding: 0 10px;
  border: 0;
  border-radius: var(--gv-radius-sm);
  background: transparent;
  color: var(--gv-text-dim);
  font-family: inherit;
  font-size: var(--gv-fs-body);
  cursor: pointer;
  white-space: nowrap;
  overflow: hidden;
  transition: color 0.18s var(--gv-ease), background 0.18s var(--gv-ease);
}

.shell--collapsed .rail__item {
  justify-content: center;
  padding: 0;
  gap: 0;
}

.rail__item:hover {
  color: var(--gv-text);
  background: var(--gv-surface-2);
}

.rail__item--active {
  color: var(--gv-text);
  background: transparent;
}

/* 当前项：一道主色的线脚，不加底色 */
.rail__item--active::before {
  content: "";
  position: absolute;
  left: -14px;
  top: 9px;
  bottom: 9px;
  width: 2px;
  background: var(--gv-accent);
}

.shell--collapsed .rail__item--active::before {
  left: -10px;
}

.rail__item--active .rail__label {
  font-weight: 600;
}

.rail__label {
  transition: opacity 0.16s var(--gv-ease);
}

.shell--collapsed .rail__label {
  opacity: 0;
  width: 0;
  pointer-events: none;
}

/* ---------------- 时钟 ---------------- */

.clock {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 0 10px;
  overflow: hidden;
}

.shell--collapsed .clock {
  padding: 0;
  align-items: center;
}

.clock__time {
  font-family: var(--gv-mono);
  font-size: 21px;
  line-height: 1.1;
  font-variant-numeric: tabular-nums;
  letter-spacing: 0;
  color: var(--gv-text);
}

.shell--collapsed .clock__time {
  font-size: var(--gv-fs-label);
  letter-spacing: 0;
}

.clock__date {
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-faint);
  white-space: nowrap;
}

.rail__collapse {
  align-self: flex-start;
  display: grid;
  place-items: center;
  width: 26px;
  height: 26px;
  margin-left: 6px;
  border: 0;
  border-radius: var(--gv-radius-sm);
  background: transparent;
  color: var(--gv-text-faint);
  cursor: pointer;
  transition: color 0.18s var(--gv-ease), background 0.18s var(--gv-ease);
}

.shell--collapsed .rail__collapse {
  align-self: center;
  margin-left: 0;
}

.rail__collapse:hover {
  color: var(--gv-text);
  background: var(--gv-surface-2);
}

/* ---------------- 主区 ---------------- */

.stage {
  flex: 1 1 auto;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.topbar {
  flex: 0 0 auto;
  display: flex;
  align-items: baseline;
  gap: 22px;
  height: 78px;
  padding: 0 30px;
  border-bottom: 1px solid var(--gv-border);
}

.topbar__heading {
  display: flex;
  align-items: baseline;
  gap: 9px;
  min-width: 0;
}

.topbar__heading h1 {
  margin: 0;
  font-family: var(--gv-font-display);
  font-size: var(--gv-fs-head);
  font-weight: 600;
  letter-spacing: var(--gv-track-display);
  line-height: 1.2;
  white-space: nowrap;
}

.topbar__count {
  font-family: var(--gv-font);
  font-size: var(--gv-fs-label);
  font-variant-numeric: tabular-nums;
  color: var(--gv-text-faint);
}

/* 搜索：一根下划线，不是填充块 */
.searchline {
  flex: 1 1 auto;
  max-width: 380px;
  margin: 0 0 -6px auto;
  display: flex;
  align-items: center;
  gap: 7px;
  padding-bottom: 3px;
  border-bottom: 1px solid var(--gv-border);
  transition: border-color 0.2s var(--gv-ease);
}

.searchline:focus-within {
  border-color: var(--gv-accent-line);
}

.searchline__icon {
  flex: 0 0 auto;
  color: var(--gv-text-faint);
}

.searchline :deep(.n-input) {
  --n-color: transparent !important;
  --n-color-focus: transparent !important;
  --n-border: none !important;
  --n-border-hover: none !important;
  --n-border-focus: none !important;
  --n-box-shadow-focus: none !important;
  width: 100%;
}

.searchline :deep(.n-input__input-el) {
  padding: 0;
  font-size: var(--gv-fs-body);
}

.searchline :deep(.n-input__border),
.searchline :deep(.n-input__state-border) {
  display: none;
}

.addbtn {
  flex: 0 0 auto;
  align-self: center;
  font-weight: 500;
}

.scanbtn {
  flex: 0 0 auto;
  align-self: center;
  color: var(--gv-text-dim);
  font-weight: 500;
}

.scanbtn:hover {
  color: var(--gv-accent);
}

.viewport {
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}

@media (max-width: 1080px) {
  .searchline {
    max-width: 210px;
  }
}
</style>
