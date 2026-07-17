import { onBeforeUnmount, ref } from "vue";

export type CopyState = "idle" | "copied" | "failed";

export function useClipboard(resetMs = 2200) {
  const copyState = ref<CopyState>("idle");
  let timer: number | undefined;

  async function copy(text: string): Promise<void> {
    if (timer !== undefined) window.clearTimeout(timer);
    try {
      await writeText(text);
      copyState.value = "copied";
    } catch {
      copyState.value = "failed";
    }
    timer = window.setTimeout(() => {
      copyState.value = "idle";
    }, resetMs);
  }

  onBeforeUnmount(() => {
    if (timer !== undefined) window.clearTimeout(timer);
  });

  return { copyState, copy };
}

async function writeText(text: string): Promise<void> {
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
