<script setup lang="ts">
import { computed } from "vue";
import { LoaderCircle, Radar, ShieldCheck } from "@lucide/vue";
import StatusPanel from "./StatusPanel.vue";
import { useBrowserCheck } from "../composables/useBrowserCheck";

const { running, runQuickCheck } = useBrowserCheck();

const actionText = computed(() => (running.value ? "检测中" : "开始检测"));
</script>

<template>
  <section class="hero">
    <div class="hero-copy">
      <p class="eyebrow">网络适配检测</p>
      <h1>确认当前网络是否满足连接要求</h1>
      <p class="lead">
        在浏览器中检测公网出口、HTTPS 连通性与 UDP/STUN 路径，通常 5 秒内完成。
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
  font-size: 40px;
  line-height: 1.18;
  font-weight: 750;
}

.lead {
  margin: 16px 0 0;
  max-width: 480px;
  color: var(--muted);
  font-size: 15px;
  line-height: 1.7;
}

.hero-actions {
  margin-top: 28px;
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
  transition: background 0.15s ease, opacity 0.15s ease;
}

.primary:hover:not(:disabled) {
  background: var(--accent-hover);
}

.primary:active:not(:disabled) {
  background: var(--accent-active);
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
    font-size: 32px;
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
    font-size: 28px;
  }

  .primary {
    width: 100%;
    justify-content: center;
  }
}
</style>
