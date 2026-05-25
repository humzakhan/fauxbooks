/**
 * ascii-bg.js — "ruled ledger paper, ultra-slow drift"
 *
 * Draws faint horizontal rule lines and sparse balanced-ledger figures onto
 * #ascii-bg. The entire sheet drifts downward at ~6 px/sec and loops
 * seamlessly. No cursor interaction of any kind.
 *
 * Canvas:  #ascii-bg — fixed, full viewport, pointer-events:none.
 * Colors:  ink  rgba(26,26,24, …)   rule lines and figures
 *          pine rgb(30,111,68)       '=' accent in figures
 * Background fill: NEVER — clearRect only; CSS paper-glow vignette handles bg.
 */

(function () {
  'use strict';

  // ── Constants ────────────────────────────────────────────────────────────────

  const MAX_DPR      = 2;
  const FONT_SIZE    = 12;        // px, logical
  const FONT         = FONT_SIZE + 'px "JetBrains Mono", "Courier New", monospace';

  const RULE_SPACING = 40;        // px between horizontal rule lines
  const DRIFT_SPEED  = 6;         // px/sec downward — barely perceptible

  // Rule line style
  const RULE_COLOR   = 'rgba(26,26,24,0.05)';

  // Figure text: ink at low alpha
  const FIGURE_ALPHA = 0.07;
  const INK          = '26,26,24';

  // '=' sign: pine green, slightly more present
  const PINE_EQ      = 'rgba(30,111,68,0.13)';

  // Centre-band horizontal range (fraction of W) — figures slightly dimmer here
  const BAND_LEFT    = 0.25;
  const BAND_RIGHT   = 0.75;
  const BAND_DIM     = 0.55;      // multiply alpha by this when inside band

  // Balanced ledger figure strings (left side = right side)
  const FIGURES = [
    '1,240.00 = 1,240.00',
    '5,910.00 = 5,910.00',
    '  800.00 =   800.00',
    '3,072.50 = 3,072.50',
    '12,500.00 = 12,500.00',
    '  415.00 =   415.00',
    ' 6,000.00 =  6,000.00',
    '9,875.25 = 9,875.25',
  ];

  // ── State ────────────────────────────────────────────────────────────────────

  let canvas, ctx;
  let dpr = 1;
  let W = 0, H = 0;

  // Each "row" is one rule line that travels downward.
  // rows[i] = { baseY: float, figure: string|null, figureX: float }
  // baseY is the Y offset of this row within one "sheet" height.
  // The visible Y is (baseY + offset) mod sheetH.
  let rows = [];
  let sheetH = 0;    // total height of one seamless-loop unit
  let offset = 0;    // current downward drift in px (always grows)

  let lastTime    = null;
  let raf         = null;
  let resizeTimer = null;

  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  // ── Bootstrap ────────────────────────────────────────────────────────────────

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
      drawFrame(0);   // one static frame, no animation
      return;
    }

    raf = requestAnimationFrame(tick);
  }

  // ── Layout ───────────────────────────────────────────────────────────────────

  /** Resize canvas and rebuild row layout. Resets transform before re-scaling. */
  function resize() {
    W = window.innerWidth;
    H = window.innerHeight;

    canvas.width  = Math.round(W * dpr);
    canvas.height = Math.round(H * dpr);
    canvas.style.width  = W + 'px';
    canvas.style.height = H + 'px';

    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.scale(dpr, dpr);

    buildRows();
  }

  /**
   * Build the list of rule rows that make up one seamless loop.
   * We need enough rows to cover the viewport plus one extra RULE_SPACING
   * above and below so the loop is invisible.
   *
   * sheetH = total vertical span of one loop = rows * RULE_SPACING.
   * We keep sheetH just large enough to cover H + RULE_SPACING,
   * then drift offset modulo sheetH.
   *
   * For each row, decide (sparsely) whether to place a figure and at what X.
   * Figure placement: every 3rd or 4th row, not every column.
   */
  function buildRows() {
    const numRows = Math.ceil((H + RULE_SPACING * 2) / RULE_SPACING) + 1;
    sheetH = numRows * RULE_SPACING;

    rows = [];
    for (let i = 0; i < numRows; i++) {
      const baseY = i * RULE_SPACING;

      // Sparse figure: roughly every 3rd–4th row gets one
      const placeFigure = pseudoRand(i * 137 + 7) < 0.30;
      let figure  = null;
      let figureX = 0;

      if (placeFigure) {
        const fi = Math.floor(pseudoRand(i * 293 + 41) * FIGURES.length);
        figure = FIGURES[fi];

        // Distribute figures toward margins — avoid dead centre
        // Pick from left quarter or right quarter of viewport
        const side = pseudoRand(i * 503 + 19) < 0.5 ? 'left' : 'right';
        if (side === 'left') {
          figureX = pseudoRand(i * 701 + 3) * W * 0.28 + W * 0.02;
        } else {
          figureX = W * 0.68 + pseudoRand(i * 701 + 3) * W * 0.28;
        }
      }

      rows.push({ baseY, figure, figureX });
    }
  }

  function onResize() {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(function () {
      resize();
    }, 120);
  }

  // ── Figures ───────────────────────────────────────────────────────────────────

  /**
   * Draw a balanced-figure string at (x, y).
   * Figures are rendered character-by-character so the '=' can be pine green.
   * Spaces are skipped. Alpha is dimmed when x falls in the centre band.
   */
  function drawFigure(figure, x, y) {
    // Determine if this x position is in the centre band
    const xNorm   = x / W;
    const inBand  = xNorm > BAND_LEFT && xNorm < BAND_RIGHT;
    const alpha   = inBand ? FIGURE_ALPHA * BAND_DIM : FIGURE_ALPHA;

    ctx.textBaseline = 'alphabetic';
    ctx.font = FONT;

    let cx = x;
    for (let i = 0; i < figure.length; i++) {
      const ch = figure[i];
      if (ch === ' ') {
        cx += FONT_SIZE * 0.6;   // monospace space advance
        continue;
      }
      if (ch === '=') {
        ctx.fillStyle = PINE_EQ;
      } else {
        ctx.fillStyle = 'rgba(' + INK + ',' + alpha.toFixed(3) + ')';
      }
      ctx.fillText(ch, cx, y);
      cx += FONT_SIZE * 0.6;
    }
  }

  // ── Draw ──────────────────────────────────────────────────────────────────────

  /**
   * Render one frame. `currentOffset` is the current downward drift in px.
   * Rule lines and figures are drawn using modular arithmetic so the sheet
   * loops seamlessly.
   */
  function drawFrame(currentOffset) {
    ctx.clearRect(0, 0, W, H);

    ctx.lineWidth     = 0.5 / dpr;
    ctx.strokeStyle   = RULE_COLOR;

    // Compute the phase: how far into the current sheet we are
    const phase = currentOffset % sheetH;

    for (let i = 0; i < rows.length; i++) {
      const row = rows[i];
      // Visible Y: baseY shifted by drift phase, wrapped into view
      let y = row.baseY + phase;
      // Wrap so lines are always within [-RULE_SPACING, H + RULE_SPACING]
      if (y > H + RULE_SPACING) y -= sheetH;

      // Skip rows that are definitely off-screen
      if (y < -RULE_SPACING || y > H + RULE_SPACING) continue;

      // Draw rule line
      ctx.beginPath();
      ctx.moveTo(0, y);
      ctx.lineTo(W, y);
      ctx.stroke();

      // Draw figure sitting on this rule line (if assigned)
      if (row.figure !== null) {
        drawFigure(row.figure, row.figureX, y - 3);
      }
    }
  }

  // ── Animation loop ────────────────────────────────────────────────────────────

  function tick(now) {
    raf = requestAnimationFrame(tick);

    if (lastTime === null) { lastTime = now; }
    const dt = Math.min((now - lastTime) * 0.001, 0.05); // cap at 50 ms
    lastTime = now;

    offset += DRIFT_SPEED * dt;

    drawFrame(offset);
  }

  // ── Utilities ─────────────────────────────────────────────────────────────────

  /**
   * Deterministic float [0,1) from an integer seed.
   * Used only during layout (buildRows / drawFigure) — not in the hot draw path.
   */
  function pseudoRand(n) {
    n = n | 0;
    n = ((n >> 16) ^ n) * 0x45d9f3b | 0;
    n = ((n >> 16) ^ n) * 0x45d9f3b | 0;
    n = (n >> 16) ^ n;
    return (n >>> 0) / 0x100000000;
  }
})();
