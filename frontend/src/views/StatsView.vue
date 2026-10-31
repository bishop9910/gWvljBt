<script setup lang="ts">
/**
 * 统计页：纯 CSS 画的图表，不引额外的图表库。
 */
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { NButton, NIcon, NSpin, useMessage } from "naive-ui";
import {
  AlertCircleOutline,
  BarChartOutline,
  HeartOutline,
  ImageOutline,
  StarOutline,
  TimeOutline,
  TrophyOutline,
} from "@vicons/ionicons5";
import { api, errorText, onGamesChanged } from "../lib/api";
import { formatHours, formatPlaytime } from "../lib/format";
import { STATUS_META, kindMeta, type Stats, type PlayStatus } from "../lib/types";
import { refreshLibrary } from "../state/library";

const router = useRouter();
const message = useMessage();

const stats = ref<Stats | null>(null);
const loading = ref(true);

async function load(): Promise<void> {
  loading.value = true;
  try {
    stats.value = await api.games.stats();
  } catch (err) {
    message.error(errorText(err));
    stats.value = null;
  } finally {
    loading.value = false;
  }
}

const statusRows = computed(() => {
  const items = stats.value?.byStatus ?? [];
  const total = items.reduce((sum, item) => sum + item.count, 0) || 1;
  return items
    .map((item) => ({
      label: STATUS_META[item.name as PlayStatus]?.label ?? item.name,
      color: STATUS_META[item.name as PlayStatus]?.color ?? "var(--gv-text-faint)",
      count: item.count,
      percent: Math.round((item.count / total) * 100),
    }))
    .sort((a, b) => b.count - a.count);
});

const timeline = computed(() => {
  const items = [...(stats.value?.timeline ?? [])].sort((a, b) => a.year - b.year);
  const max = Math.max(1, ...items.map((item) => item.total));
  return items.map((item) => ({
    ...item,
    height: Math.max(3, Math.round((item.total / max) * 100)),
    clearedHeight: item.total > 0 ? Math.round((item.cleared / item.total) * 100) : 0,
  }));
});

const developerRows = computed(() => {
  const items = (stats.value?.byDeveloper ?? []).slice(0, 10);
  const max = Math.max(1, ...items.map((item) => item.count));
  return items.map((item) => ({
    name: item.name || "未知",
    count: item.count,
    width: Math.round((item.count / max) * 100),
  }));
});

const patchRows = computed(() => {
  const items = stats.value?.byPatchKind ?? [];
  const total = items.reduce((sum, item) => sum + item.count, 0);
  let cursor = 0;
  const segments = items.map((item) => {
    const percent = total > 0 ? (item.count / total) * 100 : 0;
    const from = cursor;
    cursor += percent;
    return {
      label: kindMeta(item.name).label,
      color: kindMeta(item.name).color,
      count: item.count,
      percent,
      from,
    };
  });
  return { segments, total };
});

const donutStyle = computed(() => {
  const { segments } = patchRows.value;
  if (segments.length === 0) return { background: "var(--gv-surface-3)" };
  const stops = segments.map(
    (segment) => `${segment.color} ${segment.from}% ${segment.from + segment.percent}%`,
  );
  return { background: `conic-gradient(${stops.join(", ")})` };
});

const tagRows = computed(() => {
  const items = (stats.value?.topTags ?? []).slice(0, 24);
  const max = Math.max(1, ...items.map((item) => item.count));
  return items.map((item) => ({
    name: item.name,
    count: item.count,
    size: 11 + Math.round((item.count / max) * 4),
    alpha: 0.4 + (item.count / max) * 0.6,
  }));
});

const kpis = computed(() => {
  const data = stats.value;
  return [
    {
      key: "total",
      label: "收录作品",
      value: data?.total ?? 0,
      suffix: "部",
      icon: BarChartOutline,
      tone: "accent",
    },
    {
      key: "cleared",
      label: "已通关",
      value: data?.clearedCount ?? 0,
      suffix: "部",
      icon: TrophyOutline,
      tone: "success",
    },
    {
      key: "favorite",
      label: "收藏",
      value: data?.favorite ?? 0,
      suffix: "部",
      icon: HeartOutline,
      tone: "danger",
    },
    {
      key: "time",
      label: "总时长",
      value: formatHours(data?.totalPlaytime ?? 0),
      suffix: "",
      icon: TimeOutline,
      tone: "warn",
    },
    {
      key: "rating",
      label: "平均分",
      value: data?.rated ? Number(data.avgRating).toFixed(2) : "—",
      suffix: data?.rated ? `（${data.rated} 部已评分）` : "",
      icon: StarOutline,
      tone: "accent",
    },
  ];
});

