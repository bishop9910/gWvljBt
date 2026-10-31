<script setup lang="ts">
/**
 * 设置页：外观（预设纸墨 / 强调色 / 壁纸 / 版式）、数据源、数据目录。
 * 所有改动都走 patchConfig —— 本地立即生效，同时写回 config.json。
 */
import { computed, onMounted, ref } from "vue";
import { NButton, NIcon, NInput, NPopconfirm, NSlider, NSwitch, NTooltip, useMessage } from "naive-ui";
import {
  CheckmarkOutline,
  FolderOpenOutline,
  ImagesOutline,
  RefreshOutline,
} from "@vicons/ionicons5";
import { api, errorText } from "../lib/api";
import { ACCENTS, POSTER_ASPECTS, PRESETS, accentOf } from "../lib/theme";
import { configStore, patchConfig, resetConfig } from "../state/config";

const message = useMessage();
const dataDir = ref("");
const pinging = ref("");
const picking = ref(false);

const cfg = computed(() => configStore.data);
const effectiveAccent = computed(() => accentOf(cfg.value));
const wallName = computed(() => {
  const path = cfg.value.wallpaper ?? "";
  if (!path) return "";
  const parts = path.split("/");
  return parts[parts.length - 1] || path;
});

const densityOptions = [
  { label: "舒展", value: "cozy" },
  { label: "紧凑", value: "compact" },
];

const sourceOptions = [
  { label: "VNDB", value: "vndb" },
  { label: "Bangumi", value: "bangumi" },
];

const posterOptions = POSTER_ASPECTS.map((item) => ({ label: item.label, value: item.value }));

function setAccent(value: string): void {
  void patchConfig("accent", value);
}

async function pickWallpaper(): Promise<void> {
  picking.value = true;
  try {
    const path = await api.config.pickWallpaper();
    if (path) message.success("壁纸已换");
  } catch (err) {
    message.error(errorText(err));
  } finally {
    picking.value = false;
  }
}

async function clearWallpaper(): Promise<void> {
  try {
    await api.config.clearWallpaper();
  } catch (err) {
    message.error(errorText(err));
  }
}

async function ping(source: string): Promise<void> {
  pinging.value = source;
  try {
    const result = await api.meta.ping(source);
    message.success(result || `${source} 连接正常`);
  } catch (err) {
    message.error(errorText(err));
  } finally {
    pinging.value = "";
  }
}

async function openDataDir(): Promise<void> {
  try {
    await api.config.openDataDir();
  } catch (err) {
    message.error(errorText(err));
  }
}

async function openConfigFile(): Promise<void> {
  try {
    await api.config.openConfigFile();
  } catch (err) {
    message.error(errorText(err));
  }
}

async function doReset(): Promise<void> {
  try {
    await resetConfig();
    message.success("已恢复默认设置");
  } catch (err) {
    message.error(errorText(err));
  }
}

onMounted(async () => {
  try {
    dataDir.value = await api.config.dataDir();
  } catch {
    dataDir.value = "";
  }
});
</script>

