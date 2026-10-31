import { createMemoryHistory, createRouter } from "vue-router";

// 视图全部走动态 import：每个页面单独一个 chunk，首屏只加载游戏库那一份。
// 静态 import 会把五个视图连同它们的 naive-ui 组件一起塞进主 chunk。
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
    component: () => import("./views/LibraryView.vue"),
    meta: { title: "游戏库", subtitle: "全部收藏" },
  },
  {
    path: "/game/:id",
    name: "game",
    component: () => import("./views/GameDetailView.vue"),
    props: true,
    meta: { title: "作品详情" },
  },
  {
    path: "/patches",
    name: "patches",
    component: () => import("./views/PatchesView.vue"),
    meta: { title: "补丁管理", subtitle: "汉化 / 解锁 / 修复" },
  },
  {
    path: "/stats",
    name: "stats",
    component: () => import("./views/StatsView.vue"),
    meta: { title: "数据统计", subtitle: "你的收藏画像" },
  },
  {
    path: "/settings",
    name: "settings",
    component: () => import("./views/SettingsView.vue"),
    meta: { title: "设置", subtitle: "外观 · 数据源 · 存储" },
  },
  { path: "/:pathMatch(.*)*", redirect: "/" },
];

export const router = createRouter({
  history: createMemoryHistory(),
  routes,
});
