/**
 * ascii-bg.js — "Flow Field Particle Stream"
 *
 * A sparse, living particle field drawn into #ascii-bg (fixed canvas,
 * full viewport, pointer-events:none, clearRect only — never fills bg).
 *
 * Sections:
 *   CONSTANTS          — tuning knobs
 *   COLOR / GLYPH      — palette and glyph pools
 *   FLOW FIELD         — smooth time-varying vector field (sum-of-sines)
 *   PARTICLES          — spawn, update, trail, lifespan
 *   CURSOR             — mousemove / touch → push + swirl + pine brighten
 *   DRAW               — trail polyline + head glyph/dot
 *   LOOP               — rAF, delta-time, orchestration
 *   STATIC             — prefers-reduced-motion: one calm scatter frame
 *   BOOTSTRAP          — DOM-ready guard
 */

(function () {
  'use strict';

  // ── CONSTANTS ─────────────────────────────────────────────────────────────────

  const MAX_DPR          = 2;
  const RESIZE_DEBOUNCE  = 120;   // ms

  // Density: ~1 particle per PX_PER_PARTICLE px², clamped.
  const PX_PER_PARTICLE  = 6500;
  const MIN_PARTICLES    = 60;
  const MAX_PARTICLES    = 400;

  const TRAIL_LEN        = 10;    // positions kept per particle (ring buffer)
  const TRAIL_SAMPLE_DIST = 6;   // px of travel required before a new trail point is recorded
  const SPEED_MIN        = 12;    // px/s
  const SPEED_MAX        = 28;    // px/s

  // Lifespan in seconds; each particle picks uniformly in [MIN, MAX].
  const LIFE_MIN         = 4.0;
  const LIFE_MAX         = 12.0;
  const FADE_IN_FRAC     = 0.12;  // fraction of lifespan spent fading in
  const FADE_OUT_FRAC    = 0.18;  // fraction of lifespan spent fading out

  // Head alpha range (ink on paper — quiet but visible).
  const ALPHA_HEAD_MIN   = 0.16;
  const ALPHA_HEAD_MAX   = 0.22;

  // Fraction of particles that show an accounting glyph at the head;
  // the rest render as a dot character.
  const GLYPH_FRAC       = 0.175; // ~17.5%

  // Flow field: spatial and temporal frequencies (low → broad, graceful streams).
  const FF_SCALE         = 0.0018; // spatial frequency (world units → radians)
  const FF_TIME_SCALE    = 0.09;   // how fast the field evolves (rad/s)
  const FF_OCTAVES       = 3;      // how many sine terms sum up the field angle

  // Cursor interaction.
  const CURSOR_R         = 140;    // px — radius of cursor influence
  const PUSH_STR         = 90;     // px/s — outward push at contact
  const SWIRL_STR        = 55;     // px/s — tangential swirl component
  const CURSOR_ALPHA_BOOST = 0.28; // extra alpha at full proximity
  const RETURN_RATE      = 2.0;    // how fast perturbed velocity returns to field (1/s)

  // ── COLOR / GLYPH ─────────────────────────────────────────────────────────────

  const INK_R  = 26,  INK_G  = 26,  INK_B  = 24;
  const PINE_R = 30,  PINE_G = 111, PINE_B = 68;

  const DOTS   = '·';                    // faint dot head for ~82.5% of particles
  const DIGITS = '0123456789';
  const ACCTING = '=$Σ+-.,'  ;

  const FONT_SIZE = 13;
  const FONT_MONO = '"JetBrains Mono","Courier New",monospace';

  // ── STATE ─────────────────────────────────────────────────────────────────────

  let canvas, ctx;
  let dpr = 1;
  let W = 0, H = 0;            // logical viewport px

  // Particle pool: preallocated fixed-size objects.
  let particles = [];
  let targetCount = 0;

  // Cursor state (logical px, null when off-screen).
  let cursorX = null, cursorY = null;

  let lastTime = null;
  let resizeTimer = null;

  const reducedMotion =
    window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  // ── FLOW FIELD ────────────────────────────────────────────────────────────────

  /**
   * Return a flow angle (radians) for world position (x, y) at time t.
   * Uses a sum of sines/cosines with irrational frequencies so the field
   * is smooth, non-repeating, and slowly morphing.
   *
   * Low FF_SCALE → broad, gentle streams (not turbulent).
   */
  function flowAngle(x, y, t) {
    const sx = x * FF_SCALE;
    const sy = y * FF_SCALE;
    const tt = t * FF_TIME_SCALE;

    // Three octaves; each shifts slightly in space + time to avoid symmetry.
    const a0 = Math.sin(sx * 1.00 + sy * 0.70 + tt * 1.00);
    const a1 = Math.sin(sx * 1.73 + sy * 1.31 + tt * 0.67 + 1.3);
    const a2 = Math.cos(sx * 0.89 + sy * 1.97 + tt * 0.43 + 2.7);

    // Weighted sum → [−1, 1], then scale to a full angle sweep.
    const v = (a0 * 0.5 + a1 * 0.3 + a2 * 0.2);
    return v * Math.PI * 2;
  }

  // ── PARTICLES ─────────────────────────────────────────────────────────────────

  /**
   * A single particle. Reused across its whole lifespan (reset in place).
   * trail: ring buffer of {x, y} positions, TRAIL_LEN long.
   */
  function makeParticle(t, staggerAge) {
    const p = {
      x: 0, y: 0,
      vx: 0, vy: 0,         // current velocity (px/s)
      speed: 0,              // magnitude chosen at spawn
      lifespan: 0,           // total lifespan (s)
      age: 0,                // current age (s)
      alpha: 0,              // base head alpha (paper colour through)
      glyph: DOTS,           // dot or an accounting char
      isGlyph: false,        // true → draw font char, false → filled circle
      // Trail ring buffer
      trail: new Array(TRAIL_LEN).fill(null).map(() => ({ x: 0, y: 0 })),
      trailHead: 0,          // index of most-recent trail position
      trailLen: 0,           // how many valid trail positions exist
      // Last sampled trail position (distance-based sampling)
      lastTrailX: 0,
      lastTrailY: 0,
      // Cursor perturbation
      perturbVx: 0,
      perturbVy: 0,
      pineFrac: 0,           // 0 = ink, 1 = pine (cursor proximity tint)
    };
    spawnParticle(p, t, staggerAge);
    return p;
  }

  /**
   * (Re)initialise a particle at a random viewport position.
   * staggerAge lets newly-created particles start at a random point in their
   * life so they don't all blink into existence simultaneously.
   */
  function spawnParticle(p, t, staggerAge) {
    p.x = Math.random() * W;
    p.y = Math.random() * H;

    p.lifespan = LIFE_MIN + Math.random() * (LIFE_MAX - LIFE_MIN);
    p.age = staggerAge ? Math.random() * p.lifespan : 0;

    p.speed = SPEED_MIN + Math.random() * (SPEED_MAX - SPEED_MIN);
    p.alpha = ALPHA_HEAD_MIN + Math.random() * (ALPHA_HEAD_MAX - ALPHA_HEAD_MIN);

    // Glyph vs. dot
    const r = Math.random();
    if (r < GLYPH_FRAC) {
      const pool = DIGITS + ACCTING;
      p.glyph  = pool[Math.floor(Math.random() * pool.length)];
      p.isGlyph = true;
    } else {
      p.glyph   = DOTS;
      p.isGlyph = false;
    }

    // Seed initial velocity from the flow field at birth position.
    const angle = flowAngle(p.x, p.y, t + p.age);
    p.vx = Math.cos(angle) * p.speed;
    p.vy = Math.sin(angle) * p.speed;

    p.perturbVx = 0;
    p.perturbVy = 0;
    p.pineFrac  = 0;

    // Seed the trail ring at the spawn position.
    for (let i = 0; i < TRAIL_LEN; i++) {
      p.trail[i].x = p.x;
      p.trail[i].y = p.y;
    }
    p.trailHead  = 0;
    p.trailLen   = 0;
    // Distance-based trail sampling: record the spawn position as the last sampled point.
    p.lastTrailX = p.x;
    p.lastTrailY = p.y;
  }

  /**
   * Age-based envelope: 0→1 fade-in, plateau, 1→0 fade-out.
   */
  function lifeFade(p) {
    const frac = p.age / p.lifespan;
    if (frac < FADE_IN_FRAC)  return frac / FADE_IN_FRAC;
    if (frac > 1 - FADE_OUT_FRAC) return (1 - frac) / FADE_OUT_FRAC;
    return 1;
  }

  /**
   * Update all particles for one frame.
   * t  — elapsed time (s)
   * dt — frame delta (s, already capped)
   */
  function updateParticles(t, dt) {
    for (let i = 0; i < particles.length; i++) {
      const p = particles[i];

      // Advance age; respawn if expired or off-screen.
      p.age += dt;
      if (p.age >= p.lifespan ||
          p.x < -40 || p.x > W + 40 ||
          p.y < -40 || p.y > H + 40) {
        spawnParticle(p, t, false);
        continue;
      }

      // ── Flow field: steer velocity toward field direction ──
      const targetAngle = flowAngle(p.x, p.y, t);
      const targetVx    = Math.cos(targetAngle) * p.speed;
      const targetVy    = Math.sin(targetAngle) * p.speed;

      // Smooth turn toward field (exponential approach, frame-rate independent).
      const steer = 1 - Math.exp(-2.5 * dt);
      p.vx += (targetVx - p.vx) * steer;
      p.vy += (targetVy - p.vy) * steer;

      // ── Cursor perturbation: decay back toward zero ──
      const returnFactor = Math.exp(-RETURN_RATE * dt);
      p.perturbVx *= returnFactor;
      p.perturbVy *= returnFactor;
      p.pineFrac  *= returnFactor;

      // ── Integrate position ──
      const totalVx = p.vx + p.perturbVx;
      const totalVy = p.vy + p.perturbVy;
      p.x += totalVx * dt;
      p.y += totalVy * dt;

      // ── Append to trail ring buffer (distance-based sampling) ──
      // Only record a new trail point when the particle has moved ≥ TRAIL_SAMPLE_DIST
      // from the last recorded position. This makes trails span real distance regardless
      // of frame rate, turning per-frame invisible micro-steps into visible streamlines.
      const ddx = p.x - p.lastTrailX;
      const ddy = p.y - p.lastTrailY;
      if (ddx * ddx + ddy * ddy >= TRAIL_SAMPLE_DIST * TRAIL_SAMPLE_DIST) {
        p.trailHead = (p.trailHead + 1) % TRAIL_LEN;
        p.trail[p.trailHead].x = p.x;
        p.trail[p.trailHead].y = p.y;
        if (p.trailLen < TRAIL_LEN) p.trailLen++;
        p.lastTrailX = p.x;
        p.lastTrailY = p.y;
      }
    }
  }

  // ── CURSOR ────────────────────────────────────────────────────────────────────

  function onMouseMove(e) {
    cursorX = e.clientX;
    cursorY = e.clientY;
    applyCursorForce();
  }

  function onTouchMove(e) {
    if (e.touches.length > 0) {
      cursorX = e.touches[0].clientX;
      cursorY = e.touches[0].clientY;
      applyCursorForce();
    }
  }

  function onPointerLeave() {
    cursorX = null;
    cursorY = null;
    // Perturbations decay naturally; no hard reset.
  }

  /**
   * Apply an outward push + tangential swirl to all particles within CURSOR_R.
   * Also sets pineFrac (→ color brighten) proportional to proximity.
   * Called on each pointer event (not per-frame), so the impulse is
   * event-driven; the decay is handled each frame.
   */
  function applyCursorForce() {
    if (cursorX === null || cursorY === null) return;

    for (let i = 0; i < particles.length; i++) {
      const p = particles[i];
      const dx = p.x - cursorX;
      const dy = p.y - cursorY;
      const dist = Math.sqrt(dx * dx + dy * dy);
      if (dist > CURSOR_R || dist < 0.5) continue;

      const t = 1 - dist / CURSOR_R; // 0 at edge, 1 at cursor

      // Outward (radial) unit vector.
      const nx = dx / dist;
      const ny = dy / dist;

      // Tangential (perpendicular, clockwise swirl).
      const tx = -ny;
      const ty =  nx;

      p.perturbVx += (nx * PUSH_STR + tx * SWIRL_STR) * t;
      p.perturbVy += (ny * PUSH_STR + ty * SWIRL_STR) * t;

      // Brighten toward pine near cursor.
      if (t > p.pineFrac) p.pineFrac = t;
    }
  }

  // ── DRAW ──────────────────────────────────────────────────────────────────────

  /**
   * Draw all particles: a fading trail polyline + the head dot or glyph.
   * Never fills the background — clearRect only.
   */
  function drawFrame() {
    ctx.clearRect(0, 0, W, H);

    ctx.font         = FONT_SIZE + 'px ' + FONT_MONO;
    ctx.textAlign    = 'center';
    ctx.textBaseline = 'middle';
    ctx.lineCap      = 'round';
    ctx.lineJoin     = 'round';

    for (let i = 0; i < particles.length; i++) {
      const p = particles[i];
      const fade = lifeFade(p);
      if (fade < 0.01) continue;

      // Blend ink → pine based on cursor proximity.
      const pr = p.pineFrac;
      const cr = Math.round(INK_R + pr * (PINE_R - INK_R));
      const cg = Math.round(INK_G + pr * (PINE_G - INK_G));
      const cb = Math.round(INK_B + pr * (PINE_B - INK_B));

      const headAlpha   = (p.alpha + pr * CURSOR_ALPHA_BOOST) * fade;

      // ── Trail: draw segments oldest→newest, alpha fading toward the tail ──
      if (p.trailLen >= 2) {
        const segCount = p.trailLen - 1;
        ctx.lineWidth = p.isGlyph ? 1.0 : 1.2;
        for (let s = 0; s < segCount; s++) {
          // idxA = s-th oldest position in the ring.
          const idxA = (p.trailHead - p.trailLen + 1 + s + TRAIL_LEN * 2) % TRAIL_LEN;
          const idxB = (idxA + 1) % TRAIL_LEN;

          const tFrac  = s / segCount;           // 0 = oldest tail, 1 = head
          const tAlpha = tFrac * tFrac * headAlpha * 0.60; // quadratic fade to tail
          if (tAlpha < 0.005) continue;

          ctx.strokeStyle = 'rgba(' + cr + ',' + cg + ',' + cb + ',' +
                            tAlpha.toFixed(3) + ')';
          ctx.beginPath();
          ctx.moveTo(p.trail[idxA].x, p.trail[idxA].y);
          ctx.lineTo(p.trail[idxB].x, p.trail[idxB].y);
          ctx.stroke();
        }
      }

      // ── Head: glyph or filled circle ──
      ctx.fillStyle = 'rgba(' + cr + ',' + cg + ',' + cb + ',' +
                      headAlpha.toFixed(3) + ')';

      if (p.isGlyph) {
        ctx.fillText(p.glyph, p.x, p.y);
      } else {
        // Small filled circle — slightly softer than a font dot.
        ctx.beginPath();
        ctx.arc(p.x, p.y, 1.6, 0, Math.PI * 2);
        ctx.fill();
      }
    }
  }

  // ── LOOP ──────────────────────────────────────────────────────────────────────

  function tick(now) {
    requestAnimationFrame(tick);

    if (lastTime === null) { lastTime = now; }
    const dt = Math.min((now - lastTime) * 0.001, 0.05); // cap at 50 ms
    lastTime = now;

    const t = now * 0.001;

    updateParticles(t, dt);
    drawFrame();
  }

  // ── RESIZE ────────────────────────────────────────────────────────────────────

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

    // Recompute target particle count from new viewport area.
    const area = W * H;
    targetCount = Math.min(MAX_PARTICLES,
                  Math.max(MIN_PARTICLES, Math.round(area / PX_PER_PARTICLE)));

    // Grow or shrink the particle pool to match.
    const t = (lastTime || 0) * 0.001;
    while (particles.length < targetCount) {
      particles.push(makeParticle(t, true)); // staggered so no sync pop-in
    }
    particles.length = targetCount; // trim excess
  }

  function onResize() {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(resize, RESIZE_DEBOUNCE);
  }

  // ── STATIC (prefers-reduced-motion) ──────────────────────────────────────────

  /**
   * Render a single calm scatter of faint dots/glyphs; no rAF, no listeners.
   * Count is the same medium density formula, clamped.
   */
  function drawStaticFrame() {
    ctx.clearRect(0, 0, W, H);
    ctx.font         = FONT_SIZE + 'px ' + FONT_MONO;
    ctx.textAlign    = 'center';
    ctx.textBaseline = 'middle';

    const area  = W * H;
    const count = Math.min(MAX_PARTICLES,
                  Math.max(MIN_PARTICLES, Math.round(area / PX_PER_PARTICLE)));

    for (let i = 0; i < count; i++) {
      const x = Math.random() * W;
      const y = Math.random() * H;
      const alpha = ALPHA_HEAD_MIN + Math.random() * (ALPHA_HEAD_MAX - ALPHA_HEAD_MIN);
      const isG   = Math.random() < GLYPH_FRAC;

      ctx.fillStyle = 'rgba(' + INK_R + ',' + INK_G + ',' + INK_B + ',' +
                      alpha.toFixed(3) + ')';

      if (isG) {
        const pool = DIGITS + ACCTING;
        ctx.fillText(pool[Math.floor(Math.random() * pool.length)], x, y);
      } else {
        ctx.beginPath();
        ctx.arc(x, y, 1.6, 0, Math.PI * 2);
        ctx.fill();
      }
    }
  }

  // ── BOOTSTRAP ─────────────────────────────────────────────────────────────────

  function init() {
    canvas = document.getElementById('ascii-bg');
    if (!canvas) return;

    ctx = canvas.getContext('2d');
    dpr = Math.min(window.devicePixelRatio || 1, MAX_DPR);

    W = window.innerWidth;
    H = window.innerHeight;

    canvas.width  = Math.round(W * dpr);
    canvas.height = Math.round(H * dpr);
    canvas.style.width  = W + 'px';
    canvas.style.height = H + 'px';
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.scale(dpr, dpr);

    // ── prefers-reduced-motion: one static scatter, nothing more ──
    if (reducedMotion) {
      drawStaticFrame();
      // No rAF, no pointer listeners, no resize handler (one-shot render).
      return;
    }

    // ── Animated path ──

    // Seed the initial particle pool (staggered ages).
    const area = W * H;
    targetCount = Math.min(MAX_PARTICLES,
                  Math.max(MIN_PARTICLES, Math.round(area / PX_PER_PARTICLE)));
    for (let i = 0; i < targetCount; i++) {
      particles.push(makeParticle(0, true));
    }

    // Resize handler (debounced).
    window.addEventListener('resize', onResize, { passive: true });

    // Cursor / touch listeners.
    window.addEventListener('mousemove',  onMouseMove,    { passive: true });
    window.addEventListener('touchmove',  onTouchMove,    { passive: true });
    window.addEventListener('mouseleave', onPointerLeave, { passive: true });
    window.addEventListener('touchend',   onPointerLeave, { passive: true });

    requestAnimationFrame(tick);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

})();
