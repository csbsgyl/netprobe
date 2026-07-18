<script setup lang="ts">
import { computed } from "vue";
import { Globe, LoaderCircle, Radar, ShieldCheck } from "@lucide/vue";
import StatusPanel from "./StatusPanel.vue";
import { useBrowserCheck } from "../composables/useBrowserCheck";

const { running, runQuickCheck } = useBrowserCheck();

const actionText = computed(() => (running.value ? "检测中" : "开始检测"));
const serviceOrigin = window.location.origin;
</script>

<template>
  <section class="hero">
    <div class="hero-copy reveal">
      <p class="eyebrow">NetProbe 服务</p>
      <h1>网络检测服务<span class="hl">已部署</span></h1>
      <p class="lead">
        这是部署后的服务入口。打开此页面即可检测当前设备的公网出口、HTTPS 连通性与 UDP/STUN 路径。
      </p>
      <p class="origin-note">
        <Globe :size="14" aria-hidden="true" />
        当前服务地址 <span class="mono">{{ serviceOrigin }}</span>
      </p>
      <div class="hero-actions">
        <button
          class="primary"
          type="button"
          :disabled="running"
          :aria-busy="running"
          @click="runQuickCheck"
        >
          <LoaderCircle v-if="running" class="spin" :size="17" aria-hidden="true" />
          <Radar v-else :size="17" aria-hidden="true" />
          {{ actionText }}
        </button>
        <span class="privacy">
          <ShieldCheck :size="14" aria-hidden="true" />
          不会扫描局域网，也不需要摄像头或麦克风权限
        </span>
      </div>
    </div>
    <StatusPanel />
  </section>
</template>

<style scoped>
.hero {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 56px;
  align-items: center;
  padding: 56px 0 44px;
}

h1 {
  margin: 0;
  font-size: 44px;
  line-height: 1.18;
  font-weight: 750;
}

.hl {
  color: var(--accent);
  white-space: nowrap;
}

.origin-note {
  display: flex;
  align-items: center;
  gap: 7px;
  max-width: 100%;
  margin: 16px 0 0;
  color: var(--muted);
  font-size: 13px;
}

.origin-note svg {
  color: var(--accent);
  flex-shrink: 0;
}

.origin-note .mono {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--ink);
}

.lead {
  margin: 16px 0 0;
  max-width: 480px;
  color: var(--muted);
  font-size: 15px;
  line-height: 1.7;
}

.hero-actions {
  margin-top: 24px;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 14px;
}

.primary {
  height: 44px;
  padding: 0 20px;
  border: 1px solid transparent;
  border-radius: 6px;
  background: var(--accent);
  color: #ffffff;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 9px;
  font-size: 14.5px;
  font-weight: 600;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.16), 0 8px 20px -10px rgba(22, 122, 104, 0.55);
  transition: background 0.15s ease, transform 0.15s ease, box-shadow 0.2s ease, opacity 0.15s ease;
}

.primary:hover:not(:disabled) {
  background: var(--accent-hover);
  transform: translateY(-1px);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.16), 0 12px 26px -10px rgba(22, 122, 104, 0.6);
}

.primary:active:not(:disabled) {
  background: var(--accent-active);
  transform: translateY(0);
}

.primary:disabled {
  opacity: 0.55;
  cursor: wait;
}

.spin {
  animation: spin 0.9s linear infinite;
}

.privacy {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: var(--muted);
  font-size: 13px;
}

.privacy svg {
  color: var(--accent);
  flex-shrink: 0;
}

@media (max-width: 900px) {
  .hero {
    grid-template-columns: 1fr;
    gap: 32px;
    padding: 40px 0 36px;
  }

  h1 {
    font-size: 34px;
  }

  .lead {
    max-width: none;
  }
}

@media (max-width: 560px) {
  .hero {
    padding: 32px 0 32px;
    gap: 28px;
  }

  h1 {
    font-size: 30px;
  }

  .primary {
    width: 100%;
    justify-content: center;
  }
}
</style>
