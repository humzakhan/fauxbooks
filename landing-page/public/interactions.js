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
    initFileExplorer();
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

  // ── D) Interactive file explorer (Examples section) ──────────────────────────

  /**
   * Two-pane ./books/ browser. Clicking a file in the tree renders its real
   * sample rows (from a seeded golden run) into the preview table.
   *
   * Column kinds: 'dr' → pine (debit), 'cr' → oxblood (credit), 'num' → right-
   * aligned, default → left ink text. Files marked balanced show a "Σ debits =
   * Σ credits ✓" badge. All values are real fixture data; tables are built with
   * DOM APIs (textContent), never innerHTML on data.
   */
  function initFileExplorer() {
    var root = document.getElementById('file-explorer');
    if (!root) return;

    var nameEl     = document.getElementById('exp-name');
    var balancedEl = document.getElementById('exp-balanced');
    var tableEl    = document.getElementById('exp-table');
    var noteEl     = document.getElementById('exp-note');
    if (!nameEl || !tableEl || !noteEl) return;

    // Real rows from testdata/golden/monthly-close/2026-01/small.
    var FILES = {
      'gl.csv': {
        balanced: true,
        cols: [{ l: 'entry_id' }, { l: 'account' }, { l: 'debit', k: 'dr' }, { l: 'credit', k: 'cr' }],
        rows: [
          ['JE-00000001', '5000', '5910.100000000', '0.000000000'],
          ['JE-00000001', '1200', '0.000000000', '5910.100000000'],
          ['JE-00000002', '1010', '3477.180000000', '0.000000000'],
          ['JE-00000002', '1100', '0.000000000', '3477.180000000']
        ],
        note: 'Debits and credits in every journal entry net to zero by construction.'
      },
      'journal_entries.csv': {
        cols: [{ l: 'entry_id' }, { l: 'description' }, { l: 'source_event_id' }],
        rows: [
          ['JE-00000001', 'Inventory COGS', 'evt-000001'],
          ['JE-00000002', 'Customer payment', 'evt-000002'],
          ['JE-00000003', 'Vendor bill', 'evt-000003'],
          ['JE-00000004', 'Sale invoice', 'evt-000004']
        ],
        note: 'One header row per business event; gl.csv holds its balanced lines.'
      },
      'coa.csv': {
        cols: [{ l: 'code' }, { l: 'name' }, { l: 'type' }, { l: 'normal_balance' }],
        rows: [
          ['1000', 'Cash and Cash Equivalents', 'asset', 'debit'],
          ['1010', 'Checking Account', 'asset', 'debit'],
          ['1020', 'Savings Account', 'asset', 'debit'],
          ['1030', 'Petty Cash', 'asset', 'debit']
        ],
        note: 'The chart of accounts that every account code in the books resolves to.'
      },
      'bank_statement.csv': {
        balanced: true,
        cols: [{ l: 'entry_id' }, { l: 'account' }, { l: 'debit', k: 'dr' }, { l: 'credit', k: 'cr' }],
        rows: [
          ['JE-00000002', '1010', '3477.180000000', '0.000000000'],
          ['JE-00000005', '1010', '0.000000000', '644.035140000'],
          ['JE-00000006', '1010', '0.000000000', '469.390000000'],
          ['JE-00000008', '1010', '0.000000000', '9480.580000000']
        ],
        note: 'Cash postings tie back to GL entries, so bank totals reconcile to GL cash.'
      },
      'customers.csv': {
        cols: [{ l: 'id' }, { l: 'name' }, { l: 'city' }, { l: 'state' }],
        rows: [
          ['cust-001', 'CostQuest', 'Los Angeles', 'MI'],
          ['cust-002', 'Healthline', 'Miami', 'AA'],
          ['cust-003', 'Harris Corporation', 'Scottsdale', 'NC'],
          ['cust-004', 'MarketSense', 'Washington', 'ID']
        ],
        note: 'Customer master records that invoices and AR aging reference by id.'
      },
      'vendors.csv': {
        cols: [{ l: 'id' }, { l: 'name' }, { l: 'city' }, { l: 'state' }],
        rows: [
          ['vend-001', 'Apextech LLC', 'Riverside', 'UT'],
          ['vend-002', 'Construction Monitor LLC', 'Mesa', 'MN'],
          ['vend-003', 'Compliance and Risks', 'St. Petersburg', 'WV'],
          ['vend-004', 'FlightView', 'Atlanta', 'MA']
        ],
        note: 'Vendor master records that bills and AP aging reference by id.'
      },
      'invoices.csv': {
        cols: [{ l: 'entry_id' }, { l: 'amount', k: 'num' }, { l: 'sub_ledger_type' }],
        rows: [
          ['JE-00000004', '480.460000000', 'invoice'],
          ['JE-00000012', '7309.660000000', 'invoice'],
          ['JE-00000016', '3399.640000000', 'invoice'],
          ['JE-00000018', '3928.830000000', 'invoice']
        ],
        note: 'Sub-ledger rows each reference an existing journal entry.'
      },
      'trial-balance.csv': {
        balanced: true,
        cols: [{ l: 'account' }, { l: 'name' }, { l: 'debit', k: 'dr' }, { l: 'credit', k: 'cr' }],
        rows: [
          ['1010', 'Checking Account', '0.000000000', '188234.574775000'],
          ['1100', 'Accounts Receivable', '0.000000000', '23156.520000000'],
          ['1200', 'Inventory', '62790.300000000', '0.000000000'],
          ['1510', 'Accumulated Depreciation', '0.000000000', '27668.700000000']
        ],
        note: 'Debit and credit totals per account, and the whole trial balance ties out.'
      },
      'balance-sheet.csv': {
        cols: [{ l: 'section' }, { l: 'account' }, { l: 'name' }, { l: 'amount', k: 'num' }],
        rows: [
          ['asset', '1010', 'Checking Account', '-188234.574775000'],
          ['asset', '1100', 'Accounts Receivable', '-23156.520000000'],
          ['asset', '1200', 'Inventory', '62790.300000000'],
          ['asset', '1510', 'Accumulated Depreciation', '-27668.700000000']
        ],
        note: 'Assets, liabilities, and equity as of period end.'
      },
      'profit-loss.csv': {
        cols: [{ l: 'section' }, { l: 'account' }, { l: 'name' }, { l: 'amount', k: 'num' }],
        rows: [
          ['revenue', '4000', 'Sales Revenue', '63680.410000000'],
          ['revenue', '4200', 'Interest Income', '25961.940000000'],
          ['total_revenue', '', '', '89642.350000000'],
          ['expense', '5000', 'Cost of Goods Sold', '34830.390000000']
        ],
        note: 'P&L derived from GL revenue and expense accounts.'
      },
      'ar-aging.csv': {
        cols: [{ l: 'party' }, { l: '0–30', k: 'num' }, { l: '31–60', k: 'num' }, { l: '61–90', k: 'num' }, { l: '91+', k: 'num' }],
        rows: [
          ['CostQuest', '2326.640000000', '0.000000000', '0.000000000', '0.000000000'],
          ['Healthline', '3284.610000000', '0.000000000', '0.000000000', '0.000000000'],
          ['Harris Corporation', '11238.490000000', '0.000000000', '0.000000000', '0.000000000'],
          ['MarketSense', '7769.390000000', '0.000000000', '0.000000000', '0.000000000']
        ],
        note: 'Receivables bucketed by age, per customer.'
      }
    };

    var buttons = root.querySelectorAll('.tree__file');

    function thClass(kind) { return kind ? 'num' : ''; }
    function tdClass(kind) {
      if (kind === 'dr') return 'num cell--dr';
      if (kind === 'cr') return 'num cell--cr';
      if (kind === 'num') return 'num';
      return '';
    }

    function render(file) {
      var data = FILES[file];
      if (!data) return;

      nameEl.textContent = file;

      if (balancedEl) {
        if (data.balanced) { balancedEl.hidden = false; }
        else { balancedEl.hidden = true; }
      }

      // Build an Excel-style sheet: column letters (A,B,C…), a row-number
      // gutter, gridlines, and a bold field-name header row. DOM APIs only.
      var ncols = data.cols.length;
      var table = document.createElement('table');
      table.className = 'sheet';

      // Column-letter strip: corner + A, B, C …
      var thead = document.createElement('thead');
      var letterRow = document.createElement('tr');
      var corner = document.createElement('th');
      corner.className = 'sheet__corner';
      letterRow.appendChild(corner);
      for (var c = 0; c < ncols; c++) {
        var lth = document.createElement('th');
        lth.className = 'sheet__letter';
        lth.textContent = String.fromCharCode(65 + c);
        letterRow.appendChild(lth);
      }
      thead.appendChild(letterRow);
      table.appendChild(thead);

      var tbody = document.createElement('tbody');

      // Row 1 — the field names, styled like a typed header row
      var fieldRow = document.createElement('tr');
      var fnum = document.createElement('th');
      fnum.className = 'sheet__rownum';
      fnum.textContent = '1';
      fieldRow.appendChild(fnum);
      for (var fc = 0; fc < ncols; fc++) {
        var fcell = document.createElement('td');
        fcell.className = 'sheet__field' + (data.cols[fc].k ? ' num' : '');
        fcell.textContent = data.cols[fc].l;
        fieldRow.appendChild(fcell);
      }
      tbody.appendChild(fieldRow);

      // Data rows — numbered 2, 3, …
      for (var r = 0; r < data.rows.length; r++) {
        var tr = document.createElement('tr');
        var rn = document.createElement('th');
        rn.className = 'sheet__rownum';
        rn.textContent = String(r + 2);
        tr.appendChild(rn);
        for (var ci = 0; ci < ncols; ci++) {
          var td = document.createElement('td');
          td.textContent = data.rows[r][ci];
          var cls = tdClass(data.cols[ci].k);
          if (cls) td.className = cls;
          tr.appendChild(td);
        }
        tbody.appendChild(tr);
      }
      table.appendChild(tbody);

      tableEl.innerHTML = '';
      tableEl.appendChild(table);

      noteEl.textContent = data.note;

      // Update selected state
      for (var b = 0; b < buttons.length; b++) {
        if (buttons[b].getAttribute('data-file') === file) {
          buttons[b].setAttribute('aria-current', 'true');
        } else {
          buttons[b].removeAttribute('aria-current');
        }
      }
    }

    for (var i = 0; i < buttons.length; i++) {
      (function (btn) {
        btn.addEventListener('click', function () {
          render(btn.getAttribute('data-file'));
        });
      })(buttons[i]);
    }

    // Initial render — the file marked aria-current in markup, else gl.csv
    var initial = root.querySelector('.tree__file[aria-current="true"]');
    render(initial ? initial.getAttribute('data-file') : 'gl.csv');
  }

})();
