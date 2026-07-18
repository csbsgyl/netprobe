<script setup lang="ts">
import { ArrowRightLeft, Globe, ShieldCheck, Wifi } from "@lucide/vue";
import MetricCard from "./MetricCard.vue";
import { useBrowserCheck } from "../composables/useBrowserCheck";

const {
  running,
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
  <section class="results" aria-labelledby="resultHeading">
    <div class="section-head">
      <div>
        <p class="eyebrow">检测项目</p>
        <h2 id="resultHeading">当前网络画像</h2>
      </div>
      <time v-if="testedAtISO" :datetime="testedAtISO">{{ testedAt }}</time>
      <span v-else class="tested-at">{{ testedAt }}</span>
    </div>

    <div class="metric-grid">
      <MetricCard
        label="公网 IPv4 / IPv6"
        :value="publicIP"
        :helper="ipStatus"
        :tone="ipTone"
        :icon="Globe"
        :running="running"
      />
      <MetricCard
        label="HTTPS 连接"
        :value="httpsResult"
        helper="浏览器到检测服务"
        :tone="httpsTone"
        :icon="ShieldCheck"
        :running="running"
      />
      <MetricCard
        label="UDP / STUN 路径"
        :value="udpResult"
        :helper="udpStatus"
        :tone="udpTone"
        :icon="Wifi"
        :running="running"
      />
      <MetricCard
        label="NAT 映射行为"
        value="需深度检测"
        helper="使用同一端口访问双探测端口"
        tone="idle"
        :icon="ArrowRightLeft"
      />
    </div>
  </section>
</template>

<style scoped>
.results {
  border-top: 1px solid var(--line);
  padding-top: 32px;
}

.metric-grid {
  margin-top: 24px;
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
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
}
</style>
