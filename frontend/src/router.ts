import { createMemoryHistory, createRouter } from "vue-router";

import LibraryView from "./views/LibraryView.vue";
import GameDetailView from "./views/GameDetailView.vue";
import PatchesView from "./views/PatchesView.vue";
import StatsView from "./views/StatsView.vue";
import SettingsView from "./views/SettingsView.vue";

declare module "vue-router" {
  interface RouteMeta {
    title?: string;
    subtitle?: string;
  }
}

export const routes = [
  {
    path: "/",
    name: "library",
    component: LibraryView,
    meta: { title: "游戏库", subtitle: "全部收藏" },
  },
  {
    path: "/game/:id",
    name: "game",
    component: GameDetailView,
    props: true,
    meta: { title: "作品详情" },
  },
  {
    path: "/patches",
    name: "patches",
    component: PatchesView,
    meta: { title: "补丁管理", subtitle: "汉化 / 解锁 / 修复" },
  },
  {
    path: "/stats",
    name: "stats",
    component: StatsView,
    meta: { title: "数据统计", subtitle: "你的收藏画像" },
  },
  {
    path: "/settings",
    name: "settings",
    component: SettingsView,
    meta: { title: "设置", subtitle: "外观 · 数据源 · 存储" },
  },
  { path: "/:pathMatch(.*)*", redirect: "/" },
];

export const router = createRouter({
  history: createMemoryHistory(),
  routes,
});
