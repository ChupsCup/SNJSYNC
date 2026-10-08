





const $ = (id) => document.getElementById(id);

function esc(s) {
  return String(s == null ? '' : s).replace(/[&<>"']/g, (c) => (
    { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]
  ));
}
function el(tag, cls, html) {
  const n = document.createElement(tag);
  if (cls) n.className = cls;
  if (html != null) n.innerHTML = html;
  return n;
}
async function api(url) {
  const r = await fetch(url);
  if (!r.ok) {
    let msg = 'Gagal memuat data.';
    try { msg = (await r.json()).error || msg; } catch (e) {  }
    throw new Error(msg);
  }
  return r.json();
}
function debounce(fn, ms) {
  let t;
  return (...a) => { clearTimeout(t); t = setTimeout(() => fn(...a), ms); };
}




document.querySelectorAll('.tab').forEach((btn) => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.tab').forEach((b) => b.classList.remove('active'));
    document.querySelectorAll('.panel').forEach((p) => p.classList.remove('active'));
    btn.classList.add('active');
    $('p-' + btn.dataset.tab).classList.add('active');
  });
});




$('navSettings').addEventListener('click', () => $('setModal').classList.add('show'));
$('setX').addEventListener('click', () => $('setModal').classList.remove('show'));
$('setModal').addEventListener('click', (e) => {
  if (e.target === $('setModal')) $('setModal').classList.remove('show');
});




async function loadSummary() {
  try {
    const d = await api('/api/user/summary');
    const f = d.foto || {};
    const cards = [
      ['Foto', f.total != null ? f.total.toLocaleString('id-ID') : '-'],
      
      
      ['Master Item', (d.masterItem != null ? d.masterItem.toLocaleString('id-ID') : '-')],
      ['Masterlist', (d.masterlistAktif != null ? d.masterlistAktif.toLocaleString('id-ID') : '-') + ' aktif'],
    ];
    $('stats').innerHTML = cards
      .map(([l, v]) => `<div class="stat"><b>${esc(v)}</b><span>${esc(l)}</span></div>`)
      .join('');
  } catch (e) {  }
}







let ctfSeq = 0;

async function ctfSearch() {
  const q = $('ctfQ').value.trim();
  const meta = $('ctfMeta');
  const box = $('ctfPhotos');
  const seq = ++ctfSeq;
  if (!q) {
    meta.textContent = '';
    box.innerHTML = `<div class="empty"><b>Mulai dengan mengetik</b>Masukkan kata kunci di atas untuk mencari foto.</div>`;
    return;
  }
  meta.textContent = 'Mencari...';
  box.innerHTML = '';
  try {
    const d = await api('/api/search?q=' + encodeURIComponent(q));
    if (seq !== ctfSeq) return;
    const exact = d.results || [];
    const partial = d.partial || [];
    if (!exact.length && !partial.length) {
      meta.textContent = '';
      box.innerHTML = `<div class="empty"><b>Tidak ditemukan</b>Tidak ada foto yang cocok dengan &quot;${esc(q)}&quot;.</div>`;
      return;
    }
    
    
    
    const head = exact.length
      ? `<div class="grp">Cocok semua kata &mdash; <b>${exact.length}</b> foto</div>`
      : `<div class="grp warn">Tidak ada foto yang cocok semua kata. ${partial.length} foto terdekat (kurang ${esc(String(partial[0].total - partial[0].matched))} kata):</div>`;
    const list = exact.length ? exact : partial;
    meta.innerHTML = `<span class="chip">${list.length} foto</span>`
      + (d.ms ? `<span>${esc(d.ms)} ms</span>` : '');
    box.insertAdjacentHTML('beforeend', head);
    
    let i = 0;
    const step = 40;
    (function chunk() {
      if (seq !== ctfSeq) return;
      const slice = list.slice(i, i + step);
      for (const p of slice) {
        const card = el('div', 'photo');
        const src = '/api/photo?id=' + encodeURIComponent(p.id) + '&v=20261007';
        const miss = p.total - p.matched;
        card.innerHTML = `<img src="${esc(src)}" alt="" loading="lazy">
          <div class="info"><div class="nm">${esc(p.name || p.id)}</div>
          <div class="sub">${esc(p.dir || '')}</div>${miss > 0 ? `<div class="miss">kurang ${miss} dari ${p.total} kata</div>` : ''}</div>`;
        card.addEventListener('click', () => openLightbox(p, src));
        box.appendChild(card);
      }
      i += step;
      if (i < list.length) setTimeout(chunk, 0);
    })();
  } catch (e) {
    if (seq !== ctfSeq) return;
    meta.textContent = '';
    box.innerHTML = `<div class="empty"><b>Gagal</b>${esc(e.message)}</div>`;
  }
}

