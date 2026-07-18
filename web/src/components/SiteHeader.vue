<script setup lang="ts">
import { computed } from "vue";
import { Network } from "@lucide/vue";
import { useBrowserCheck } from "../composables/useBrowserCheck";
import type { CheckState } from "../types";

type ServiceStatus = "standby" | "checking" | "online" | "down";

const { state } = useBrowserCheck();

const statusMap: Record<CheckState, ServiceStatus> = {
  idle: "standby",
  running: "checking",
  ok: "online",
  warning: "online",
  error: "down",
};

const statusText: Record<ServiceStatus, string> = {
  standby: "检测服务待命",
  checking: "正在检测",
  online: "检测服务在线",
  down: "检测服务异常",
};

const status = computed(() => statusMap[state.value]);
const text = computed(() => statusText[status.value]);
</script>

<template>
  <header class="site-header">
    <a class="brand" href="/" aria-label="NetProbe 网络检测首页">
      <span class="brand-mark"><Network :size="17" aria-hidden="true" /></span>
      <span class="brand-name">NetProbe</span>
    </a>
    <span class="service" :class="status" role="status" aria-live="polite">
      <i aria-hidden="true" />{{ text }}
    </span>
  </header>
</template>

<style scoped>
.site-header {
  position: sticky;
  top: 0;
  z-index: 10;
  height: 64px;
  padding: 0 max(24px, calc((100vw - 1120px) / 2));
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  border-bottom: 1px solid var(--line);
  background: var(--surface);
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
  background: var(--surface-2);
  border: 1px solid var(--line);
  border-radius: 6px;
}

.brand-name {
  font-weight: 700;
  font-size: 16px;
}

.service {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--muted);
  font-size: 13px;
  white-space: nowrap;
}

.service i {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--line-strong);
  flex-shrink: 0;
}

.service.checking {
  color: var(--accent-2);
}

.service.checking i {
  background: var(--accent-2);
  animation: pulse 1.2s ease-in-out infinite;
}

.service.online {
  color: var(--accent);
}

.service.online i {
  background: var(--accent);
}

.service.down {
  color: var(--error);
}

.service.down i {
  background: var(--error);
}

@media (max-width: 560px) {
  .site-header {
    height: 56px;
    padding: 0 16px;
  }

  .service {
    font-size: 12px;
  }
}
</style>
