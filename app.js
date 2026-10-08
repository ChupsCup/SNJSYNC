const $ = (id) => document.getElementById(id);
const searchInput = $('search'), btn = $('btn'), statusEl = $('status');
const gridEl = $('grid'), resCount = $('resCount'), chipsEl = $('chips'), gdot = $('gdot');
const panelEl = $('statusPanel');

let lastData = null, terms = [], folderFilter = null, visibleCount = 10;
let modalArr = [], modalIdx = -1, toastTimer = null, searchSeq = 0;
let gdriveOk = false;
let readyData = null;      
let driveData = null;      
let panelShown = false;

function fmtWaktu(ms) {
  if (!ms) return 'belum pernah';
  const d = new Date(ms);
  if (isNaN(d.getTime())) return 'tidak diketahui';
  return d.toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' });
}
function fmtNum(n) { return (+n || 0).toLocaleString('id-ID'); }
function shortId(id) {
  if (!id) return '-';
  const s = String(id);
  return s.length <= 12 ? s : s.slice(0, 6) + '…' + s.slice(-4);
}

function esc(s) { const d = document.createElement('div'); d.textContent = s; return d.innerHTML; }
function photoUrl(id) {
  
  
  return '/api/photo?id=' + encodeURIComponent(id) + '&v=20261007';
}
function fmtSize(n) {
  n = +n || 0;
  if (n > 1048576) return (n / 1048576).toFixed(1) + ' MB';
  if (n > 1024) return Math.round(n / 1024) + ' KB';
  return n + ' B';
}
function setStatus(t, cls) { statusEl.textContent = t || ''; statusEl.className = cls || ''; }
function showAuthBanner() { $('authBanner').style.display = 'flex'; }
function hideAuthBanner() { $('authBanner').style.display = 'none'; }
function toast(msg, cls) {
  const t = $('toast');
  t.textContent = msg; t.className = 'show' + (cls ? ' ' + cls : '');
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { t.className = ''; }, 4000);
}
function highlight(text) {
  let out = esc(text || '');
  if (terms.length) {
    out = out.replace(/<mark>/g, '').replace(/<\/mark>/g, '');
    for (const t of terms) out = out.replace(new RegExp('(' + t.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + ')', 'gi'), '<mark>$1</mark>');
  }
  return out;
}
function snippet(text) {
  text = text || '';
  if (!terms.length) return text.slice(0, 300);
  const lower = text.toLowerCase();
  let best = -1;
  for (const t of terms) { const i = lower.indexOf(t.toLowerCase()); if (i >= 0 && (best < 0 || i < best)) best = i; }
  if (best < 0) return text.slice(0, 300);
  const start = Math.max(0, best - 60);
  return (start > 0 ? '...' : '') + text.slice(start, start + 300) + (text.length > start + 300 ? '...' : '');
}

let statsAt = 0;



async function loadStats(force) {
  if (!force && readyData && Date.now() - statsAt < 15000) return readyData;
  try {
    const st = await (await fetch('/api/stats')).json();
    readyData = st;
    statsAt = Date.now();
  } catch (_) {  }
  return readyData;
}

function renderChips(dirs) {
  chipsEl.innerHTML = '';
  const all = document.createElement('span');
  all.className = 'chip' + (folderFilter === null ? ' active' : '');
  all.textContent = 'Semua folder';
  all.onclick = () => { folderFilter = null; visibleCount = 10; renderChips(dirs); render(); };
  chipsEl.appendChild(all);
  (dirs || []).forEach((d) => {
    const leaf = d.split(/[\\/]/).filter(Boolean).pop() || d;
    const c = document.createElement('span');
    c.className = 'chip' + (folderFilter === d ? ' active' : '');
    c.textContent = leaf;
    c.title = d;
    c.onclick = () => { folderFilter = (folderFilter === d ? null : d); visibleCount = 10; renderChips(dirs); render(); };
    chipsEl.appendChild(c);
  });
}

