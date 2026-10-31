/** 全局 UI 状态：编辑器的开关、侧栏折叠这些跟界面走的东西。 */
import { reactive } from "vue";

interface UiState {
  gameEditorOpen: boolean;
  gameEditorId: number;
  patchEditorOpen: boolean;
  patchEditorId: number;
  patchEditorGameId: number;
  scanOpen: boolean;
  railCollapsed: boolean;
}

export const uiStore = reactive<UiState>({
  gameEditorOpen: false,
  gameEditorId: 0,
  patchEditorOpen: false,
  patchEditorId: 0,
  patchEditorGameId: 0,
  scanOpen: false,
  railCollapsed: false,
});

/** id 为 0 表示新建。 */
export function openGameEditor(id = 0): void {
  uiStore.gameEditorId = id;
  uiStore.gameEditorOpen = true;
}

export function closeGameEditor(): void {
  uiStore.gameEditorOpen = false;
  uiStore.gameEditorId = 0;
}

export function openPatchEditor(options: { patchId?: number; gameId?: number } = {}): void {
  uiStore.patchEditorId = options.patchId ?? 0;
  uiStore.patchEditorGameId = options.gameId ?? 0;
  uiStore.patchEditorOpen = true;
}

export function closePatchEditor(): void {
  uiStore.patchEditorOpen = false;
  uiStore.patchEditorId = 0;
}

export function openScan(): void {
  uiStore.scanOpen = true;
}

export function closeScan(): void {
  uiStore.scanOpen = false;
}