// ---------- Lightbox: zoom & pan ----------
// Logikanya sama dengan viewer CTF (#fsOverlay): gambar di-absolute lalu
// digeser lewat translate() dan diperbesar lewat scale(), dengan
// transform-origin 0 0. Efeknya zoom membesar dari titik kursor, bukan
// dari tengah gambar.
const lb = $('lb'), lbStage = $('lbStage'), lbImg = $('lbImg');
let lbScale = 1, lbOx = 0, lbOy = 0, lbDrag = false, lbLastX = 0, lbLastY = 0,
  lbURL = '', lbName = '';

const LB_MIN = 0.05, LB_MAX = 15;

function lbApply() {
  lbImg.style.transform = 'translate(' + lbOx + 'px,' + lbOy + 'px) scale(' + lbScale + ')';
  $('lbPct').textContent = Math.round(lbScale * 100 / (lbFitScale || 1)) + '%';
}

function lbFit() {
  const nw = lbImg.naturalWidth, nh = lbImg.naturalHeight;
  if (!nw || !nh) return;
  const box = lbStage.getBoundingClientRect();
  lbScale = Math.min(box.width * 0.94 / nw, box.height * 0.9 / nh, 1);
  lbFitScale = lbScale;
  lbOx = (box.width - nw * lbScale) / 2;
  lbOy = (box.height - nh * lbScale) / 2;
  lbApply();
}
let lbFitScale = 1;

// Zoom dari titik kursor: titik yang di bawah kursor tetap di sana.
function lbZoomAt(nextScale, cx, cy) {
  const clamped = Math.max(LB_MIN, Math.min(LB_MAX, nextScale));
  const ratio = clamped / lbScale;
  lbOx = cx - ratio * (cx - lbOx);
  lbOy = cy - ratio * (cy - lbOy);
  lbScale = clamped;
  lbApply();
}

function openLightbox(p, src) {
  const codes = (p.barcodes || []).map((b) => b.text).filter(Boolean);
  $('lbCap').innerHTML = `<b>${esc(p.name || p.id)}</b>`
    + `<span class="d">${esc(p.dir || '')}`
    + (codes.length ? ` &middot; Barcode: ${esc(codes.join(', '))}` : '') + `</span>`;

  lbURL = src;
  lbName = p.name || p.id || '';
  lbImg.src = src;
  lbImg.onload = () => {
    lbFit();
    // Kalau gambar sangat kecil, zoom ke 1:1 supaya tidak minuscule.
    const box = lbStage.getBoundingClientRect();
    if (lbImg.naturalWidth < box.width * 0.6 || lbImg.naturalHeight < box.height * 0.6) {
      lbScale = 1; lbFitScale = lbScale;
      lbOx = (box.width - lbImg.naturalWidth) / 2;
      lbOy = (box.height - lbImg.naturalHeight) / 2;
      lbApply();
    }
  };
  lb.classList.add('show');
  lbStage.classList.remove('drag');
}

function closeLightbox() {
  lb.classList.remove('show');
  lbImg.removeAttribute('src');
  lbURL = '';
}