function makeCard(r, arr) {
  const card = document.createElement('div');
  card.className = 'card';
  const bcLen = (r.barcodes || []).length;
  card.innerHTML =
    '<div class="thumbbox">' +
    '<div class="fbadge">' + esc((r.dir.split(/[\\/]/).filter(Boolean).pop() || '').slice(0, 26)) + '</div>' +
    (bcLen ? '<div class="bcbadge">Barcode ' + bcLen + '</div>' : '') +
    '<img class="thumb" loading="lazy" src="' + photoUrl(r.id) + '">' +
    '</div>' +
    '<div class="meta">' +
    '<div class="fname">' + esc(r.name) + '</div>' +
    '<div class="snippet">' + highlight(snippet(r.text)) + '</div>' +
    '<div class="foot"><span>' + (bcLen ? 'Barcode ' + bcLen : 'No barcode') + '</span><span class="size">' + fmtSize(r.size) + '</span></div>' +
    '</div>';
  card.onclick = () => openModal(r, arr);
  return card;
}
function hidePanel() {
  panelShown = false;
  panelEl.innerHTML = '';
  panelEl.style.display = 'none';
  gridEl.style.display = '';
  chipsEl.style.display = 'none';
}



function renderReady() {
  const st = readyData;
  if (!st) { hidePanel(); return; }
  const dv = driveData || {};
  const total = st.total || 0;
  const ocr = st.withOcr || 0;
  const ratio = total ? Math.round((ocr / total) * 100) : 0;

  let driveCls = 'good', driveTitle = 'Google Drive terhubung', driveBody = '';
  if (!dv.configured) {
    driveCls = 'bad';
    driveTitle = 'client_secret.json belum ada';
    driveBody = 'Unggah client_secret.json lewat menu ekspor, lalu hubungkan Drive.';
  } else if (!dv.authorized) {
    driveCls = 'bad';
    driveTitle = 'Google Drive belum terhubung';
    driveBody = 'Token Google kedaluwarsa. Klik tombol di bawah untuk login ulang.';
  } else {
    driveBody = 'Terhubung sebagai <b>' + esc(dv.email || 'akun Google') + '</b>. ' +
      'Terakhir sinkron daftar foto: <b>' + fmtWaktu(st.photosSavedAt) + '</b>.';
  }

  let ocrCls = 'good';
  let ocrNote = '';
  if (!total) {
    ocrCls = 'bad';
    ocrNote = 'Belum ada foto sama sekali. Pilih folder foto lalu sinkronkan Google Drive.';
  } else if (ratio < 80) {
    ocrCls = 'warn';
    ocrNote = 'Teks OCR menambah <b>kata kunci baru</b> di ' + fmtNum(ocr) +
      ' foto (label, komposisi, nomor barcode). Foto yang belum di-OCR <b>tidak</b> hilang dari pencarian &mdash; ' +
      'nama file dan nama foldernya tetap terbaca. Jadi <b>semua ' + fmtNum(total) +
      ' foto</b> tetap bisa dicari; OCR cuma membuat sebagian foto bisa dicari juga dari isi fotonya.';
  } else {
    ocrNote = 'Semua foto punya teks OCR. Pencarian bisa memakai nama file, folder, isi label, dan barcode.';
  }

  const cards = [];

  cards.push(
    '<div class="rd ' + driveCls + '">' +
      '<div class="rd-h"><span class="dot"></span>' + driveTitle + '</div>' +
      '<div class="rd-note">' + driveBody +
        (driveCls === 'bad' ? ' <button class="ghost" id="rdConnect">Hubungkan sekarang</button>' : '') +
      '</div>' +
    '</div>'
  );

  // Daftar akar folder bisa lebih dari satu, jadi ditampilkan per baris
  // beserta ID-nya — bukan disatukan jadi satu teks yang tidak jelas.
  const roots = (st.photosFolders && st.photosFolders.length)
    ? st.photosFolders
    : (st.photosFolderId ? [{ id: st.photosFolderId, name: st.photosFolderName }] : []);
  const rootRows = roots.length
    ? roots.map((f, i) =>
        '<dt>' + (i === 0 ? 'Folder foto' : '&nbsp;') + '</dt><dd>' + esc(f.name || '(tanpa nama)') +
        ' <span class="mono">' + shortId(f.id) + '</span></dd>'
      ).join('')
    : '<dt>Folder foto</dt><dd>(belum dipilih)</dd>';

  cards.push(
    '<div class="rd">' +
      '<div class="rd-h"><span class="dot" style="background:var(--accent)"></span>Sumber data</div>' +
      '<dl class="rd-kv">' +
        rootRows +
        '<dt>Folder index.json</dt><dd>' + esc(st.dataFolderName || '-') +
          ' <span class="mono">' + shortId(st.dataFolderId) + '</span></dd>' +
        '<dt>Total foto</dt><dd>' + fmtNum(total) + '</dd>' +
        '<dt>Sinonim alias</dt><dd>' + fmtNum(st.aliasCount) + '</dd>' +
      '</dl>' +
      (st.photosFolderSet
        ? (roots.length > 1
            ? '<div class="rd-note"><b>' + roots.length + ' induk folder</b> dipakai sebagai sumber pencarian.</div>'
            : '')
        : '<div class="rd-note"><b>Folder foto belum dipilih.</b> Buka menu Ekspor untuk memilihnya.</div>') +
    '</div>'
  );

  cards.push(
    '<div class="rd ' + ocrCls + '">' +
      '<div class="rd-h"><span class="dot"></span>Cakupan pencarian</div>' +
      '<dl class="rd-kv">' +
        '<dt>Cari via nama file &amp; folder</dt><dd><b style="color:var(--green)">100% &mdash; ' + fmtNum(total) + ' foto</b></dd>' +
        '<dt>Cari via isi teks (OCR)</dt><dd>' + fmtNum(ocr) + ' foto (' + ratio + '%)</dd>' +
        '<dt>Di antaranya punya barcode</dt><dd>' + fmtNum(st.withBarcodes) + '</dd>' +
        '<dt>Jumlah alias/sinonim</dt><dd>' + fmtNum(st.aliasCount) + '</dd>' +
      '</dl>' +
      '<div class="rd-note">' + ocrNote + '</div>' +
    '</div>'
  );

  if (st.dirs && st.dirs.length) {
    const top = st.dirs.slice(0, 8);
    cards.push(
      '<div class="rd">' +
        '<div class="rd-h"><span class="dot" style="background:var(--text2)"></span>Folder teratas &mdash; klik untuk mencari</div>' +
        '<div class="chips" style="margin:0">' + top.map((d) =>
          '<span class="chip" data-dir="' + esc(d.dir) + '">' +
          esc(d.dir.split('/')[0]) + ' <span style="opacity:.65">' + fmtNum(d.count) + '</span></span>'
        ).join('') + '</div>' +
      '</div>'
    );
  }

  panelEl.innerHTML = cards.join('');
  panelEl.style.display = 'grid';
  panelShown = true;
  gridEl.style.display = 'none';
  chipsEl.style.display = 'none';
  resCount.innerHTML = '';
  setStatus('Ketik di kotak pencarian untuk mulai mencari foto.');

  const stamp = document.createElement('div');
  stamp.style.cssText = 'font-size:10.5px;color:var(--text2);opacity:.6;text-align:center;padding:4px 0 2px';
  stamp.textContent = 'build 20261003a · panel status v2';
  panelEl.appendChild(stamp);

  const c = $('rdConnect');
  if (c) c.onclick = () => { const e = $('mExport'); if (e) e.click(); };
  panelEl.querySelectorAll('.chip[data-dir]').forEach((ch) => {
    ch.onclick = () => {
      searchInput.value = ch.dataset.dir;
      doSearch();
    };
  });
}

