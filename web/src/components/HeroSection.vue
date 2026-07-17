<script setup lang="ts">
import { computed } from "vue";
import { LoaderCircle, Radar, ShieldCheck } from "@lucide/vue";
import StatusRing from "./StatusRing.vue";
import { useBrowserCheck } from "../composables/useBrowserCheck";

const { state, running, verdictTitle, verdictDetail, runQuickCheck } = useBrowserCheck();

const actionText = computed(() => (running.value ? "检测中" : "开始检测"));
</script>

<template>
  <section class="hero">
    <div class="hero-copy rise">
      <p class="eyebrow">// 网络适配检测</p>
      <h1>确认当前网络<br />是否满足<span class="hl">连接要求</span></h1>
      <p class="lead">
        快速检查公网出口与 HTTPS 连通性。需要 UDP 和 NAT 行为结论时，运行下方的一键深度检测。
      </p>
      <div class="hero-actions">
        <button
          class="primary"
          type="button"
          :disabled="running"
          :aria-busy="running"
          @click="runQuickCheck"
        >
          <LoaderCircle v-if="running" class="spin" :size="18" aria-hidden="true" />
          <Radar v-else :size="18" aria-hidden="true" />
          {{ actionText }}
        </button>
        <span class="privacy">
          <ShieldCheck :size="14" aria-hidden="true" />
          不会扫描局域网，也不需要摄像头或麦克风权限
        </span>
      </div>
    </div>

    <aside class="verdict rise" :class="state" aria-live="polite" aria-atomic="true">
      <StatusRing :state="state" />
      <div class="verdict-text">
        <p class="verdict-label">快速检测结果</p>
        <h2>{{ verdictTitle }}</h2>
        <p>{{ verdictDetail }}</p>
      </div>
    </aside>
  </section>
</template>

<style scoped>
.hero {
  display: grid;
  grid-template-columns: 1.2fr 0.8fr;
  gap: 64px;
  align-items: center;
  padding: 72px 0 64px;
}

h1 {
  margin: 0;
  max-width: 720px;
  font-size: 62px;
  line-height: 1.08;
  font-weight: 760;
  letter-spacing: 0;
}

.hl {
  color: var(--accent);
}

.lead {
  margin: 24px 0 0;
  max-width: 620px;
  color: var(--muted);
  line-height: 1.75;
  font-size: 16px;
}

.hero-actions {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-top: 36px;
  flex-wrap: wrap;
}

.primary {
  min-height: 50px;
  padding: 0 24px;
  border: 0;
  border-radius: 8px;
  color: var(--accent-ink);
  background: var(--accent);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 10px;
  font-weight: 700;
  font-size: 15px;
  box-shadow: 0 10px 28px -10px rgba(46, 230, 166, 0.5);
  transition: transform 0.15s ease, box-shadow 0.2s ease, opacity 0.2s ease;
}

.primary:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 14px 34px -10px rgba(46, 230, 166, 0.6);
}

.primary:active:not(:disabled) {
  transform: translateY(0);
}

.primary:disabled {
  opacity: 0.6;
  cursor: wait;
}

.spin {
  animation: spin 0.9s linear infinite;
}

.privacy {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: var(--faint);
  font-size: 13px;
}

.privacy svg {
  color: var(--accent);
  flex-shrink: 0;
}

.verdict {
  display: flex;
  align-items: center;
  gap: 26px;
  padding: 30px 32px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--panel);
  box-shadow: 0 24px 48px -28px rgba(0, 0, 0, 0.7);
  animation-delay: 0.12s;
  transition: border-color 0.3s ease;
}

.verdict.ok {
  border-color: rgba(46, 230, 166, 0.4);
}

.verdict.warning {
  border-color: rgba(242, 176, 78, 0.4);
}

.verdict.error {
  border-color: rgba(239, 106, 86, 0.4);
}

.verdict-label {
  margin: 0 0 8px;
  color: var(--faint);
  font-family: var(--mono);
  font-size: 11px;
  letter-spacing: 0;
  text-transform: uppercase;
}

.verdict-text h2 {
  margin: 0 0 10px;
  font-size: 22px;
  font-weight: 700;
}

.verdict-text p:last-child {
  margin: 0;
  color: var(--muted);
  line-height: 1.6;
  font-size: 13.5px;
}

@media (max-width: 900px) {
  .hero {
    grid-template-columns: 1fr;
    gap: 36px;
    padding: 48px 0 52px;
  }

  h1 {
    font-size: 52px;
  }
}

@media (max-width: 560px) {
  .hero {
    padding: 36px 0 44px;
  }

  h1 {
    font-size: 40px;
  }

  .hero-actions {
    flex-direction: column;
    align-items: stretch;
  }

  .primary {
    justify-content: center;
  }

  .privacy {
    justify-content: center;
  }

  .verdict {
    flex-direction: column;
    text-align: center;
    padding: 26px 22px;
    gap: 18px;
  }
}
</style>