<template>
  <div class="settings gv-scroll">
    <!-- 外观 -->
    <section class="block">
      <div class="block__head">
        <h2 class="block__title">外观</h2>
        <!-- <p class="block__note">改完立刻生效，同时写进 config.json</p> -->
      </div>

      <div class="field field--stack">
        <span class="field__label">整本样式</span>
        <div class="books">
          <button
            v-for="preset in PRESETS"
            :key="preset.key"
            type="button"
            class="book"
            :class="{ 'is-active': cfg.preset === preset.key }"
            @click="patchConfig('preset', preset.key)"
          >
            <span class="book__bands">
              <i :style="{ background: preset.palette.bg }" />
              <i :style="{ background: preset.palette.surface2 }" />
              <i :style="{ background: preset.palette.accent }" />
            </span>
            <span class="book__name">
              {{ preset.label }}
              <n-icon
                v-if="cfg.preset === preset.key"
                :size="11"
                :component="CheckmarkOutline"
                class="book__tick"
              />
            </span>
            <span class="book__note">{{ preset.note }}</span>
          </button>
        </div>
      </div>

      <div class="field">
        <span class="field__label">强调色</span>
        <div class="swatches">
          <button
            v-for="item in ACCENTS"
            :key="item.value"
            type="button"
            class="swatch"
            :class="{ 'is-active': effectiveAccent.toLowerCase() === item.value.toLowerCase() }"
            :style="{ '--swatch': item.value }"
            :title="item.label"
            @click="setAccent(item.value)"
          />
          <label class="swatch swatch--custom" title="自定义颜色">
            <input type="color" :value="effectiveAccent" @input="setAccent(($event.target as HTMLInputElement).value)" />
          </label>
          <button v-if="cfg.accent" type="button" class="swatch-reset" @click="setAccent('')">
            跟随样式
          </button>
        </div>
      </div>

      <div class="field">
        <span class="field__label">壁纸</span>
        <div class="field__body">
          <div class="wallrow">
            <n-button size="small" :loading="picking" @click="pickWallpaper">
              <template #icon>
                <n-icon :component="ImagesOutline" />
              </template>
              选择图片
            </n-button>
            <n-button v-if="cfg.wallpaper" size="small" quaternary @click="clearWallpaper">
              移除
            </n-button>
            <span class="wallrow__name">{{ wallName || "未设置，使用纸张颗粒底" }}</span>
          </div>

          <div class="slider">
            <span class="slider__label">模糊 <em>{{ cfg.wallpaperBlur }}px</em></span>
            <n-slider
              :value="cfg.wallpaperBlur"
              :min="0"
              :max="40"
              :step="1"
              :disabled="!cfg.wallpaper"
              @update:value="(value: number) => patchConfig('wallpaperBlur', value)"
            />
          </div>

          <div class="slider">
            <span class="slider__label">压色 <em>{{ cfg.wallpaperVeil }}%</em></span>
            <n-slider
              :value="cfg.wallpaperVeil"
              :min="0"
              :max="95"
              :step="1"
              :disabled="!cfg.wallpaper"
              @update:value="(value: number) => patchConfig('wallpaperVeil', value)"
            />
          </div>
        </div>
      </div>

      <div class="field">
        <span class="field__label">圆角 <em>{{ cfg.radius }}px</em></span>
        <n-slider
          :value="cfg.radius"
          :min="0"
          :max="24"
          :step="1"
          style="max-width: 320px"
          @update:value="(value: number) => patchConfig('radius', value)"
        />
      </div>

      <div class="field">
        <span class="field__label">密度</span>
        <div class="seg">
          <button
            v-for="option in densityOptions"
            :key="option.value"
            type="button"
            class="seg__item"
            :class="{ 'is-active': cfg.density === option.value }"
            @click="patchConfig('density', option.value)"
          >
            {{ option.label }}
          </button>
        </div>
      </div>

      <div class="field">
        <span class="field__label">海报比例</span>
        <div class="seg">
          <button
            v-for="option in posterOptions"
            :key="option.value"
            type="button"
            class="seg__item"
            :class="{ 'is-active': cfg.posterAspect === option.value }"
            @click="patchConfig('posterAspect', option.value)"
          >
            {{ option.label }}
          </button>
        </div>
      </div>

      <div class="field">
        <span class="field__label">动效</span>
        <div class="field__body field__body--inline">
          <n-switch
            :value="cfg.motion"
            @update:value="(value: boolean) => patchConfig('motion', value)"
          />
          <span class="hint">关掉之后所有过渡与入场动画立即停止</span>
        </div>
      </div>
    </section>

    <!-- 数据源 -->
    <section class="block">
      <div class="block__head">
        <h2 class="block__title">元数据</h2>
        <!-- <p class="block__note">抓取标题、封面、会社、标签这些信息时用</p> -->
      </div>

      <div class="field">
        <span class="field__label">默认来源</span>
        <div class="field__body field__body--inline">
          <div class="seg">
            <button
              v-for="option in sourceOptions"
              :key="option.value"
              type="button"
              class="seg__item"
              :class="{ 'is-active': cfg.defaultSource === option.value }"
              @click="patchConfig('defaultSource', option.value)"
            >
              {{ option.label }}
            </button>
          </div>
          <n-button text :loading="pinging === 'vndb'" @click="ping('vndb')">测试 VNDB</n-button>
          <n-button text :loading="pinging === 'bangumi'" @click="ping('bangumi')">
            测试 Bangumi
          </n-button>
        </div>
      </div>

      <div class="field">
        <span class="field__label">VNDB Token</span>
        <n-input
          :value="cfg.vndbToken"
          type="password"
          show-password-on="click"
          placeholder="可选，填了配额更高"
          style="max-width: 360px"
          @update:value="(value: string) => patchConfig('vndbToken', value)"
        />
      </div>

      <div class="field">
        <span class="field__label">Bangumi Token</span>
        <n-input
          :value="cfg.bangumiToken"
          type="password"
          show-password-on="click"
          placeholder="可选"
          style="max-width: 360px"
          @update:value="(value: string) => patchConfig('bangumiToken', value)"
        />
      </div>

      <div class="field">
        <span class="field__label">补丁站点</span>
        <n-input
          :value="cfg.twoDFanUrl"
          placeholder="https://www.2dfan.com/"
          style="max-width: 360px"
          @update:value="(value: string) => patchConfig('twoDFanUrl', value)"
        />
      </div>

      <p class="note">
        本工具不抓取、也不提供补丁文件，只记录「哪个补丁动了哪些文件」，方便以后一键清理。
      </p>
    </section>

    <!-- 数据 -->
    <section class="block">
      <div class="block__head">
        <h2 class="block__title">数据</h2>
        <!-- <p class="block__note">全部存在本地，不上传任何东西</p> -->
      </div>

      <div class="field">
        <span class="field__label">数据目录</span>
        <div class="field__body field__body--inline">
          <code class="path">{{ dataDir || "读取中…" }}</code>
          <n-button size="small" @click="openDataDir">
            <template #icon>
              <n-icon :component="FolderOpenOutline" />
            </template>
            打开
          </n-button>
        </div>
      </div>

      <div class="field">
        <span class="field__label">配置文件</span>
        <div class="field__body field__body--inline">
          <code class="path">config.json</code>
          <n-button size="small" text @click="openConfigFile">编辑</n-button>
        </div>
      </div>

      <div class="field">
        <span class="field__label">重置</span>
        <n-popconfirm @positive-click="doReset">
          <template #trigger>
            <n-button size="small" quaternary>
              <template #icon>
                <n-icon :component="RefreshOutline" />
              </template>
              恢复默认设置
            </n-button>
          </template>
          只重置 config.json 里的外观与来源设置，游戏库和补丁记录不受影响。
        </n-popconfirm>
      </div>

    </section>

    <section class="block block--quiet">
      <p class="about">
        <strong>gWvljBt</strong> — 本地 galgame 管理。元数据来自
        <n-tooltip trigger="hover">
          <template #trigger>
            <span class="about__link" @click="api.games.openUrl('https://vndb.org')">VNDB</span>
          </template>
          视觉小说数据库
        </n-tooltip>
        与
        <n-tooltip trigger="hover">
          <template #trigger>
            <span class="about__link" @click="api.games.openUrl('https://bgm.tv')">Bangumi</span>
          </template>
          番组计划
        </n-tooltip>
        ；补丁请自行到
        <n-tooltip trigger="hover">
          <template #trigger>
            <span class="about__link" @click="api.games.openUrl(cfg.twoDFanUrl)">2DFan</span>
          </template>
          中文补丁站
        </n-tooltip>
        下载。
      </p>
    </section>
  </div>
