const $ = (id) => document.getElementById(id);
const commands = {
  linux: `curl -fsSL ${location.origin}/install.sh | sh`,
  windows: `irm ${location.origin}/install.ps1 | iex`,
};
let platform = "linux";

function renderCommand() {
  $("command").textContent = commands[platform];
  $("linuxTab").classList.toggle("active", platform === "linux");
  $("windowsTab").classList.toggle("active", platform === "windows");
  $("linuxTab").setAttribute("aria-selected", String(platform === "linux"));
  $("windowsTab").setAttribute("aria-selected", String(platform === "windows"));
}

async function runQuickCheck() {
  const button = $("startButton");
  button.disabled = true;
  $("dial").className = "dial running";
  $("dialText").textContent = "检测中";
  $("verdictTitle").textContent = "正在连接检测服务";
  $("verdictDetail").textContent = "正在获取当前出口和 HTTPS 状态。";
  try {
    const response = await fetch("/api/v1/browser-check", { cache: "no-store" });
    if (!response.ok) throw new Error("服务返回异常");
    const result = await response.json();
    $("publicIP").textContent = result.public_ip || "未识别";
    $("ipStatus").textContent = result.public_ip?.includes(":") ? "IPv6 出口" : "IPv4 出口";
    $("httpsResult").textContent = result.https ? "可用" : "当前为 HTTP";
    const udp = await checkStun(result.stun_url);
    $("udpResult").textContent = udp.ok ? "可用" : "未确认";
    $("udpStatus").textContent = udp.ok ? "已取得公网映射候选" : udp.detail;
    $("testedAt").textContent = new Date(result.tested_at).toLocaleString();
    $("dial").className = "dial ok";
    $("dialText").textContent = "通过";
    $("verdictTitle").textContent = "基础网络可用";
    $("verdictDetail").textContent = udp.ok
      ? "HTTPS 与浏览器 UDP/STUN 路径均可用。运行深度命令可继续判断 NAT 行为。"
      : "HTTPS 可用，但浏览器未确认 UDP/STUN 路径。建议运行深度命令复测。";
  } catch (error) {
    $("dial").className = "dial";
    $("dialText").textContent = "失败";
    $("verdictTitle").textContent = "未能完成检测";
    $("verdictDetail").textContent = error.message || "请稍后重试。";
  } finally {
    button.disabled = false;
  }
}

async function checkStun(stunUrl) {
  if (!stunUrl || typeof RTCPeerConnection === "undefined") {
    return { ok: false, detail: "当前浏览器不支持 WebRTC 检测" };
  }
  const peer = new RTCPeerConnection({ iceServers: [{ urls: stunUrl }] });
  let srflx = false;
  peer.createDataChannel("probe");
  peer.addEventListener("icecandidate", (event) => {
    if (!event.candidate) return;
    const type = event.candidate.type || (/ typ ([a-z]+)/.exec(event.candidate.candidate)?.[1]);
    if (type === "srflx") srflx = true;
  });
  try {
    const offer = await peer.createOffer();
    await peer.setLocalDescription(offer);
    await Promise.race([
      new Promise((resolve) => peer.addEventListener("icegatheringstatechange", () => {
        if (peer.iceGatheringState === "complete") resolve();
      })),
      new Promise((resolve) => setTimeout(resolve, 4500)),
    ]);
    return { ok: srflx, detail: srflx ? "" : "未取得公网映射候选" };
  } catch (error) {
    return { ok: false, detail: "WebRTC 探测受浏览器策略限制" };
  } finally {
    peer.close();
  }
}

$("startButton").addEventListener("click", runQuickCheck);
$("linuxTab").addEventListener("click", () => { platform = "linux"; renderCommand(); });
$("windowsTab").addEventListener("click", () => { platform = "windows"; renderCommand(); });
$("copyButton").addEventListener("click", async () => {
  await navigator.clipboard.writeText(commands[platform]);
  $("copyButton").title = "已复制";
  setTimeout(() => { $("copyButton").title = "复制命令"; }, 1500);
});
renderCommand();
