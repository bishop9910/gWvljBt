<script setup lang="ts">
/** 海报/横幅：有图就显示图，没图用一套确定性渐变兜底，永远不留空洞。 */
import { computed, ref, watch } from "vue";

const props = withDefaults(
  defineProps<{
    src?: string | null;
    title?: string | null;
    ratio?: string;
    rounded?: string;
    fit?: "cover" | "contain";
  }>(),
  { src: "", title: "", ratio: "var(--gv-poster-ratio)", rounded: "var(--gv-radius-sm)", fit: "cover" },
);

const broken = ref(false);

watch(
  () => props.src,
  () => {
    broken.value = false;
  },
);

const visible = computed(() => !!props.src && !broken.value);
const initial = computed(() => (props.title ?? "?").trim().slice(0, 1).toUpperCase() || "?");

/** 用标题算个稳定的色相，同一部作品每次都是同一个底色。 */
const shade = computed(() => {
  let hash = 0;
  for (const ch of props.title ?? "") hash = (hash * 31 + ch.charCodeAt(0)) % 360;
  const hue = (hash * 7) % 360;
  return `linear-gradient(150deg, hsl(${hue} 46% 32%), hsl(${(hue + 46) % 360} 42% 18%))`;
});

function onError(): void {
  broken.value = true;
}
</script>

<template>
  <div class="poster" :style="{ aspectRatio: props.ratio, borderRadius: props.rounded }">
    <img
      v-if="visible"
      class="poster__img"
      :src="props.src ?? ''"
      :alt="props.title ?? ''"
      :style="{ objectFit: props.fit }"
      draggable="false"
      @error="onError"
    />
    <div v-else class="poster__fallback" :style="{ backgroundImage: shade }">
      <span>{{ initial }}</span>
    </div>
    <slot />
  </div>
</template>

<style scoped>
.poster {
  position: relative;
  overflow: hidden;
  width: 100%;
  background: var(--gv-surface-2);
  border: 1px solid var(--gv-border);
  isolation: isolate;
}

.poster__img {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  display: block;
  user-select: none;
}

.poster__fallback {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  color: rgba(255, 255, 255, 0.72);
  font-family: var(--gv-font-display);
  font-size: 42px;
  font-weight: 700;
  letter-spacing: 0.04em;
}
</style>
