<script setup lang="ts">
import { computed } from "vue";
import { Check, Copy, Globe, HeartPulse } from "@lucide/vue";
import { useClipboard } from "../composables/useClipboard";
import { useServiceHealth } from "../composables/useServiceHealth";

const serviceOrigin = window.location.origin;
const { copyState, copy } = useClipboard();
const { state: healthState } = useServiceHealth();

const copyLabel = computed(() => (copyState.value === "copied" ? "已复制" : "复制服务地址"));
const copyFeedback = computed(() => {
  if (copyState.value === "copied") return "服务地址已复制";
  if (copyState.value === "failed") return "复制失败，请手动选择地址";
  return "";
});
const healthLabel = computed(() => {
  if (healthState.value === "online") return "健康检查通过";
  if (healthState.value === "error") return "健康检查异常";
  if (healthState.value === "idle") return "检测服务待命";
  return "正在检查健康状态";
});
</script>

<template>
  <section class="deployment-info reveal reveal-d3" aria-labelledby="deploymentHeading">
    <div class="section-head">
      <div>
        <p class="eyebrow">部署信息</p>
        <h2 id="deploymentHeading">当前服务入口</h2>
      </div>
      <span class="deployment-state" :class="healthState" role="status" aria-live="polite">
        <HeartPulse :size="14" aria-hidden="true" />
        {{ healthLabel }}
      </span>
    </div>

    <div class="deployment-strip">
      <div class="address-block">
        <span class="address-label">
          <Globe :size="15" aria-hidden="true" />
          当前访问地址
        </span>
        <div class="address-row">
          <code class="mono">{{ serviceOrigin }}</code>
          <button
            class="copy-address"
            :class="{ copied: copyState === 'copied' }"
            type="button"
            :title="copyLabel"
            :aria-label="copyLabel"
            @click="copy(serviceOrigin)"
          >
            <Check v-if="copyState === 'copied'" :size="16" aria-hidden="true" />
            <Copy v-else :size="16" aria-hidden="true" />
          </button>
        </div>
        <span class="copy-feedback" :class="copyState" role="status" aria-live="polite">
          {{ copyFeedback }}
        </span>
      </div>

      <div class="service-note">
        <HeartPulse :size="15" aria-hidden="true" />
        <span>打开当前地址即可使用浏览器网络检测。</span>
      </div>
    </div>
  </section>
</template>

<style scoped>
.deployment-info {
  margin-top: 56px;
  border-top: 1px solid var(--line);
  padding-top: 32px;
}

.deployment-state {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: var(--muted);
  font-size: 12px;
  white-space: nowrap;
}

.deployment-state.online {
  color: var(--accent);
}

.deployment-state.error {
  color: var(--error);
}

.deployment-state.checking {
  color: var(--accent-2);
}

.deployment-strip {
  margin-top: 24px;
  display: grid;
  grid-template-columns: minmax(0, 1.05fr) minmax(0, 0.95fr);
  gap: 28px;
  padding: 18px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
}

.address-block {
  min-width: 0;
}

.address-label,
.service-note {
  display: flex;
  align-items: center;
  gap: 7px;
  color: var(--muted);
  font-size: 12.5px;
}

.address-label svg,
.service-note svg {
  flex: 0 0 auto;
  color: var(--accent);
}

.address-row {
  min-width: 0;
  margin-top: 12px;
  display: flex;
  align-items: center;
  gap: 10px;
}

.address-row code {
  min-width: 0;
  flex: 1;
  overflow-x: auto;
  color: var(--ink);
  font-size: 15px;
  white-space: nowrap;
}

.copy-address {
  width: 34px;
  height: 34px;
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--surface);
  color: var(--muted);
  cursor: pointer;
  transition: color 0.15s ease, border-color 0.15s ease, background 0.15s ease;
}

.copy-address:hover {
  color: var(--ink);
  border-color: var(--line-strong);
  background: var(--surface-2);
}

.copy-address.copied {
  color: var(--accent);
  border-color: #bcdfd3;
  background: var(--accent-dim);
}

.copy-feedback {
  display: block;
  min-height: 18px;
  margin-top: 7px;
  color: transparent;
  font-size: 12px;
}

.copy-feedback.copied {
  color: var(--accent);
}

.copy-feedback.failed {
  color: var(--error);
}

.service-note {
  align-self: center;
  justify-content: flex-end;
  color: var(--muted);
}

@media (max-width: 900px) {
  .deployment-strip {
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .service-note {
    justify-content: flex-start;
  }
}

@media (max-width: 560px) {
  .deployment-info {
    margin-top: 44px;
  }

  .section-head {
    align-items: flex-start;
  }

  .deployment-strip {
    padding: 16px;
  }
}
</style>
