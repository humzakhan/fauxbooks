/**
 * ascii-bg.js — "the ledger reconciles around your cursor"
 *
 * Draws a full-viewport grid of faint accounting glyphs on #ascii-bg.
 * Near the cursor, glyphs resolve into balanced bookkeeping columns.
 * Respects prefers-reduced-motion.
 *
 * Colors:
 *   paper   #FAF8F3  (CSS background — canvas stays transparent)
 *   ink     #1A1A18
 *   oxblood #8C2F2A
 */

(function () {
  // ── Constants ────────────────────────────────────────────────────────────────

  const FONT_SIZE   = 13;           // px, logical
  const CELL_W      = 9;            // px per column (monospace pitch)
  const CELL_H      = 18;           // px per row
  const MAX_DPR     = 2;

  const INK         = '26,26,24';   // RGB components of #1A1A18
  const OXBLOOD     = '140,47,42';  // RGB components of #8C2F2A

  const ALPHA_REST  = 0.055;        // resting glyph opacity
  const ALPHA_MAX   = 0.48;         // max opacity near cursor
  const RADIUS      = 190;          // px — brightening radius around cursor
  const RIPPLE_AMP  = 0.08;         // additional alpha from ripple sine
  const SHIMMER_AMP = 0.018;        // ambient shimmer amplitude
  const LERP_SPEED  = 0.09;         // cursor easing factor per frame

  // Far-from-cursor texture: accounting punctuation, faint rules
  const REST_GLYPHS = [
    '·', '·', '·', '.', ',', '¦', '–', ' ', ' ', '·', '·',
    '░', '-', '·', ',', '.', '¦', '·', '·', ' ', '–',
  ];

  // Near-cursor resolved glyphs: digits and bookkeeping symbols
  const NEAR_GLYPHS = [
    '0','1','2','3','4','5','6','7','8','9',
    '=','+','-','.','$','Σ','0','0','0','1',
  ];

  // Balanced pairs rendered as a mini ledger row near cursor origin
  // Each entry: [left value string, right value string] — they balance by design
  const LEDGER_SEQS = [
    ['1,240', '1,240'],
    ['5,910', '5,910'],
    ['  800', '  800'],
    ['3,072', '3,072'],
    ['  415', '  415'],
    ['12,500','12,500'],
    ['  990', '  990'],
    [' 6,000',' 6,000'],
  ];

  // ── State ────────────────────────────────────────────────────────────────────

  let canvas, ctx;
  let dpr = 1;
  let cols = 0, rows = 0;
  let cells = [];           // flat array of { glyph, seed }
  let pointer  = { x: -9999, y: -9999 };  // eased cursor position (logical px)
  let target   = { x: -9999, y: -9999 };  // raw cursor position
  let hasPointer = false;
  let fadeAlpha  = 0;       // 0 = no highlight, 1 = full highlight
  let raf        = null;
  let resizeTimer = null;

  // Precomputed ledger layout anchored at cursor — rebuilt each frame
  // Maps flat cell index → forced glyph string (may be multi-char drawn at that cell)
  let ledgerOverlay = new Map();

  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  // ── Init ─────────────────────────────────────────────────────────────────────

  function init() {
    canvas = document.getElementById('ascii-bg');
    if (!canvas) return;

    ctx = canvas.getContext('2d');
    dpr = Math.min(window.devicePixelRatio || 1, MAX_DPR);

    resize();
    bindEvents();

    if (reducedMotion) {
      drawStatic();
      return;
    }

    loop();
  }

  // ── Grid ─────────────────────────────────────────────────────────────────────

  function resize() {
    const W = window.innerWidth;
    const H = window.innerHeight;

    canvas.width  = Math.round(W * dpr);
    canvas.height = Math.round(H * dpr);
    canvas.style.width  = W + 'px';
    canvas.style.height = H + 'px';

    ctx.scale(dpr, dpr);

    cols = Math.ceil(W / CELL_W) + 1;
    rows = Math.ceil(H / CELL_H) + 1;

    buildGrid();
  }

  function buildGrid() {
    cells = [];
    const total = cols * rows;
    for (let i = 0; i < total; i++) {
      // Stable per-cell seed for deterministic glyph selection & shimmer phase
      const seed = pseudoRand(i * 7919 + 1);
      const glyph = REST_GLYPHS[Math.floor(seed * REST_GLYPHS.length)];
      cells.push({ glyph, seed });
    }
  }

  // ── Glyphs / alphabet ────────────────────────────────────────────────────────

  /**
   * Cheap deterministic float [0,1) from an integer seed.
   * Used only at build time, not in hot path.
   */
  function pseudoRand(n) {
    n = ((n >> 16) ^ n) * 0x45d9f3b;
    n = ((n >> 16) ^ n) * 0x45d9f3b;
    n = (n >> 16) ^ n;
    return (n >>> 0) / 0xffffffff;
  }

  // ── Pointer ──────────────────────────────────────────────────────────────────

  function bindEvents() {
    window.addEventListener('mousemove', onMouseMove, { passive: true });
    window.addEventListener('mouseleave', onMouseLeave, { passive: true });
    window.addEventListener('touchmove', onTouchMove, { passive: true });
    window.addEventListener('resize', onResize, { passive: true });
  }

  function onMouseMove(e) {
    target.x = e.clientX;
    target.y = e.clientY;
    hasPointer = true;
  }

  function onMouseLeave() {
    hasPointer = false;
  }

  function onTouchMove(e) {
    if (e.touches.length > 0) {
      target.x = e.touches[0].clientX;
      target.y = e.touches[0].clientY;
      hasPointer = true;
    }
  }

  function onResize() {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(function () {
      // Reset scale before resizing to avoid compounding transforms
      ctx.setTransform(1, 0, 0, 1, 0, 0);
      resize();
    }, 120);
  }

  // ── Ledger overlay ───────────────────────────────────────────────────────────

  /**
   * Compute which cells near the cursor should show a "balanced" ledger row.
   * We pick a few rows anchored around the cursor row, writing:
   *   [debit] = [credit]
   * in digit columns, so it reads like a real ledger.
   */
  function buildLedgerOverlay(cx, cy) {
    ledgerOverlay.clear();

    const centerCol = Math.floor(cx / CELL_W);
    const centerRow = Math.floor(cy / CELL_H);

    // Render 5 ledger rows centered on the cursor row
    const seqCount = LEDGER_SEQS.length;
    for (let dr = -2; dr <= 2; dr++) {
      const row = centerRow + dr;
      if (row < 0 || row >= rows) continue;

      const seqIdx = ((row % seqCount) + seqCount) % seqCount;
      const [lv, rv] = LEDGER_SEQS[seqIdx];

      // Build the string: "  1,240.00 = 1,240.00  "
      const ledgerStr = lv + '.00 = ' + rv + '.00';
      const strLen = ledgerStr.length;

      // Center the ledger string around centerCol
      const startCol = centerCol - Math.floor(strLen / 2);

      for (let ci = 0; ci < strLen; ci++) {
        const col = startCol + ci;
        if (col < 0 || col >= cols) continue;
        const idx = row * cols + col;
        // Only override cells within radius (checked at draw time)
        ledgerOverlay.set(idx, ledgerStr[ci]);
      }
    }
  }

  // ── Draw loop ────────────────────────────────────────────────────────────────

  function loop() {
    raf = requestAnimationFrame(loop);
    draw();
  }

  function draw() {
    const W = canvas.width / dpr;
    const H = canvas.height / dpr;
    const t = performance.now() * 0.001; // seconds

    // Ease pointer
    pointer.x += (target.x - pointer.x) * LERP_SPEED;
    pointer.y += (target.y - pointer.y) * LERP_SPEED;

    // Fade highlight in/out
    const targetFade = hasPointer ? 1 : 0;
    fadeAlpha += (targetFade - fadeAlpha) * 0.05;

    // Rebuild ledger overlay if cursor is visible
    if (fadeAlpha > 0.01) {
      buildLedgerOverlay(pointer.x, pointer.y);
    } else {
      ledgerOverlay.clear();
    }

    // Clear to transparent — CSS paper shows through
    ctx.clearRect(0, 0, W, H);

    ctx.font = FONT_SIZE + 'px "JetBrains Mono", monospace';
    ctx.textBaseline = 'top';

    const r2 = RADIUS * RADIUS;

    for (let row = 0; row < rows; row++) {
      const cy = row * CELL_H;
      for (let col = 0; col < cols; col++) {
        const cx = col * CELL_W;
        const idx = row * cols + col;
        const cell = cells[idx];
        if (!cell) continue;

        const dx = cx - pointer.x;
        const dy = cy - pointer.y;
        const dist2 = dx * dx + dy * dy;
        const dist  = Math.sqrt(dist2);

        // Normalized distance [0,1] within radius
        const normDist = Math.min(dist / RADIUS, 1);

        // Cursor influence [0,1], eased by fadeAlpha
        const influence = (1 - normDist * normDist) * fadeAlpha;

        // Ambient shimmer: low-freq sine per cell
        const shimmer = Math.sin(t * 0.6 + cell.seed * 6.28) * SHIMMER_AMP;

        // Ripple: sine based on distance and time, fades with normDist
        const ripple = dist2 < r2
          ? Math.sin(dist * 0.06 - t * 3.0) * RIPPLE_AMP * (1 - normDist) * fadeAlpha
          : 0;

        const alpha = ALPHA_REST + shimmer + ripple + influence * (ALPHA_MAX - ALPHA_REST);
        const clampedAlpha = Math.max(0, Math.min(1, alpha));

        // Glyph: use ledger overlay if available and within radius, else near/rest glyphs
        let glyph;
        let useOxblood = false;

        if (dist2 < r2 && fadeAlpha > 0.01) {
          if (ledgerOverlay.has(idx)) {
            glyph = ledgerOverlay.get(idx);
            // Oxblood for the '=' character and digits nearest the cursor center
            useOxblood = (glyph === '=') || (dist < RADIUS * 0.35 && /[0-9]/.test(glyph));
          } else {
            // Scattered near-cursor glyphs
            const nearIdx = Math.floor(cell.seed * NEAR_GLYPHS.length);
            glyph = NEAR_GLYPHS[nearIdx];
          }
        } else {
          glyph = cell.glyph;
        }

        if (!glyph || glyph === ' ') continue;

        // Oxblood uses lower alpha to remain accent-level
        const color = useOxblood
          ? 'rgba(' + OXBLOOD + ',' + (clampedAlpha * 0.75).toFixed(3) + ')'
          : 'rgba(' + INK    + ',' + clampedAlpha.toFixed(3) + ')';

        ctx.fillStyle = color;
        ctx.fillText(glyph, cx, cy + 1);
      }
    }
  }

  // ── Static frame (prefers-reduced-motion) ────────────────────────────────────

  function drawStatic() {
    const W = canvas.width / dpr;
    const H = canvas.height / dpr;

    ctx.clearRect(0, 0, W, H);
    ctx.font = FONT_SIZE + 'px "JetBrains Mono", monospace';
    ctx.textBaseline = 'top';

    // Draw only the resting texture — no cursor influence, no animation
    for (let row = 0; row < rows; row++) {
      for (let col = 0; col < cols; col++) {
        const idx = row * cols + col;
        const cell = cells[idx];
        if (!cell || cell.glyph === ' ') continue;

        ctx.fillStyle = 'rgba(' + INK + ',' + ALPHA_REST.toFixed(3) + ')';
        ctx.fillText(cell.glyph, col * CELL_W, row * CELL_H + 1);
      }
    }

    // One soft, centered highlight — static, no cursor
    const cx = W / 2;
    const cy = H / 2;
    const seq = LEDGER_SEQS[0];
    const ledgerStr = seq[0] + '.00 = ' + seq[1] + '.00';
    const startX = cx - (ledgerStr.length * CELL_W) / 2;

    ctx.fillStyle = 'rgba(' + INK + ',0.18)';
    ctx.fillText(ledgerStr, startX, cy);
  }

  // ── Bootstrap ────────────────────────────────────────────────────────────────

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