function render() {
  if (!lastData) { gridEl.innerHTML = ''; return; }
  const inFilter = (r) => !folderFilter || r.dir === folderFilter;
  const exact = (lastData.results || []).filter(inFilter);
  const partial = (lastData.partial || []).filter(inFilter);
  if (!exact.length && !partial.length) {
    if (readyData && !readyData.total) {
      resCount.innerHTML = '';
      gridEl.innerHTML = '<div class="empty"><b>Belum ada foto.</b><br>Sinkronkan Google Drive dulu lewat menu Ekspor.</div>';
      return;
    }
    resCount.innerHTML = 'Tidak ada hasil.';
    gridEl.innerHTML = '<div class="empty"><b>Tidak ada foto yang cocok.</b></div>';
    return;
  }
  // Kalau tidak ada yang cocok semua kata, tampilkan foto terdekat (partial)
  // dengan label jelas. Dulu panel admin tidak pernah me-render partial sama
  // sekali, jadi hasilnya terlihat kosong padahal server menemukan foto.
  const showPartial = !exact.length;
  const arr = showPartial ? partial : exact;
  if (showPartial) {
    const miss = partial.length ? Math.max(0, partial[0].total - partial[0].matched) : 0;
    resCount.innerHTML = '<b>0</b> hasil pasti &mdash; <b>' + arr.length.toLocaleString('id-ID') +
      '</b> foto terdekat (kurang ' + miss + ' kata)' +
      (lastData.terms && lastData.terms.length ? ' untuk: ' + lastData.terms.join(' ') : '');
    setStatus('Hasil pasti tidak ada. Foto di bawah hanya cocok sebagian kata.', 'error');
  } else {
    resCount.innerHTML = '<b>' + arr.length.toLocaleString('id-ID') + '</b> foto' +
      (lastData.terms && lastData.terms.length ? ' untuk: ' + lastData.terms.join(' ') : '') +
      (lastData.codes && lastData.codes.length ? ' <span style="color:var(--green)">(negara: ' + lastData.codes.map((c) => c.code.toUpperCase() + '=' + c.label).join(', ') + ')</span>' : '');
    setStatus('');
  }
  gridEl.innerHTML = '';
  for (let i = 0; i < Math.min(arr.length, visibleCount); i++) gridEl.appendChild(makeCard(arr[i], arr));
  if (arr.length > visibleCount) {
    const step = 60;
    const more = document.createElement('div');
    more.className = 'loadmore';
    more.innerHTML = '<button class="ghost">Muat ' + Math.min(step, arr.length - visibleCount) +
      ' lagi &mdash; sisa ' + (arr.length - visibleCount).toLocaleString('id-ID') + '</button>';
    more.querySelector('button').onclick = () => { visibleCount += step; render(); };
    gridEl.appendChild(more);
  }
}

