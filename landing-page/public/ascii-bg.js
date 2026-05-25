/**
 * ascii-bg.js — dot-matrix ripple background with number mix-in
 *
 * Draws into <canvas id="ascii-bg"> (fixed full-viewport, z-index 0).
 * clearRect only — never fills the background.
 * DPR capped at 2.  Single rAF loop with delta-time.
 *
 * Grid: evenly-spaced dots, ~12% of cells replaced by a faint glyph
 * (digit or accounting symbol), chosen deterministically per cell so
 * they never flicker.  Both dots and glyphs follow the same slow wave
 * field + cursor ripple + pine tint near the cursor.
 *
 * prefers-reduced-motion: one static frame, no animation, no listeners.
 */

(function () {
  'use strict';

  /* ── reduced-motion guard ───────────────────────────────────────── */
  const prefersReduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  /* ── colour tokens ──────────────────────────────────────────────── */
  const INK_R  = 26,  INK_G  = 26,  INK_B  = 24;   /* #1A1A18 */
  const PINE_R = 30,  PINE_G = 111, PINE_B = 68;    /* #1E6F44 */

  /* ── grid / dot params ──────────────────────────────────────────── */
  const SPACING    = 22;    /* grid pitch, logical px */
  const BASE_R     = 0.55;  /* minimum dot radius, logical px */
  const AMP        = 2.0;   /* wave swell amplitude */
  const MAX_R      = 2.8;   /* hard cap on dot radius */
  const BASE_ALPHA = 0.11;  /* base dot/glyph alpha (quiet resting level) */

  /* ── wave field ─────────────────────────────────────────────────── */
  /* two slow crossing sine fields */
  const FX1 = 0.016, FY1 = 0.012, SPD1 = 0.42;
  const FX2 = 0.010, FY2 = 0.018, SPD2 = 0.31, PH2 = 1.9;

  /* ── global breathing pulse ─────────────────────────────────────── */
  /* slow sine that modulates ALL dots together — the whole field breathes */
  const PULSE_PERIOD = 4.5;   /* seconds per full breath cycle */
  const PULSE_ALPHA  = 0.07;  /* alpha swing ±0.07 from global pulse (visibly breathes) */
  const PULSE_R      = 1.0;   /* radius swing ±1.0 from global pulse (dots noticeably grow/shrink) */

  /* ── cursor ripple ──────────────────────────────────────────────── */
  const RIPPLE_K          = 0.07;    /* spatial frequency of ring (logical px⁻¹; wavelength ~90px) */
  const RIPPLE_SPD        = 3.8;    /* ring travel speed */
  const RIPPLE_AMP        = 3.0;    /* max extra radius (logical px; clearly swells near dots) */
  const RIPPLE_FALLOFF    = 0.00004; /* gaussian falloff per logical px² (~half-strength at 120px) */
  const RIPPLE_NEAR       = 90;     /* logical px inside which pine tint applies */
  const RIPPLE_DECAY      = 1.8;    /* ripple amplitude decay /s when cursor stops */
  const RIPPLE_ALPHA_BOOST = 0.12;  /* extra alpha boost near cursor */

  /* ── glyph mix-in ───────────────────────────────────────────────── */
  /* ~12% of cells get a glyph; the rest stay dots */
  const GLYPH_FRAC   = 0.12;
  const GLYPH_POOL   = '0123456789=$Σ+-.,' ;
  const FONT_SIZE    = 12;   /* px, logical */
  const FONT_FAMILY  = '"JetBrains Mono","Courier New",monospace';
  /* glyphs scale slightly with the wave (font-size multiplier range) */
  const GLYPH_SCALE_BASE = 0.85;
  const GLYPH_SCALE_AMP  = 0.30;  /* ±0.30 of font-size at wave peak */

  /* ── canvas / DPR ───────────────────────────────────────────────── */
  const DPR = Math.min(window.devicePixelRatio || 1, 2);

  /* ── state ──────────────────────────────────────────────────────── */
  let canvas, ctx;
  let W = 0, H = 0;     /* physical px (canvas buffer dimensions) */

  /**
   * Each cell: { cx, cy, isGlyph, glyphChar }
   * cx/cy are logical px (pre-DPR).
   * isGlyph and glyphChar are fixed at grid-build time (deterministic).
   */
  let cells = [];

  /* cursor / ripple */
  let cursor     = { x: -99999, y: -99999 }; /* physical px */
  let rippleTime = 0;
  let rippleAmp  = 0;
  let tPrev      = null;
  let t          = 0;

  const TAU = Math.PI * 2;

  /* ── deterministic per-cell hash ────────────────────────────────── */
  /* Given integer grid indices (col, row), return a stable float [0,1). */
  function cellHash(col, row) {
    /* simple integer hash — no Math.random() so it survives resize rebuild */
    let h = (col * 2654435761) ^ (row * 2246822519);
    h = ((h >>> 16) ^ h) * 0x45d9f3b;
    h = ((h >>> 16) ^ h) * 0x45d9f3b;
    h = (h >>> 16) ^ h;
    /* keep positive, map to [0,1) */
    return (h >>> 0) / 0xffffffff;
  }

  /* ── build grid ─────────────────────────────────────────────────── */
  function buildGrid() {
    const wL = Math.ceil(W / DPR);
    const hL = Math.ceil(H / DPR);
    /* wider spacing on very large screens to keep dot count sane */
    const sp  = (wL > 1600 || hL > 1200) ? 28 : SPACING;
    const offX = (wL % sp) / 2;
    const offY = (hL % sp) / 2;

    cells = [];
    let col = 0;
    for (let cx = offX; cx < wL; cx += sp, col++) {
      let row = 0;
      for (let cy = offY; cy < hL; cy += sp, row++) {
        const h1 = cellHash(col, row);
        const isGlyph = h1 < GLYPH_FRAC;
        let glyphChar = '';
        if (isGlyph) {
          /* pick glyph from pool using a second hash value derived from h1 */
          const h2 = cellHash(col + 10007, row + 10007);
          glyphChar = GLYPH_POOL[Math.floor(h2 * GLYPH_POOL.length)];
        }
        cells.push({ cx, cy, isGlyph, glyphChar });
      }
    }
  }

  /* ── resize ─────────────────────────────────────────────────────── */
  function resize() {
    W = window.innerWidth  * DPR;
    H = window.innerHeight * DPR;
    canvas.width  = W;
    canvas.height = H;
    canvas.style.width  = window.innerWidth  + 'px';
    canvas.style.height = window.innerHeight + 'px';
    buildGrid();

    if (prefersReduced) {
      renderStatic();
    }
  }

  /* ── static render (prefers-reduced-motion) ─────────────────────── */
  function renderStatic() {
    ctx.clearRect(0, 0, W, H);

    /* dots — use new fainter BASE_ALPHA, no pulse/motion */
    const dotR   = (BASE_R + AMP * 0.35) * DPR;
    const dotA   = BASE_ALPHA;
    ctx.fillStyle = 'rgba(' + INK_R + ',' + INK_G + ',' + INK_B + ',' + dotA + ')';

    for (let i = 0; i < cells.length; i++) {
      const { cx, cy, isGlyph } = cells[i];
      if (isGlyph) continue;
      ctx.beginPath();
      ctx.arc(cx * DPR, cy * DPR, dotR, 0, TAU);
      ctx.fill();
    }

    /* glyphs — slightly softer alpha for static frame */
    ctx.font         = FONT_SIZE * DPR + 'px ' + FONT_FAMILY;
    ctx.textAlign    = 'center';
    ctx.textBaseline = 'middle';
    const glyphA = BASE_ALPHA * 0.75;
    ctx.fillStyle = 'rgba(' + INK_R + ',' + INK_G + ',' + INK_B + ',' + glyphA + ')';

    for (let i = 0; i < cells.length; i++) {
      const { cx, cy, isGlyph, glyphChar } = cells[i];
      if (!isGlyph) continue;
      ctx.fillText(glyphChar, cx * DPR, cy * DPR);
    }
  }

  /* ── animated frame ─────────────────────────────────────────────── */
  function frame(ts) {
    requestAnimationFrame(frame);

    /* delta-time, clamped to avoid spiral-of-death */
    if (tPrev === null) tPrev = ts;
    const dt = Math.min((ts - tPrev) / 1000, 0.05);
    tPrev = ts;
    t    += dt;

    /* decay ripple when cursor is idle */
    if (rippleAmp > 0) {
      rippleAmp = Math.max(0, rippleAmp - dt * RIPPLE_DECAY);
    }

    ctx.clearRect(0, 0, W, H);

    /* ── global breathing pulse (same for all dots this frame) ────── */
    /* normalised [0,1]: 0 = exhale, 1 = inhale */
    const globalPulse = 0.5 + 0.5 * Math.sin((t / PULSE_PERIOD) * TAU);

    const cx0      = cursor.x;
    const cy0      = cursor.y;
    const rippleAge = (t - rippleTime) * RIPPLE_SPD;

    /* cache font setup — only glyphs need it; dots use arc */
    let fontSet = false;

    for (let i = 0; i < cells.length; i++) {
      const { cx, cy, isGlyph, glyphChar } = cells[i];
      /* physical px coordinates */
      const px = cx * DPR;
      const py = cy * DPR;

      /* ── ambient wave field ──────────────────────────────── */
      const wave1   = 0.5 + 0.5 * Math.sin(cx * FX1 + cy * FY1 + t * SPD1);
      const wave2   = 0.5 + 0.5 * Math.sin(cx * FX2 + cy * FY2 + t * SPD2 + PH2);
      const waveMix = (wave1 + wave2) * 0.5;   /* [0, 1] */

      /* base dot radius: spatial wave + global pulse together */
      let r = BASE_R + AMP * waveMix + PULSE_R * globalPulse;

      /* ── cursor ripple ───────────────────────────────────── */
      let pine      = 0;
      let nearBoost = 0;
      if (rippleAmp > 0.001) {
        const dx    = px - cx0;
        const dy    = py - cy0;
        const dist  = Math.sqrt(dx * dx + dy * dy);
        /* convert to logical px so constants are DPR-independent */
        const distL = dist / DPR;

        const ring     = Math.sin(distL * RIPPLE_K - rippleAge);
        const gaussian = Math.exp(-distL * distL * RIPPLE_FALLOFF);
        const extra    = RIPPLE_AMP * ring * gaussian * rippleAmp;
        r += extra;

        if (distL < RIPPLE_NEAR) {
          const nearFrac = Math.max(0, 1 - distL / RIPPLE_NEAR);
          pine      = nearFrac * rippleAmp;
          nearBoost = RIPPLE_ALPHA_BOOST * nearFrac * rippleAmp;
        }
      }

      r = Math.max(0.05, Math.min(r, MAX_R));

      /* ── alpha: base + spatial wave lift + global pulse + near-cursor boost */
      const alpha = BASE_ALPHA
                  + 0.04 * waveMix          /* spatial local variation */
                  + PULSE_ALPHA * globalPulse  /* global breathing */
                  + nearBoost;              /* cursor proximity highlight */

      /* ── colour: ink → pine near cursor ─────────────────── */
      const rr = Math.round(INK_R + (PINE_R - INK_R) * pine);
      const gg = Math.round(INK_G + (PINE_G - INK_G) * pine);
      const bb = Math.round(INK_B + (PINE_B - INK_B) * pine);

      ctx.fillStyle = 'rgba(' + rr + ',' + gg + ',' + bb + ',' + alpha.toFixed(3) + ')';

      if (!isGlyph) {
        /* ── dot ─────────────────────────────────────────── */
        ctx.beginPath();
        ctx.arc(px, py, r * DPR, 0, TAU);
        ctx.fill();
      } else {
        /* ── glyph ───────────────────────────────────────── */
        /* font-size scales with wave + global pulse */
        const scaledSize = FONT_SIZE * DPR * (GLYPH_SCALE_BASE + GLYPH_SCALE_AMP * waveMix + 0.08 * globalPulse);
        const sizeStr    = scaledSize.toFixed(1) + 'px ' + FONT_FAMILY;
        if (!fontSet || ctx.font !== sizeStr) {
          ctx.font         = sizeStr;
          ctx.textAlign    = 'center';
          ctx.textBaseline = 'middle';
          fontSet = true;
        }
        ctx.fillText(glyphChar, px, py);
      }
    }
  }

  /* ── bootstrap ──────────────────────────────────────────────────── */
  function init() {
    canvas = document.getElementById('ascii-bg');
    if (!canvas) return;
    ctx = canvas.getContext('2d');

    resize();

    if (prefersReduced) {
      /* static frame already drawn by resize(); no further setup */
      return;
    }

    window.addEventListener('resize', resize, { passive: true });

    /* cursor / touch tracking */
    function onMove(x, y) {
      cursor.x   = x * DPR;
      cursor.y   = y * DPR;
      rippleTime = t;
      rippleAmp  = 1;
    }
    window.addEventListener('mousemove', function (e) { onMove(e.clientX, e.clientY); }, { passive: true });
    window.addEventListener('touchmove', function (e) {
      var t0 = e.touches[0];
      if (t0) onMove(t0.clientX, t0.clientY);
    }, { passive: true });
    window.addEventListener('mouseleave', function () {
      cursor.x = -99999;
      cursor.y = -99999;
    }, { passive: true });

    requestAnimationFrame(frame);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

})();
