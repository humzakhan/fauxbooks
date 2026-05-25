/**
 * ascii-bg.js — "Number Field + Cursor Wake"
 *
 * Full-viewport ambient grid of accounting digits that shimmers calmly at
 * rest and is disrupted by the cursor:  a "parting bubble" directly under
 * the pointer, a bright/pine scramble ring around it, and a ripple-shut
 * trail as agitation decays behind the moving cursor.
 *
 * Canvas:  #ascii-bg — fixed, full viewport, pointer-events:none.
 * Colors:  ink  rgba(26,26,24,α)    resting digits
 *          pine rgba(30,111,68,α)   agitation tint (on-brand wake)
 * Background fill: NEVER — clearRect only; CSS vignette handles bg.
 */

(function () {
  'use strict';

  // ── Grid constants ────────────────────────────────────────────────────────────

  const CELL_W = 16;          // logical px per cell (horizontal)
  const CELL_H = 22;          // logical px per cell (vertical)
  const MAX_DPR = 2;

  // Glyph pools
  const DIGITS  = '0123456789';
  const ACCTING = '=$Σ+-.,'   ;  // accounting flavour symbols
  const ACCENT_FRAC = 0.065;     // ~6.5 % of cells get an accounting glyph

  const FONT_SIZE = 13;          // px — slightly smaller than cell for breathing room
  const FONT_MONO = '"JetBrains Mono","Courier New",monospace';

  // ── Color constants ───────────────────────────────────────────────────────────

  const INK_R  = 26,  INK_G  = 26,  INK_B  = 24;
  const PINE_R = 30,  PINE_G = 111, PINE_B = 68;

  // Alpha range
  const ALPHA_REST = 0.11;   // resting digit opacity (quiet presence)
  const ALPHA_WAKE = 0.46;   // max opacity at full agitation (bright ring)

  // ── Animation constants ───────────────────────────────────────────────────────

  const REST_RATE    = 0.18;  // digit changes per second at rest (slow shimmer)
  const SCRAMBLE_RATE = 16;   // extra changes/s at full agitation (fast scramble)
  const K_SEED = 137.508;     // irrational multiplier for per-cell phase stagger

  // Agitation
  const DECAY      = 3.2;    // agitation decay rate (settles in ~0.3–0.4 s)
  const OUTER_R    = 150;    // px — outer influence radius of cursor
  const INNER_R    = 26;     // px — parting bubble radius (alpha → 0)
  const MAX_PUSH   = 5;      // px — max radial displacement of glyphs near cursor

  // ── State ─────────────────────────────────────────────────────────────────────

  let canvas, ctx;
  let dpr = 1;
  let W = 0, H = 0;         // logical viewport size
  let cols = 0, rows = 0;   // grid dimensions
  let agitation = null;      // Float32Array, length cols*rows
  let seeds     = null;      // Float32Array, per-cell deterministic seed [0,1)

  let cursorX = null, cursorY = null;

  let lastTime    = null;
  let resizeTimer = null;

  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  // ── Bootstrap ─────────────────────────────────────────────────────────────────

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

  function init() {
    canvas = document.getElementById('ascii-bg');
    if (!canvas) return;

    ctx = canvas.getContext('2d');
    dpr = Math.min(window.devicePixelRatio || 1, MAX_DPR);

    resize();
    window.addEventListener('resize', onResize, { passive: true });

    if (reducedMotion) {
      // Static frame — resting digit grid, no listeners, no rAF.
      drawFrame(0, 0);
      return;
    }

    // Pointer listeners — cursor drives agitation.
    window.addEventListener('mousemove',  onMouseMove,   { passive: true });
    window.addEventListener('touchmove',  onTouchMove,   { passive: true });
    window.addEventListener('mouseleave', onPointerLeave, { passive: true });
    window.addEventListener('touchend',   onPointerLeave, { passive: true });

    requestAnimationFrame(tick);
  }

  // ── Grid / resize ─────────────────────────────────────────────────────────────

  /**
   * Resize canvas to current viewport; recompute grid; reset transform.
   * Called on load and on the debounced resize event.
   */
  function resize() {
    W = window.innerWidth;
    H = window.innerHeight;

    canvas.width  = Math.round(W * dpr);
    canvas.height = Math.round(H * dpr);
    canvas.style.width  = W + 'px';
    canvas.style.height = H + 'px';

    // Reset transform before re-scaling (required after canvas resize).
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.scale(dpr, dpr);

    buildGrid();
  }

  /**
   * Build the per-cell seed and agitation arrays from the new grid dimensions.
   * Seeds are deterministic (hash of col+row) so glyphs don't jump on resize.
   */
  function buildGrid() {
    cols = Math.ceil(W / CELL_W) + 1;
    rows = Math.ceil(H / CELL_H) + 1;
    const n = cols * rows;

    agitation = new Float32Array(n);  // all zero — calm start

    seeds = new Float32Array(n);
    for (let r = 0; r < rows; r++) {
      for (let c = 0; c < cols; c++) {
        // Cheap deterministic hash → [0,1)
        const h = Math.sin(c * 127.1 + r * 311.7) * 43758.5453;
        seeds[r * cols + c] = h - Math.floor(h);
      }
    }
  }

  function onResize() {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(resize, 120);
  }

  // ── Glyph pool ────────────────────────────────────────────────────────────────

  /**
   * Return the glyph for cell (c, r) at elapsed time t seconds.
   * agit controls how fast the digit cycles (0 = slow rest, 1 = fast scramble).
   */
  function glyphAt(c, r, t, agit) {
    const seed = seeds[r * cols + c];

    // Determine whether this cell is an accounting-symbol cell (deterministic).
    const isAccent = seed < ACCENT_FRAC;

    const rate = REST_RATE + agit * SCRAMBLE_RATE;
    const idx  = Math.floor(t * rate + seed * K_SEED);

    if (isAccent) {
      return ACCTING[Math.abs(idx) % ACCTING.length];
    }
    return DIGITS[Math.abs(idx) % DIGITS.length];
  }

  // ── Agitation + pointer ───────────────────────────────────────────────────────

  function onMouseMove(e) {
    cursorX = e.clientX;
    cursorY = e.clientY;
  }

  function onTouchMove(e) {
    if (e.touches.length > 0) {
      cursorX = e.touches[0].clientX;
      cursorY = e.touches[0].clientY;
    }
  }

  function onPointerLeave() {
    cursorX = null;
    cursorY = null;
    // Existing agitation decays naturally — no hard reset.
  }

  /**
   * Decay all agitation and raise it near the cursor.
   * Called once per frame before drawing.
   *
   * exp(-DECAY*dt) keeps the decay time-based and frame-rate independent.
   */
  function updateAgitation(dt) {
    const decayFactor = Math.exp(-DECAY * dt);

    for (let i = 0, n = agitation.length; i < n; i++) {
      agitation[i] *= decayFactor;
      if (agitation[i] < 0.001) agitation[i] = 0;
    }

    if (cursorX === null || cursorY === null) return;

    // Raise agitation for cells within OUTER_R of cursor.
    // Iterate only over the relevant cell range for performance.
    const cMin = Math.max(0, Math.floor((cursorX - OUTER_R) / CELL_W));
    const cMax = Math.min(cols - 1, Math.ceil((cursorX + OUTER_R) / CELL_W));
    const rMin = Math.max(0, Math.floor((cursorY - OUTER_R) / CELL_H));
    const rMax = Math.min(rows - 1, Math.ceil((cursorY + OUTER_R) / CELL_H));

    for (let r = rMin; r <= rMax; r++) {
      const cy = r * CELL_H + CELL_H * 0.5;
      for (let c = cMin; c <= cMax; c++) {
        const cx = c * CELL_W + CELL_W * 0.5;
        const dist = Math.sqrt((cx - cursorX) * (cx - cursorX) + (cy - cursorY) * (cy - cursorY));
        if (dist >= OUTER_R) continue;

        const influence = 1 - dist / OUTER_R;
        const idx = r * cols + c;
        if (influence > agitation[idx]) {
          agitation[idx] = influence;
        }
      }
    }
  }

  // ── Draw ──────────────────────────────────────────────────────────────────────

  /**
   * Draw all cells for the current frame.
   * t  — elapsed seconds (for glyph cycling)
   * dt — frame delta seconds (unused here; used in agitation already)
   */
  function drawFrame(t, _dt) {
    ctx.clearRect(0, 0, W, H);

    ctx.font         = FONT_SIZE + 'px ' + FONT_MONO;
    ctx.textAlign    = 'center';
    ctx.textBaseline = 'middle';

    for (let r = 0; r < rows; r++) {
      const baseCY = r * CELL_H + CELL_H * 0.5;

      for (let c = 0; c < cols; c++) {
        const baseCX = c * CELL_W + CELL_W * 0.5;
        const idx    = r * cols + c;
        const agit   = agitation ? agitation[idx] : 0;

        // ── Parting bubble: fade toward 0 inside INNER_R ──
        let distToCursor = Infinity;
        if (cursorX !== null && cursorY !== null) {
          const dx = baseCX - cursorX;
          const dy = baseCY - cursorY;
          distToCursor = Math.sqrt(dx * dx + dy * dy);
        }

        // Inner fade: smooth falloff from 0 (right at cursor) to 1 (at INNER_R+)
        let innerFade = 1;
        if (distToCursor < INNER_R) {
          innerFade = distToCursor / INNER_R;
          innerFade = innerFade * innerFade; // quadratic — sharper hole
        }

        // ── Alpha: rest → wake brightness ──
        const alpha = (ALPHA_REST + agit * (ALPHA_WAKE - ALPHA_REST)) * innerFade;
        if (alpha < 0.005) continue; // skip invisible cells (perf)

        // ── Pine tint: blend ink → pine as agitation rises ──
        const r_ch = Math.round(INK_R + agit * (PINE_R - INK_R));
        const g_ch = Math.round(INK_G + agit * (PINE_G - INK_G));
        const b_ch = Math.round(INK_B + agit * (PINE_B - INK_B));

        ctx.fillStyle = 'rgba(' + r_ch + ',' + g_ch + ',' + b_ch + ',' + alpha.toFixed(3) + ')';

        // ── Subtle displacement: push cell outward from cursor ──
        let drawX = baseCX;
        let drawY = baseCY;
        if (cursorX !== null && distToCursor < OUTER_R && distToCursor > 0.1) {
          const push = agit * MAX_PUSH * (1 - distToCursor / OUTER_R);
          const nx   = (baseCX - cursorX) / distToCursor;
          const ny   = (baseCY - cursorY) / distToCursor;
          drawX = baseCX + nx * push;
          drawY = baseCY + ny * push;
        }

        // ── Glyph ──
        const glyph = reducedMotion
          ? glyphAt(c, r, 0, 0)   // static snapshot at t=0 for reduced-motion
          : glyphAt(c, r, t, agit);

        ctx.fillText(glyph, drawX, drawY);
      }
    }
  }

  // ── Animation loop ────────────────────────────────────────────────────────────

  function tick(now) {
    requestAnimationFrame(tick);

    if (lastTime === null) { lastTime = now; }
    const dt = Math.min((now - lastTime) * 0.001, 0.05); // cap at 50 ms
    lastTime = now;

    const t = now * 0.001;

    updateAgitation(dt);
    drawFrame(t, dt);
  }
})();
