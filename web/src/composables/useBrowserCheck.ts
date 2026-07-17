import { computed, ref } from "vue";
import type { BrowserCheck, CheckState, StunResult, Tone } from "../types";

const state = ref<CheckState>("idle");
const publicIP = ref("--");
const ipStatus = ref("等待获取");
const ipTone = ref<Tone>("idle");
const httpsResult = ref("--");
const httpsTone = ref<Tone>("idle");
const udpResult = ref("--");
const udpStatus = ref("通过 WebRTC ICE 快速判断");
const udpTone = ref<Tone>("idle");
const testedAtISO = ref("");
const verdictDetail = ref("点击开始后，通常可在 5 秒内完成。");

const verdictTitles: Record<CheckState, string> = {
  idle: "尚未开始",
  running: "正在连接检测服务",
  ok: "基础网络可用",
  warning: "建议继续复测",
  error: "未能完成检测",
};

const running = computed(() => state.value === "running");
const verdictTitle = computed(() => verdictTitles[state.value]);
const testedAt = computed(() => {
  if (state.value === "running") return "检测进行中";
  if (!testedAtISO.value) return "等待检测";
  return new Date(testedAtISO.value).toLocaleString();
});

async function runQuickCheck(): Promise<void> {
  state.value = "running";
  publicIP.value = "--";
  ipStatus.value = "正在获取";
  ipTone.value = "idle";
  httpsResult.value = "检测中";
  httpsTone.value = "idle";
  udpResult.value = "检测中";
  udpStatus.value = "正在收集 WebRTC ICE 候选";
  udpTone.value = "idle";
  testedAtISO.value = "";
  verdictDetail.value = "正在获取当前出口、HTTPS 和 UDP/STUN 状态。";

  const controller = new AbortController();
  const requestTimeout = window.setTimeout(() => controller.abort(), 8000);

  try {
    const response = await fetch("/api/v1/browser-check", {
      cache: "no-store",
      headers: { Accept: "application/json" },
      signal: controller.signal,
    });
    if (!response.ok) throw new Error(`检测服务返回异常（HTTP ${response.status}）`);

    const payload: unknown = await response.json();
    if (!isBrowserCheck(payload)) throw new Error("检测服务返回了无法识别的数据");

    publicIP.value = payload.public_ip || "未识别";
    ipStatus.value = payload.public_ip
      ? payload.public_ip.includes(":") ? "IPv6 出口" : "IPv4 出口"
      : "服务端未返回出口地址";
    ipTone.value = payload.public_ip ? "ok" : "warning";
    httpsResult.value = payload.https ? "可用" : "当前为 HTTP";
    httpsTone.value = payload.https ? "ok" : "warning";

    const udp = await checkStun(payload.stun_url);
    udpResult.value = udp.ok ? "可用" : "未确认";
    udpStatus.value = udp.ok ? "已取得公网映射候选" : udp.detail;
    udpTone.value = udp.ok ? "ok" : "warning";

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
    httpsTone.value = "error";
    udpResult.value = "未检测";
    udpStatus.value = "等待基础连接恢复";
    udpTone.value = "idle";
    verdictDetail.value = error instanceof DOMException && error.name === "AbortError"
      ? "连接检测服务超时，请稍后重试。"
      : error instanceof Error ? error.message : "请稍后重试。";
  } finally {
    window.clearTimeout(requestTimeout);
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

export function useBrowserCheck() {
  return {
    state,
    running,
    publicIP,
    ipStatus,
    ipTone,
    httpsResult,
    httpsTone,
    udpResult,
    udpStatus,
    udpTone,
    testedAtISO,
    testedAt,
    verdictTitle,
    verdictDetail,
    runQuickCheck,
  };
}
