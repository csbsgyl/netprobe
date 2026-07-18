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
  if (copyState.value === "copied") return "已复制";
  if (copyState.value === "failed") return "复制失败，请手动选择命令复制";
  return "";
});

onMounted(() => {
  if (/Windows/i.test(navigator.userAgent)) platform.value = "windows";
});
</script>

<template>
  <section class="deep-test" aria-labelledby="deepHeading">
    <div class="section-head">
      <div>
        <p class="eyebrow">完整诊断</p>
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
        <span class="terminal-title">
          <Terminal :size="14" aria-hidden="true" />{{ shellName }}
        </span>
        <span class="copy-feedback" :class="copyState" role="status" aria-live="polite">{{ copyFeedback }}</span>
        <button
          class="copy-btn"
          :class="{ copied: copyState === 'copied' }"
          type="button"
          :title="copyButtonLabel"
          :aria-label="copyButtonLabel"
          aria-describedby="installCommand"
          @click="copy(command)"
        >
          <Check v-if="copyState === 'copied'" :size="15" aria-hidden="true" />
          <Copy v-else :size="15" aria-hidden="true" />
        </button>
      </div>
      <div class="terminal-body">
        <code id="installCommand" aria-describedby="commandHelp"><span class="prompt">{{ prompt }}</span> {{ command }}</code>
      </div>
    </div>

    <p class="deep-facts">UDP 双端口 · 映射稳定性 · 备用端口回包 · RTT 与丢包</p>
  </section>
</template>

<style scoped>
.deep-test {
  margin-top: 56px;
  border-top: 1px solid var(--line);
  padding-top: 32px;
}

.deep-copy {
  max-width: 640px;
  margin: 12px 0 0;
  color: var(--muted);
  font-size: 14px;
  line-height: 1.65;
}

.tabs {
  display: inline-flex;
  gap: 2px;
  background: var(--surface-2);
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 3px;
}

.tab {
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: var(--muted);
  padding: 6px 18px;
  cursor: pointer;
  font-size: 13.5px;
  font-weight: 600;
  transition: color 0.15s ease, background 0.15s ease, border-color 0.15s ease;
}

.tab:hover:not(.active) {
  color: var(--ink);
}

.tab.active {
  background: var(--surface);
  border-color: var(--line);
  color: var(--accent);
}

.terminal {
  margin-top: 20px;
  background: var(--surface-2);
  border: 1px solid var(--line);
  border-radius: 8px;
  overflow: hidden;
}

.terminal-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 10px 8px 16px;
  background: var(--surface);
  border-bottom: 1px solid var(--line);
}

.terminal-title {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: var(--muted);
  font-family: var(--mono);
  font-size: 12px;
}

.copy-feedback {
  margin-left: auto;
  font-size: 12.5px;
}

.copy-feedback:empty {
  display: none;
}

.copy-feedback.copied {
  color: var(--accent);
}

.copy-feedback.failed {
  color: var(--error);
}

.copy-feedback:empty + .copy-btn {
  margin-left: auto;
}

.copy-btn {
  width: 32px;
  height: 32px;
  display: grid;
  place-items: center;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--surface);
  color: var(--muted);
  cursor: pointer;
  transition: color 0.15s ease, border-color 0.15s ease, background 0.15s ease;
}

.copy-btn:hover {
  color: var(--ink);
  border-color: var(--line-strong);
  background: var(--surface-2);
}

.copy-btn.copied {
  color: var(--accent);
  border-color: #bcdfd3;
  background: var(--accent-dim);
}

.terminal-body {
  padding: 16px;
  overflow-x: auto;
}

.terminal-body code {
  font-family: var(--mono);
  font-size: 13.5px;
  color: var(--ink);
  white-space: nowrap;
}

.prompt {
  color: var(--muted);
  user-select: none;
}

.deep-facts {
  margin: 14px 0 0;
  color: var(--muted);
  font-size: 12.5px;
}

@media (max-width: 560px) {
  .deep-test {
    margin-top: 44px;
  }

  .section-head {
    align-items: flex-start;
    flex-direction: column;
    gap: 16px;
  }

  .tabs {
    width: 100%;
  }

  .tab {
    flex: 1;
  }
}
</style>
