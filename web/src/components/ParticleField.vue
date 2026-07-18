<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useBrowserCheck } from "../composables/useBrowserCheck";
import type { CheckState } from "../types";

interface Particle {
  x: number;
  y: number;
  vx: number;
  vy: number;
  r: number;
}

interface Theme {
  rgb: [number, number, number];
  energy: number;
}

interface NavigatorWithConnection extends Navigator {
  connection?: { saveData?: boolean };
}

/** 粒子颜色与活跃度随检测状态平滑过渡 */
const THEMES: Record<CheckState, Theme> = {
  idle: { rgb: [22, 122, 104], energy: 1 },
  running: { rgb: [53, 111, 209], energy: 1.9 },
  ok: { rgb: [22, 122, 104], energy: 1.2 },
  warning: { rgb: [180, 83, 9], energy: 1.2 },
  error: { rgb: [190, 61, 46], energy: 1.2 },
};

const LINK_DIST = 132;
const MOUSE_DIST = 170;
const BASE_FRAME_INTERVAL = 1000 / 60;
const FRAME_INTERVAL = 1000 / 30;

const { state } = useBrowserCheck();
const canvas = ref<HTMLCanvasElement | null>(null);

let ctx: CanvasRenderingContext2D | null = null;
let particles: Particle[] = [];
let raf = 0;
let width = 0;
let height = 0;
let running = false;
let lastFrame = 0;
const mouse = { x: -9999, y: -9999 };
const current = { r: 22, g: 122, b: 104, energy: 1 };
let target: Theme = THEMES[state.value];

const motionQuery = window.matchMedia("(prefers-reduced-motion: reduce)");
const saveData = (navigator as NavigatorWithConnection).connection?.saveData === true;
let reducedMotion = motionQuery.matches;

watch(state, (next) => {
  target = THEMES[next];
  if (reducedMotion || saveData) {
    setCurrentTheme(target);
    drawFrame();
  }
});

function resize(): void {
  const el = canvas.value;
  if (!el || !ctx) return;
  const dpr = Math.min(window.devicePixelRatio || 1, 2);
  width = el.clientWidth;
  height = el.clientHeight;
  el.width = Math.round(width * dpr);
  el.height = Math.round(height * dpr);
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  seed();
  if (reducedMotion || saveData) drawFrame();
}

function seed(): void {
  const count = Math.min(96, Math.max(24, Math.floor((width * height) / 14000)));
  particles = Array.from({ length: count }, () => ({
    x: Math.random() * width,
    y: Math.random() * height,
    vx: (Math.random() - 0.5) * 0.45,
    vy: (Math.random() - 0.5) * 0.45,
    r: 1.4 + Math.random() * 1.3,
  }));
}

function tick(timestamp: number): void {
  if (!running) return;
  raf = requestAnimationFrame(tick);

  if (lastFrame && timestamp - lastFrame < FRAME_INTERVAL) return;
  const elapsed = lastFrame ? Math.min(timestamp - lastFrame, FRAME_INTERVAL * 2) : FRAME_INTERVAL;
  const frameScale = elapsed / BASE_FRAME_INTERVAL;
  lastFrame = timestamp;

  const ease = 1 - Math.pow(1 - 0.045, frameScale);
  current.r += (target.rgb[0] - current.r) * ease;
  current.g += (target.rgb[1] - current.g) * ease;
  current.b += (target.rgb[2] - current.b) * ease;
  current.energy += (target.energy - current.energy) * ease;

  for (const p of particles) {
    p.x += p.vx * current.energy * frameScale;
    p.y += p.vy * current.energy * frameScale;

    const dx = p.x - mouse.x;
    const dy = p.y - mouse.y;
    const md2 = dx * dx + dy * dy;
    if (md2 > 0.01 && md2 < MOUSE_DIST * MOUSE_DIST) {
      const md = Math.sqrt(md2);
      const force = ((MOUSE_DIST - md) / MOUSE_DIST) * 0.4;
      p.x += (dx / md) * force * frameScale;
      p.y += (dy / md) * force * frameScale;
    }

    if (p.x < -24) p.x = width + 24;
    else if (p.x > width + 24) p.x = -24;
    if (p.y < -24) p.y = height + 24;
    else if (p.y > height + 24) p.y = -24;
  }

  drawFrame();
}

