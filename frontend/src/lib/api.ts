/**
 * 绑定层封装。
 *
 * 视图只从这里调后端，好处是参数/返回值的类型可以按领域类型收口，
 * 生成的 bindings 换生成路径时也只改这一个文件。
 */
import { Events } from "@wailsio/runtime";
import { ConfigService, GameService, MetaService, PatchService, ScanService, TouchGalService } from "../../bindings/gWvljBt";
import type { ScanImportInput, ScanImportResult, TouchGalHit, TouchGalResource } from "../../bindings/gWvljBt/models";
import type { ConfigData } from "../../bindings/gWvljBt/config/models";
import type { ScanMeta, ScanResult } from "../../bindings/gWvljBt/data/models";
import type {
  DetectResult,
  DirEntry,
  FolderStats,
  CountItem,
  Game,
  GameFilter,
  GameInput,
  MetaDraft,
  Patch,
  PatchInput,
  PatchRow,
  SearchHit,
  Stats,
  UninstallResult,
} from "./types";

/** 把绑定的 $CancellablePromise 收成普通 Promise<T>，顺手统一类型。 */
async function call<T>(p: PromiseLike<unknown> | unknown): Promise<T> {
  return (await p) as T;
}

/** 传参给生成函数时不再让 TS 逐个比对生成类型。 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const arg = (value: unknown) => value as any;

export const api = {
  config: {
    get: () => call<ConfigData>(ConfigService.Get()),
    change: (key: keyof ConfigData | string, value: unknown) =>
      call<void>(ConfigService.Change(key as string, arg(value))),
    setAll: (next: ConfigData) => call<void>(ConfigService.SetAll(arg(next))),
    reset: () => call<void>(ConfigService.Reset()),
    dataDir: () => call<string>(ConfigService.DataDir()),
    openDataDir: () => call<void>(ConfigService.OpenDataDir()),
    openConfigFile: () => call<void>(ConfigService.OpenConfigFile()),
    pickWallpaper: () => call<string>(ConfigService.PickWallpaper()),
    clearWallpaper: () => call<void>(ConfigService.ClearWallpaper()),
  },

  games: {
    list: (filter: GameFilter) => call<Game[]>(GameService.List(arg(filter))),
    get: (id: number) => call<Game>(GameService.Get(id)),
    create: (input: GameInput) => call<Game>(GameService.Create(arg(input))),
    update: (id: number, input: GameInput) => call<Game>(GameService.Update(id, arg(input))),
    remove: (id: number) => call<void>(GameService.Delete(id)),
    setFavorite: (id: number, value: boolean) => call<void>(GameService.SetFavorite(id, value)),
    setStatus: (id: number, status: string) => call<void>(GameService.SetStatus(id, status)),
    setRating: (id: number, rating: number) => call<void>(GameService.SetRating(id, rating)),
    addPlaytime: (id: number, minutes: number) => call<void>(GameService.AddPlaytime(id, minutes)),
    launch: (id: number) => call<void>(GameService.Launch(id)),
    openFolder: (id: number) => call<void>(GameService.OpenFolder(id)),
    openPath: (path: string) => call<void>(GameService.OpenPath(path)),
    openUrl: (url: string) => call<void>(GameService.OpenURL(url)),
    pickFolder: () => call<string>(GameService.PickFolder()),
    pickExecutable: (startDir: string) => call<string>(GameService.PickExecutable(startDir)),
    pickImage: () => call<string>(GameService.PickImage()),
    folderStats: (path: string) => call<FolderStats>(GameService.FolderStats(path)),
    developers: () => call<CountItem[]>(GameService.Developers()),
    tags: () => call<CountItem[]>(GameService.Tags()),
    stats: () => call<Stats>(GameService.Stats()),
    missing: () => call<number[]>(GameService.MissingGames()),
  },

  patches: {
    listByGame: (gameId: number) => call<Patch[]>(PatchService.List(gameId)),
    all: () => call<PatchRow[]>(PatchService.All()),
    get: (id: number) => call<Patch>(PatchService.Get(id)),
    create: (input: PatchInput) => call<Patch>(PatchService.Create(arg(input))),
    update: (id: number, input: PatchInput) => call<Patch>(PatchService.Update(id, arg(input))),
    remove: (id: number) => call<void>(PatchService.Delete(id)),
    uninstall: (id: number, deleteFiles: boolean) =>
      call<UninstallResult>(PatchService.Uninstall(id, deleteFiles)),
    addFiles: (id: number, relPaths: string[]) => call<void>(PatchService.AddFiles(id, arg(relPaths))),
    removeFiles: (id: number, relPaths: string[]) =>
      call<void>(PatchService.RemoveFiles(id, arg(relPaths))),
    verify: (id: number) => call<string[]>(PatchService.Verify(id)),
    detect: (gameId: number) => call<DetectResult>(PatchService.DetectChanges(gameId)),
    hasSnapshot: (gameId: number) => call<boolean>(PatchService.HasSnapshot(gameId)),
    rebuild: (gameId: number) => call<number>(PatchService.RebuildSnapshot(gameId)),
    browse: (gameId: number, relDir: string) =>
      call<DirEntry[]>(PatchService.Browse(gameId, relDir)),
  },

  meta: {
    search: (source: string, query: string) => call<SearchHit[]>(MetaService.Search(source, query)),
    detail: (source: string, id: string) => call<MetaDraft>(MetaService.Detail(source, id)),
    ping: (source: string) => call<string>(MetaService.Ping(source)),
  },

  scan: {
    pickRoot: () => call<string>(ScanService.PickRoot()),
    scan: (root: string, maxDepth: number) => call<ScanResult>(ScanService.Scan(root, maxDepth)),
    lookup: (name: string, source: string) => call<ScanMeta>(ScanService.Lookup(name, source)),
    import: (items: ScanImportInput[]) => call<ScanImportResult>(ScanService.Import(items)),
  },

  // TouchGal 的接口全在 Cloudflare 后面，能不能用取决于设置里那段 Cookie。
  touchgal: {
    ping: () => call<string>(TouchGalService.Ping()),
    search: (query: string, limit: number) =>
      call<TouchGalHit[] | null>(TouchGalService.Search(query, limit)),
    resources: (patchId: number) => call<TouchGalResource[] | null>(TouchGalService.Resources(patchId)),
  },
};

/** Wails 事件回调的载荷有时是事件对象、有时直接是数据，这里统一取一次。 */
function eventData<T>(payload: unknown): T {
  if (payload && typeof payload === "object" && "data" in (payload as Record<string, unknown>)) {
    return (payload as { data: T }).data;
  }
  return payload as T;
}

export function onConfigChanged(handler: (cfg: ConfigData) => void): () => void {
  return Events.On("config:changed", (payload) => handler(eventData<ConfigData>(payload)));
}

export function onGamesChanged(handler: (id: number) => void): () => void {
  return Events.On("game:changed", (payload) => handler(Number(eventData<number>(payload) ?? 0)));
}

export function onPatchesChanged(handler: (gameId: number) => void): () => void {
  return Events.On("patch:changed", (payload) => handler(Number(eventData<number>(payload) ?? 0)));
}

/** 统一把后端抛出来的错误转成可读文本。 */
export function errorText(err: unknown): string {
  if (!err) return "";
  if (typeof err === "string") return err;
  if (err instanceof Error) return err.message;
  const anyErr = err as { message?: string };
  return anyErr?.message ?? String(err);
}
