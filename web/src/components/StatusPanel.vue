<script setup lang="ts">
import { computed } from "vue";
import {
  CircleCheck,
  CircleDashed,
  CircleX,
  LoaderCircle,
  TriangleAlert,
} from "@lucide/vue";
import { useBrowserCheck } from "../composables/useBrowserCheck";
import type { CheckState } from "../types";

const { state, verdictTitle, verdictDetail } = useBrowserCheck();

const labels: Record<CheckState, string> = {
  idle: "待检测",
  running: "检测中",
  ok: "通过",
  warning: "待复测",
  error: "失败",
};

const icons = {
  idle: CircleDashed,
  running: LoaderCircle,
  ok: CircleCheck,
  warning: TriangleAlert,
  error: CircleX,
};

const label = computed(() => labels[state.value]);
const statusIcon = computed(() => icons[state.value]);
</script>

<template>
  <aside class="panel" aria-live="polite" aria-atomic="true">
    <div class="panel-top">
      <span class="chip" :class="state">
        <component :is="statusIcon" :size="15" aria-hidden="true" class="chip-icon" />
        {{ label }}
      </span>
      <span class="panel-caption">快速检测</span>
    </div>
    <div class="progress" aria-hidden="true">
      <span class="progress-fill" :class="state" />
    </div>
    <h2>{{ verdictTitle }}</h2>
    <p class="panel-detail">{{ verdictDetail }}</p>
  </aside>
</template>

<style scoped>
.panel {
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 22px 24px;
}

.panel-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.panel-caption {
  color: var(--muted);
  font-size: 12px;
}

.chip {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 5px 10px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--surface-2);
  color: var(--muted);
  font-size: 13px;
  font-weight: 600;
}

.chip.running {
  color: var(--accent-2);
  border-color: #c9d8f3;
  background: var(--accent-2-dim);
}

.chip.running .chip-icon {
  animation: spin 0.9s linear infinite;
}

.chip.ok {
  color: var(--accent);
  border-color: #bcdfd3;
  background: var(--accent-dim);
}

.chip.warning {
  color: var(--warn);
  border-color: #ecd9b8;
  background: var(--warn-dim);
}

.chip.error {
  color: var(--error);
  border-color: #f0cdc6;
  background: var(--error-dim);
}

.progress {
  height: 4px;
  margin: 18px 0 20px;
  border-radius: 2px;
  background: var(--surface-2);
  overflow: hidden;
}

.progress-fill {
  display: block;
  height: 100%;
  width: 0;
  border-radius: 2px;
  background: var(--line-strong);
  transition: width 0.5s ease, background 0.3s ease;
}

.progress-fill.running {
  width: 38%;
  background: var(--accent-2);
  animation: slide 1.2s ease-in-out infinite;
}

.progress-fill.ok {
  width: 100%;
  background: var(--accent);
}

.progress-fill.warning {
  width: 100%;
  background: var(--warn);
}

.progress-fill.error {
  width: 100%;
  background: var(--error);
}

h2 {
  margin: 0 0 8px;
  font-size: 20px;
  font-weight: 700;
}

.panel-detail {
  margin: 0;
  color: var(--muted);
  font-size: 14px;
  line-height: 1.65;
}
</style>
