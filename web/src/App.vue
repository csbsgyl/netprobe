<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { ArrowRight, Copy, Network } from "@lucide/vue";

type Platform = "linux" | "windows";
type State = "idle" | "running" | "ok" | "warning" | "error";
type CopyState = "idle" | "copied" | "failed";

interface BrowserCheck {
  public_ip?: string;
  https: boolean;
  method: string;
  verdict: string;
  tested_at: string;
  stun_url?: string;
}

interface StunResult {
  ok: boolean;
  detail: string;
}

const state = ref<State>("idle");
const platform = ref<Platform>("linux");
const publicIP = ref("--");
const ipStatus = ref("等待获取");
const httpsResult = ref("--");
const udpResult = ref("--");
const udpStatus = ref("通过 WebRTC ICE 快速判断");
const testedAtISO = ref("");
const copyState = ref<CopyState>("idle");
const verdictDetail = ref("点击开始后，通常可在 5 秒内完成。");
let copyResetTimer: number | undefined;

const commands: Record<Platform, string> = {
  linux: `curl -fsSL ${window.location.origin}/install.sh | sh`,
  windows: `irm ${window.location.origin}/install.ps1 | iex`,
};

const dialLabels: Record<State, string> = {
  idle: "待检测",
  running: "检测中",
  ok: "通过",
  warning: "待复测",
  error: "失败",
};

const verdictTitles: Record<State, string> = {
  idle: "尚未开始",
  running: "正在连接检测服务",
  ok: "基础网络可用",
  warning: "建议继续复测",
  error: "未能完成检测",
};

const command = computed(() => commands[platform.value]);
const dialText = computed(() => dialLabels[state.value]);
const verdictTitle = computed(() => verdictTitles[state.value]);
const actionText = computed(() => state.value === "running" ? "检测中" : "开始检测");
const testedAt = computed(() => {
  if (state.value === "running") return "检测进行中";
  if (!testedAtISO.value) return "等待检测";
  return new Date(testedAtISO.value).toLocaleString();
});
const copyButtonLabel = computed(() => copyState.value === "copied" ? "已复制" : "复制命令");
const copyFeedback = computed(() => {
  if (copyState.value === "copied") return "命令已复制到剪贴板";
  if (copyState.value === "failed") return "复制失败，请手动选择命令";
  return "";
});

onMounted(() => {
  if (/Windows/i.test(navigator.userAgent)) platform.value = "windows";
});

onBeforeUnmount(() => {
  if (copyResetTimer !== undefined) window.clearTimeout(copyResetTimer);
});

async function runQuickCheck(): Promise<void> {
  state.value = "running";
  publicIP.value = "--";
  ipStatus.value = "正在获取";
  httpsResult.value = "检测中";
  udpResult.value = "检测中";
  udpStatus.value = "正在收集 WebRTC ICE 候选";
  testedAtISO.value = "";
  verdictDetail.value = "正在获取当前出口、HTTPS 和 UDP/STUN 状态。";

  try {
    const response = await fetch("/api/v1/browser-check", {
      cache: "no-store",
      headers: { Accept: "application/json" },
    });
    if (!response.ok) throw new Error(`检测服务返回异常（HTTP ${response.status}）`);

    const payload: unknown = await response.json();
    if (!isBrowserCheck(payload)) throw new Error("检测服务返回了无法识别的数据");

    publicIP.value = payload.public_ip || "未识别";
    ipStatus.value = payload.public_ip
      ? payload.public_ip.includes(":") ? "IPv6 出口" : "IPv4 出口"
      : "服务端未返回出口地址";
    httpsResult.value = payload.https ? "可用" : "当前为 HTTP";

    const udp = await checkStun(payload.stun_url);
    udpResult.value = udp.ok ? "可用" : "未确认";
    udpStatus.value = udp.ok ? "已取得公网映射候选" : udp.detail;

    const testedDate = new Date(payload.tested_at);
    testedAtISO.value = Number.isNaN(testedDate.getTime())
      ? new Date().toISOString()
      : testedDate.toISOString();

    state.value = payload.https && udp.ok ? "ok" : "warning";
    if (!payload.https) {
      verdictDetail.value = "检测服务当前未通过 HTTPS 访问。请先检查域名证书，再运行深度命令复测。";
    } else if (!udp.ok) {
      verdictDetail.value = "HTTPS 可用，但浏览器未确认 UDP/STUN 路径。建议运行深度命令复测。";
    } else {
      verdictDetail.value = "HTTPS 与浏览器 UDP/STUN 路径均可用。运行深度命令可继续判断 NAT 行为。";
    }
  } catch (error) {
    state.value = "error";
    httpsResult.value = "失败";
    udpResult.value = "未检测";
    udpStatus.value = "等待基础连接恢复";
    verdictDetail.value = error instanceof Error ? error.message : "请稍后重试。";
  }
}

