<script setup lang="ts">
import { computed } from "vue";
import type { Component } from "vue";
import {
  CircleCheck,
  CircleDashed,
  CircleX,
  LoaderCircle,
  TriangleAlert,
} from "@lucide/vue";
import type { Tone } from "../types";

const props = withDefaults(
  defineProps<{
    label: string;
    value: string;
    helper: string;
    tone: Tone;
    icon: Component;
    running?: boolean;
  }>(),
  { running: false },
);

const toneIcons: Record<Tone, Component> = {
  idle: CircleDashed,
  ok: CircleCheck,
  warning: TriangleAlert,
  error: CircleX,
};

const statusIcon = computed(() => (props.running ? LoaderCircle : toneIcons[props.tone]));
const statusClass = computed(() => (props.running ? "running" : props.tone));
</script>

<template>
  <div class="metric" :class="`s-${statusClass}`">
    <div class="metric-top">
      <span class="metric-label">
        <span class="label-icon"><component :is="icon" :size="14" aria-hidden="true" /></span>
        {{ label }}
      </span>
      <component :is="statusIcon" :size="16" aria-hidden="true" class="status-icon" />
    </div>
    <Transition name="swap" mode="out-in">
      <strong class="metric-value mono" :key="value">{{ value }}</strong>
    </Transition>
    <small>{{ helper }}</small>
  </div>
</template>

<style scoped>
.metric {
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 16px;
  min-width: 0;
  transition: transform 0.2s ease, border-color 0.2s ease, box-shadow 0.2s ease;
}

.metric:hover {
  transform: translateY(-2px);
  border-color: var(--line-strong);
  box-shadow: 0 12px 26px -16px rgba(20, 32, 28, 0.2);
}

.metric-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.metric-label {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--muted);
  font-size: 13px;
  white-space: nowrap;
}

.label-icon {
  width: 26px;
  height: 26px;
  display: grid;
  place-items: center;
  border-radius: 6px;
  background: var(--surface-2);
  border: 1px solid var(--line);
  color: var(--muted);
  flex-shrink: 0;
}

.status-icon {
  color: var(--line-strong);
  flex-shrink: 0;
}

.s-running .status-icon {
  color: var(--accent-2);
  animation: spin 0.9s linear infinite;
}

.s-ok .status-icon {
  color: var(--accent);
}

.s-warning .status-icon {
  color: var(--warn);
}

.s-error .status-icon {
  color: var(--error);
}

.metric-value {
  display: block;
  margin: 14px 0 6px;
  font-size: 16px;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.metric small {
  display: block;
  color: var(--muted);
  font-size: 12.5px;
  line-height: 1.5;
}
</style>
