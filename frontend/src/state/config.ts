/** 全局配置状态：单例，读写都过这里，写完立刻应用到 CSS 变量。 */
import { reactive } from "vue";
import type { ConfigData } from "../../bindings/gWvljBt/config/models";
import { api, errorText, onConfigChanged } from "../lib/api";
import { applyTheme } from "../lib/theme";
import { DEFAULT_CONFIG } from "../lib/types";

interface ConfigState {
  data: ConfigData;
  ready: boolean;
  error: string;
}

export const configStore = reactive<ConfigState>({
  data: { ...DEFAULT_CONFIG },
  ready: false,
  error: "",
});

let wired = false;

function adopt(next: ConfigData | null | undefined) {
  if (!next) return;
  configStore.data = next;
  applyTheme(next);
}

/** 应用启动时调一次：拉配置 + 订阅后端变更。 */
export async function initConfig(): Promise<void> {
  applyTheme(configStore.data);

  if (!wired) {
    wired = true;
    onConfigChanged((cfg) => adopt(cfg));
  }

  try {
    const cfg = await api.config.get();
    adopt(cfg);
    configStore.ready = true;
  } catch (err) {
    configStore.error = errorText(err);
  }
}

/**
 * 改一个字段：先本地生效（界面不等网络），再落库；
 * 后端会回发 config:changed 做最终对齐。
 */
export async function patchConfig<K extends keyof ConfigData>(
  key: K,
  value: ConfigData[K],
): Promise<void> {
  const next = { ...configStore.data, [key]: value } as ConfigData;
  adopt(next);
  try {
    await api.config.change(key as string, value);
  } catch (err) {
    configStore.error = errorText(err);
    throw err;
  }
}

export async function resetConfig(): Promise<void> {
  await api.config.reset();
  adopt(await api.config.get());
}
