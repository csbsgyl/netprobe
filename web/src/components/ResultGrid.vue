<script setup lang="ts">
import { ArrowRightLeft, Globe, ShieldCheck, Wifi } from "@lucide/vue";
import { useBrowserCheck } from "../composables/useBrowserCheck";

const {
  publicIP,
  ipStatus,
  ipTone,
  httpsResult,
  httpsTone,
  udpResult,
  udpStatus,
  udpTone,
  testedAt,
  testedAtISO,
} = useBrowserCheck();
</script>

<template>
  <section class="results rise" aria-labelledby="resultHeading">
    <div class="section-head">
      <div>
        <p class="eyebrow">// 检测项目</p>
        <h2 id="resultHeading">当前网络画像</h2>
      </div>
      <time v-if="testedAtISO" :datetime="testedAtISO">{{ testedAt }}</time>
      <span v-else class="tested-at">{{ testedAt }}</span>
    </div>

    <div class="metric-grid">
      <div class="metric" :class="`tone-${ipTone}`">
        <div class="metric-top">
          <span class="metric-label"><Globe :size="14" aria-hidden="true" />公网 IPv4 / IPv6</span>
          <i class="dot" aria-hidden="true" />
        </div>
        <strong class="metric-value mono">{{ publicIP }}</strong>
        <small>{{ ipStatus }}</small>
      </div>

      <div class="metric" :class="`tone-${httpsTone}`">
        <div class="metric-top">
          <span class="metric-label"><ShieldCheck :size="14" aria-hidden="true" />HTTPS 连接</span>
          <i class="dot" aria-hidden="true" />
        </div>
        <strong class="metric-value mono">{{ httpsResult }}</strong>
        <small>浏览器到检测服务</small>
      </div>

      <div class="metric" :class="`tone-${udpTone}`">
        <div class="metric-top">
          <span class="metric-label"><Wifi :size="14" aria-hidden="true" />UDP / STUN 路径</span>
          <i class="dot" aria-hidden="true" />
        </div>
        <strong class="metric-value mono">{{ udpResult }}</strong>
        <small>{{ udpStatus }}</small>
      </div>

      <div class="metric tone-idle">
        <div class="metric-top">
          <span class="metric-label"><ArrowRightLeft :size="14" aria-hidden="true" />NAT 映射行为</span>
          <i class="dot" aria-hidden="true" />
        </div>
        <strong class="metric-value mono">需深度检测</strong>
        <small>使用同一端口访问双探测端口</small>
      </div>
    </div>
  </section>
</template>

<style scoped>
.results {
  border-top: 1px solid var(--line);
  padding-top: 40px;
  animation-delay: 0.2s;
}

.metric-grid {
  margin-top: 28px;
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 14px;
}

.metric {
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 20px;
  min-height: 158px;
  display: flex;
  flex-direction: column;
  transition: border-color 0.2s ease, transform 0.2s ease;
}

.metric:hover {
  border-color: var(--line-strong);
  transform: translateY(-2px);
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
  gap: 7px;
  color: var(--muted);
  font-size: 13px;
}

.metric-label svg {
  color: var(--faint);
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--line-strong);
  flex-shrink: 0;
  transition: background 0.3s ease, box-shadow 0.3s ease;
}

.tone-ok .dot {
  background: var(--accent);
  box-shadow: 0 0 8px rgba(46, 230, 166, 0.7);
}

.tone-warning .dot {
  background: var(--warn);
  box-shadow: 0 0 8px rgba(242, 176, 78, 0.7);
}

.tone-error .dot {
  background: var(--danger);
  box-shadow: 0 0 8px rgba(239, 106, 86, 0.7);
}

.metric-value {
  display: block;
  margin: auto 0 8px;
  padding-top: 22px;
  font-size: 19px;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.tone-ok .metric-value {
  color: var(--accent);
}

.tone-warning .metric-value {
  color: var(--warn);
}

.tone-error .metric-value {
  color: var(--danger);
}

.metric small {
  color: var(--faint);
  line-height: 1.5;
  font-size: 12.5px;
}

@media (max-width: 900px) {
  .metric-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 560px) {
  .metric-grid {
    grid-template-columns: 1fr;
  }

  .metric {
    min-height: 0;
  }

  .metric-value {
    padding-top: 16px;
  }
}
</style>
