import { onMounted, ref } from "vue";

export type ServiceHealthState = "idle" | "checking" | "online" | "error";

const state = ref<ServiceHealthState>("idle");
let healthRequestStarted = false;
let retryTimer: number | undefined;

async function refreshServiceHealth(): Promise<void> {
  state.value = "checking";
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), 4000);

  try {
    const response = await fetch("/healthz", {
      cache: "no-store",
      headers: { Accept: "application/json" },
      signal: controller.signal,
    });
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const payload: unknown = await response.json();
    if (!isHealthyPayload(payload)) throw new Error("invalid health response");
    state.value = "online";
    if (retryTimer !== undefined) {
      window.clearTimeout(retryTimer);
      retryTimer = undefined;
    }
  } catch {
    state.value = "error";
    if (retryTimer === undefined) {
      retryTimer = window.setTimeout(() => {
        retryTimer = undefined;
        void refreshServiceHealth();
      }, 5000);
    }
  } finally {
    window.clearTimeout(timeout);
  }
}

function isHealthyPayload(value: unknown): value is { status: "ok" } {
  return typeof value === "object"
    && value !== null
    && (value as { status?: unknown }).status === "ok";
}

export function useServiceHealth() {
  onMounted(() => {
    if (healthRequestStarted) return;
    healthRequestStarted = true;
    void refreshServiceHealth();
  });
  return { state, refreshServiceHealth };
}
