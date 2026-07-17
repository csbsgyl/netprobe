<script setup lang="ts">
import { computed } from "vue";
import { Network } from "@lucide/vue";
import { useBrowserCheck } from "../composables/useBrowserCheck";

const { state } = useBrowserCheck();
const serviceLabel = computed(() => {
  if (state.value === "running") return "正在检测";
  if (state.value === "error") return "检测服务异常";
  if (state.value === "ok" || state.value === "warning") return "检测服务在线";
  return "检测服务待命";
});
</script>

<template>
  <header class="site-header">
    <a class="brand" href="/" aria-label="NetProbe 网络检测首页">
      <span class="brand-mark"><Network :size="17" aria-hidden="true" /></span>
      <span class="brand-name">NetProbe</span>
    </a>
    <span class="service" :class="state" aria-live="polite"><i aria-hidden="true" />{{ serviceLabel }}</span>
  </header>
</template>

<style scoped>
.site-header {
  position: sticky;
  top: 0;
  z-index: 10;
  height: 62px;
  padding: 0 max(24px, calc((100vw - 1120px) / 2));
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--line);
  background: rgba(8, 13, 11, 0.82);
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
}

.brand {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: var(--ink);
  text-decoration: none;
}

.brand-mark {
  width: 32px;
  height: 32px;
  display: grid;
  place-items: center;
  color: var(--accent);
  background: var(--accent-dim);
  border: 1px solid rgba(46, 230, 166, 0.3);
  border-radius: 8px;
}

.brand-name {
  font-weight: 700;
  font-size: 16px;
  letter-spacing: 0;
}

.service {
  color: var(--muted);
  font-family: var(--mono);
  font-size: 12px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.service i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--faint);
  box-shadow: 0 0 0 3px rgba(113, 135, 124, 0.12);
}

.service.running i,
.service.ok i,
.service.warning i {
  background: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-dim);
}

.service.running i {
  animation: pulse 1.2s ease-in-out infinite;
}

.service.error {
  color: var(--danger);
}

.service.error i {
  background: var(--danger);
  box-shadow: 0 0 0 3px var(--danger-dim);
}

@media (max-width: 560px) {
  .site-header {
    height: 56px;
    padding: 0 18px;
  }
}
</style>