$('lbX').addEventListener('click', closeLightbox);

lbStage.addEventListener('wheel', (e) => {
  e.preventDefault();
  const b = lbStage.getBoundingClientRect();
  lbZoomAt(lbScale * (e.deltaY < 0 ? 1.15 : 1 / 1.15), e.clientX - b.left, e.clientY - b.top);
}, { passive: false });

// Drag untuk menggeser. Pointer event supaya jalan di mouse, touch, dan pen.
lbStage.addEventListener('pointerdown', (e) => {
  lbDrag = true; lbLastX = e.clientX; lbLastY = e.clientY;
  lbStage.classList.add('drag');
  lbStage.setPointerCapture(e.pointerId);
  e.preventDefault();
});
lbStage.addEventListener('pointermove', (e) => {
  if (!lbDrag) return;
  lbOx += e.clientX - lbLastX;
  lbOy += e.clientY - lbLastY;
  lbLastX = e.clientX; lbLastY = e.clientY;
  lbApply();
});
function lbEndDrag(e) {
  if (!lbDrag) return;
  lbDrag = false;
  lbStage.classList.remove('drag');
  try { lbStage.releasePointerCapture(e.pointerId); } catch (_) {}
}
lbStage.addEventListener('pointerup', lbEndDrag);
lbStage.addEventListener('pointercancel', lbEndDrag);

// Double click / double tap = zoom 1:2 dari titik klik.
lbStage.addEventListener('dblclick', (e) => {
  const b = lbStage.getBoundingClientRect();
  lbZoomAt(lbScale > lbFitScale * 1.6 ? lbFitScale : lbScale * 2, e.clientX - b.left, e.clientY - b.top);
});

$('lbIn').addEventListener('click', () => {
  const b = lbStage.getBoundingClientRect();
  lbZoomAt(lbScale * 1.3, b.width / 2, b.height / 2);
});
$('lbOut').addEventListener('click', () => {
  const b = lbStage.getBoundingClientRect();
  lbZoomAt(lbScale / 1.3, b.width / 2, b.height / 2);
});
$('lbFit').addEventListener('click', () => lbFit());

// Download foto ukuran asli. Minta server kirim Content-Disposition
// attachment supaya browser simpan ke disk, bukan membuka tab baru.
$('lbDl').addEventListener('click', () => {
  if (!lbURL) return;
  const btn = $('lbDl');
  btn.disabled = true;
  btn.textContent = '…';
  const sep = lbURL.indexOf('?') >= 0 ? '&' : '?';
  const a = document.createElement('a');
  a.href = lbURL + sep + 'download=1&name=' + encodeURIComponent(lbName);
  a.download = lbName || '';
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => {
    btn.disabled = false;
    btn.innerHTML = '&#8595;';
  }, 1200);
});

// Keyboard: +/-/0/1, pan dengan panah, tutup dengan Esc.
document.addEventListener('keydown', (e) => {
  if (!lb.classList.contains('show')) return;
  const b = lbStage.getBoundingClientRect();
  const step = e.shiftKey ? 120 : 40;
  if (e.key === 'Escape') { closeLightbox(); return; }
  if (e.key === '+' || e.key === '=') { lbZoomAt(lbScale * 1.3, b.width / 2, b.height / 2); e.preventDefault(); return; }
  if (e.key === '-' || e.key === '_') { lbZoomAt(lbScale / 1.3, b.width / 2, b.height / 2); e.preventDefault(); return; }
  if (e.key === '0') { lbFit(); return; }
  if (e.key === '1') { lbZoomAt(1, b.width / 2, b.height / 2); return; }
  const move = { ArrowLeft: [-step, 0], ArrowRight: [step, 0], ArrowUp: [0, -step], ArrowDown: [0, step] }[e.key];
  if (move) { lbOx += move[0]; lbOy += move[1]; lbApply(); e.preventDefault(); }
});

