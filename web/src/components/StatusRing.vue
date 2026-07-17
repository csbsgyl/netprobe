<script setup lang="ts">
import { computed } from "vue";
import type { CheckState } from "../types";

const props = defineProps<{ state: CheckState }>();

const labels: Record<CheckState, string> = {
  idle: "待检测",
  running: "检测中",
  ok: "通过",
  warning: "待复测",
  error: "失败",
};

/** 各状态对应的圆弧填充比例 */
const fills: Record<CheckState, number> = {
  idle: 0,
  running: 0.32,
  ok: 1,
  warning: 0.65,
  error: 0.3,
};

const RADIUS = 52;
const CIRCUMFERENCE = 2 * Math.PI * RADIUS;

const label = computed(() => labels[props.state]);
const dash = computed(
  () => `${(fills[props.state] * CIRCUMFERENCE).toFixed(2)} ${CIRCUMFERENCE.toFixed(2)}`,
);
</script>

<template>
  <div class="ring" :class="state" role="img" :aria-label="`检测状态：${label}`">
    <svg viewBox="0 0 120 120" aria-hidden="true">
      <circle class="track" cx="60" cy="60" :r="RADIUS" />
      <circle class="arc" cx="60" cy="60" :r="RADIUS" :stroke-dasharray="dash" />
    </svg>
    <span class="ring-label">{{ label }}</span>
  </div>
</template>

<style scoped>
.ring {
  position: relative;
  width: 124px;
  aspect-ratio: 1;
  flex-shrink: 0;
}

.ring svg {
  width: 100%;
  height: 100%;
  transform: rotate(-90deg);
}

.ring.running svg {
  animation: spin 1.1s linear infinite;
}

.track,
.arc {
  fill: none;
  stroke-width: 7;
  stroke-linecap: round;
}

.track {
  stroke: var(--line);
}

.arc {
  stroke: var(--line-strong);
  transition: stroke-dasharray 0.6s ease, stroke 0.3s ease;
}

.ring.running .arc {
  stroke: var(--accent);
}

.ring.ok .arc {
  stroke: var(--accent);
  filter: drop-shadow(0 0 6px rgba(46, 230, 166, 0.55));
}

.ring.warning .arc {
  stroke: var(--warn);
  filter: drop-shadow(0 0 6px rgba(242, 176, 78, 0.45));
}

.ring.error .arc {
  stroke: var(--danger);
  filter: drop-shadow(0 0 6px rgba(239, 106, 86, 0.45));
}

.ring-label {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  font-size: 14px;
  font-weight: 700;
  color: var(--muted);
}

.ring.ok .ring-label {
  color: var(--accent);
}

.ring.warning .ring-label {
  color: var(--warn);
}

.ring.error .ring-label {
  color: var(--danger);
}

.ring.running .ring-label {
  color: var(--ink);
}
</style>