async function doSearch(silent) {
  const q = searchInput.value.trim();
  if (!q) {
    // Tanpa query: tampilkan panel kesiapan data, jangan panggil API search.
    lastData = null;
    renderReady();
    return;
  }
  if (!silent) gridEl.innerHTML = '<div class="empty">Mencari...</div>';
  const seq = ++searchSeq;
  try {
    hidePanel();
    const data = await (await fetch('/api/search?q=' + encodeURIComponent(q))).json();
    if (seq !== searchSeq) return;
    lastData = data;
    terms = (data.groups || []).flatMap((g) => g.any);
    folderFilter = null;
    visibleCount = silent ? 10 : 60;
    // /api/stats mesti dihitung dari puluhan ribu foto + ribuan folder.
    // Dulu dipanggil SETIAP pencarian (termasuk tiap ketikan setelah debounce),
    // sekarang hasilnya dipakai ulang selama masih segar.
    const st = await loadStats();
    renderChips(st ? (st.dirs || []).map((d) => d.dir) : []);
    render();
  } catch (err) {
    gridEl.innerHTML = '<div class="empty"><b>Error:</b> ' + esc(err.message) + '</div>';
  }
}
btn.onclick = () => doSearch();
searchInput.addEventListener('keydown', (e) => { if (e.key === 'Enter') doSearch(); });
let debounceT = null;
searchInput.addEventListener('input', () => {
  clearTimeout(debounceT);
  debounceT = setTimeout(() => { if (searchInput.value.trim()) doSearch(); }, 400);
});

