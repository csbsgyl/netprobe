export type CheckState = "idle" | "running" | "ok" | "warning" | "error";

/** 指标卡的状态色调，与检测结论无关的展示态用 idle。 */
export type Tone = "idle" | "ok" | "warning" | "error";

export interface BrowserCheck {
  public_ip?: string;
  https: boolean;
  method: string;
  verdict: string;
  tested_at: string;
  stun_url?: string;
}

export interface StunResult {
  ok: boolean;
  detail: string;
}
