<script setup lang="ts">
/**
 * TouchGal 找资源：搜作品 → 看资源 → 拿网盘链接。
 *
 * 站点的所有接口都在 Cloudflare 后面，所以设置里那段 cookie 失效时这里会直接
 * 把 403 的原因显示出来，而不是静默空列表。
 */
import { computed, nextTick, ref, watch } from "vue";
import { NButton, NIcon, NInput, NModal, useMessage } from "naive-ui";
import {
  CopyOutline,
  OpenOutline,
  SearchOutline,
} from "@vicons/ionicons5";
import { api, errorText } from "../lib/api";
import { configStore } from "../state/config";
import { closeTouchgal, uiStore } from "../state/ui";
import type { TouchGalHit, TouchGalResource } from "../../bindings/gWvljBt/models";

const message = useMessage();

const query = ref("");
const inputEl = ref<{ focus?: () => void } | null>(null);
const loading = ref(false);
const searched = ref(false);
const hits = ref<TouchGalHit[]>([]);
const activeId = ref(0);
const resLoading = ref(false);
const resources = ref<TouchGalResource[]>([]);
const error = ref("");

const hasCookie = computed(() => !!configStore.data.touchgalCookie);

async function search(): Promise<void> {
  const q = query.value.trim();
  if (!q) {
    message.warning("先填一个标题");
    return;
  }
  loading.value = true;
  error.value = "";
  activeId.value = 0;
  resources.value = [];
  try {
    hits.value = (await api.touchgal.search(q, 12)) ?? [];
    searched.value = true;
  } catch (err) {
    hits.value = [];
    searched.value = true;
    error.value = errorText(err);
  } finally {
    loading.value = false;
  }
}

async function loadResources(hit: TouchGalHit): Promise<void> {
  activeId.value = hit.id;
  resLoading.value = true;
  error.value = "";
  resources.value = [];
  try {
    resources.value = (await api.touchgal.resources(hit.id)) ?? [];
  } catch (err) {
    error.value = errorText(err);
  } finally {
    resLoading.value = false;
  }
}

async function openPage(url: string): Promise<void> {
  if (!url) return;
  try {
    await api.games.openUrl(url);
  } catch (err) {
    message.error(errorText(err));
  }
}

async function copy(text: string): Promise<void> {
  const value = (text ?? "").trim();
  if (!value) return;
  try {
    await navigator.clipboard.writeText(value);
    message.success("已复制");
  } catch {
    // WebView 里偶尔拿不到剪贴板权限，退回到手动选择。
    message.warning(`复制失败，手动选中：${value}`);
  }
}

function openSite(): void {
  void openPage(configStore.data.touchgalUrl || "https://www.touchgal.ink/");
}

watch(
  () => uiStore.touchgalOpen,
  (open) => {
    if (!open) return;
    query.value = uiStore.touchgalTitle || "";
    hits.value = [];
    resources.value = [];
    searched.value = false;
    error.value = "";
    activeId.value = 0;
    // 标题已经有了就直接搜一次，省一次点击。
    if (query.value) void search();
    // 从顶栏进来时没有预填标题，把光标放进输入框。
    else void nextTick(() => inputEl.value?.focus?.());
  },
);
</script>

