/**
 * interactions.js — small DOM interactions for the Ledger landing page.
 *
 * A) Copy-to-clipboard for button.copy-btn
 * B) Balance count-up animation for .bal-num[data-target]
 * C) Animated hero terminal (#hero-terminal)
 *
 * Vanilla JS, no libraries, no build step. Loaded with defer.
 * All selectors are guarded — missing elements are silently skipped.
 */

(function () {
  'use strict';

  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  // ── Bootstrap ────────────────────────────────────────────────────────────────

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', setup);
  } else {
    setup();
  }

  function setup() {
    initCopyButtons();
    initBalanceCountUp();
    initHeroTerminal();
  }

  // ── A) Copy-to-clipboard ─────────────────────────────────────────────────────

  /**
   * For every button.copy-btn, read data-copy attribute, find the element with
   * that id, and copy its trimmed textContent to the clipboard on click.
   * On success: button text → "copied ✓" + class "copied" for ~1300ms.
   */
  function initCopyButtons() {
    var buttons = document.querySelectorAll('button.copy-btn');
    if (!buttons || buttons.length === 0) return;

    for (var i = 0; i < buttons.length; i++) {
      (function (btn) {
        btn.addEventListener('click', function () {
          var targetId = btn.getAttribute('data-copy');
          if (!targetId) return;

          var sourceEl = document.getElementById(targetId);
          if (!sourceEl) return;

          var text = (sourceEl.textContent || '').trim();

          copyText(text, function (ok) {
            if (!ok) return;
            var originalText = btn.textContent;
            btn.textContent = 'copied ✓';
            btn.classList.add('copied');
            setTimeout(function () {
              btn.textContent = 'copy';
              btn.classList.remove('copied');
            }, 1300);
          });
        });
      })(buttons[i]);
    }
  }

  /**
   * Copy `text` to clipboard. Calls `cb(true)` on success, `cb(false)` on failure.
   * Tries navigator.clipboard first, falls back to execCommand.
   */
  function copyText(text, cb) {
    if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
      navigator.clipboard.writeText(text).then(
        function () { cb(true); },
        function () { fallbackCopy(text, cb); }
      );
    } else {
      fallbackCopy(text, cb);
    }
  }

  function fallbackCopy(text, cb) {
    try {
      var ta = document.createElement('textarea');
      ta.value = text;
      ta.setAttribute('readonly', '');
      ta.style.position = 'absolute';
      ta.style.left     = '-9999px';
      ta.style.top      = '-9999px';
      document.body.appendChild(ta);
      ta.select();
      var ok = document.execCommand('copy');
      document.body.removeChild(ta);
      cb(ok);
    } catch (e) {
      cb(false);
    }
  }

  // ── B) Balance count-up ──────────────────────────────────────────────────────

  /**
   * Find .bal-num[data-target] elements. Parse their data-target as a float.
   * Animate text from 0 → target over ~1200ms with ease-out, formatted as
   * thousands-separated with 2 decimal places (e.g. "5,910.10").
   * When done, add class "is-balanced" to the closest .balance-badge.
   *
   * Triggered by IntersectionObserver (threshold 0.5) or immediately on load
   * if IntersectionObserver is unavailable.
   *
   * If prefers-reduced-motion: set final value immediately, add is-balanced.
   */
  function initBalanceCountUp() {
    var elems = document.querySelectorAll('.bal-num[data-target]');
    if (!elems || elems.length === 0) return;

    for (var i = 0; i < elems.length; i++) {
      (function (el) {
        var raw    = el.getAttribute('data-target');
        var target = parseFloat(raw);
        if (isNaN(target)) return;

        if (reducedMotion) {
          // Skip animation: apply final state immediately
          el.textContent = formatNumber(target);
          var badge = el.closest('.balance-badge');
          if (badge) badge.classList.add('is-balanced');
          return;
        }

        if (typeof IntersectionObserver !== 'undefined') {
          var observer = new IntersectionObserver(function (entries, obs) {
            for (var j = 0; j < entries.length; j++) {
              if (entries[j].isIntersecting) {
                obs.unobserve(el);
                runCountUp(el, target);
              }
            }
          }, { threshold: 0.5 });
          observer.observe(el);
        } else {
          // Fallback: run immediately
          runCountUp(el, target);
        }
      })(elems[i]);
    }
  }

  /**
   * Animate el.textContent from 0 to `target` over DURATION ms.
   * Uses ease-out cubic: progress = 1 - (1 - t)^3.
   */
  function runCountUp(el, target) {
    var DURATION = 1200; // ms
    var startTime = null;

    function step(now) {
      if (startTime === null) startTime = now;
      var elapsed  = now - startTime;
      var t        = Math.min(elapsed / DURATION, 1);
      // Ease-out cubic
      var eased    = 1 - Math.pow(1 - t, 3);
      var current  = eased * target;

      el.textContent = formatNumber(current);

      if (t < 1) {
        requestAnimationFrame(step);
      } else {
        // Finished — set exact final value and mark badge balanced
        el.textContent = formatNumber(target);
        var badge = el.closest('.balance-badge');
        if (badge) badge.classList.add('is-balanced');
      }
    }

    requestAnimationFrame(step);
  }

  /**
   * Format a number with thousands separators and exactly 2 decimal places.
   * e.g. 5910.1 → "5,910.10"
   */
  function formatNumber(n) {
    // Round to 2 dp first to avoid floating-point display glitches
    var fixed = n.toFixed(2);
    // Insert thousands separators into the integer part
    var parts = fixed.split('.');
    parts[0]  = parts[0].replace(/\B(?=(\d{3})+(?!\d))/g, ',');
    return parts.join('.');
  }

  // ── C) Hero terminal animation ───────────────────────────────────────────────

  /**
   * Drives the animated terminal in #hero-terminal.
   * Types the gen command char-by-char, then reveals output lines one at a time,
   * holds, clears, and loops. Click on the terminal restarts the sequence.
   * prefers-reduced-motion: renders final state statically, no timers.
   */
  function initHeroTerminal() {
    var wrap    = document.getElementById('hero-terminal');
    var typed   = document.getElementById('term-typed');
    var caret   = document.getElementById('term-caret');
    var out     = document.getElementById('term-out');

    // Guard: if any element is missing, do nothing
    if (!wrap || !typed || !caret || !out) return;

    var COMMAND = 'fbx gen monthly-close --seed 1 --out ./books';
    var CHAR_MS  = 48;   // ms per character while typing
    var LINE_MS  = 200;  // ms between output lines appearing
    var HOLD_MS  = 2600; // hold before clearing and looping
    var CLEAR_MS = 340;  // fade-to-clear duration

    // Output line descriptors: [cssClass, tickChar|null, filenameText, extraText]
    // Each object: { cls: String, tick: Boolean, text: String }
    var OUTPUT_LINES = [
      { cls: 'terminal__line--ok',       tick: true,  text: 'gl.csv' },
      { cls: 'terminal__line--ok',       tick: true,  text: 'journal_entries.csv' },
      { cls: 'terminal__line--ok',       tick: true,  text: 'coa.csv' },
      { cls: 'terminal__line--ok',       tick: true,  text: 'trial-balance.csv' },
      { cls: 'terminal__line--ok',       tick: true,  text: 'balance-sheet.csv' },
      { cls: 'terminal__line--ok',       tick: true,  text: 'profit-loss.csv' },
      { cls: 'terminal__line--ok',       tick: true,  text: '… +13 more' },
      { cls: 'terminal__line--spacer',   tick: false, text: '' },
      { cls: 'terminal__line--sum',      tick: false, text: 'Σ debits = Σ credits' },
      { cls: 'terminal__line--balanced', tick: true,  text: 'balanced · 19 files · seed 1 · 2026-01' },
    ];

    var timers = [];

    function schedule(ms, fn) {
      var id = setTimeout(fn, ms);
      timers.push(id);
      return id;
    }

    function clearTimers() {
      for (var i = 0; i < timers.length; i++) {
        clearTimeout(timers[i]);
      }
      timers = [];
    }

    /**
     * Build a single output line element.
     * Uses DOM APIs — no innerHTML on variable content.
     */
    function buildLine(descriptor) {
      var el = document.createElement('span');
      el.className = 'terminal__line ' + descriptor.cls;

      if (descriptor.cls === 'terminal__line--spacer') {
        // spacer: already visible, just height
        el.classList.add('visible');
        return el;
      }

      if (descriptor.tick) {
        var tickSpan = document.createElement('span');
        tickSpan.className = 'tick';
        tickSpan.textContent = '✓ ';
        el.appendChild(tickSpan);
      }

      var textNode = document.createTextNode(descriptor.text);
      el.appendChild(textNode);

      return el;
    }

    /**
     * Render the fully-completed terminal state without any animation.
     * Used for prefers-reduced-motion.
     */
    function renderStatic() {
      typed.textContent = COMMAND;
      // caret hidden by CSS under reduced-motion

      out.innerHTML = '';
      for (var i = 0; i < OUTPUT_LINES.length; i++) {
        var el = buildLine(OUTPUT_LINES[i]);
        el.classList.add('visible');
        out.appendChild(el);
      }
    }

    /**
     * Run the full animated sequence, then loop.
     */
    function runSequence() {
      clearTimers();

      // Reset state
      typed.textContent = '';
      out.innerHTML = '';
      // Restore caret display (may have been hidden during fade)
      wrap.style.transition = '';
      wrap.style.opacity    = '';
      caret.style.display   = '';

      // 1. Type the command character by character
      var charIdx = 0;
      function typeNext() {
        if (charIdx < COMMAND.length) {
          typed.textContent += COMMAND[charIdx];
          charIdx++;
          schedule(CHAR_MS, typeNext);
        } else {
          // Typing done — pause, then reveal output lines
          schedule(320, showOutputLines);
        }
      }
      schedule(CHAR_MS, typeNext);

      // 2. Pre-build output line elements (hidden until revealed)
      var lineEls = [];
      for (var i = 0; i < OUTPUT_LINES.length; i++) {
        var lineEl = buildLine(OUTPUT_LINES[i]);
        out.appendChild(lineEl);
        lineEls.push(lineEl);
      }

      function showOutputLines() {
        // Stop caret blinking once output starts
        caret.style.opacity = '0';

        for (var j = 0; j < lineEls.length; j++) {
          (function (el, idx) {
            schedule(idx * LINE_MS, function () {
              el.classList.add('visible');
            });
          })(lineEls[j], j);
        }

        // 3. After all lines revealed, hold, then fade and loop
        var totalReveal = lineEls.length * LINE_MS;
        schedule(totalReveal + HOLD_MS, clearAndLoop);
      }

      function clearAndLoop() {
        wrap.style.transition = 'opacity ' + CLEAR_MS + 'ms ease';
        wrap.style.opacity    = '0';
        schedule(CLEAR_MS, function () {
          wrap.style.transition = '';
          wrap.style.opacity    = '';
          caret.style.opacity   = '';
          runSequence();
        });
      }
    }

    // Restart sequence on click (only when animated)
    if (!reducedMotion) {
      wrap.addEventListener('click', function () {
        runSequence();
      });
    }

    // Init
    if (reducedMotion) {
      renderStatic();
    } else {
      runSequence();
    }
  }

})();