function isBrowserCheck(value: unknown): value is BrowserCheck {
  if (typeof value !== "object" || value === null) return false;
  const result = value as Partial<BrowserCheck>;
  return typeof result.https === "boolean"
    && typeof result.method === "string"
    && typeof result.verdict === "string"
    && typeof result.tested_at === "string"
    && (result.public_ip === undefined || typeof result.public_ip === "string")
    && (result.stun_url === undefined || typeof result.stun_url === "string");
}

async function checkStun(stunUrl?: string): Promise<StunResult> {
  if (!stunUrl || typeof RTCPeerConnection === "undefined") {
    return { ok: false, detail: "当前浏览器不支持 WebRTC 检测" };
  }

  const peer = new RTCPeerConnection({ iceServers: [{ urls: stunUrl }] });
  let hasServerReflexiveCandidate = false;
  peer.createDataChannel("probe");
  peer.addEventListener("icecandidate", (event) => {
    if (!event.candidate) return;
    const parsedType = / typ ([a-z]+)/.exec(event.candidate.candidate)?.[1];
    if (event.candidate.type === "srflx" || parsedType === "srflx") {
      hasServerReflexiveCandidate = true;
    }
  });

  try {
    const gatheringComplete = waitForIceGathering(peer, 4500);
    await peer.setLocalDescription(await peer.createOffer());
    await gatheringComplete;
    return {
      ok: hasServerReflexiveCandidate,
      detail: hasServerReflexiveCandidate ? "" : "未取得公网映射候选",
    };
  } catch {
    return { ok: false, detail: "WebRTC 探测受浏览器策略限制" };
  } finally {
    peer.close();
  }
}

function waitForIceGathering(peer: RTCPeerConnection, timeoutMs: number): Promise<void> {
  if (peer.iceGatheringState === "complete") return Promise.resolve();

  return new Promise((resolve) => {
    let timeout = 0;
    const finish = (): void => {
      window.clearTimeout(timeout);
      peer.removeEventListener("icegatheringstatechange", handleStateChange);
      resolve();
    };
    const handleStateChange = (): void => {
      if (peer.iceGatheringState === "complete") finish();
    };

    peer.addEventListener("icegatheringstatechange", handleStateChange);
    timeout = window.setTimeout(finish, timeoutMs);
  });
}

async function copyCommand(): Promise<void> {
  if (copyResetTimer !== undefined) window.clearTimeout(copyResetTimer);
  try {
    await copyText(command.value);
    copyState.value = "copied";
  } catch {
    copyState.value = "failed";
  }
  copyResetTimer = window.setTimeout(() => { copyState.value = "idle"; }, 2200);
}

async function copyText(text: string): Promise<void> {
  if (window.isSecureContext && navigator.clipboard) {
    await navigator.clipboard.writeText(text);
    return;
  }

  const textArea = document.createElement("textarea");
  textArea.value = text;
  textArea.setAttribute("readonly", "");
  textArea.style.position = "fixed";
  textArea.style.opacity = "0";
  document.body.append(textArea);
  textArea.select();
  const copied = document.execCommand("copy");
  textArea.remove();
  if (!copied) throw new Error("clipboard unavailable");
}
</script>

