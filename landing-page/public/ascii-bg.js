/**
 * ascii-bg.js — "ledger rain"
 *
 * Falling columns of accounting glyphs (matrix-rain style), pine-green tinted.
 * The cursor "reconciles" the rain: nearby streams brighten and balanced ledger
 * rows appear centred on the pointer.
 *
 * Canvas:   #ascii-bg — fixed, full viewport, pointer-events:none.
 * Colors:   pine      rgb(30,111,68)   ambient trail
 *           pine-bright rgb(46,158,107) head glyph
 *           oxblood   rgb(140,47,42)   rare "credit" accent on heads
 * Paper + CSS vignette show through; we only clearRect, never fill the bg.
 */

(function () {
  'use strict';

  // ── Constants ────────────────────────────────────────────────────────────────

  const CELL_W        = 11;     // column pitch (px, logical)
  const CELL_H        = 18;     // row pitch (px, logical)
  const FONT_SIZE     = 13;     // px
  const MAX_DPR       = 2;

  // Pine green palette (RGB component strings)
  const PINE           = '30,111,68';
  const PINE_BRIGHT    = '46,158,107';
  const OXBLOOD        = '140,47,42';

  // Trail alpha: head alpha, fade to tail alpha
  const HEAD_ALPHA     = 0.30;
  const TAIL_ALPHA     = 0.03;
  const HEAD_BRIGHT    = 0.55;  // boosted head alpha for the leading glyph
  const CURSOR_BOOST   = 0.25;  // extra alpha added to streams inside cursor radius

  // Streams
  const SPEED_MIN      = 30;    // rows per second (logical)
  const SPEED_MAX      = 80;
  const TRAIL_MIN      = 6;     // min trail length in rows
  const TRAIL_MAX      = 22;
  const OXBLOOD_CHANCE = 0.04;  // probability a stream head uses oxblood

  // Cursor
  const RADIUS         = 190;   // px — influence radius
  const LERP_SPEED     = 0.09;  // easing factor per frame
  const FADE_SPEED     = 0.05;  // in/out fade speed per frame

  // Glyph alphabet: digits, accounting symbols
  const GLYPHS = [
    '0','1','2','3','4','5','6','7','8','9',
    '$','=','+','-','.',',','Σ',
  ];

  // Account code "spellings" — occasionally a stream's glyph at the head cycles
  // through these digit sequences to hint at real CoA codes.
  const ACCT_CODES = ['1000','4000','5000','1300','2100','6000','3000','1500'];

  // Ledger rows shown near cursor — balanced pairs
  const LEDGER_PAIRS = [
    ['1,240.00', '1,240.00'],
    ['5,910.00', '5,910.00'],
    ['  800.00', '  800.00'],
    ['3,072.00', '3,072.00'],
    ['  415.00', '  415.00'],
    ['12,500.00','12,500.00'],
    ['  990.00', '  990.00'],
    [' 6,000.00',' 6,000.00'],
  ];

  // ── State ────────────────────────────────────────────────────────────────────

  let canvas, ctx;
  let dpr     = 1;
  let W       = 0, H = 0;      // logical dimensions
  let numCols = 0;              // number of stream columns

  // Per-column stream state
  let streams = [];
  /*  stream = {
   *    headRow:   float  — current head position in rows (can be negative = offscreen)
   *    speed:     float  — rows/sec
   *    length:    int    — trail length in rows
   *    glyphs:    string[] — per-row glyph, indexed from head downward (index 0 = head)
   *    mutTimer:  float  — seconds until next mutation
   *    useOxblood: bool  — head rendered in oxblood this run
   *    acctIdx:   int or null — if set, index into ACCT_CODES for spelling mode
   *  }
   */

  let pointer    = { x: -9999, y: -9999 };  // eased cursor (logical px)
  let target     = { x: -9999, y: -9999 };  // raw cursor
  let hasPointer = false;
  let fadeAlpha  = 0;       // 0 = no cursor highlight, 1 = full

  let lastTime   = null;
  let raf        = null;
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
    bindEvents();

    if (reducedMotion) {
      drawStatic();
      return;
    }

    raf = requestAnimationFrame(tick);
  }

  // ── Grid / streams ───────────────────────────────────────────────────────────

  function resize() {
    W = window.innerWidth;
    H = window.innerHeight;

    canvas.width  = Math.round(W * dpr);
    canvas.height = Math.round(H * dpr);
    canvas.style.width  = W + 'px';
    canvas.style.height = H + 'px';

    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.scale(dpr, dpr);

    numCols = Math.ceil(W / CELL_W) + 1;
    buildStreams();
  }

  /** Initialise or rebuild all column streams, staggered so they don't all start together. */
  function buildStreams() {
    const numRows = Math.ceil(H / CELL_H);
    streams = [];

    for (let col = 0; col < numCols; col++) {
      streams.push(makeStream(col, numRows, /* stagger */ true));
    }
  }

  function makeStream(col, numRows, stagger) {
    const speed  = rand(SPEED_MIN, SPEED_MAX);
    const length = Math.floor(rand(TRAIL_MIN, TRAIL_MAX));
    // Stagger initial head anywhere between -(full trail) and bottom, so rain
    // starts scattered rather than all heads at row 0 simultaneously.
    const headRow = stagger
      ? rand(-(length + numRows * 0.6), numRows)
      : rand(-length - 5, -1);

    const glyphs = [];
    for (let i = 0; i < length; i++) {
      glyphs.push(randomGlyph());
    }

    // Occasionally assign a CoA code "spelling" mode to this stream
    const acctIdx = Math.random() < 0.12
      ? Math.floor(Math.random() * ACCT_CODES.length)
      : null;

    return {
      headRow,
      speed,
      length,
      glyphs,
      mutTimer: rand(0.4, 1.8),
      useOxblood: Math.random() < OXBLOOD_CHANCE,
      acctIdx,
    };
  }

  // ── Glyphs ───────────────────────────────────────────────────────────────────

  function randomGlyph() {
    return GLYPHS[Math.floor(Math.random() * GLYPHS.length)];
  }

  /** Mutate one random glyph in the stream's trail (classic matrix flicker). */
  function mutateStream(s) {
    const idx = Math.floor(Math.random() * s.glyphs.length);
    s.glyphs[idx] = randomGlyph();
  }

  /**
   * Return the glyph for trail position `trailIdx` (0 = head) in stream `s`.
   * Streams in spelling mode cycle through their account-code digits near the head.
   */
  function getGlyph(s, trailIdx) {
    if (s.acctIdx !== null && trailIdx < 4) {
      return ACCT_CODES[s.acctIdx][trailIdx] || s.glyphs[trailIdx];
    }
    return s.glyphs[trailIdx] || randomGlyph();
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
      resize();
    }, 120);
  }

  // ── Draw loop ────────────────────────────────────────────────────────────────

  function tick(now) {
    raf = requestAnimationFrame(tick);

    if (lastTime === null) { lastTime = now; }
    const dt = Math.min((now - lastTime) * 0.001, 0.05); // seconds, capped at 50ms
    lastTime = now;

    update(dt);
    draw();
  }

  /** Advance stream positions and handle mutations. */
  function update(dt) {
    const numRows = Math.ceil(H / CELL_H);

    // Ease pointer toward target
    pointer.x += (target.x - pointer.x) * LERP_SPEED;
    pointer.y += (target.y - pointer.y) * LERP_SPEED;

    // Fade cursor highlight in/out
    const targetFade = hasPointer ? 1 : 0;
    fadeAlpha += (targetFade - fadeAlpha) * FADE_SPEED;

    for (let col = 0; col < streams.length; col++) {
      const s = streams[col];

      // Advance head
      s.headRow += s.speed * dt;

      // Respawn when the tail has fully passed the bottom
      if (s.headRow - s.length > numRows) {
        const ns = makeStream(col, numRows, false);
        streams[col] = ns;
        continue;
      }

      // Glyph mutation
      s.mutTimer -= dt;
      if (s.mutTimer <= 0) {
        mutateStream(s);
        s.mutTimer = rand(0.4, 2.2);
      }
    }
  }

  /** Render all streams and the cursor ledger overlay. */
  function draw() {
    ctx.clearRect(0, 0, W, H);

    ctx.font = FONT_SIZE + 'px "JetBrains Mono", "Courier New", monospace';
    ctx.textBaseline = 'top';

    const numRows = Math.ceil(H / CELL_H);
    const px = pointer.x;
    const py = pointer.y;
    const r2 = RADIUS * RADIUS;

    // ── Rain streams ──────────────────────────────────────────────────────────

    for (let col = 0; col < streams.length; col++) {
      const s      = streams[col];
      const xPx    = col * CELL_W;
      const headInt = Math.floor(s.headRow);

      // Cursor distance to this column's head position
      const headYPx = headInt * CELL_H;
      const dx0     = xPx - px;
      const dy0     = headYPx - py;
      const colDist2 = dx0 * dx0 + dy0 * dy0;
      const nearCursor = fadeAlpha > 0.01 && colDist2 < r2;
      const cursorBoost = nearCursor
        ? CURSOR_BOOST * (1 - Math.sqrt(colDist2) / RADIUS) * fadeAlpha
        : 0;

      for (let ti = 0; ti < s.length; ti++) {
        const row = headInt - ti;
        if (row < 0 || row >= numRows) continue;

        const yPx   = row * CELL_H;
        const glyph = getGlyph(s, ti);

        if (ti === 0) {
          // Head glyph — brightest, optionally oxblood
          const a = Math.min(1, HEAD_BRIGHT + cursorBoost);
          if (s.useOxblood) {
            ctx.fillStyle = 'rgba(' + OXBLOOD + ',' + a.toFixed(3) + ')';
          } else {
            ctx.fillStyle = 'rgba(' + PINE_BRIGHT + ',' + a.toFixed(3) + ')';
          }
        } else {
          // Trail glyph — fade from head toward tail
          const t      = ti / s.length;             // 0 at head, 1 at tail
          const base   = HEAD_ALPHA * (1 - t) + TAIL_ALPHA * t;
          const a      = Math.min(1, base + cursorBoost * (1 - t * 0.5));
          ctx.fillStyle = 'rgba(' + PINE + ',' + a.toFixed(3) + ')';
        }

        ctx.fillText(glyph, xPx, yPx + 1);
      }
    }

    // ── Cursor ledger overlay ─────────────────────────────────────────────────

    if (fadeAlpha > 0.01) {
      drawLedgerOverlay(px, py);
    }
  }

  /**
   * Render aligned balanced ledger rows centred on the cursor.
   * '=' and central digits render in oxblood; figures in pine-bright.
   * Only drawn when the cursor is present (fadeAlpha > 0).
   */
  function drawLedgerOverlay(cx, cy) {
    const centerRow = Math.floor(cy / CELL_H);
    const numRows   = Math.ceil(H / CELL_H);
    const pairCount = LEDGER_PAIRS.length;
    const fa        = fadeAlpha;

    // Show 5 rows of balanced ledger centred on cursor row
    for (let dr = -2; dr <= 2; dr++) {
      const row = centerRow + dr;
      if (row < 0 || row >= numRows) continue;

      const pairIdx = ((row % pairCount) + pairCount) % pairCount;
      const [lv, rv] = LEDGER_PAIRS[pairIdx];
      const str  = lv + ' = ' + rv;
      const startX = cx - (str.length * CELL_W) / 2;
      const yPx    = row * CELL_H;

      // Row alpha: brighter at cursor row, dimmer at ±2
      const rowAlpha = (0.70 - Math.abs(dr) * 0.12) * fa;

      for (let ci = 0; ci < str.length; ci++) {
        const ch   = str[ci];
        const xPx  = startX + ci * CELL_W;
        if (xPx < 0 || xPx > W) continue;

        // '=' and digits near centre use oxblood accent
        const isEq     = ch === '=';
        const isCentreDigit = /[0-9]/.test(ch) && ci > lv.length - 3 && ci < lv.length + 3;
        const useOx    = isEq || isCentreDigit;

        if (ch === ' ') continue;

        if (useOx) {
          ctx.fillStyle = 'rgba(' + OXBLOOD + ',' + (rowAlpha * 0.85).toFixed(3) + ')';
        } else {
          ctx.fillStyle = 'rgba(' + PINE_BRIGHT + ',' + rowAlpha.toFixed(3) + ')';
        }

        ctx.fillText(ch, xPx, yPx + 1);
      }
    }
  }

  // ── Static frame (prefers-reduced-motion) ────────────────────────────────────

  /**
   * Single calm frame: scattered faint pine glyphs and one centred ledger row.
   * No animation, no cursor listeners.
   */
  function drawStatic() {
    const numRows = Math.ceil(H / CELL_H);

    ctx.clearRect(0, 0, W, H);
    ctx.font = FONT_SIZE + 'px "JetBrains Mono", "Courier New", monospace';
    ctx.textBaseline = 'top';

    // Scatter faint pine glyphs — roughly 1 in 8 cells
    for (let row = 0; row < numRows; row++) {
      for (let col = 0; col < numCols; col++) {
        const roll = pseudoRand(col * 997 + row * 31);
        if (roll > 0.125) continue;
        const gi    = Math.floor(pseudoRand(col * 1301 + row * 53) * GLYPHS.length);
        const alpha = 0.04 + pseudoRand(col * 71 + row * 113) * 0.06;
        ctx.fillStyle = 'rgba(' + PINE + ',' + alpha.toFixed(3) + ')';
        ctx.fillText(GLYPHS[gi], col * CELL_W, row * CELL_H + 1);
      }
    }

    // One centred balanced ledger row at modest opacity
    const cx  = W / 2;
    const cy  = H / 2;
    const row = Math.floor(cy / CELL_H);
    const [lv, rv] = LEDGER_PAIRS[0];
    const str  = lv + ' = ' + rv;
    const startX = cx - (str.length * CELL_W) / 2;

    for (let ci = 0; ci < str.length; ci++) {
      const ch   = str[ci];
      if (ch === ' ') continue;
      const xPx  = startX + ci * CELL_W;
      const isEq = ch === '=';
      ctx.fillStyle = isEq
        ? 'rgba(' + OXBLOOD + ',0.22)'
        : 'rgba(' + PINE_BRIGHT + ',0.22)';
      ctx.fillText(ch, xPx, row * CELL_H + 1);
    }
  }

  // ── Utilities ────────────────────────────────────────────────────────────────

  /** Uniform random float in [lo, hi). */
  function rand(lo, hi) {
    return lo + Math.random() * (hi - lo);
  }

  /**
   * Deterministic float [0,1) from integer seed.
   * Used only in drawStatic (not in hot animation path).
   */
  function pseudoRand(n) {
    n = ((n >> 16) ^ n) * 0x45d9f3b | 0;
    n = ((n >> 16) ^ n) * 0x45d9f3b | 0;
    n = (n >> 16) ^ n;
    return (n >>> 0) / 0x100000000;
  }
})();