</template>

<style scoped>
.settings {
  height: 100%;
  overflow: auto;
  padding: 26px 30px 56px;
  display: flex;
  flex-direction: column;
  gap: 34px;
  max-width: 940px;
}

/* 区块：只有一条上边的发丝线，没有卡片壳 */
.block {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.block__head {
  display: flex;
  align-items: baseline;
  gap: 12px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--gv-border);
}

.block__title {
  margin: 0;
  font-family: var(--gv-font-display);
  font-size: var(--gv-fs-subhead);
  font-weight: 600;
  letter-spacing: var(--gv-track-display);
}

.block__note {
  margin: 0;
  font-size: var(--gv-fs-label);
  color: var(--gv-text-faint);
}

.block--quiet {
  border-top: 1px solid var(--gv-border);
  padding-top: 18px;
}

.field {
  display: flex;
  align-items: center;
  gap: 18px;
  flex-wrap: wrap;
}

.field--stack {
  align-items: flex-start;
}

.field__label {
  flex: 0 0 82px;
  display: inline-flex;
  align-items: baseline;
  gap: 5px;
  font-size: var(--gv-fs-label);
  letter-spacing: var(--gv-track-label);
  color: var(--gv-text-dim);
}

.field__label em {
  font-style: normal;
  font-family: var(--gv-mono);
  font-size: var(--gv-fs-caption);
  font-variant-numeric: tabular-nums;
  color: var(--gv-accent);
}

.field__body {
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
}

.field__body--inline {
  flex-direction: row;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

/* ---------------- 预设样式卡 ---------------- */

.books {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(132px, 1fr));
  gap: 10px;
  flex: 1 1 auto;
}