<template>
  <header class="topbar">
    <a class="brand" href="/" aria-label="NetProbe 网络检测首页">
      <span class="brand-mark"><Network :size="19" aria-hidden="true" /></span>
      <span>NetProbe</span>
    </a>
    <span class="service"><i aria-hidden="true" />检测服务在线</span>
  </header>

  <main>
    <section class="intro">
      <div>
        <p class="eyebrow">网络适配检测</p>
        <h1>确认当前网络<br />是否满足连接要求</h1>
        <p class="lead">快速检查公网出口与 HTTPS 连通性。需要 UDP 和 NAT 行为结论时，运行下方的一键深度检测。</p>
      </div>
      <aside class="verdict-panel" aria-live="polite" aria-atomic="true">
        <div class="dial" :class="state"><span>{{ dialText }}</span></div>
        <div>
          <p class="panel-label">快速检测结果</p>
          <h2>{{ verdictTitle }}</h2>
          <p>{{ verdictDetail }}</p>
        </div>
      </aside>
    </section>

    <section class="actions" aria-label="检测操作">
      <button
        class="primary"
        type="button"
        :disabled="state === 'running'"
        :aria-busy="state === 'running'"
        @click="runQuickCheck"
      >
        <ArrowRight :size="19" aria-hidden="true" />{{ actionText }}
      </button>
      <span class="privacy">不会扫描局域网，也不需要摄像头或麦克风权限</span>
    </section>

    <section class="results" aria-labelledby="resultHeading">
      <div class="section-head">
        <div><p class="eyebrow">检测项目</p><h2 id="resultHeading">当前网络画像</h2></div>
        <time v-if="testedAtISO" :datetime="testedAtISO">{{ testedAt }}</time>
        <span v-else class="tested-at">{{ testedAt }}</span>
      </div>
      <div class="metric-grid">
        <div class="metric"><span>公网 IPv4 / IPv6</span><strong>{{ publicIP }}</strong><small>{{ ipStatus }}</small></div>
        <div class="metric"><span>HTTPS 连接</span><strong>{{ httpsResult }}</strong><small>浏览器到检测服务</small></div>
        <div class="metric"><span>UDP / STUN 路径</span><strong>{{ udpResult }}</strong><small>{{ udpStatus }}</small></div>
        <div class="metric"><span>NAT 映射行为</span><strong>需深度检测</strong><small>使用同一端口访问双探测端口</small></div>
      </div>
    </section>

    <section class="deep-test" aria-labelledby="deepHeading">
      <div class="section-head">
        <div><p class="eyebrow">完整诊断</p><h2 id="deepHeading">一条命令完成深度检测</h2></div>
        <div class="tabs" role="group" aria-label="选择操作系统">
          <button class="tab" :class="{ active: platform === 'linux' }" type="button" :aria-pressed="platform === 'linux'" @click="platform = 'linux'">Linux</button>
          <button class="tab" :class="{ active: platform === 'windows' }" type="button" :aria-pressed="platform === 'windows'" @click="platform = 'windows'">Windows</button>
        </div>
      </div>
      <p id="commandHelp" class="deep-copy">自动识别系统架构、下载并校验客户端、运行 UDP/NAT 测试，完成后清理临时文件。</p>
      <div class="command-row">
        <code id="installCommand" aria-describedby="commandHelp">{{ command }}</code>
        <button
          class="icon-button"
          type="button"
          :title="copyButtonLabel"
          :aria-label="copyButtonLabel"
          aria-describedby="installCommand"
          @click="copyCommand"
        >
          <Copy :size="20" aria-hidden="true" />
        </button>
      </div>
      <span class="sr-only" role="status" aria-live="polite">{{ copyFeedback }}</span>
      <div class="deep-facts"><span>UDP 双端口</span><span>映射稳定性</span><span>备用端口回包</span><span>RTT 与丢包</span></div>
    </section>
  </main>

  <footer><span>NetProbe</span><span>检测只代表当前设备、当前网络和本次测试时间。</span></footer>
</template>

