/**
 * ascii-bg.js — "Double-Entry Balance Scale"
 *
 * Draws a large, high-contrast balance scale (a double-entry accounting metaphor)
 * centered in the viewport on #ascii-bg. The cursor disturbs the beam; a spring
 * restores it to level. Pine green (= balanced) / oxblood (≠ disturbed).
 *
 * Canvas:  #ascii-bg — fixed, full viewport, pointer-events:none.
 * Colors:  ink  rgba(26,26,24, …)   structural strokes and labels
 *          pine #1E6F44             balanced indicator "="
 *          oxblood #8C2F2A          disturbed indicator "≠"
 * Background fill: NEVER — clearRect only; CSS paper-glow vignette handles bg.
 */

(function () {
  'use strict';

  // ── Constants ─────────────────────────────────────────────────────────────────

  const MAX_DPR = 2;

  // Color palette
  const INK_RGB     = '26,26,24';
  const PINE_RGB    = '30,111,68';
  const OXBLOOD_RGB = '140,47,42';

  // Stroke alphas (high-contrast — bold but tasteful)
  const ALPHA_STRUCT   = 0.52;   // post, base, beam, chains, pans
  const ALPHA_LABEL    = 0.50;   // DEBIT / CREDIT labels
  const ALPHA_AMOUNT   = 0.48;   // amount on each pan
  const ALPHA_INDICATOR = 0.82;  // = / ≠ indicator near pivot

  // Spring physics
  const SPRING_STIFFNESS = 14;   // higher = faster return
  const SPRING_DAMPING   = 5.5;  // higher = less wobble
  const MAX_TILT_DEG     = 11;   // max beam tilt in degrees (cursor at edge)
  const AMBIENT_AMP_DEG  = 0.5;  // subtle ambient sway amplitude (degrees)
  const AMBIENT_PERIOD   = 6.2;  // seconds for one ambient sway cycle
  const LEVEL_THRESHOLD  = 1.2;  // degrees — within this → "balanced" (pine)

  // Scale sizing (logical px, before DPR)
  const BASE_SCALE = 0.38;       // rig height as fraction of viewport height
  const MIN_HEIGHT = 260;        // px minimum rig height
  const MAX_HEIGHT = 520;        // px maximum rig height

  const AMOUNT_TEXT = '5,910.00';
  const FONT_MONO   = '"JetBrains Mono", "Courier New", monospace';

  // ── State ─────────────────────────────────────────────────────────────────────

  let canvas, ctx;
  let dpr = 1;
  let W = 0, H = 0;

  // Rig geometry (recomputed on resize)
  let rig = {};

  // Spring state
  let angle     = 0;   // current beam angle, radians (+ = right side down)
  let velocity  = 0;   // angular velocity, rad/s
  let targetAngle = 0; // driven by cursor

  // Cursor state
  let cursorX = null;
  let cursorY = null;

  let lastTime    = null;
  let raf         = null;
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
      // One static frame: scale perfectly level, pine "=", no listeners.
      drawScale(0, 0);
      return;
    }

    // Pointer listeners — cursor disturbs the beam.
    window.addEventListener('mousemove', onMouseMove, { passive: true });
    window.addEventListener('touchmove', onTouchMove, { passive: true });
    window.addEventListener('mouseleave', onPointerLeave, { passive: true });
    window.addEventListener('touchend',  onPointerLeave, { passive: true });

    raf = requestAnimationFrame(tick);
  }

  // ── Layout ────────────────────────────────────────────────────────────────────

  /**
   * Resize canvas and recompute rig geometry.
   * Resets transform before re-scaling (required after canvas resize).
   */
  function resize() {
    W = window.innerWidth;
    H = window.innerHeight;

    canvas.width  = Math.round(W * dpr);
    canvas.height = Math.round(H * dpr);
    canvas.style.width  = W + 'px';
    canvas.style.height = H + 'px';

    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.scale(dpr, dpr);

    buildRig();
  }

  /**
   * Compute all static rig measurements from current W/H.
   * rigH = total visual height of the rig (base to pivot), clamped.
   */
  function buildRig() {
    const rigH  = Math.min(MAX_HEIGHT, Math.max(MIN_HEIGHT, H * BASE_SCALE));

    const cx     = W / 2;
    const pivotY = H * 0.5 - rigH * 0.18; // pivot slightly above midpoint
    const baseY  = pivotY + rigH;
    const baseHalfW = rigH * 0.30;         // half-width of the base bar
    const postW     = rigH * 0.015;        // visual half-width of post (for drawing)

    // Beam: half-span from pivot
    const beamHalf  = rigH * 0.60;

    // Chain length from beam end to pan top
    const chainLen  = rigH * 0.28;

    // Pan: shallow bowl dimensions
    const panHalfW  = rigH * 0.18;
    const panDepth  = rigH * 0.05;

    // Font sizes scaled with rig
    const labelSize = Math.max(10, Math.min(15, rigH * 0.042));
    const amtSize   = Math.max(9,  Math.min(13, rigH * 0.036));
    const indSize   = Math.max(14, Math.min(24, rigH * 0.058));

    rig = {
      cx, pivotY, baseY, baseHalfW, postW,
      beamHalf, chainLen,
      panHalfW, panDepth,
      labelSize, amtSize, indSize,
      rigH,
    };
  }

  function onResize() {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(function () {
      resize();
    }, 120);
  }

  // ── Spring + Pointer ──────────────────────────────────────────────────────────

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
  }

  /**
   * Compute targetAngle from cursor position.
   * Horizontal offset from center → tilt direction.
   * Optional vertical proximity weighting: cursor near beam level → stronger effect.
   */
  function computeTargetAngle(t) {
    const ambient = (AMBIENT_AMP_DEG * Math.PI / 180) * Math.sin((2 * Math.PI * t) / AMBIENT_PERIOD);

    if (cursorX === null || cursorY === null) {
      return ambient;
    }

    // Normalized horizontal offset [-1, +1]
    const normX = (cursorX - W / 2) / (W / 2);

    // Vertical proximity: 1.0 when cursor at beam level, fades over ~40% of H
    const beamY = rig.pivotY;
    const vDist = Math.abs(cursorY - beamY) / (H * 0.4);
    const vFactor = Math.max(0, 1 - vDist);

    // Base tilt scaled by cursor offset, boosted by vertical proximity
    const tiltMag = normX * (0.5 + 0.5 * vFactor);
    const maxRad  = MAX_TILT_DEG * Math.PI / 180;
    return tiltMag * maxRad + ambient;
  }

  /**
   * Step the spring by dt seconds toward targetAngle.
   * Frame-rate independent semi-implicit Euler integration.
   */
  function stepSpring(dt) {
    const err = angle - targetAngle;
    const acc = -SPRING_STIFFNESS * err - SPRING_DAMPING * velocity;
    velocity += acc * dt;
    angle    += velocity * dt;
  }

  // ── Draw Scale ────────────────────────────────────────────────────────────────

  /**
   * Draw the complete balance scale for the given beam angle (radians) and
   * elapsed time t (seconds, used only for ambient sway phase label).
   */
  function drawScale(beamAngle, _t) {
    ctx.clearRect(0, 0, W, H);

    const {
      cx, pivotY, baseY, baseHalfW, postW,
      beamHalf, chainLen,
      panHalfW, panDepth,
      labelSize, amtSize, indSize,
    } = rig;

    // Determine balanced vs. disturbed
    const tiltDeg = Math.abs(beamAngle * 180 / Math.PI);
    const isLevel = tiltDeg < LEVEL_THRESHOLD;

    // Color for structural elements: slight tint toward oxblood when disturbed,
    // toward pine when settled — but always mostly ink.
    const structColor = 'rgba(' + INK_RGB + ',' + ALPHA_STRUCT + ')';

    ctx.lineCap  = 'round';
    ctx.lineJoin = 'round';

    // ── Base and post ──

    ctx.strokeStyle = structColor;

    // Base bar (horizontal footing)
    const baseThick = Math.max(3, rig.rigH * 0.018);
    ctx.lineWidth = baseThick;
    ctx.beginPath();
    ctx.moveTo(cx - baseHalfW, baseY);
    ctx.lineTo(cx + baseHalfW, baseY);
    ctx.stroke();

    // Center foot (small vertical stub below base for stability look)
    ctx.lineWidth = baseThick * 0.7;
    ctx.beginPath();
    ctx.moveTo(cx, baseY);
    ctx.lineTo(cx, baseY + baseThick * 1.2);
    ctx.stroke();

    // Vertical post
    const postLineW = Math.max(2, rig.rigH * 0.012);
    ctx.lineWidth = postLineW;
    ctx.beginPath();
    ctx.moveTo(cx, baseY);
    ctx.lineTo(cx, pivotY);
    ctx.stroke();

    // ── Pivot marker: bold dot ──

    ctx.fillStyle = structColor;
    const pivotR = Math.max(4, rig.rigH * 0.025);
    ctx.beginPath();
    ctx.arc(cx, pivotY, pivotR, 0, Math.PI * 2);
    ctx.fill();

    // ── Beam ──

    // Beam endpoints (rotated about pivot)
    const cosA = Math.cos(beamAngle);
    const sinA = Math.sin(beamAngle);

    const lx = cx - beamHalf * cosA;
    const ly = pivotY - beamHalf * sinA;  // left end (negative x offset)
    const rx = cx + beamHalf * cosA;
    const ry = pivotY + beamHalf * sinA;  // right end (positive x offset)
    // Note: +angle means right side goes DOWN (sinA positive → ry > pivotY)

    const beamThick = Math.max(3, rig.rigH * 0.016);
    ctx.strokeStyle = structColor;
    ctx.lineWidth   = beamThick;
    ctx.beginPath();
    ctx.moveTo(lx, ly);
    ctx.lineTo(rx, ry);
    ctx.stroke();

    // ── Chains and pans ──

    // Chains hang straight down from each beam end (gravity, not rotated with beam).
    // Left pan
    drawChainAndPan(lx, ly, chainLen, panHalfW, panDepth, structColor,
                    'DEBIT', labelSize, amtSize, 'left');
    // Right pan
    drawChainAndPan(rx, ry, chainLen, panHalfW, panDepth, structColor,
                    'CREDIT', labelSize, amtSize, 'right');

    // ── Balance indicator near pivot ──

    const indText  = isLevel ? '=' : '≠';
    const indRGB   = isLevel ? PINE_RGB : OXBLOOD_RGB;
    const indColor = 'rgba(' + indRGB + ',' + ALPHA_INDICATOR + ')';

    ctx.fillStyle    = indColor;
    ctx.font         = 'bold ' + indSize + 'px ' + FONT_MONO;
    ctx.textAlign    = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(indText, cx, pivotY - pivotR - indSize * 0.9);
  }

  /**
   * Draw chain (two converging lines from beam endpoint down to pan top)
   * then a shallow pan bowl, label, and amount.
   *
   * chainTopX/Y: where the chain meets the beam end.
   * Pans hang vertically (straight down) regardless of beam angle.
   */
  function drawChainAndPan(chainTopX, chainTopY, chainLen, panHalfW, panDepth,
                            strokeColor, label, labelSize, amtSize, side) {
    const panTopY = chainTopY + chainLen;
    const panBotY = panTopY + panDepth;

    ctx.strokeStyle = strokeColor;

    // Chain: two lines converging from a wide spread at the beam end
    // to the pan corners — classic "V" bale shape.
    const spreadX = panHalfW * 0.45; // spread at beam end attachment
    const chainW  = Math.max(1.5, rig.rigH * 0.009);

    ctx.lineWidth = chainW;
    ctx.beginPath();
    // Left chain strand
    ctx.moveTo(chainTopX - spreadX, chainTopY);
    ctx.lineTo(chainTopX - panHalfW, panTopY);
    ctx.stroke();

    ctx.beginPath();
    // Right chain strand
    ctx.moveTo(chainTopX + spreadX, chainTopY);
    ctx.lineTo(chainTopX + panHalfW, panTopY);
    ctx.stroke();

    // Pan bowl: arc from left corner through bottom center to right corner
    const panThick = Math.max(2.5, rig.rigH * 0.014);
    ctx.lineWidth  = panThick;
    ctx.beginPath();
    ctx.moveTo(chainTopX - panHalfW, panTopY);
    ctx.quadraticCurveTo(chainTopX, panBotY + panDepth * 0.6, chainTopX + panHalfW, panTopY);
    ctx.stroke();

    // Horizontal top rail of pan (the rim)
    ctx.lineWidth = panThick * 0.7;
    ctx.beginPath();
    ctx.moveTo(chainTopX - panHalfW, panTopY);
    ctx.lineTo(chainTopX + panHalfW, panTopY);
    ctx.stroke();

    // Label (DEBIT / CREDIT) — above the pan rim
    const labelY    = panTopY - labelSize * 1.0;
    const labelColor = 'rgba(' + INK_RGB + ',' + ALPHA_LABEL + ')';
    ctx.fillStyle    = labelColor;
    ctx.font         = 'bold ' + labelSize + 'px ' + FONT_MONO;
    ctx.textAlign    = 'center';
    ctx.textBaseline = 'alphabetic';
    ctx.fillText(label, chainTopX, labelY);

    // Amount — inside/below pan (sits in the bowl visually)
    const amtY    = panBotY + panDepth * 0.9 + amtSize * 1.1;
    const amtColor = 'rgba(' + INK_RGB + ',' + ALPHA_AMOUNT + ')';
    ctx.fillStyle  = amtColor;
    ctx.font       = amtSize + 'px ' + FONT_MONO;
    ctx.textAlign  = 'center';
    ctx.textBaseline = 'alphabetic';
    ctx.fillText(AMOUNT_TEXT, chainTopX, amtY);
  }

  // ── Animation loop ────────────────────────────────────────────────────────────

  function tick(now) {
    raf = requestAnimationFrame(tick);

    if (lastTime === null) { lastTime = now; }
    const dt = Math.min((now - lastTime) * 0.001, 0.05); // cap at 50 ms
    lastTime = now;

    const t = now * 0.001; // seconds since page load

    targetAngle = computeTargetAngle(t);
    stepSpring(dt);

    drawScale(angle, t);
  }
})();