.book {
  display: flex;
  flex-direction: column;
  gap: 5px;
  padding: 0 0 9px;
  border: 1px solid var(--gv-border);
  background: transparent;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.2s var(--gv-ease), transform 0.2s var(--gv-ease);
}

.book:hover {
  border-color: var(--gv-border-strong);
}

.book.is-active {
  border-color: var(--gv-accent-line);
}

.book__bands {
  display: flex;
  height: 34px;
}

.book__bands i {
  flex: 1 1 auto;
}

.book__bands i:last-child {
  flex: 0 0 26%;
}

.book__name {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 0 8px;
  font-family: var(--gv-font);
  font-size: var(--gv-fs-body);
  font-weight: 600;
  letter-spacing: var(--gv-track-label);
  color: var(--gv-text);
}

.book__tick {
  color: var(--gv-accent);
}

.book__note {
  padding: 0 8px;
  font-size: var(--gv-fs-caption);
  line-height: 1.5;
  color: var(--gv-text-faint);
}

/* ---------------- 强调色 ---------------- */

.swatches {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.swatch {
  position: relative;
  width: 22px;
  height: 22px;
  padding: 0;
  border: 1px solid var(--gv-border-strong);
  border-radius: 50%;
  background: var(--swatch);
  cursor: pointer;
  transition: transform 0.16s var(--gv-ease), box-shadow 0.16s var(--gv-ease);
}

.swatch:hover {
  transform: scale(1.1);
}

.swatch.is-active {
  box-shadow: 0 0 0 2px var(--gv-bg), 0 0 0 3px var(--swatch);
}

.swatch--custom {
  overflow: hidden;
  background: conic-gradient(from 0deg, #c05a4e, #b8495f, #7b6aa8, #4a6b8a, #4d8089, #5c8a63, #a8873f, #c05a4e);
  border-color: transparent;
}

.swatch--custom input {
  position: absolute;
  inset: -6px;
  width: 40px;
  height: 40px;
  padding: 0;
  border: 0;
  opacity: 0;
  cursor: pointer;
}

.swatch-reset {
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--gv-text-faint);
  font-family: inherit;
  font-size: var(--gv-fs-label);
  cursor: pointer;
  border-bottom: 1px solid var(--gv-border-strong);
}

.swatch-reset:hover {
  color: var(--gv-accent);
}

/* ---------------- 壁纸 ---------------- */

.wallrow {
  display: flex;
  align-items: center;
  gap: 9px;
  flex-wrap: wrap;
}

.wallrow__name {
  font-family: var(--gv-mono);
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-faint);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 320px;
}

.slider {
  display: flex;
  align-items: center;
  gap: 12px;
}

.slider__label {
  flex: 0 0 78px;
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
  font-size: var(--gv-fs-label);
  color: var(--gv-text-faint);
}

.slider__label em {
  font-style: normal;
  font-family: var(--gv-mono);
  font-size: var(--gv-fs-caption);
  font-variant-numeric: tabular-nums;
  color: var(--gv-text-dim);
}

.slider :deep(.n-slider) {
  flex: 1 1 auto;
  max-width: 300px;
}

/* ---------------- 分段 / 其它 ---------------- */

.seg {
  display: flex;
  gap: 1px;
  border: 1px solid var(--gv-border);
  border-radius: var(--gv-radius-sm);
  overflow: hidden;
}

.seg__item {
  padding: 5px 12px;
  border: 0;
  background: transparent;
  color: var(--gv-text-dim);
  font-family: inherit;
  font-size: var(--gv-fs-label);
  cursor: pointer;
  transition: color 0.16s var(--gv-ease), background 0.16s var(--gv-ease);
}

.seg__item:hover {
  color: var(--gv-text);
  background: var(--gv-surface-2);
}

.seg__item.is-active {
  background: var(--gv-accent-soft);
  color: var(--gv-accent);
}

.hint {
  font-size: var(--gv-fs-label);
  color: var(--gv-text-faint);
}

.path {
  max-width: 340px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  padding: 3px 7px;
  border: 1px solid var(--gv-border);
  font-family: var(--gv-mono);
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-dim);
}

.note {
  margin: 0;
  font-size: var(--gv-fs-label);
  line-height: 1.7;
  color: var(--gv-text-faint);
  max-width: 640px;
}

.about {
  margin: 0;
  font-size: var(--gv-fs-body);
  line-height: 1.9;
  color: var(--gv-text-dim);
}

.about strong {
  color: var(--gv-text);
}

.about__link {
  color: var(--gv-accent);
  cursor: pointer;
  border-bottom: 1px solid var(--gv-accent-line);
}
</style>