window.addEventListener('resize', () => { if (lb.classList.contains('show')) lbFit(); });
$('ctfBtn').addEventListener('click', ctfSearch);
$('ctfQ').addEventListener('keydown', (e) => { if (e.key === 'Enter') ctfSearch(); });
$('ctfQ').addEventListener('input', debounce(ctfSearch, 450));
$('ctfPhotos').innerHTML = `<div class="empty"><b>Mulai dengan mengetik</b>Masukkan kata kunci di atas untuk mencari foto.</div>`;

// ============================================================
// MC - master item
// ============================================================
let mcPage = 1;

async function loadMCDates() {
  try {
    const d = await api('/api/user/compares');
    const sel = $('mcDate');
    sel.innerHTML = '<option value="">Semua hasil compare</option>';
    for (const m of d.compares || []) {
      const o = document.createElement('option');
      o.value = m.id;
      o.textContent = m.plUploaded + ' - ' + (m.plFilename || m.filename || '');
      sel.appendChild(o);
    }
  } catch (e) { /* filter opsional */ }
}

function fillSelect(sel, label, values) {
  const cur = sel.value;
  sel.innerHTML = '<option value="">' + label + '</option>';
  for (const v of values || []) {
    const o = document.createElement('option');
    o.value = v;
    o.textContent = v;
    sel.appendChild(o);
  }
  if (values && values.indexOf(cur) >= 0) sel.value = cur;
}

function flagBadge(flag) {
  const isNew = flag === 'NEW ITEM';
  const color = isNew ? 'var(--yellow)' : 'var(--green)';
  return `<span class="chip" style="color:${color};border-color:${color}">${esc(flag || '-')}</span>`;
}

async function mcSearch(page) {
  mcPage = page || 1;
  const q = $('mcQ').value.trim();
  const st = $('mcStatus').value;
  const im = $('mcImp').value;
  const sh = $('mcSheet').value;
  const cmp = $('mcDate').value;
  const body = $('mcBody');
  $('mcMeta').textContent = 'Memuat...';
  body.innerHTML = '';
  try {
    const d = await api(`/api/user/master-items?page=${mcPage}&limit=60`
      + (q ? '&q=' + encodeURIComponent(q) : '')
      + (st ? '&status=' + encodeURIComponent(st) : '')
      + (im ? '&importir=' + encodeURIComponent(im) : '')
      + (sh ? '&sheet=' + encodeURIComponent(sh) : '')
      + (cmp ? '&compareId=' + encodeURIComponent(cmp) : ''));
    const items = d.items || [];

    fillSelect($('mcImp'), 'Semua importir', d.importirs);
    fillSelect($('mcSheet'), 'Semua sheet', d.sheets);

    $('mcMeta').innerHTML =
      `<span class="chip">${(d.total || 0).toLocaleString('id-ID')} item</span>`
      + `<span class="chip" style="color:var(--green);border-color:var(--green)">EXISTING ${(d.existing || 0).toLocaleString('id-ID')}</span>`
      + `<span class="chip" style="color:var(--yellow);border-color:var(--yellow)">NEW ITEM ${(d.newItem || 0).toLocaleString('id-ID')}</span>`
      + (q ? `<span>cari: &quot;${esc(q)}&quot;</span>` : '');

    if (!items.length) {
      body.innerHTML = `<tr><td colspan="8"><div class="empty"><b>Tidak ditemukan</b>Tidak ada item yang cocok.</div></td></tr>`;
      $('mcPager').innerHTML = '';
      return;
    }
    for (const m of items) {
      const tr = el('tr');
      tr.innerHTML = `<td class="mono">${esc(m.noContainer)}</td>
        <td>${esc(m.brand)}</td><td>${esc(m.namaItem)}</td>
        <td class="mono">${esc(m.eta || '-')}</td>
        <td>${esc(m.importir || '-')}</td>
        <td>${flagBadge(m.flag)}</td>
        <td class="mono">${esc(m.masterCode || '-')}</td>
        <td>${esc(m.masterDesc || '-')}</td>`;
      body.appendChild(tr);
    }
    renderPager('mcPager', d.total, d.limit, mcPage, mcSearch);
  } catch (e) {
    $('mcMeta').textContent = '';
    body.innerHTML = `<tr><td colspan="8"><div class="empty"><b>Gagal</b>${esc(e.message)}</div></td></tr>`;
  }
}

