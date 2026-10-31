<script setup lang="ts">
import { computed, onMounted } from "vue";
import {
  NConfigProvider,
  NDialogProvider,
  NGlobalStyle,
  NMessageProvider,
  darkTheme,
  dateZhCN,
  lightTheme,
  zhCN,
} from "naive-ui";
import MainWindow from "./MainWindow.vue";
import { buildOverrides, isDark } from "./lib/theme";
import { configStore, initConfig } from "./state/config";
import { initLibrary } from "./state/library";

const theme = computed(() => (isDark(configStore.data) ? darkTheme : lightTheme));
const overrides = computed(() => buildOverrides(configStore.data));

const wallStyle = computed(() => {
  const url = configStore.data.wallpaper;
  if (!url) return null;
  const blur = Math.min(40, Math.max(0, Number(configStore.data.wallpaperBlur) || 0));
  return {
    backgroundImage: `url("${url}")`,
    filter: blur > 0 ? `blur(${blur}px)` : "none",
  };
});

const veilStyle = computed(() => {
  const veil = Math.min(95, Math.max(0, Number(configStore.data.wallpaperVeil) || 0));
  return { opacity: veil / 100 };
});

onMounted(async () => {
  await initConfig();
  await initLibrary();
});
</script>

<template>
  <n-config-provider
    abstract
    :theme="theme"
    :theme-overrides="overrides"
    :locale="zhCN"
    :date-locale="dateZhCN"
  >
    <n-global-style />
    <n-message-provider :max="3">
      <n-dialog-provider>
        <div v-if="wallStyle" class="gv-wall" :style="wallStyle" aria-hidden="true" />
        <div v-if="wallStyle" class="gv-wall__veil" :style="veilStyle" aria-hidden="true" />
        <main-window />
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>