function drawFrame(): void {
  if (!ctx) return;
  const r = Math.round(current.r);
  const g = Math.round(current.g);
  const b = Math.round(current.b);
  ctx.clearRect(0, 0, width, height);

  const linkDist2 = LINK_DIST * LINK_DIST;
  for (let i = 0; i < particles.length; i += 1) {
    const a = particles[i];
    if (!a) continue;
    for (let j = i + 1; j < particles.length; j += 1) {
      const c = particles[j];
      if (!c) continue;
      const dx = a.x - c.x;
      const dy = a.y - c.y;
      const d2 = dx * dx + dy * dy;
      if (d2 < linkDist2) {
        const alpha = (1 - Math.sqrt(d2) / LINK_DIST) * 0.15;
        ctx.strokeStyle = `rgba(${r},${g},${b},${alpha.toFixed(3)})`;
        ctx.lineWidth = 1;
        ctx.beginPath();
        ctx.moveTo(a.x, a.y);
        ctx.lineTo(c.x, c.y);
        ctx.stroke();
      }
    }
  }

  const mouseDist2 = MOUSE_DIST * MOUSE_DIST;
  for (const p of particles) {
    const dx = p.x - mouse.x;
    const dy = p.y - mouse.y;
    const d2 = dx * dx + dy * dy;
    if (d2 < mouseDist2) {
      const alpha = (1 - Math.sqrt(d2) / MOUSE_DIST) * 0.28;
      ctx.strokeStyle = `rgba(${r},${g},${b},${alpha.toFixed(3)})`;
      ctx.lineWidth = 1;
      ctx.beginPath();
      ctx.moveTo(p.x, p.y);
      ctx.lineTo(mouse.x, mouse.y);
      ctx.stroke();
    }
  }

  ctx.fillStyle = `rgba(${r},${g},${b},0.4)`;
  for (const p of particles) {
    ctx.beginPath();
    ctx.arc(p.x, p.y, p.r, 0, Math.PI * 2);
    ctx.fill();
  }
}

function start(): void {
  if (running || document.hidden || reducedMotion || saveData) return;
  running = true;
  lastFrame = 0;
  raf = requestAnimationFrame(tick);
}

function stop(): void {
  running = false;
  cancelAnimationFrame(raf);
}

function onPointerMove(event: PointerEvent): void {
  mouse.x = event.clientX;
  mouse.y = event.clientY;
}

function onPointerLeave(): void {
  mouse.x = -9999;
  mouse.y = -9999;
}

function onVisibilityChange(): void {
  if (document.hidden) stop();
  else start();
}

function setCurrentTheme(theme: Theme): void {
  [current.r, current.g, current.b] = theme.rgb;
  current.energy = theme.energy;
}

function onMotionChange(event: MediaQueryListEvent): void {
  reducedMotion = event.matches;
  if (reducedMotion) {
    stop();
    setCurrentTheme(target);
    drawFrame();
  } else if (!document.hidden) {
    start();
  }
}

onMounted(() => {
  const el = canvas.value;
  if (!el) return;
  ctx = el.getContext("2d");
  if (!ctx) return;

  resize();
  window.addEventListener("resize", resize);
  window.addEventListener("pointermove", onPointerMove, { passive: true });
  document.documentElement.addEventListener("pointerleave", onPointerLeave);
  document.addEventListener("visibilitychange", onVisibilityChange);
  motionQuery.addEventListener("change", onMotionChange);
  start();
});

onBeforeUnmount(() => {
  stop();
  window.removeEventListener("resize", resize);
  window.removeEventListener("pointermove", onPointerMove);
  document.documentElement.removeEventListener("pointerleave", onPointerLeave);
  document.removeEventListener("visibilitychange", onVisibilityChange);
  motionQuery.removeEventListener("change", onMotionChange);
});
</script>

<template>
  <canvas ref="canvas" class="particle-field" aria-hidden="true" />
</template>

<style scoped>
.particle-field {
  position: fixed;
  inset: 0;
  z-index: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
}
</style>