$('mcBtn').addEventListener('click', () => mcSearch(1));
$('mcQ').addEventListener('keydown', (e) => { if (e.key === 'Enter') mcSearch(1); });
$('mcQ').addEventListener('input', debounce(() => mcSearch(1), 450));
$('mcDate').addEventListener('change', () => mcSearch(1));
$('mcStatus').addEventListener('change', () => mcSearch(1));
$('mcImp').addEventListener('change', () => mcSearch(1));
$('mcSheet').addEventListener('change', () => mcSearch(1));
$('mcDl').addEventListener('click', () => {
  const cmp = $('mcDate').value;
  location.href = '/api/user/export-ml' + (cmp ? '?compareId=' + encodeURIComponent(cmp) : '');
});

// ============================================================
// MMD - master descriptor
// ============================================================
let mmdPage = 1;

async function loadMMDMeta() {
  try {
    const d = await api('/api/user/mmd-meta');
    const sel = $('mmdKat');
    for (const k of d.kategori || []) {
      const o = document.createElement('option');
      o.value = k; o.textContent = k;
      sel.appendChild(o);
    }
    const bits = [];
    if (d.total != null) bits.push(`<span class="chip">${d.total.toLocaleString('id-ID')} total</span>`);
    if (d.aktif != null) bits.push(`<span class="chip grn">${d.aktif.toLocaleString('id-ID')} aktif</span>`);
    $('mmdNote').innerHTML = 'Status dihitung dari kolom Non Aktif pada file MMD.'
      + (bits.length ? ' ' + bits.join(' ') : '');
  } catch (e) { /* meta opsional */ }
}

async function mmdSearch(page) {
  mmdPage = page || 1;
  const q = $('mmdQ').value.trim();
  const kat = $('mmdKat').value;
  const body = $('mmdBody');
  $('mmdMeta').textContent = 'Memuat...';
  body.innerHTML = '';
  try {
    const d = await api(`/api/user/mmd-items?page=${mmdPage}&limit=60`
      + (q ? '&q=' + encodeURIComponent(q) : '')
      + (kat ? '&kategori=' + encodeURIComponent(kat) : ''));
    const items = d.items || [];
    $('mmdMeta').innerHTML = `<span class="chip">${(d.total || 0).toLocaleString('id-ID')} item</span>`
      + (q ? `<span>cari: &quot;${esc(q)}&quot;</span>` : '')
      + (kat ? `<span class="chip acc">${esc(kat)}</span>` : '');
    if (!items.length) {
      body.innerHTML = `<tr><td colspan="6"><div class="empty"><b>Belum ada data</b>Data master descriptor belum dimuat. Admin perlu menyimpan MMD terlebih dulu.</div></td></tr>`;
      $('mmdPager').innerHTML = '';
      return;
    }
    for (const m of items) {
      const tr = el('tr');
      const aktif = m.aktif === 1;
      // Badge revisi: penanda kalau deskripsi pernah diganti file baru.
      // Klik untuk lihat riwayatnya (deskripsi lama -> baru).
      const rev = (m.revisi > 0)
        ? `<span class="rev-badge" role="button" tabindex="0" data-kode="${esc(m.itemCode)}"`
          + ` title="Deskripsi pernah direvisi — klik untuk lihat riwayatnya">revisi`
          + (m.revisi > 1 ? ` &times;${m.revisi}` : '') + `</span>`
        : '';
      tr.innerHTML = `<td class="mono">${esc(m.itemCode)}</td>
        <td>${esc(m.deskripsi)}${rev}</td>
        <td>${m.kategori ? `<span class="chip acc">${esc(m.kategori)}</span>` : '-'}</td>
        <td>${aktif ? '<span class="chip grn">AKTIF</span>' : '<span class="chip red">TIDAK AKTIF</span>'}</td>
        <td>${esc(m.potonganUtama) || '-'}</td>
        <td>${esc(m.potonganSpesifik) || '-'}</td>`;
      body.appendChild(tr);
    }
    renderPager('mmdPager', d.total, d.limit, mmdPage, mmdSearch);
  } catch (e) {
    $('mmdMeta').textContent = '';
    body.innerHTML = `<tr><td colspan="7"><div class="empty"><b>Gagal</b>${esc(e.message)}</div></td></tr>`;
  }
}