function openModal(r, arr) {
  modalArr = arr; modalIdx = arr.indexOf(r);
  $('mName').textContent = r.name;
  $('mImg').src = photoUrl(r.id);
  $('mText').innerHTML = highlight(r.text) || '<div class="nocontent">Tidak ada teks terdeteksi.</div>';
  $('pane-bc').innerHTML = (r.barcodes || []).length
    ? r.barcodes.map((b) => '<div class="bcline"><span class="bc-format">' + esc(b.format) + '</span>' + highlight(b.text) + '</div>').join('')
    : '<div class="nocontent">Tidak ada barcode.</div>';
  $('pane-info').innerHTML =
    '<div class="infocell"><div class="il">Nama</div><div>' + esc(r.name) + '</div></div>' +
    '<div class="infocell"><div class="il">Ukuran</div><div>' + fmtSize(r.size) + '</div></div>' +
    '<div class="infocell"><div class="il">Folder</div><div>' + esc(r.dir) + '</div></div>' +
    '<div class="infocell"><div class="il">Path</div><div>' + esc(r.path) + '</div></div>';
  $('modal').style.display = 'flex';
}
document.querySelectorAll('.tab').forEach((t) => t.addEventListener('click', () => {
  document.querySelectorAll('.tab').forEach((x) => x.classList.toggle('active', x === t));
  document.querySelectorAll('.tab-pane').forEach((x) => x.classList.toggle('active', x.id === 'pane-' + t.dataset.pane));
}));
$('mClose').onclick = () => $('modal').style.display = 'none';
$('modal').onclick = (e) => { if (e.target === $('modal')) $('modal').style.display = 'none'; };
$('mExport').onclick = () => {
  const r = modalArr[modalIdx];
  if (!r) return;
  const q = searchInput.value.trim();
  const safe = (q || r.name || 'foto').replace(/[\/\\:*?"<>|#]/g, '_').replace(/\s+/g, ' ').trim().slice(0, 80);
  const fname = safe + '.jpg';
  openExport([{ id: r.id, name: fname, path: r.path }]);
  toast('Export sebagai: ' + fname, 'ok');
};
$('mViewPhoto').onclick = () => {
  const r = modalArr[modalIdx];
  if (!r) return;
  const img = $('fsImg');
  img.src = photoUrl(r.id);
  $('fsOverlay').style.display = 'flex';
  if (img.complete && img.naturalWidth) { fsCenter(); }
};
$('fsBack').onclick = () => $('fsOverlay').style.display = 'none';

let fsScale = 1, fsOx = 0, fsOy = 0, fsDrag = false, fsLastX = 0, fsLastY = 0;

function fsCenter() {
  const ov = $('fsOverlay');
  const img = $('fsImg');
  const nw = img.naturalWidth, nh = img.naturalHeight;
  if (!nw || !nh) return;
  fsScale = Math.min(ov.clientWidth * 0.95 / nw, (ov.clientHeight - 40) * 0.9 / nh, 1);
  fsOx = (ov.clientWidth - nw * fsScale) / 2;
  fsOy = ((ov.clientHeight - 40) - nh * fsScale) / 2;
  fsApply();
}

function fsApply() {
  $('fsImg').style.transform = 'translate(' + fsOx + 'px,' + fsOy + 'px) scale(' + fsScale + ')';
}

$('fsImg').addEventListener('load', () => { if ($('fsOverlay').style.display === 'flex') fsCenter(); });

$('fsOverlay').addEventListener('wheel', (e) => {
  e.preventDefault();
  const old = fsScale;
  fsScale *= e.deltaY < 0 ? 1.15 : 1 / 1.15;
  fsScale = Math.max(0.05, Math.min(15, fsScale));
  const r = fsScale / old;
  fsOx = e.clientX - r * (e.clientX - fsOx);
  fsOy = e.clientY - r * (e.clientY - fsOy);
  fsApply();
}, { passive: false });

$('fsImg').addEventListener('mousedown', (e) => {
  fsDrag = true; fsLastX = e.clientX; fsLastY = e.clientY;
  e.preventDefault();
});
document.addEventListener('mousemove', (e) => {
  if (!fsDrag) return;
  fsOx += e.clientX - fsLastX;
  fsOy += e.clientY - fsLastY;
  fsLastX = e.clientX; fsLastY = e.clientY;
  fsApply();
});
document.addEventListener('mouseup', () => { fsDrag = false; });


/* ===== gdrive / exp modal ===== */
const expModal = $('expModal'), expStatus = $('expStatus');
const expFolder = $('expFolder'), expRun = $('expRun'), expResult = $('expResult');
let authPoller = null, pendingExportIds = null;
function showStep(n) {
  $('expStep1').style.display = n === 1 ? 'block' : 'none';
  $('expStep2').style.display = n === 2 ? 'block' : 'none';
  $('expStep3').style.display = n === 3 ? 'block' : 'none';
}
async function loadFolders() {
  expFolder.innerHTML = '<option>Memuat...</option>';
  try {
    const d = await (await fetch('/api/gdrive/folders?q=' + encodeURIComponent($('expFolderSearch').value.trim()))).json();
    if (d.error) throw new Error(d.error);
    if (pickMode) { renderPickList(d.files || []); return; }
    expFolder.innerHTML = '<option value="">Pilih folder...</option>' + d.files.map((f) => '<option value="' + esc(f.id) + '">' + esc(f.name) + '</option>').join('');
  } catch (err) { expStatus.textContent = 'Error: ' + err.message; }
}

/* ============================================================
   Pilih folder foto — mode MULTI (bisa banyak induk folder).

   Pilihan user disimpan sebagai daftar kerja `picked` ({id,name}).
   Yang tampil digabung dari dua sumber:
     1. folder yang sudah dipilih  -> selalu di atas, walau tidak ada di
        hasil pencarian (supaya tidak "hilang" begitu pencarian diketik)
     2. hasil pencarian folder     -> untuk nambah yang baru
   Jadi menambah dan menghapus sama-sama jalan tanpa kehilangan pilihan lama.
   ============================================================ */
let pickMode = false;
let picked = [];   // pilihan kerja (belum tentu tersimpan)

function renderPickList(files) {
  const box = $('pickList');
  const chosen = picked.slice();
  const chosenIds = new Set(chosen.map((f) => f.id));
  // hasil pencarian, buang yang sudah ke-centang
  const sisa = files.filter((f) => f.id && !chosenIds.has(f.id));
  const rows = [];
  chosen.forEach((f, i) => rows.push(pickRow(f, true, i)));
  sisa.forEach((f, i) => rows.push(pickRow(f, false, chosen.length + i)));
  box.innerHTML = rows.length
    ? rows.join('')
    : '<div class="pick-empty">Tidak ada folder yang cocok. Coba kata kunci lain.</div>';
  updatePickCount();
}

function pickRow(f, tercentang, i) {
  return '<label class="pick-item">' +
    '<input type="checkbox" data-id="' + esc(f.id) + '" data-i="' + i + '"' + (tercentang ? ' checked' : '') + '>' +
    '<span class="pn" title="' + esc(f.name) + '">' + esc(f.name || '(tanpa nama)') + '</span>' +
    (tercentang ? '<span class="badge">terpilih</span>' : '') +
    '<span class="nid">' + shortId(f.id) + '</span>' +
    '</label>';
}

function readPickBoxes() {
  // centang yang ada sekarang dipakai sebagai pilihan kerja terbaru
  const boxes = Array.from($('pickList').querySelectorAll('input[type=checkbox]'));
  if (!boxes.length) return;
  picked = boxes
    .filter((b) => b.checked)
    .map((b) => {
      const nama = b.closest('.pick-item').querySelector('.pn').textContent;
      return { id: b.dataset.id, name: nama };
    });
}

function updatePickCount() {
  const n = picked.length;
  $('pickCount').innerHTML = n
    ? '<b>' + n + '</b> induk folder terpilih &mdash; semua isinya akan ikut dicari.'
    : 'Belum ada folder terpilih.';
  expRun.textContent = n ? 'Simpan ' + n + ' Folder' : 'Simpan Pilihan';
}

$('pickList').addEventListener('change', (e) => {
  if (e.target.type !== 'checkbox') return;
  readPickBoxes();
  updatePickCount();
});
$('pickAll').onclick = () => {
  $('pickList').querySelectorAll('input[type=checkbox]').forEach((b) => { b.checked = true; });
  readPickBoxes(); updatePickCount();
};
$('pickNone').onclick = () => {
  $('pickList').querySelectorAll('input[type=checkbox]').forEach((b) => { b.checked = false; });
  readPickBoxes(); updatePickCount();
};
async function checkGdrive() {
  expStatus.textContent = 'Memeriksa koneksi Google Drive...';
  try {
    const d = await (await fetch('/api/gdrive/status')).json();
    if (!d.configured) { showStep(1); expStatus.textContent = 'Langkah 1: pilih client_secret.json.'; return; }
    if (!d.authorized) { showStep(2); expStatus.textContent = 'Langkah 2: hubungkan akun Google.'; return; }
    if (authPoller) { clearInterval(authPoller); authPoller = null; }
    showStep(3);
    expStatus.textContent = 'Terhubung sebagai ' + (d.email || 'akun') + '.';
    gdriveOk = true;
    hideAuthBanner();
    await loadFolders();
  } catch (err) { expStatus.textContent = 'Error: ' + err.message; }
}
function openGdriveModal() {
  $('expTitle').textContent = 'Ekspor ke Google Drive';
  pendingExportIds = null;
  pickMode = false;
  $('pickWrap').style.display = 'none';
  $('pickSelRow').style.display = '';
  expResult.innerHTML = ''; expResult.className = '';
  expModal.style.display = 'flex';
  checkGdrive();
}
$('expClose').onclick = () => { if (authPoller) { clearInterval(authPoller); authPoller = null; } expModal.style.display = 'none'; };
expModal.onclick = (e) => { if (e.target === expModal) { if (authPoller) { clearInterval(authPoller); authPoller = null; } expModal.style.display = 'none'; } };
$('expSecret').addEventListener('change', async () => {
  const f = $('expSecret').files[0];
  if (!f) return;
  try {
    const d = await (await fetch('/api/gdrive/secret', { method: 'POST', body: f })).json();
    if (d.error) throw new Error(d.error);
    await checkGdrive();
  } catch (err) { expStatus.textContent = 'Error: ' + err.message; }
});
$('expConnect').onclick = async () => {
  try {
    const d = await (await fetch('/api/gdrive/auth')).json();
    if (d.error) throw new Error(d.error);
    window.open(d.url, '_blank');
    expStatus.textContent = 'Izinkan akses di tab Google. Menunggu...';
    $('expConnect').disabled = true;
    if (authPoller) { clearInterval(authPoller); authPoller = null; }
    authPoller = setInterval(async () => {
      const s = await (await fetch('/api/gdrive/status')).json();
      if (s.authorized && authPoller) {
        clearInterval(authPoller); authPoller = null;
        $('expConnect').disabled = false;
        checkGdrive();
      }
    }, 2000);
  } catch (err) { expStatus.textContent = 'Error: ' + err.message; }
};
$('expFolderRefresh').onclick = loadFolders;
$('expFolderSearch').addEventListener('keydown', (e) => { if (e.key === 'Enter') loadFolders(); });

async function ensureAuth() {
  const g = await (await fetch('/api/gdrive/status')).json();
  gdriveOk = g.configured && g.authorized;
  if (!gdriveOk) {
    showAuthBanner();
    if (!g.admin) { toast('Google belum dikonfigurasi admin.', 'fail'); return false; }
    openGdriveModal();
    return false;
  }
  hideAuthBanner();
  return true;
}

const _reconnectBtn = $('authReconnect');
if (_reconnectBtn) {
  _reconnectBtn.addEventListener('click', async () => {
    const g = await (await fetch('/api/gdrive/status')).json();
    if (g.configured && g.authorized) { hideAuthBanner(); return; }
    openGdriveModal();
  });
} else {
  console.warn('authReconnect button not found');
}

/* tombol pilih folder foto (multi) */
$('pickBtn').onclick = async () => {
  if (!(await ensureAuth())) return;
  pickMode = true;
  $('expTitle').textContent = 'Pilih Folder Foto';
  pendingExportIds = 'pick';
  expResult.innerHTML = ''; expResult.className = '';
  // Muat pilihan yang sudah tersimpan supaya centang lama tidak hilang.
  try {
    const s = await (await fetch('/api/gdrive/status')).json();
    picked = (s.photosFolders && s.photosFolders.length)
      ? s.photosFolders.slice()
      : (s.photosFolderId ? [{ id: s.photosFolderId, name: s.photosFolderName }] : []);
  } catch (err) { picked = []; }
  $('pickWrap').style.display = 'block';
  $('pickSelRow').style.display = 'none';
  expRun.disabled = false;
  expStatus.textContent = 'Cari lalu centang induk folder yang mau dicari. Boleh lebih dari satu.';
  showStep(3);
  expModal.style.display = 'flex';
  await loadFolders();
  updatePickCount();
};

/* tombol ambil index */
$('idxBtn').onclick = async () => {
  if (!(await ensureAuth())) return;
  $('idxBtn').disabled = true;
  setStatus('Mengambil daftar foto dari Google Drive...');
  try {
    const d = await (await fetch('/api/index/refresh?force=1')).json();
    if (d.error) throw new Error(d.error);
    let msg = 'Sinkron Foto: ' + d.total.toLocaleString('id-ID') + ' foto';
    if (d.withOcr > 0) msg += ', OCR ' + d.withOcr.toLocaleString('id-ID');
    if (d.indexError) msg += ' (' + d.indexError + ')';
    setStatus(msg, 'ok');
    toast(msg, 'ok');
    doSearch(true);
  } catch (err) {
    setStatus('Gagal mengambil foto. Hubungkan ulang Google Drive, lalu coba lagi.', 'error');
    showAuthBanner();
  }
  $('idxBtn').disabled = false;
};

/* expRun: dua mode (pilih folder vs ekspor) */
expRun.onclick = async () => {
  if (pendingExportIds === 'pick') {
    readPickBoxes();
    if (!picked.length) { expStatus.textContent = 'Centang minimal satu folder dulu.'; return; }
    const nama = picked.map((f) => f.name).join(', ');
    expRun.disabled = true;
    const labelLama = expRun.textContent;
    expRun.textContent = 'Menyimpan...';
    try {
      const d = await (await fetch('/api/gdrive/setphotos', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ folders: picked })
      })).json();
      if (d.error) throw new Error(d.error);
      const n = (d.photosFolders || picked).length;
      expResult.className = 'ok';
      expResult.innerHTML = '\u2714 <b>' + n + ' folder</b> tersimpan: ' + esc(nama) +
        '.<br>Klik <b>"Sinkron Foto"</b> untuk memindai semua isi folder ini.';
      toast(n + ' folder foto disimpan', 'ok');
      picked = (d.photosFolders || picked);
      statsAt = 0; // folder sumber berubah -> stats harus dihitung ulang
      if (typeof renderReady === 'function' && panelShown) renderReady();
    } catch (err) {
      expResult.className = 'fail';
      expResult.innerHTML = 'Error: ' + esc(err.message);
    }
    expRun.disabled = false;
    expRun.textContent = labelLama;
    updatePickCount();
    return;
  }
  const folderId = expFolder.value;
  if (!folderId) { expStatus.textContent = 'Pilih folder dulu.'; return; }
  if (!pendingExportIds || !pendingExportIds.length) { expStatus.textContent = 'Tidak ada foto untuk diekspor.'; return; }
  expRun.disabled = true;
  expRun.textContent = 'Mengirim...';
  expResult.innerHTML = ''; expResult.className = '';
  try {
    const d = await (await fetch('/api/batch/export', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ folderId, items: pendingExportIds })
    })).json();
    if (d.error) throw new Error(d.error);
    expResult.className = d.fails.length ? 'fail' : 'ok';
    expResult.innerHTML = '\u2714 ' + d.done.length + ' foto terkirim ke Google Drive.' +
      (d.fails.length ? '<br>\u2716 Gagal: ' + esc(d.fails.join(' | ')) : '') +
      (d.done.length ? ' <a href="' + esc(d.done[0].link) + '" target="_blank">Buka foto pertama</a>' : '');
    expRun.disabled = false;
  } catch (err) {
    expResult.className = 'fail';
    expResult.innerHTML = 'Error: ' + esc(err.message);
    expRun.disabled = false;
  }
};
function openExport(items) {
  $('expTitle').textContent = 'Ekspor ke Google Drive';
  pendingExportIds = items;
  pickMode = false;              // mode ini pakai select tunggal (folder tujuan ekspor)
  $('pickWrap').style.display = 'none';
  $('pickSelRow').style.display = '';
  expResult.innerHTML = ''; expResult.className = '';
  expRun.textContent = 'Ekspor ' + items.length + ' foto';
  expModal.style.display = 'flex';
  checkGdrive();
}