<template>
  <n-modal
    :show="uiStore.touchgalOpen"
    :mask-closable="true"
    :auto-focus="false"
    @update:show="(value: boolean) => !value && closeTouchgal()"
  >
    <div class="tg">
      <header class="tg__head">
        <div>
          <h2 class="tg__title">找资源</h2>
          <p class="tg__note">
            链接来自 TouchGal，本站只做跳转；网盘提取码会一起列出来。
          </p>
        </div>
        <button type="button" class="tg__close" @click="closeTouchgal">关闭</button>
      </header>

      <div class="tg__bar">
        <n-input
          ref="inputEl"
          v-model:value="query"
          placeholder="输入作品标题，回车搜索"
          :disabled="loading"
          @keyup.enter="search"
        />
        <n-button type="primary" :loading="loading" @click="search">
          <template #icon>
            <n-icon :component="SearchOutline" />
          </template>
          搜索
        </n-button>
        <n-button quaternary @click="openSite">
          <template #icon>
            <n-icon :component="OpenOutline" />
          </template>
          打开站点
        </n-button>
      </div>

      <p v-if="!hasCookie" class="tg__warn">
        还没填 TouchGal Cookie。Cloudflare 会把人机验证页丢回来 —— 先去
        <a class="tg__link" @click="closeTouchgal">设置 → 元数据 → TouchGal</a>
        里粘贴一次 cookie。
      </p>

      <p v-if="error" class="tg__error">{{ error }}</p>

      <div class="tg__body">
        <section class="tg__col">
          <h3 class="tg__label">作品</h3>
          <div v-if="loading" class="tg__empty">搜索中…</div>
          <div v-else-if="!hits.length" class="tg__empty">
            {{ searched ? "没有匹配的作品" : "输入标题后回车" }}
          </div>
          <ul v-else class="tg__hits">
            <li
              v-for="hit in hits"
              :key="hit.id"
              class="hit"
              :class="{ 'is-active': hit.id === activeId }"
            >
              <button type="button" class="hit__main" @click="loadResources(hit)">
                <img v-if="hit.banner" class="hit__banner" :src="hit.banner" alt="" />
                <span class="hit__text">
                  <span class="hit__name">{{ hit.name }}</span>
                  <span class="hit__meta">
                    {{ [hit.language, hit.platform].filter(Boolean).join(" · ") || "—" }}
                  </span>
                </span>
                <span v-if="hit.rating" class="hit__rate">{{ hit.rating }}</span>
              </button>
              <button type="button" class="hit__open" title="在浏览器打开" @click="openPage(hit.pageUrl)">
                <n-icon :component="OpenOutline" />
              </button>
            </li>
          </ul>
        </section>

        <section class="tg__col tg__col--res">
          <h3 class="tg__label">资源</h3>
          <div v-if="resLoading" class="tg__empty">读取中…</div>
          <div v-else-if="!resources.length" class="tg__empty">
            {{ activeId ? "这个作品下没有公开资源" : "左侧点一个作品" }}
          </div>
          <ul v-else class="tg__res">
            <li v-for="res in resources" :key="res.id" class="res">
              <div class="res__head">
                <span class="res__name">{{ res.name || res.section || "资源" }}</span>
                <span v-if="res.type" class="tag">{{ res.type }}</span>
                <span v-if="res.language" class="tag">{{ res.language }}</span>
              </div>
              <p v-if="res.note" class="res__note">{{ res.note }}</p>
              <ul class="res__links">
                <li v-for="(link, index) in res.links ?? []" :key="index" class="link">
                  <span class="link__storage">{{ link.storage || "网盘" }}</span>
                  <span v-if="link.size" class="link__size">{{ link.size }}</span>
                  <span v-if="link.code" class="link__code">提取码 {{ link.code }}</span>
                  <span v-if="link.password" class="link__code">密码 {{ link.password }}</span>
                  <span class="link__acts">
                    <button type="button" class="mini" @click="openPage(link.url)">
                      <n-icon :component="OpenOutline" />打开
                    </button>
                    <button type="button" class="mini" @click="copy(link.url)">
                      <n-icon :component="CopyOutline" />复制
                    </button>
                  </span>
                </li>
              </ul>
            </li>
          </ul>
        </section>
      </div>
    </div>
  </n-modal>
</template>

<style scoped>
.tg {
  width: min(900px, 94vw);
  max-height: 84vh;
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 18px 20px 16px;
  background: var(--gv-surface);
  border: 1px solid var(--gv-border-strong);
  border-radius: var(--gv-radius);
  box-shadow: var(--gv-shadow-2);
}

.tg__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--gv-border);
}

.tg__title {
  margin: 0;
  font-family: var(--gv-font-display);
  font-size: var(--gv-fs-subhead);
  font-weight: 600;
}

.tg__note {
  margin: 4px 0 0;
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-faint);
}

.tg__close {
  padding: 0;
  border: 0;
  background: transparent;
  font-family: inherit;
  font-size: var(--gv-fs-label);
  color: var(--gv-text-faint);
  cursor: pointer;
}