$('mmdBtn').addEventListener('click', () => mmdSearch(1));
$('mmdQ').addEventListener('keydown', (e) => { if (e.key === 'Enter') mmdSearch(1); });
$('mmdQ').addEventListener('input', debounce(() => mmdSearch(1), 450));
$('mmdKat').addEventListener('change', () => mmdSearch(1));

// Klik badge "revisi" -> buka riwayat perubahan deskripsi item itu.
$('mmdBody').addEventListener('click', (e) => {
  const badge = e.target.closest('.rev-badge');
  if (badge) showRevisi(badge.dataset.kode);
});
$('mmdBody').addEventListener('keydown', (e) => {
  if (e.key !== 'Enter' && e.key !== ' ') return;
  const badge = e.target.closest('.rev-badge');
  if (badge) { e.preventDefault(); showRevisi(badge.dataset.kode); }
});

async function showRevisi(kode) {
  $('revKode').textContent = kode;
  $('revList').innerHTML = 'Memuat riwayat...';
  $('revModal').classList.add('show');
  try {
    const d = await api('/api/user/mmd-revisi?kode=' + encodeURIComponent(kode));
    const items = d.items || [];
    if (!items.length) {
      $('revList').innerHTML = '<div class="hint">Belum ada riwayat revisi untuk item ini.</div>';
      return;
    }
    $('revList').innerHTML = items.map((x) => `<div class="rev-item">
      <div class="rev-time">${esc(x.tanggal)}${x.upload ? ' &middot; ' + esc(x.upload) : ''}</div>
      <div class="rev-old">${esc(x.lama) || '-'}</div>
      <div class="rev-arrow">&rarr;</div>
      <div class="rev-new">${esc(x.baru) || '-'}</div>
    </div>`).join('');
  } catch (err) {
    $('revList').innerHTML = `<div class="hint">${esc(err.message)}</div>`;
  }
}
$('revX').addEventListener('click', () => $('revModal').classList.remove('show'));
$('revModal').addEventListener('click', (e) => {
  if (e.target === $('revModal')) $('revModal').classList.remove('show');
});
// Download Masterlist dihapus dari halaman user: data MMD read-only di
// portal, tidak ada endpoint export publik untuk itu.

// ============================================================
// Paginator
// ============================================================
function renderPager(containerId, total, limit, page, onGo) {
  const box = $(containerId);
  box.innerHTML = '';
  if (!total || total <= limit) return;
  const pages = Math.ceil(total / limit);
  const prev = el('button', 'btn', 'Sebelumnya');
  prev.disabled = page <= 1;
  prev.addEventListener('click', () => onGo(page - 1));
  const next = el('button', 'btn', 'Berikutnya');
  next.disabled = page >= pages;
  next.addEventListener('click', () => onGo(page + 1));
  box.appendChild(prev);
  box.appendChild(el('span', '', `Halaman ${page} / ${pages} dari ${total.toLocaleString('id-ID')} item`));
  box.appendChild(next);
}

// ============================================================
// Init
// ============================================================
loadSummary();
loadMCDates();
loadMMDMeta();
mcSearch(1);
mmdSearch(1);
