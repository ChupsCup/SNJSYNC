
(function () {
  'use strict';
  var TAU = Math.PI * 2;

  var css =
    '.pm-wrap{display:flex;flex-direction:column;align-items:center;gap:2px}' +
    '.pm-wrap .pm-cv{display:block;width:100%}' +
    '.pm-pct{font:700 12px/1 ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-variant-numeric:tabular-nums;color:var(--text2,#a1a1aa)}' +
    '.pm-pct.done{color:var(--green,#22c55e)}' +
    '.pm-bwrap{display:inline-flex;line-height:0;vertical-align:middle}' +
    '.pm-bwrap canvas{display:block;width:100%;height:100%}';
  var _styleEl = null;
  function ensureCss() {
    if (_styleEl) return;
    _styleEl = document.createElement('style');
    _styleEl.textContent = css;
    document.head.appendChild(_styleEl);
  }

  function create(host, opts) {
    opts = opts || {};
    ensureCss();
    var showPct = !!opts.showPct;
    var spin = !!opts.spin;
    var R = opts.r || Math.max(4, Math.round((opts.height || 26) / 4.57));
    var G0 = Math.max(9, Math.round(R * 2.14));
    var H = Math.max(16, Math.round(R * 4.57));
    var CY = Math.round(R * 2.43);
    var wide = opts.width || Math.max(120, host.clientWidth || 380);

    host.classList.add('pm-wrap');
    host.style.width = wide + 'px';
    while (host.firstChild) host.removeChild(host.firstChild);

    var cv = document.createElement('canvas');
    cv.className = 'pm-cv';
    host.appendChild(cv);
    var numEl = null, pctEl = null;
    if (showPct) {
      numEl = document.createElement('div');
      numEl.className = 'pm-pct';
      numEl.innerHTML = '<span>0</span><small>%</small>';
      host.appendChild(numEl);
      pctEl = numEl.querySelector('span');
    }

    var ctx = cv.getContext('2d');
    var W, dpr, px0, pxMax, dots, stage, p, dp, px, pv, gx, gv, gap, gb, spd, idle, stalled, t, ph, gulp, look, blink, nextBlink,
      wait, pelletX, popT, v, raf, last, shown, demoT, sent, scared;

    function clamp(a, lo, hi) { return Math.max(lo, Math.min(hi, a)); }

    function currentWidth() {
      var cw = Math.round(host.clientWidth);
      return !cw || cw < 40 ? wide : cw;
    }

    function build() {
      cancelAnimationFrame(raf);
      if (demoT) { clearInterval(demoT); demoT = null; }
      W = currentWidth();
      dpr = Math.min(window.devicePixelRatio || 1, 2);
      cv.width = W * dpr; cv.height = H * dpr;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      px0 = R + 4;
      pxMax = Math.max(px0 + 14, W - 3 * R - G0 - 8);
      dots = [];
      for (var x = px0 + 14; x <= pxMax; x += Math.max(9, Math.round(R * 1.14))) dots.push({ x: x, e: false, t: 0 });
      stage = 'run'; p = 0; dp = 0; px = px0; pv = 0; gb = G0; gap = G0; gx = px + 2 * R + gap; gv = 0; spd = 0; idle = 0; stalled = false;
      t = 0; ph = 0; gulp = 0; look = 1; blink = 0; nextBlink = 2; wait = 0; popT = 0; v = 0; shown = -1; sent = false; scared = false;
      if (numEl) numEl.classList.remove('done');
      if (pctEl) pctEl.textContent = '0';
      last = performance.now();
      raf = requestAnimationFrame(tick);
    }

    function tick(now) {
      if (!host.isConnected) return;
      var dt = Math.min(.05, (now - last) / 1000); last = now; t += dt;
      if (wait > 0) wait -= dt; else step(dt);
      anim(dt); render();
      if (stage === 'done' && sent && popT > .9) return;
      raf = requestAnimationFrame(tick);
    }

    function step(dt) {
      if (stage === 'run') {
        dp += (p - dp) * Math.min(1, dt * 2.5); if (p - dp < .003) dp = p;
        var xt = px0 + dp * (pxMax - px0), ox = px;
        pv += ((xt - px) * 70 - pv * 12) * dt; px += pv * dt;
        spd += (Math.abs(px - ox) / dt - spd) * Math.min(1, dt * 8);
        idle = spd < 8 ? idle + dt : 0; stalled = idle > .6 && dp < 1;
        gb += ((stalled ? 10 : G0) - gb) * Math.min(1, dt * 4);
        gap = gb + (stalled ? Math.sin(t * 3) * 1.5 : Math.sin(t * 1.7) * 5 + Math.sin(t * 3.9) * 2);
        if (dp >= 1 && Math.abs(px - xt) < 1.5 && Math.abs(pv) < 20) { stage = 'pellet'; stalled = false; gap = G0; pelletX = px + R + G0 / 2; }
      } else if (stage === 'pellet') {
        pv = 150; px += 150 * dt; gap = G0;
        if (px + R >= pelletX - 2) { stage = 'dash'; scared = true; wait = .28; gulp = 1.4; v = 150; }
      } else if (stage === 'dash') {
        v = Math.min(500, v + 1100 * dt); pv = v; px += v * dt; gap = Math.max(0, gap - dt * 60);
        if (px + R >= gx - R + 4) { stage = 'done'; popT = 0; if (numEl) numEl.classList.add('done'); if (pctEl) pctEl.textContent = '100'; }
      }
    }

    function anim(dt) {
      var tx = Math.min(px + 2 * R + gap, W - R - 2);
      gv += ((tx - gx) * 55 - gv * 8) * dt; gx += gv * dt;
      ph += dt * (stage === 'dash' ? 22 : stalled ? 5 : 8 + Math.min(1, spd / 80) * 8);
      gulp = Math.max(0, gulp - dt * 5);
      look += ((stalled ? -1 : 1) - look) * Math.min(1, dt * 7);
      nextBlink -= dt; if (nextBlink < 0) { blink = .13; nextBlink = 1.5 + Math.random() * 3; } blink -= dt;
      if (stage === 'done') {
        popT += dt;
        if (popT > .7 && !sent) { sent = true; window.dispatchEvent(new Event('pacman:done')); }
      }
      for (var i = 0; i < dots.length; i++) {
        var d = dots[i];
        if (!d.e && px + R * .7 >= d.x) { d.e = true; d.t = t; if (stage === 'run') gulp = Math.max(gulp, .8); }
      }
      if (stage !== 'done') {
        var n = Math.min(99, Math.floor(dp * 100 + .001)); if (dp >= 1) n = 100;
        if (n !== shown) { shown = n; if (pctEl) pctEl.textContent = n; }
      }
    }

    function render() {
      ctx.clearRect(0, 0, W, H);
      for (var i = 0; i < dots.length; i++) {
        var d = dots[i], s = 1, a = 1;
        if (d.e) { var age = (t - d.t) / .2; if (age >= 1) continue; s = 1 + age * .9; a = 1 - age; }
        ctx.globalAlpha = a; ctx.fillStyle = '#ff9f43';
        ctx.beginPath(); ctx.arc(d.x, CY, 3 * s, 0, TAU); ctx.fill();
      }
      ctx.globalAlpha = 1;
      if (stage === 'pellet') {
        var pr = 7 + Math.sin(t * 14) * 1.3, g = ctx.createRadialGradient(pelletX, CY, 1, pelletX, CY, pr * 2.2);
        g.addColorStop(0, '#fff0d0'); g.addColorStop(.35, '#ff9f43'); g.addColorStop(.4, '#ff9f43'); g.addColorStop(1, 'rgba(255,159,67,0)');
        ctx.fillStyle = g; ctx.beginPath(); ctx.arc(pelletX, CY, pr * 2.2, 0, TAU); ctx.fill();
      }
      drawGhost(); drawPac();
      if (stage === 'done' && popT > .1) {
        ctx.globalAlpha = clamp(1 - (popT - .3) / .6, 0, 1); ctx.fillStyle = '#5ff2ff';
        ctx.font = '700 13px ui-monospace,Menlo,Consolas,monospace'; ctx.textAlign = 'center';
        ctx.fillText('200', gx, CY - R - 4 - popT * 26); ctx.globalAlpha = 1;
      }
    }

    function drawPac() {
      var m = (Math.sin(ph) * .5 + .5) * .75;
      if (stage === 'done') m = .08 + Math.max(0, .4 - popT) * .8;
      var hop = stage === 'dash' ? 0 : Math.abs(Math.sin(ph * .5)) * 1.5;
      if (stage === 'done') hop = Math.sin(Math.min(popT, .4) / .4 * Math.PI) * 6;
      var jx = stalled && gap < 16 ? Math.sin(t * 35) * .8 : 0, k = 1 + gulp * .05;
      ctx.save(); ctx.translate(px + jx, CY - hop); ctx.scale(k, k);
      var g = ctx.createRadialGradient(-R * .3, -R * .5, R * .1, 0, 0, R * 1.35);
      g.addColorStop(0, '#fff08a'); g.addColorStop(.5, '#ffd400'); g.addColorStop(1, '#b88a00');
      ctx.fillStyle = g; ctx.beginPath(); ctx.moveTo(0, 0); ctx.arc(0, 0, R, m, TAU - m); ctx.closePath(); ctx.fill();
      ctx.fillStyle = 'rgba(255,255,255,.22)'; ctx.beginPath(); ctx.ellipse(-R * .32, -R * .55, R * .3, R * .16, -.5, 0, TAU); ctx.fill();
      ctx.fillStyle = '#1d1d1d'; ctx.beginPath(); ctx.ellipse(R * .14, -R * .48, 2.2, 3.1, 0, 0, TAU); ctx.fill();
      ctx.fillStyle = '#fff'; ctx.beginPath(); ctx.arc(R * .14 + .7, -R * .48 - 1.1, .85, 0, TAU); ctx.fill();
      ctx.restore();
    }

    function drawGhost() {
      var sc = 1;
      if (stage === 'done') sc = popT < .1 ? 1 + popT * 2.5 : Math.max(0, 1.25 - (popT - .1) * 8.3);
      if (sc <= .01) return;
      var sx = 1 + clamp(gv / 700, -.12, .12), sy = 1 / sx, sh = scared && stage !== 'done';
      var gj = sh ? Math.sin(t * 70) * 1.2 : 0, bob = Math.sin(t * 6) * 1.6 + (sh ? Math.abs(Math.sin(t * 45)) * 1.5 : 0);
      var c1 = '#ff9a9a', c2 = '#ff3030', c3 = '#9a0a0a';
      if (scared) {
        c1 = '#9a9aff'; c2 = '#2a2aff'; c3 = '#0a0a78';
        if (stage === 'dash' && Math.floor(t * 8) % 2) { c1 = '#ffffff'; c2 = '#e8e8ff'; c3 = '#9a9ad0'; }
      }
      ctx.save(); ctx.translate(gx + gj, CY - bob); ctx.scale(sx * sc, sy * sc);
      var halo = ctx.createRadialGradient(0, 0, R * .5, 0, 0, R * 2.2);
      halo.addColorStop(0, sh ? 'rgba(80,80,255,.2)' : 'rgba(255,48,48,.16)'); halo.addColorStop(1, 'rgba(0,0,0,0)');
      ctx.fillStyle = halo; ctx.beginPath(); ctx.arc(0, 0, R * 2.2, 0, TAU); ctx.fill();
      var g = ctx.createRadialGradient(-R * .3, -R * .5, R * .1, 0, 0, R * 1.35);
      g.addColorStop(0, c1); g.addColorStop(.5, c2); g.addColorStop(1, c3);
      ctx.fillStyle = g; ctx.beginPath(); ctx.arc(0, 0, R, Math.PI, 0); ctx.lineTo(R, R * .85);
      var N = 18, gp = t * (sh ? 24 : 9);
      for (var i = 1; i <= N; i++) { var u = i / N; ctx.lineTo(R - 2 * R * u, R * .85 + (Math.abs(Math.sin(u * 3 * Math.PI + gp)) - .5) * R * .34); }
      ctx.closePath(); ctx.fill();
      ctx.fillStyle = 'rgba(255,255,255,.22)'; ctx.beginPath(); ctx.ellipse(-R * .32, -R * .55, R * .3, R * .16, -.5, 0, TAU); ctx.fill();
      if (!scared) {
        var op = blink > 0 ? .1 : 1;
        for (var kk = -1; kk <= 1; kk += 2) {
          var ex = kk * R * .38, ey = -R * .1;
          ctx.fillStyle = '#fff'; ctx.beginPath(); ctx.ellipse(ex, ey, R * .3, R * .36 * op, 0, 0, TAU); ctx.fill();
          ctx.fillStyle = '#1a1aa0'; ctx.beginPath(); ctx.ellipse(ex + look * R * .13, ey + R * .04, R * .15, R * .18 * op, 0, 0, TAU); ctx.fill();
        }
      } else {
        ctx.fillStyle = '#ffd9a8';
        for (var k2 = -1; k2 <= 1; k2 += 2) { ctx.beginPath(); ctx.arc(k2 * R * .34, -R * .2, R * .13, 0, TAU); ctx.fill(); }
        ctx.strokeStyle = '#ffd9a8'; ctx.lineWidth = 1.6; ctx.lineJoin = 'round'; ctx.beginPath();
        for (var z = 0; z <= 6; z++) { var zx = -R * .6 + z * R * .2; z ? ctx.lineTo(zx, R * .3 + (z % 2 ? -2.2 : 2.2)) : ctx.moveTo(zx, R * .3 + 2.2); }
        ctx.stroke();
      }
      ctx.restore();
    }

    function spinLoop() {
      build();
      var off = Math.random() * 2;
      var K = [[0, 0], [1, .22], [2, .3], [3.2, .62], [5.2, .62], [6.2, .9], [7.2, .94], [8.2, 1]], s = -off;
      demoT = setInterval(function () {
        if (!host.isConnected) { clearInterval(demoT); demoT = null; return; }
        s += .1;
        for (var i = 1; i < K.length; i++) if (s <= K[i][0]) {
          var a = K[i - 1], b = K[i];
          p = Math.max(p, a[1] + (b[1] - a[1]) * (s - a[0]) / (b[0] - a[0]));
          return;
        }
        p = 1;
      }, 100);
      window.addEventListener('pacman:done', function again() {
        window.removeEventListener('pacman:done', again);
        if (!host.isConnected) return;
        clearInterval(demoT); demoT = null;
        setTimeout(spinLoop, 700);
      });
    }

    var api = {
      set: function (q) {
        if (spin) return;
        p = Math.max(p, Math.min(1, q));
      },
      done: function () {
        if (spin) return;
        p = 1;
      },
      restart: build,
      destroy: function () {
        cancelAnimationFrame(raf);
        if (demoT) { clearInterval(demoT); demoT = null; }
        while (host.firstChild) host.removeChild(host.firstChild);
      }
    };

    if (spin) {
      spinLoop();
    } else {
      build();
    }

    window.addEventListener('resize', function () {
      if (!host.isConnected) return;
      if (stage !== 'done') {
        var q = p;
        build();
        if (!spin) p = q;
      }
    });

    return api;
  }

  function mountSpin(el) {
    if (!el || el.__pmMounted) return;
    el.__pmMounted = true;
    var h = parseInt(el.getAttribute('data-h'), 10) || 20;
    var w = parseInt(el.getAttribute('data-w'), 10) || 76;
    var host = document.createElement('span');
    host.className = 'pm-bwrap';
    host.style.width = w + 'px';
    host.style.height = h + 'px';
    host.style.marginRight = '5px';
    el.replaceWith(host);
    create(host, { height: h, width: w, spin: true });
  }

  function mountAll(root) {
    var nodes;
    if (root && root.querySelectorAll) nodes = root.querySelectorAll('.pm-spin:not([data-pm-mount])');
    else nodes = document.querySelectorAll('.pm-spin:not([data-pm-mount])');
    for (var i = 0; i < nodes.length; i++) {
      var el = nodes[i];
      el.setAttribute('data-pm-mount', '1');
      mountSpin(el);
    }
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', function () { mountAll(); });
  } else {
    mountAll();
  }

  var mo = new MutationObserver(function (muts) {
    for (var i = 0; i < muts.length; i++) {
      var adds = muts[i].addedNodes;
      for (var j = 0; j < adds.length; j++) {
        var n = adds[j];
        if (!n || !n.querySelectorAll) continue;
        if (n.classList && n.classList.contains('pm-spin') && !n.__pmMounted) {
          n.setAttribute('data-pm-mount', '1');
          mountSpin(n);
        }
        var found = n.querySelectorAll('.pm-spin:not([data-pm-mount])');
        for (var k = 0; k < found.length; k++) {
          found[k].setAttribute('data-pm-mount', '1');
          mountSpin(found[k]);
        }
      }
    }
  });
  mo.observe(document.documentElement, { childList: true, subtree: true });

  window.Pm = { create: create };
})();