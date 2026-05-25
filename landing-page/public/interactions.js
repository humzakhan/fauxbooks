/**
 * interactions.js — small DOM interactions for the Ledger landing page.
 *
 * A) Copy-to-clipboard for button.copy-btn
 * B) Balance count-up animation for .bal-num[data-target]
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
})();
