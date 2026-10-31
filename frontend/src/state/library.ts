/** 库状态：游戏列表 + 筛选条件 + 侧边栏用的聚合数据。 */
import { computed, reactive, watch } from "vue";
import { api, errorText, onGamesChanged } from "../lib/api";
import type { CountItem, Game, GameFilter } from "../lib/types";

interface LibraryState {
  games: Game[];
  filter: GameFilter;
  developers: CountItem[];
  tags: CountItem[];
  loading: boolean;
  error: string;
  loaded: boolean;
}

export const libraryStore = reactive<LibraryState>({
  games: [],
  filter: { query: "", status: "", developer: "", tag: "", sort: "updated", favorite: false },
  developers: [],
  tags: [],
  loading: false,
  error: "",
  loaded: false,
});

export const activeFilterCount = computed(() => {
  const f = libraryStore.filter;
  let n = 0;
  if (f.query.trim()) n += 1;
  if (f.status) n += 1;
  if (f.developer) n += 1;
  if (f.tag) n += 1;
  if (f.favorite) n += 1;
  return n;
});

let reloadTimer: ReturnType<typeof setTimeout> | null = null;
let filterTimer: ReturnType<typeof setTimeout> | null = null;
let wired = false;

export async function loadGames(): Promise<void> {
  libraryStore.loading = true;
  libraryStore.error = "";
  try {
    libraryStore.games = (await api.games.list({ ...libraryStore.filter })) ?? [];
    libraryStore.loaded = true;
  } catch (err) {
    libraryStore.error = errorText(err);
  } finally {
    libraryStore.loading = false;
  }
}

export async function loadFacets(): Promise<void> {
  try {
    const [developers, tags] = await Promise.all([api.games.developers(), api.games.tags()]);
    libraryStore.developers = developers ?? [];
    libraryStore.tags = tags ?? [];
  } catch {
    // 聚合数据失败不影响主列表
  }
}

export async function refreshLibrary(): Promise<void> {
  await Promise.all([loadGames(), loadFacets()]);
}

/** 应用启动时调一次。 */
export async function initLibrary(): Promise<void> {
  if (!wired) {
    wired = true;
    onGamesChanged(() => {
      if (reloadTimer) clearTimeout(reloadTimer);
      reloadTimer = setTimeout(() => {
        void refreshLibrary();
      }, 140);
    });

    // 筛选条件一变就去后端重新查（输入框打字也能有个 180ms 的缓冲）
    watch(
      () => ({ ...libraryStore.filter }),
      () => {
        if (filterTimer) clearTimeout(filterTimer);
        filterTimer = setTimeout(() => {
          void loadGames();
        }, 180);
      },
    );
  }
  await refreshLibrary();
}

export function resetFilter(): void {
  libraryStore.filter = {
    query: "",
    status: "",
    developer: "",
    tag: "",
    sort: libraryStore.filter.sort || "updated",
    favorite: false,
  };
}

export function findGame(id: number): Game | undefined {
  return libraryStore.games.find((g) => g.id === id);
}