onMounted(async () => {
  await load();
  await refreshLibrary();
});

onGamesChanged(() => {
  void load();
});
</script>

<template>
  <div class="stats-view gv-scroll">
    <n-spin :show="loading">
      <!-- KPI -->
      <div class="kpis">
        <div v-for="kpi in kpis" :key="kpi.key" class="kpi" :class="`kpi--${kpi.tone}`">
          <n-icon class="kpi__icon" :size="17" :component="kpi.icon" />
          <strong>{{ kpi.value }}</strong>
          <span class="kpi__label">
            {{ kpi.label }}
            <em v-if="kpi.suffix">{{ kpi.suffix }}</em>
          </span>
        </div>
      </div>

      <div class="grid">
        <!-- 时间轴 -->
        <section class="panel span-2">
          <div class="panel__head">
            <h3>按年份收录</h3>
            <span class="gv-faint">柱子高度 = 该年作品数，深色部分 = 已通关</span>
          </div>

          <div v-if="timeline.length" class="timeline">
            <div v-for="item in timeline" :key="item.year" class="bar-wrap">
              <div class="bar" :style="{ height: `${item.height}%` }">
                <div class="bar__cleared" :style="{ height: `${item.clearedHeight}%` }" />
                <span class="bar__count">{{ item.total }}</span>
              </div>
              <span class="bar__label">{{ item.year }}</span>
            </div>
          </div>
          <p v-else class="gv-faint empty">还没有带发行年份的作品</p>
        </section>

        <!-- 状态分布 -->
        <section class="panel">
          <div class="panel__head">
            <h3>游玩状态</h3>
          </div>

          <div v-if="statusRows.length" class="bars">
            <div v-for="row in statusRows" :key="row.label" class="hbar">
              <span class="hbar__label">
                <i :style="{ background: row.color }" />
                {{ row.label }}
              </span>
              <div class="hbar__track">
                <div
                  class="hbar__fill"
                  :style="{ width: `${row.percent}%`, background: row.color }"
                />
              </div>
              <span class="hbar__value">{{ row.count }}</span>
            </div>
          </div>
          <p v-else class="gv-faint empty">还没有作品</p>
        </section>

        <!-- 会社 -->
        <section class="panel">
          <div class="panel__head">
            <h3>TOP 会社</h3>
          </div>

          <div v-if="developerRows.length" class="bars">
            <div v-for="row in developerRows" :key="row.name" class="hbar">
              <span class="hbar__label hbar__label--text">{{ row.name }}</span>
              <div class="hbar__track">
                <div
                  class="hbar__fill hbar__fill--accent"
                  :style="{ width: `${row.width}%` }"
                />
              </div>
              <span class="hbar__value">{{ row.count }}</span>
            </div>
          </div>
          <p v-else class="gv-faint empty">还没有填写会社的作品</p>
        </section>

        <!-- 补丁构成 -->
        <section class="panel">
          <div class="panel__head">
            <h3>补丁构成</h3>
          </div>

          <div v-if="patchRows.total > 0" class="donut-block">
            <div class="donut" :style="donutStyle">
              <div class="donut__hole">
                <strong>{{ patchRows.total }}</strong>
                <span>个补丁</span>
              </div>
            </div>
            <div class="legend">
              <div v-for="segment in patchRows.segments" :key="segment.label" class="legend__row">
                <i :style="{ background: segment.color }" />
                <span>{{ segment.label }}</span>
                <em>{{ segment.count }}</em>
              </div>
            </div>
          </div>
          <p v-else class="gv-faint empty">还没登记补丁</p>
        </section>

        <!-- 标签云 -->
        <section class="panel span-2">
          <div class="panel__head">
            <h3>高频标签</h3>
          </div>

          <div v-if="tagRows.length" class="tag-cloud">
            <span
              v-for="tag in tagRows"
              :key="tag.name"
              class="tag"
              :style="{ fontSize: `${tag.size}px`, opacity: tag.alpha }"
            >
              {{ tag.name }}
              <em>{{ tag.count }}</em>
            </span>
          </div>
          <p v-else class="gv-faint empty">还没有标签</p>
        </section>

        <!-- 库健康度 -->
        <section class="panel panel--warn">
          <div class="panel__head">
            <h3>
              <n-icon :size="15" :component="AlertCircleOutline" />
              库健康度
            </h3>
          </div>

          <div class="health">
            <button type="button" class="health__item" @click="router.push('/')">
              <n-icon :size="16" :component="ImageOutline" />
              <div>
                <strong>{{ stats?.noPoster ?? 0 }}</strong>
                <span>部作品缺封面</span>
              </div>
            </button>
            <button type="button" class="health__item" @click="router.push('/')">
              <n-icon :size="16" :component="AlertCircleOutline" />
              <div>
                <strong>{{ stats?.missingFolder ?? 0 }}</strong>
                <span>部作品目录不可达</span>
              </div>
            </button>
          </div>

          <p class="health__hint">
            目录不可达通常是移动了游戏文件夹 —— 进作品编辑，重新选一次安装目录就好，补丁记录不会丢。
          </p>
        </section>

        <!-- 最近 -->
        <section v-if="(stats?.recent?.length ?? 0) > 0" class="panel">
          <div class="panel__head">
            <h3>最近更新</h3>
          </div>
          <div class="recent">
            <button
              v-for="item in stats?.recent ?? []"
              :key="item.id"
              type="button"
              class="recent__row"
              @click="router.push(`/game/${item.id}`)"
            >
              <span class="recent__title">{{ item.title }}</span>
              <span class="recent__time">{{ formatPlaytime(item.playtime) }}</span>
            </button>
          </div>
        </section>
      </div>
    </n-spin>

    <div class="footer">
      <n-button quaternary :loading="loading" @click="load">刷新数据</n-button>
    </div>
  </div>