async function refreshGdriveStatus() {
  try {
    const g = await (await fetch('/api/gdrive/status')).json();
    driveData = g;
    gdriveOk = g.configured && g.authorized;
    gdot.className = 'gdot ' + (g.authorized ? 'on' : (g.configured ? 'off' : ''));
    if (!gdriveOk) showAuthBanner(); else hideAuthBanner();
    if (panelShown) renderReady();
  } catch (_) {}
}
async function checkSuperAdmin() {
  try {
    const d = await (await fetch('/api/auth/me')).json();
    const isSuper = d.loggedIn && d.session && d.session.role === 'superadmin';
    const wrapper = $('navSettingsWrapper');
    if (wrapper) {
      wrapper.style.display = isSuper ? '' : 'none';
    }
  } catch (_) {
    const wrapper = $('navSettingsWrapper');
    if (wrapper) wrapper.style.display = 'none';
  }
}

(async () => {
  // Semua request halaman pertama jalan bareng. Sebelumnya berurutan, jadi
  // modul sempat tampil kosong sambil nunggu /api/gdrive/status.
  await Promise.all([refreshGdriveStatus(), loadStats(true)]);
  setInterval(refreshGdriveStatus, 30000);
  doSearch(true);
  checkSuperAdmin();

/* tombol putus koneksi drive (ganti akun) */
$('revokeBtn').onclick = async () => {
  if (!confirm('Putus koneksi Google Drive?\n\nIni akan menghapus token tersimpan.\nKemudian bisa hubungkan akun Google lain lewat menu Ekspor.')) return;
  try {
    const d = await (await fetch('/api/gdrive/revoke', { method: 'POST' })).json();
    if (d.error) throw new Error(d.error);
    toast(d.msg || 'Token Drive dihapus', 'ok');
    $('navSettingsDD').classList.remove('show');
    await refreshGdriveStatus();
  } catch (err) { toast('Gagal: ' + err.message, 'fail'); }
};
})();