<style>
:root { --ink:#10231c; --muted:#5e6d66; --paper:#f5f7f4; --line:#d7ded9; --green:#177a4a; --white:#fff; color:var(--ink); background:var(--paper); font-family:Inter,"PingFang SC","Microsoft YaHei",system-ui,sans-serif; font-synthesis:none; letter-spacing:0; color-scheme:light; }
* { box-sizing:border-box; }
body { margin:0; min-width:320px; min-height:100vh; background:var(--paper); }
#app { min-height:100vh; display:flex; flex-direction:column; }
button, code { font:inherit; letter-spacing:0; }
.sr-only { position:absolute; width:1px; height:1px; padding:0; margin:-1px; overflow:hidden; clip:rect(0,0,0,0); white-space:nowrap; border:0; }
a:focus-visible,button:focus-visible { outline:3px solid #e0a32f; outline-offset:3px; }
.topbar { height:72px; padding:0 max(24px,calc((100vw - 1180px)/2)); display:flex; align-items:center; justify-content:space-between; border-bottom:1px solid var(--line); background:rgba(245,247,244,.96); }
.brand { display:inline-flex; align-items:center; gap:10px; color:var(--ink); text-decoration:none; font-weight:750; font-size:18px; }
.brand-mark { width:34px; height:34px; display:grid; place-items:center; color:var(--white); background:var(--ink); border-radius:6px; font-size:15px; }
.service { color:var(--muted); font-size:13px; display:flex; align-items:center; gap:8px; }
.service i { width:8px; height:8px; border-radius:50%; background:#27a362; box-shadow:0 0 0 4px #dceee3; }
main { width:100%; max-width:1180px; flex:1; margin:0 auto; padding:72px 24px 88px; }
.intro { display:grid; grid-template-columns:1.25fr .75fr; gap:80px; align-items:end; }
.eyebrow { margin:0 0 14px; color:var(--green); font-size:12px; font-weight:800; }
h1 { margin:0; max-width:760px; font-size:72px; line-height:1.04; font-weight:760; letter-spacing:0; }
.lead { margin:28px 0 0; max-width:650px; color:var(--muted); line-height:1.75; font-size:17px; }
.verdict-panel { min-height:190px; display:grid; grid-template-columns:116px 1fr; gap:24px; align-items:center; padding:30px 0 30px 32px; border-left:1px solid var(--line); }
.dial { width:116px; aspect-ratio:1; border-radius:50%; display:grid; place-items:center; background:conic-gradient(var(--line) 0 78%,transparent 78%); position:relative; }
.dial::before { content:""; position:absolute; inset:9px; border-radius:50%; background:var(--paper); }
.dial span { position:relative; font-size:14px; font-weight:750; }
.dial.running { background:conic-gradient(#f3bc55 0 42%,var(--line) 42%); }
.dial.ok { background:conic-gradient(var(--green) 0 92%,var(--line) 92%); }
.dial.warning { background:conic-gradient(#d39323 0 58%,var(--line) 58%); }
.dial.error { background:conic-gradient(#b84234 0 22%,var(--line) 22%); }
.panel-label { color:var(--muted); margin:0 0 8px; font-size:13px; }
.verdict-panel h2 { margin:0 0 10px; font-size:25px; }
.verdict-panel p:last-child { margin:0; color:var(--muted); line-height:1.55; font-size:14px; }
.actions { display:flex; align-items:center; gap:22px; padding:44px 0 76px; }
.primary { min-height:52px; padding:0 24px; border:0; border-radius:6px; color:var(--white); background:var(--green); cursor:pointer; display:inline-flex; align-items:center; gap:10px; font-weight:700; }
.primary:hover:not(:disabled) { background:#12653d; }
.primary:disabled { opacity:.62; cursor:wait; }
.privacy { color:var(--muted); font-size:13px; }
.results,.deep-test { border-top:1px solid var(--line); padding-top:32px; }
.section-head { display:flex; align-items:flex-end; justify-content:space-between; gap:24px; }
.section-head h2 { margin:0; font-size:27px; }
.section-head time,.tested-at { color:var(--muted); font-size:13px; }
.metric-grid { margin-top:28px; display:grid; grid-template-columns:repeat(4,1fr); border:1px solid var(--line); background:var(--line); gap:1px; }
.metric { background:var(--white); padding:24px; min-height:155px; }
.metric span { color:var(--muted); font-size:13px; }
.metric strong { display:block; margin:26px 0 8px; font-size:18px; overflow-wrap:anywhere; }
.metric small { color:var(--muted); line-height:1.45; }
.deep-test { margin-top:76px; }
.deep-copy { max-width:700px; color:var(--muted); line-height:1.65; }
.tabs { display:inline-flex; border:1px solid var(--line); padding:3px; border-radius:6px; }
.tab { border:0; background:transparent; color:var(--muted); padding:8px 14px; cursor:pointer; border-radius:4px; }
.tab:hover:not(.active) { color:var(--ink); background:#e8ece9; }
.tab.active { background:var(--ink); color:var(--white); }
.command-row { margin-top:24px; min-height:68px; background:var(--ink); color:#d7f4e2; display:grid; grid-template-columns:minmax(0,1fr) 54px; align-items:center; border-radius:6px; overflow:hidden; }
.command-row code { padding:20px 22px; overflow-x:auto; white-space:nowrap; font-size:14px; }
.icon-button { height:100%; border:0; border-left:1px solid #355048; color:#d7f4e2; background:transparent; cursor:pointer; display:grid; place-items:center; }
.icon-button:hover { background:#1c352c; }
.deep-facts { display:flex; flex-wrap:wrap; gap:12px 30px; margin-top:20px; color:var(--muted); font-size:13px; }
.deep-facts span::before { content:""; display:inline-block; width:6px; height:6px; border-radius:50%; margin-right:9px; background:var(--green); vertical-align:middle; }
footer { width:100%; max-width:1180px; margin:0 auto; padding:26px 24px; border-top:1px solid var(--line); display:flex; justify-content:space-between; color:var(--muted); font-size:12px; }
footer span:first-child { color:var(--ink); font-weight:750; }
@media (max-width:860px) { main{padding-top:48px}.intro{grid-template-columns:1fr;gap:34px}h1{font-size:58px}.verdict-panel{border-left:0;border-top:1px solid var(--line);padding:28px 0 0}.metric-grid{grid-template-columns:repeat(2,1fr)} }
@media (max-width:560px) { .topbar{height:62px;padding:0 18px}main{padding:36px 18px 64px}h1{font-size:42px}.lead{font-size:15px;margin-top:20px}.verdict-panel{grid-template-columns:88px 1fr;gap:18px}.dial{width:88px}.actions{align-items:flex-start;flex-direction:column;padding:34px 0 56px}.primary{width:100%;justify-content:center}.section-head{align-items:flex-start;flex-direction:column}.metric-grid{grid-template-columns:1fr}.metric{min-height:132px}.metric strong{margin-top:20px}.deep-test{margin-top:58px}.tabs{width:100%}.tab{flex:1}footer{width:calc(100% - 36px);margin:0 18px;padding-left:0;padding-right:0;flex-direction:column;gap:8px} }
</style>