</template>

<style scoped>
.stats-view {
  height: 100%;
  overflow: auto;
  padding: 20px 28px 40px;
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.kpis {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 12px;
}

.kpi {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 14px 16px;
  border: 1px solid var(--gv-border);
  border-radius: var(--gv-radius);
  background: var(--gv-surface);
  overflow: hidden;
}

.kpi::after {
  content: "";
  position: absolute;
  inset: auto -20% -60% auto;
  width: 120px;
  height: 120px;
  border-radius: 50%;
  background: var(--tone, var(--gv-accent));
  opacity: 0.16;
  filter: blur(24px);
}

.kpi--accent {
  --tone: var(--gv-accent);
}
.kpi--success {
  --tone: var(--gv-success);
}
.kpi--danger {
  --tone: var(--gv-danger);
}
.kpi--warn {
  --tone: var(--gv-warn);
}

.kpi__icon {
  color: var(--tone);
  margin-bottom: 2px;
}

.kpi strong {
  font-family: var(--gv-font-display);
  font-size: var(--gv-fs-head);
  font-weight: 700;
  line-height: 1.1;
  letter-spacing: -0.02em;
}

.kpi__label {
  font-size: var(--gv-fs-label);
  color: var(--gv-text-faint);
}

.kpi__label em {
  font-style: normal;
}

.grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
  align-items: start;
}

.span-2 {
  grid-column: span 2;
}

.panel {
  border: 1px solid var(--gv-border);
  border-radius: var(--gv-radius);
  background: var(--gv-surface);
  padding: 16px 18px;
  min-width: 0;
}

.panel--warn {
  border-color: color-mix(in srgb, var(--gv-warn) 28%, transparent);
}

.panel__head {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 14px;
}

.panel__head h3 {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin: 0;
  font-size: var(--gv-fs-body);
  font-weight: 600;
}

.panel__head .gv-faint {
  font-size: var(--gv-fs-label);
  margin-left: auto;
}

.empty {
  margin: 0;
  padding: 16px 0;
  font-size: var(--gv-fs-label);
  text-align: center;
}

.timeline {
  display: flex;
  align-items: flex-end;
  gap: 5px;
  height: 168px;
  padding-top: 10px;
  overflow-x: auto;
}

.bar-wrap {
  flex: 1 1 0;
  min-width: 24px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  height: 100%;
  justify-content: flex-end;
}

.bar {
  position: relative;
  width: 100%;
  max-width: 34px;
  border-radius: 6px 6px 3px 3px;
  background: var(--gv-accent-soft);
  display: flex;
  align-items: flex-start;
  justify-content: center;
  transition: height 0.4s var(--gv-ease);
}

