import { useEffect, useRef } from "react";
import { useMotion } from "../motion/MotionContext";

type Particle = {
  x: number;
  y: number;
  r: number;
  vx: number;
  vy: number;
  a: number;
  pulse: number;
  pulseSpeed: number;
};

type PerfTier = "full" | "medium" | "lite";

function perfTier(w: number, h: number): PerfTier {
  const area = w * h;
  // Phones / small laptops: keep ambient cheap.
  if (area < 900_000 || w < 900) return "lite";
  if (area < 1_600_000 || w < 1280) return "medium";
  return "full";
}

function adaptiveCount(pref: number, tier: PerfTier, w: number, h: number): number {
  const areaScale = Math.min(1, (w * h) / (1440 * 900));
  let cap = pref;
  if (tier === "lite") cap = Math.min(pref, 36);
  else if (tier === "medium") cap = Math.min(pref, 72);
  else cap = Math.min(pref, 140);
  return Math.max(8, Math.round(cap * (0.55 + 0.45 * areaScale)));
}

/**
 * Ambient particle field with adaptive cost for small / weak displays.
 * Lite tier skips O(n²) links and radial glows; pauses when the tab is hidden.
 */
export function AmbientParticles() {
  const { motionEnabled, particleCount } = useMotion();
  const canvasRef = useRef<HTMLCanvasElement | null>(null);

  useEffect(() => {
    if (!motionEnabled) return;
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d", { alpha: true });
    if (!ctx) return;

    let raf = 0;
    let particles: Particle[] = [];
    let w = 0;
    let h = 0;
    let tier: PerfTier = "full";
    let dpr = 1;
    let color = "#3bc4ae";
    let color2 = "#2aa391";
    let lastFrame = 0;
    let hidden = document.hidden;
    let linkDist2 = 0;
    let drawLinks = false;
    let drawGlow = false;
    let targetMs = 1000 / 60;

    const refreshColors = () => {
      const styles = getComputedStyle(document.documentElement);
      color =
        styles.getPropertyValue("--color-signal-2").trim() ||
        styles.getPropertyValue("--color-signal").trim() ||
        "#3bc4ae";
      color2 = styles.getPropertyValue("--color-signal").trim() || "#2aa391";
    };

    const spawn = (randomY: boolean): Particle => ({
      x: Math.random() * w,
      y: randomY ? Math.random() * h : h + Math.random() * 60,
      r: tier === "lite" ? 1.2 + Math.random() * 1.8 : 1.4 + Math.random() * 3.2,
      vx: (Math.random() - 0.5) * (tier === "lite" ? 0.28 : 0.45),
      vy: -0.14 - Math.random() * (tier === "lite" ? 0.35 : 0.55),
      a: 0.4 + Math.random() * 0.45,
      pulse: Math.random() * Math.PI * 2,
      pulseSpeed: 0.012 + Math.random() * 0.025,
    });

    const rebuild = () => {
      tier = perfTier(w, h);
      const count = adaptiveCount(particleCount, tier, w, h);
      particles = Array.from({ length: count }, () => spawn(true));
      drawLinks = tier === "full" && count <= 90;
      drawGlow = tier !== "lite";
      const linkDist = tier === "full" ? 150 : 120;
      linkDist2 = linkDist * linkDist;
      targetMs = tier === "lite" ? 1000 / 30 : tier === "medium" ? 1000 / 45 : 1000 / 60;
      dpr = tier === "lite" ? 1 : Math.min(window.devicePixelRatio || 1, tier === "medium" ? 1.25 : 1.5);
      canvas.width = Math.max(1, Math.floor(w * dpr));
      canvas.height = Math.max(1, Math.floor(h * dpr));
      canvas.style.width = `${w}px`;
      canvas.style.height = `${h}px`;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      refreshColors();
    };

    const resize = () => {
      w = window.innerWidth;
      h = window.innerHeight;
      rebuild();
    };

    const tick = (now: number) => {
      raf = window.requestAnimationFrame(tick);
      if (hidden) return;
      if (now - lastFrame < targetMs - 1) return;
      lastFrame = now;

      ctx.clearRect(0, 0, w, h);

      if (drawLinks) {
        for (let i = 0; i < particles.length; i++) {
          const a = particles[i]!;
          // Only check a short window ahead — enough for a soft field, O(n) not O(n²).
          const jMax = Math.min(particles.length, i + 12);
          for (let j = i + 1; j < jMax; j++) {
            const b = particles[j]!;
            const dx = a.x - b.x;
            const dy = a.y - b.y;
            const d2 = dx * dx + dy * dy;
            if (d2 >= linkDist2) continue;
            const t = 1 - d2 / linkDist2;
            ctx.beginPath();
            ctx.strokeStyle = color;
            ctx.globalAlpha = 0.06 + t * 0.16;
            ctx.lineWidth = 0.7 + t;
            ctx.moveTo(a.x, a.y);
            ctx.lineTo(b.x, b.y);
            ctx.stroke();
          }
        }
      }

      for (const p of particles) {
        p.x += p.vx;
        p.y += p.vy;
        p.pulse += p.pulseSpeed;
        if (p.y < -20 || p.x < -30 || p.x > w + 30) {
          Object.assign(p, spawn(false));
          p.y = h + 12;
        }

        const alpha = Math.min(1, p.a * (0.85 + Math.sin(p.pulse) * 0.15));

        if (drawGlow) {
          const glow = p.r * (1.5 + Math.sin(p.pulse) * 0.3);
          const g = ctx.createRadialGradient(p.x, p.y, 0, p.x, p.y, glow * 2.6);
          g.addColorStop(0, color);
          g.addColorStop(0.4, color2);
          g.addColorStop(1, "transparent");
          ctx.globalAlpha = alpha * 0.28;
          ctx.fillStyle = g;
          ctx.beginPath();
          ctx.arc(p.x, p.y, glow * 2.6, 0, Math.PI * 2);
          ctx.fill();
        }

        ctx.globalAlpha = alpha;
        ctx.fillStyle = color;
        ctx.beginPath();
        ctx.arc(p.x, p.y, p.r, 0, Math.PI * 2);
        ctx.fill();

        if (tier !== "lite") {
          ctx.globalAlpha = Math.min(1, alpha + 0.15);
          ctx.fillStyle = "#ffffff";
          ctx.beginPath();
          ctx.arc(p.x, p.y, Math.max(0.5, p.r * 0.32), 0, Math.PI * 2);
          ctx.fill();
        }
      }

      ctx.globalAlpha = 1;
    };

    const onVisibility = () => {
      hidden = document.hidden;
      if (!hidden) {
        lastFrame = 0;
        refreshColors();
      }
    };

    resize();
    window.addEventListener("resize", resize);
    document.addEventListener("visibilitychange", onVisibility);
    raf = window.requestAnimationFrame(tick);
    return () => {
      window.cancelAnimationFrame(raf);
      window.removeEventListener("resize", resize);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [motionEnabled, particleCount]);

  if (!motionEnabled) return null;

  return (
    <canvas
      ref={canvasRef}
      aria-hidden
      className="pointer-events-none fixed inset-0 z-0"
      style={{ opacity: 0.9 }}
    />
  );
}
