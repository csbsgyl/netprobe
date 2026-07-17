<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { Check, Copy, Terminal } from "@lucide/vue";
import { useClipboard } from "../composables/useClipboard";
import type { Platform } from "../types";

const platform = ref<Platform>("linux");
const { copyState, copy } = useClipboard();

const commands: Record<Platform, string> = {
  linux: `curl -fsSL ${window.location.origin}/install.sh | sh`,
  windows: `irm ${window.location.origin}/install.ps1 | iex`,
};

const command = computed(() => commands[platform.value]);
const prompt = computed(() => (platform.value === "linux" ? "$" : "PS>"));
const shellName = computed(() => (platform.value === "linux" ? "bash" : "powershell"));
const copyButtonLabel = computed(() => (copyState.value === "copied" ? "已复制" : "复制命令"));
const copyFeedback = computed(() => {
  if (copyState.value === "copied") return "命令已复制到剪贴板";
  if (copyState.value === "failed") return "复制失败，请手动选择命令";
  return "";
});

onMounted(() => {
  if (/Windows/i.test(navigator.userAgent)) platform.value = "windows";
});
</script>

<template>
  <section class="deep-test rise" aria-labelledby="deepHeading">
    <div class="section-head">
      <div>
        <p class="eyebrow">// 完整诊断</p>
        <h2 id="deepHeading">一条命令完成深度检测</h2>
      </div>
      <div class="tabs" role="group" aria-label="选择操作系统">
        <button
          class="tab"
          :class="{ active: platform === 'linux' }"
          type="button"
          :aria-pressed="platform === 'linux'"
          @click="platform = 'linux'"
        >Linux</button>
        <button
          class="tab"
          :class="{ active: platform === 'windows' }"
          type="button"
          :aria-pressed="platform === 'windows'"
          @click="platform = 'windows'"
        >Windows</button>
      </div>
    </div>

    <p id="commandHelp" class="deep-copy">
      自动识别系统架构、下载并校验客户端、运行 UDP/NAT 测试，完成后清理临时文件。
    </p>

    <div class="terminal">
      <div class="terminal-bar">
        <span class="lights" aria-hidden="true"><i /><i /><i /></span>
        <span class="terminal-title">
          <Terminal :size="13" aria-hidden="true" />{{ shellName }}
        </span>
        <button
          class="copy-btn"
          :class="{ copied: copyState === 'copied' }"
          type="button"
          :aria-label="copyButtonLabel"
          aria-describedby="installCommand"
          @click="copy(command)"
        >
          <Check v-if="copyState === 'copied'" :size="14" aria-hidden="true" />
          <Copy v-else :size="14" aria-hidden="true" />
          {{ copyState === "copied" ? "已复制" : "复制" }}
        </button>
      </div>
      <div class="terminal-body">
        <code id="installCommand" aria-describedby="commandHelp"><span class="prompt">{{ prompt }}</span> {{ command }}</code>
      </div>
    </div>
    <span class="sr-only" role="status" aria-live="polite">{{ copyFeedback }}</span>

    <div class="deep-facts">
      <span>UDP 双端口</span>
      <span>映射稳定性</span>
      <span>备用端口回包</span>
      <span>RTT 与丢包</span>
    </div>
  </section>
</template>

<style scoped>
.deep-test {
  margin-top: 72px;
  border-top: 1px solid var(--line);
  padding-top: 40px;
  animation-delay: 0.28s;
}

.deep-copy {
  max-width: 680px;
  color: var(--muted);
  line-height: 1.65;
  font-size: 14.5px;
}

.tabs {
  display: inline-flex;
  gap: 2px;
  background: var(--panel);
  border: 1px solid var(--line);
  padding: 4px;
  border-radius: 8px;
}

.tab {
  border: 0;
  background: transparent;
  color: var(--muted);
  padding: 7px 16px;
  cursor: pointer;
  border-radius: 7px;
  font-size: 13.5px;
  font-weight: 600;
  transition: color 0.15s ease, background 0.15s ease;
}

.tab:hover:not(.active) {
  color: var(--ink);
}

.tab.active {
  background: var(--accent-dim);
  color: var(--accent);
}

.terminal {
  margin-top: 22px;
  background: #0a100d;
  border: 1px solid var(--line);
  border-radius: 8px;
  overflow: hidden;
}

.terminal-bar {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 10px 12px 10px 16px;
  background: var(--panel-2);
  border-bottom: 1px solid var(--line);
}

.lights {
  display: inline-flex;
  gap: 7px;
}

.lights i {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.lights i:nth-child(1) {
  background: #ef6a56;
}

.lights i:nth-child(2) {
  background: #f2b04e;
}

.lights i:nth-child(3) {
  background: #2ee6a6;
}

.terminal-title {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--faint);
  font-family: var(--mono);
  font-size: 12px;
}

.copy-btn {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: 1px solid var(--line);
  border-radius: 7px;
  background: transparent;
  color: var(--muted);
  font-size: 12.5px;
  padding: 5px 11px;
  cursor: pointer;
  transition: color 0.15s ease, border-color 0.15s ease, background 0.15s ease;
}

.copy-btn:hover {
  color: var(--ink);
  border-color: var(--line-strong);
}

.copy-btn.copied {
  color: var(--accent);
  border-color: rgba(46, 230, 166, 0.4);
  background: var(--accent-dim);
}

.terminal-body {
  padding: 20px 22px;
  overflow-x: auto;
}

.terminal-body code {
  font-family: var(--mono);
  font-size: 14px;
  color: var(--accent);
  white-space: nowrap;
}

.prompt {
  color: var(--faint);
  user-select: none;
}

.deep-facts {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 20px;
}

.deep-facts span {
  border: 1px solid var(--line);
  border-radius: 999px;
  padding: 6px 14px;
  color: var(--muted);
  font-size: 12.5px;
  background: var(--panel);
}

@media (max-width: 560px) {
  .deep-test {
    margin-top: 56px;
  }

  .section-head {
    align-items: flex-start;
    flex-direction: column;
  }

  .tabs {
    width: 100%;
  }

  .tab {
    flex: 1;
  }
}
</style>