.bar__cleared {
  position: absolute;
  inset: auto 0 0 0;
  border-radius: 0 0 3px 3px;
  background: linear-gradient(180deg, var(--gv-accent), color-mix(in srgb, var(--gv-accent) 60%, transparent));
}

.bar__count {
  position: relative;
  font-family: var(--gv-mono);
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-dim);
  margin-top: -16px;
}

.bar__label {
  font-family: var(--gv-mono);
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-faint);
  white-space: nowrap;
}

.bars {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.hbar {
  display: grid;
  grid-template-columns: 92px minmax(0, 1fr) 30px;
  align-items: center;
  gap: 8px;
}

.hbar__label {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: var(--gv-fs-label);
  color: var(--gv-text-dim);
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.hbar__label i {
  flex: 0 0 auto;
  width: 6px;
  height: 6px;
  border-radius: 1px;
}

.hbar__label--text {
  font-size: var(--gv-fs-label);
}

.hbar__track {
  height: 6px;
  border-radius: 1px;
  background: var(--gv-surface-3);
  overflow: hidden;
}

.hbar__fill {
  height: 100%;
  border-radius: 1px;
  transition: width 0.4s var(--gv-ease);
  opacity: 0.85;
}

.hbar__fill--accent {
  background: linear-gradient(90deg, var(--gv-accent), color-mix(in srgb, var(--gv-accent) 40%, transparent));
}

.hbar__value {
  font-family: var(--gv-mono);
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-faint);
  text-align: right;
}

.donut-block {
  display: flex;
  align-items: center;
  gap: 18px;
}

.donut {
  position: relative;
  flex: 0 0 auto;
  width: 118px;
  height: 118px;
  border-radius: 50%;
  display: grid;
  place-items: center;
}

.donut__hole {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: 78px;
  height: 78px;
  border-radius: 50%;
  background: var(--gv-surface);
  gap: 1px;
}

.donut__hole strong {
  font-family: var(--gv-font-display);
  font-size: var(--gv-fs-subhead);
  font-weight: 700;
}

.donut__hole span {
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-faint);
}

.legend {
  display: flex;
  flex-direction: column;
  gap: 5px;
  min-width: 0;
}

.legend__row {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: var(--gv-fs-label);
  color: var(--gv-text-dim);
}

.legend__row i {
  width: 7px;
  height: 7px;
  border-radius: 2px;
  flex: 0 0 auto;
}

.legend__row em {
  font-style: normal;
  font-family: var(--gv-mono);
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-faint);
  margin-left: auto;
}

.tag-cloud {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 12px;
  align-items: baseline;
}

.tag {
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
  color: var(--gv-accent);
  cursor: default;
}

.tag em {
  font-style: normal;
  font-family: var(--gv-mono);
  font-size: var(--gv-fs-caption);
  color: var(--gv-text-faint);
}

.health {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.health__item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border: 1px solid var(--gv-border);
  border-radius: var(--gv-radius-sm);
  background: var(--gv-surface-2);
  color: var(--gv-text-dim);
  text-align: left;
  cursor: pointer;
}

.health__item:hover {
  border-color: var(--gv-border-strong);
  color: var(--gv-text);
}

.health__item div {
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.health__item strong {
  font-family: var(--gv-font-display);
  font-size: var(--gv-fs-subhead);
  font-weight: 700;
  color: var(--gv-warn);
}

.health__item span {
  font-size: var(--gv-fs-label);
}

.health__hint {
  margin: 10px 0 0;
  font-size: var(--gv-fs-label);
  line-height: 1.6;
  color: var(--gv-text-faint);
}

.recent {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.recent__row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 7px;
  border: none;
  border-radius: var(--gv-radius-sm);
  background: transparent;
  color: var(--gv-text-dim);
  font-size: var(--gv-fs-label);
  text-align: left;
  cursor: pointer;
}

.recent__row:hover {
  background: var(--gv-surface-2);
  color: var(--gv-text);
}

.recent__title {
  flex: 1 1 auto;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.recent__time {
  font-family: var(--gv-font);
  font-size: var(--gv-fs-caption);
  font-variant-numeric: tabular-nums;
  color: var(--gv-text-faint);
}

.footer {
  display: flex;
  justify-content: flex-end;
}
</style>