.tg__close:hover {
  color: var(--gv-accent);
}

.tg__bar {
  display: flex;
  align-items: center;
  gap: 8px;
}

.tg__warn,
.tg__error {
  margin: 0;
  padding: 7px 10px;
  border-left: 2px solid var(--gv-warn);
  background: color-mix(in srgb, var(--gv-warn) 8%, transparent);
  font-size: var(--gv-fs-label);
  line-height: 1.6;
  color: var(--gv-text-dim);
}

.tg__error {
  border-left-color: var(--gv-danger);
  background: color-mix(in srgb, var(--gv-danger) 8%, transparent);
}

.tg__link {
  color: var(--gv-accent);
  cursor: pointer;
  border-bottom: 1px solid var(--gv-accent-line);
}

.tg__body {
  flex: 1 1 auto;
  min-height: 0;
  display: grid;
  grid-template-columns: minmax(240px, 0.9fr) minmax(280px, 1.1fr);
  gap: 14px;
}

.tg__col {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-height: 0;
}

.tg__label {
  margin: 0;
  font-family: var(--gv-font);
  font-size: var(--gv-fs-caption);
  font-weight: 500;
  letter-spacing: 0.02em;
  color: var(--gv-text-faint);
}

.tg__empty {
  padding: 22px 4px;
  font-size: var(--gv-fs-label);
  color: var(--gv-text-faint);
  border-top: 1px solid var(--gv-border);
}

.tg__hits,
.tg__res {
  margin: 0;
  padding: 0;
  list-style: none;
  overflow: auto;
  border-top: 1px solid var(--gv-border);
}

.hit {
  display: flex;
  align-items: stretch;
  gap: 4px;
  border-bottom: 1px solid var(--gv-border);
}

.hit.is-active {
  background: var(--gv-surface-2);
}

.hit__main {
  flex: 1 1 auto;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 4px;
  border: 0;
  background: transparent;
  text-align: left;
  cursor: pointer;
}

.hit__banner {
  flex: 0 0 auto;
  width: 46px;
  height: 26px;
  object-fit: cover;
  border: 1px solid var(--gv-border);
}

.hit__text {
  flex: 1 1 auto;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.hit__name {
  font-family: var(--gv-font-display);
  font-size: var(--gv-fs-body);
  font-weight: 600;
  color: var(--gv-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.hit__meta {
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-faint);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.hit__rate {
  flex: 0 0 auto;
  font-family: var(--gv-mono);
  font-size: var(--gv-fs-caption);
  color: var(--gv-accent);
}

.hit__open {
  flex: 0 0 auto;
  width: 26px;
  border: 0;
  background: transparent;
  color: var(--gv-text-faint);
  cursor: pointer;
}

.hit__open:hover {
  color: var(--gv-accent);
}

.res {
  padding: 10px 2px;
  border-bottom: 1px solid var(--gv-border);
}

.res__head {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.res__name {
  font-family: var(--gv-font);
  font-size: var(--gv-fs-body);
  color: var(--gv-text);
}

.tag {
  padding: 1px 5px;
  border: 1px solid var(--gv-border);
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-faint);
}

.res__note {
  margin: 5px 0 0;
  font-size: var(--gv-fs-caption);
  line-height: 1.6;
  color: var(--gv-text-dim);
}

.res__links {
  margin: 6px 0 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.link {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  padding: 5px 8px;
  border: 1px solid var(--gv-border);
}

.link__storage {
  font-size: var(--gv-fs-caption);
  color: var(--gv-text);
}

.link__size,
.link__code {
  font-family: var(--gv-mono);
  font-size: var(--gv-fs-caption);
  font-variant-numeric: tabular-nums;
  color: var(--gv-text-faint);
}

.link__acts {
  margin-left: auto;
  display: inline-flex;
  gap: 6px;
}

.mini {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 2px 6px;
  border: 1px solid transparent;
  background: transparent;
  font-family: inherit;
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-dim);
  cursor: pointer;
}

.mini:hover {
  color: var(--gv-accent);
  border-color: var(--gv-accent-line);
}
</style>
