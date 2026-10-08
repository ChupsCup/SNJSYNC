function esc(s) { const d = document.createElement('div'); d.textContent = s; return d.innerHTML; }
const $ = (id) => document.getElementById(id);

let plUploadIds = [];
let compareResults = [];
let currentFilter = 'all';
let currentPlUploadId = null;
let currentResultId = null;
let currentResultApproved = false;
let _reviewGroups = [];


const plDrop = $('plDropZone'), plInput = $('plFileInput');
plDrop.addEventListener('click', () => plInput.click());
plDrop.addEventListener('dragover', (e) => { e.preventDefault(); plDrop.classList.add('dragover'); });
plDrop.addEventListener('dragleave', () => plDrop.classList.remove('dragover'));
plDrop.addEventListener('drop', (e) => { e.preventDefault(); plDrop.classList.remove('dragover'); if (e.dataTransfer.files.length) uploadPL(Array.from(e.dataTransfer.files)); });
plInput.addEventListener('change', (e) => { if (e.target.files.length) uploadPL(Array.from(e.target.files)); });

async function uploadPL(files) {
  $('plProg').style.display = 'block';
  $('plInfo').style.display = 'none';
  let totalAll = 0;
  let sheetsAll = [];

  for (let fi = 0; fi < files.length; fi++) {
    const file = files[fi];
    const ext = file.name.split('.').pop().toLowerCase();
    if (!['xlsx', 'xls', 'csv'].includes(ext)) { toast(file.name + ': bukan file Excel', 'fail'); continue; }

    $('plPct').textContent = (fi + 1) + '/' + files.length;
    $('plBar').style.width = '0%'; $('plBar').classList.remove('ok');
    $('plMsg').textContent = 'Upload ' + (fi + 1) + '/' + files.length + ': ' + file.name;

    try {
      const d = await new Promise((resolve, reject) => {
        const fd = new FormData(); fd.append('file', file);
        const xhr = new XMLHttpRequest();
        xhr.open('POST', '/api/mc/upload-pl');
        xhr.upload.onprogress = (e) => {
          if (e.lengthComputable) {
            const p = Math.round(e.loaded / e.total * 100);
            $('plPct').textContent = (fi + 1) + '/' + files.length + ' — Upload ' + p + '%';
            $('plBar').style.width = p + '%';
            $('plMsg').textContent = file.name + ': ' + fmtBytes(e.loaded) + ' / ' + fmtBytes(e.total);
          }
        };
        xhr.onload = () => {
          try { const r = JSON.parse(xhr.responseText); if (xhr.status !== 200 || !r.ok) throw new Error(r.error); resolve(r); }
          catch (e) { reject(e); }
        };
        xhr.onerror = () => reject(new Error('Network error'));
        xhr.send(fd);
      });

      plUploadIds.push(d.uploadId);

      if (d.processing) {
        $('plPct').textContent = (fi + 1) + '/' + files.length + ' — Parsing...';
        $('plBar').style.width = '100%'; $('plBar').style.opacity = '.5';
        $('plMsg').textContent = file.name + ': Server sedang parse & simpan...';
        await pollPlStatus(d.uploadId, fi, files.length, file.name);
        $('plBar').style.opacity = '1';
      }

      const st = await (await fetch('/api/mc/pl-status/' + d.uploadId)).json();
      totalAll += (st.total_rows || d.total || 0);
      toast(file.name + ': ' + (st.total_rows || d.total || 0) + ' baris', 'ok');
    } catch (e) {
      toast(file.name + ': ' + e.message, 'fail');
    }
  }

  $('plBar').style.width = '100%'; $('plBar').classList.add('ok');
  $('plMsg').textContent = files.length + ' file selesai: ' + totalAll + ' baris total';
  $('plFileName').textContent = files.length + ' file';
  $('plInfo').style.display = 'block';
  $('plInfo').textContent = totalAll + ' baris dari ' + files.length + ' file';
  currentPlUploadId = plUploadIds[plUploadIds.length - 1];
  $('btnCompare').disabled = false;
  toast('Semua packing list ter-upload: ' + totalAll + ' baris', 'ok');
  setTimeout(() => { $('plProg').style.display = 'none'; $('plBar').classList.remove('ok'); }, 2000);
  plInput.value = '';
  loadPlUploads();
}

function pollPlStatus(uploadId, fi, total, filename) {
  return new Promise((resolve) => {
    let tries = 0;
    const iv = setInterval(async () => {
      tries++;
      try {
        const r = await fetch('/api/mc/pl-status/' + uploadId);
        const d = await r.json();
        if (d.status === 'done') { clearInterval(iv); resolve(d); }
        else if (d.status && d.status.startsWith('error')) { clearInterval(iv); toast(filename + ': ' + d.status, 'fail'); resolve(d); }
        else {
          $('plPct').textContent = fi + 1 + '/' + total + ' — Processing... (' + tries + 's)';
          $('plMsg').textContent = filename + ': Server parse & simpan ke DB...';
        }
      } catch (e) { /* retry */ }
    }, 1000);
  });
}

function pollMlStatus(uploadId, filename) {
  return new Promise((resolve, reject) => {
    let tries = 0;
    const iv = setInterval(async () => {
      tries++;
      try {
        const r = await fetch('/api/mc/ml-status/' + uploadId);
        const d = await r.json();
        if (d.status === 'done') { clearInterval(iv); resolve(d); }
        else if (d.status && d.status.startsWith('error')) { clearInterval(iv); reject(new Error(d.status.replace('error: ', ''))); }
        else { $('mlMsg').textContent = filename + ': Parsing & simpan... (' + tries + 's)'; }
      } catch (e) { /* retry */ }
    }, 1000);
  });
}

/* === Upload Masterlist === */
const mlDrop = $('mlDropZone'), mlInput = $('mlFileInput');
mlDrop.addEventListener('click', () => mlInput.click());
mlDrop.addEventListener('dragover', (e) => { e.preventDefault(); mlDrop.classList.add('dragover'); });
mlDrop.addEventListener('dragleave', () => mlDrop.classList.remove('dragover'));
mlDrop.addEventListener('drop', (e) => { e.preventDefault(); mlDrop.classList.remove('dragover'); if (e.dataTransfer.files[0]) uploadML(e.dataTransfer.files[0]); });
mlInput.addEventListener('change', (e) => { if (e.target.files[0]) uploadML(e.target.files[0]); });

function uploadML(file) {
  const ext = file.name.split('.').pop().toLowerCase();
  if (!['xlsx', 'xls', 'csv'].includes(ext)) { toast('File harus .xlsx/.xls/.csv', 'fail'); return; }
  mlDrop.style.pointerEvents = 'none'; mlDrop.style.opacity = '.5';
  $('mlProg').style.display = 'block';

  const fd = new FormData(); fd.append('file', file);
  const xhr = new XMLHttpRequest();
  xhr.open('POST', '/api/mc/upload-ml');
  xhr.upload.onprogress = (e) => {
    if (e.lengthComputable) {
      const p = Math.round(e.loaded / e.total * 100);
      $('mlPct').textContent = p + '%'; $('mlBar').style.width = p + '%';
      $('mlMsg').textContent = 'Upload: ' + fmtBytes(e.loaded) + ' / ' + fmtBytes(e.total);
    }
  };
  xhr.onload = () => {
    try {
      const d = JSON.parse(xhr.responseText);
      if (xhr.status !== 200 || !d.ok) throw new Error(d.error);
      $('mlFileName').textContent = file.name;
      $('mlBar').style.width = '100%'; $('mlBar').style.opacity = '.5';
      $('mlMsg').textContent = file.name + ': Server sedang parse & simpan...';
      if (d.processing && d.uploadIds && d.uploadIds.length) {
        pollMlStatus(d.uploadIds[0], file.name).then(() => {
          $('mlBar').style.opacity = '1'; $('mlBar').classList.add('ok');
          $('mlMsg').textContent = file.name + ': selesai';
          toast('Masterlist ter-upload', 'ok');
          loadMasterlists();
          setTimeout(() => { $('mlProg').style.display = 'none'; $('mlBar').classList.remove('ok'); }, 2000);
        }).catch((e) => { toast('Gagal: ' + e.message, 'fail'); $('mlProg').style.display = 'none'; });
      } else {
        $('mlBar').style.opacity = '1'; $('mlBar').classList.add('ok');
        $('mlMsg').textContent = 'Selesai: ' + (d.total || '') + ' baris';
        toast('Masterlist ter-upload', 'ok');
        loadMasterlists();
        setTimeout(() => { $('mlProg').style.display = 'none'; $('mlBar').classList.remove('ok'); }, 2000);
      }
    } catch (e) { toast('Gagal: ' + e.message, 'fail'); $('mlProg').style.display = 'none'; }
    mlDrop.style.pointerEvents = ''; mlDrop.style.opacity = '';
    mlInput.value = '';
  };
  xhr.onerror = () => { toast('Network error', 'fail'); mlDrop.style.pointerEvents = ''; mlDrop.style.opacity = ''; $('mlProg').style.display = 'none'; };
  xhr.send(fd);
}

/* === Load Masterlist List === */
async function loadMasterlists() {
  try {
    const r = await fetch('/api/mc/masterlists');
    const d = await r.json();
    const list = $('mlList');
    if (!d.uploads || !d.uploads.length) { list.innerHTML = '<div class="hint">Belum ada masterlist (referensi matching).</div>'; return; }
    let html = '';
    for (const u of d.uploads) {
      const tag = u.status === 'done' ? 'DONE' : 'PROSES';
      const cls = u.status === 'done' ? 'approved' : 'pending';
      const src = /APPROVED|NEW ITEM|Master Data/.test(u.filename) || /APPROVED|NEW ITEM/.test(u.sheet_name) ? 'ref' : 'upload';
      html += '<div class="ml-item" data-id="' + u.id + '">' +
        '<span class="badge ' + cls + '">' + tag + '</span>' +
        (src === 'ref' ? '<span class="ml-sheet" style="color:var(--accent)">MASTER DATA</span>' : '<span class="ml-sheet">' + esc(u.sheet_name) + '</span>') +
        '<span class="ml-name">' + esc(u.filename) + '</span>' +
        '<span class="ml-date">' + esc(u.uploaded_at) + '</span>' +
        '<button class="danger" onclick="deleteML(' + u.id + ')">Hapus</button>' +
        '</div>';
    }
    list.innerHTML = html;
  } catch (e) { $('mlList').innerHTML = '<div class="hint">Gagal load: ' + e.message + '</div>'; }
}


async function loadPlUploads() {
  try {
    const r = await fetch('/api/mc/pl-uploads');
    const d = await r.json();
    const list = $('plList');
    if (!d.uploads || !d.uploads.length) { list.innerHTML = ''; return; }
    let html = '';
    for (const u of d.uploads) {
      const ok = u.status === 'done';
      if (ok && !plUploadIds.includes(u.id)) plUploadIds.push(u.id);
      if (ok && (!currentPlUploadId || !plUploadIds.includes(currentPlUploadId))) currentPlUploadId = u.id;
      html += '<div class="ml-item">' +
        '<span class="badge ' + (ok ? 'approved' : 'pending') + '">' + (ok ? 'DONE' : 'PROSES') + '</span>' +
        '<span class="ml-name">' + esc(u.filename) + '</span>' +
        '<span class="ml-sheet">' + (ok ? u.total_rows + ' baris' : '') + '</span>' +
        '<span class="ml-date">' + esc(u.uploaded_at) + '</span>' +
        '<button class="danger" style="font-size:11px;padding:6px 10px" onclick="deletePL(' + u.id + ')">Hapus</button>' +
        '</div>';
    }
    list.innerHTML = html;
    if (plUploadIds.length) $('btnCompare').disabled = false;
  } catch (_) {}
}

async function deletePL(id) {
  if (!confirm('Hapus packing list ini + baris datanya? Ini tidak bisa dibatalkan.')) return;
  try {
    const r = await fetch('/api/mc/delete-pl', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ uploadId: id }) });
    const d = await r.json();
    if (!r.ok) throw new Error(d.error);
    plUploadIds = plUploadIds.filter((x) => x !== id);
    if (currentPlUploadId === id) currentPlUploadId = null;
    if (!plUploadIds.length) $('btnCompare').disabled = true;
    toast('Packing list dihapus.', 'ok');
    loadPlUploads();
    window.deletePL = deletePL;
  } catch (e) { toast('Gagal hapus: ' + e.message, 'fail'); }
}
window.deletePL = deletePL;

async function deleteML(id) {
  if (!confirm('Hapus masterlist ini?')) return;
  await fetch('/api/mc/delete-ml', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ uploadId: id }) });
  toast('Masterlist dihapus', 'ok');
  loadMasterlists();
}


$('btnCompare').addEventListener('click', async () => {
  if (!plUploadIds.length && !currentPlUploadId) { toast('Upload packing list dulu!', 'fail'); return; }
  $('btnCompare').disabled = true; $('btnCompare').textContent = 'Compare...';
  $('status').className = ''; $('status').textContent = 'Compare berjalan...';
  let cmpSecs = 0;
  clearInterval(compareTimer);
  compareTimer = setInterval(() => {
    cmpSecs++;
    $('status').textContent = 'Compare berjalan... (' + cmpSecs + ' dtk) — matching packing list vs masterlist';
  }, 1000);
  try {
    const r = await fetch('/api/mc/compare', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ uploadIds: plUploadIds.length ? plUploadIds : [currentPlUploadId] })
    });
    const d = await r.json();
    if (!r.ok) throw new Error(d.error);
    compareResults = d.results;
    currentResultId = d.resultId || null;
    currentResultApproved = false;
    tableVisible = TABLE_PAGE;
    renderTable();
    $('filterChips').style.display = 'flex';
    $('btnExport').disabled = false;
    updateApproveButton(true);
    $('status').className = 'ok';
    $('status').textContent = d.total + ' baris \u2014 ' + d.exist + ' EXISTING, ' + d.new + ' NEW ITEM' + (d.review ? ', ' + d.review + ' REVIEW' : '') + (d.skipped ? ' \u2014 ' + d.skipped + ' baris TOTAL di-skip' : '');
    toast('Compare: ' + d.exist + ' EXISTING, ' + d.new + ' NEW ITEM' + (d.review ? ', ' + d.review + ' REVIEW' : '') + (d.skipped ? ' (' + d.skipped + ' baris TOTAL di-skip)' : ''), 'ok');
    setupReview();
    loadHistory();
  } catch (e) {
    toast('Gagal: ' + e.message, 'fail');
    $('status').className = 'error'; $('status').textContent = e.message;
  } finally { clearInterval(compareTimer); $('btnCompare').disabled = false; $('btnCompare').textContent = 'Compare'; }
});
let compareTimer = null;

function updateApproveButton(enabled) {
  const b = $('btnApproveResult');
  b.disabled = !enabled || currentResultApproved;
  b.textContent = currentResultApproved ? '\u2714 ' + (b.dataset.label || 'Approved') : 'Approve & Simpan';
  if (currentResultApproved) b.style.opacity = '.6'; else b.style.opacity = '';
  $('approveStatus').style.display = 'none';
}
$('btnApproveResult').addEventListener('click', async () => {
  if (!currentResultId) { toast('Compare dulu, lalu export, baru approve.', 'fail'); return; }
  if (!confirm('Approve & Simpan hasil compare ini sebagai Master Data referensi (single source of truth)? Data EXISTING yang sudah punya Master Item Code akan disimpan sebagai referensi.')) return;
  const b = $('btnApproveResult');
  b.disabled = true; b.textContent = 'Menyimpan...';
  try {
    const r = await fetch('/api/mc/approve-result', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ resultId: currentResultId })
    });
    const d = await r.json();
    if (!r.ok) throw new Error(d.error);
    currentResultApproved = true;
    updateApproveButton(true);
    const st = $('approveStatus');
    st.style.display = 'block';
    st.textContent = d.already
      ? '\u2714 Hasil #' + currentResultId + ' sudah di-approve sebelumnya.'
      : '\u2714 Hasil di-approve & disimpan: ' + d.saved + ' item jadi Master Data referensi.';
    toast('Approve & Simpan berhasil: ' + d.saved + ' item.', 'ok');
    loadMasterlists();
    loadHistory();
  } catch (e) {
    toast('Gagal approve: ' + e.message, 'fail');
    b.disabled = false; b.textContent = 'Approve & Simpan';
  }
});


let tableVisible = 300;

const TABLE_PAGE = 300;
let searchQuery = '';
$('searchInput').addEventListener('input', (e) => {
  searchQuery = e.target.value.trim().toLowerCase();
  tableVisible = TABLE_PAGE;
  renderTable();
});
let renderToken = 0;
function rowHtml(r, count, g) {
  const isReview = !!r.needsReview;
  const c = isReview ? 'review' : (r.flag === 'NEW ITEM' ? 'new' : 'existing');
  const label = isReview ? 'REVIEW' : r.flag;
  const confirmBtn = isReview && !currentResultApproved
    ? '<button class="confirm-btn" data-gconfirm="' + g + '" title="Setujui kandidat: ' + esc(r.candDesc || r.candCode || '') + '">\u2713 Cocok</button>'
      + '<button class="confirm-btn" data-greject="' + g + '" style="color:var(--red);border-color:color-mix(in srgb, var(--red) 40%, transparent)" title="Tolak kandidat: barang ini memang baru">\u2715 Bukan</button>' : '';
  const gbadge = count > 1 ? ' <span class="gcount" title="' + count + ' baris dengan produk sama (beda no. container)">\u00d7' + count + '</span>' : '';
  return '<tr><td>' + r.no + '</td><td style="font-weight:600">' + esc(r.noContainer) + gbadge + '</td><td>' + esc(r.brand) + '</td><td>' + esc(r.namaItem) + '</td><td>' + esc(r.eta) + '</td><td>' + esc(r.importir || '') + '</td><td><span class="flag ' + c + '">' + label + '</span>' + confirmBtn + '</td><td>' + esc(r.masterCode) + '</td><td>' + esc(r.masterDesc) + '</td><td class="note-cell" title="' + esc(r.note || '') + '">' + esc(r.note || '') + '</td></tr>';
}

function renderTable() {
  let base = compareResults;
  if (searchQuery) {
    base = base.filter((r) =>
      String(r.no || '').includes(searchQuery) ||
      (r.noContainer || '').toLowerCase().includes(searchQuery) ||
      (r.brand || '').toLowerCase().includes(searchQuery) ||
      (r.namaItem || '').toLowerCase().includes(searchQuery) ||
      (r.itemCode || '').toLowerCase().includes(searchQuery) ||
      (r.itemDesc || '').toLowerCase().includes(searchQuery) ||
      (r.importir || '').toLowerCase().includes(searchQuery) ||
      (r.masterCode || '').toLowerCase().includes(searchQuery) ||
      (r.masterDesc || '').toLowerCase().includes(searchQuery) ||
      (r.note || '').toLowerCase().includes(searchQuery) ||
      (r.flag || '').toLowerCase().includes(searchQuery));
  }
  const f = currentFilter === 'all' ? base :
    currentFilter === 'review' ? base.filter((r) => r.needsReview) :
    base.filter((r) => currentFilter === 'new' ? (r.flag === 'NEW ITEM' && !r.needsReview) : (r.flag === 'EXISTING' && !r.needsReview));
  const sc = $('searchCount');
  if (sc) sc.textContent = searchQuery || currentFilter !== 'all' ? (f.length + ' cocok') : '';
  if (!f.length) { $('tbl').style.display = 'none'; $('tbody').innerHTML = ''; $('tblLoadMore').style.display = 'none'; return; }
  $('tbl').style.display = 'table';
  const { rows, total } = buildRows(f);
  

  const hasMore = rows.length < total;
  $('tblLoadMore').style.display = hasMore ? 'block' : 'none';
  if (hasMore) $('btnTableMore').textContent = 'Muat ' + Math.min(TABLE_PAGE, total - rows.length) + ' lagi (' + rows.length + '/' + total + (currentFilter === 'review' ? ' produk' : '') + ' ditampilkan)';

  
  const token = ++renderToken;
  const tbody = $('tbody');
  tbody.innerHTML = '';
  const CHUNK = 700;
  let i = 0;
  const statusEl = $('status');
  const savedStatus = statusEl.textContent;
  const savedClass = statusEl.className;
  (function step() {
    if (token !== renderToken) return;
    const end = Math.min(i + CHUNK, rows.length);
    let h = '';
    for (; i < end; i++) { const it = rows[i]; h += rowHtml(it.r, it.count, it.g); }
    tbody.insertAdjacentHTML('beforeend', h);
    if (i < rows.length) {
      statusEl.className = '';
      statusEl.textContent = 'Menampilkan ' + i + ' / ' + rows.length + ' baris...';
      setTimeout(step, 0);
      return;
    }
    statusEl.className = savedClass;
    statusEl.textContent = savedStatus;
  })();
}
function buildRows(list) {
  _reviewGroups = [];
  if (currentFilter !== 'review') {
    const total = list.length;
    
    const shown = list.slice(0, tableVisible);
    return { rows: shown.map((r) => ({ r, count: 0, g: -1 })), total };
  }
  const groups = new Map();
  for (const r of list) {
    const k = ((r.brand || '') + '|' + (r.namaItem || '')).toLowerCase();
    if (!groups.has(k)) groups.set(k, { r, count: 0 });
    groups.get(k).count++;
  }
  const total = groups.size;
  const arr = Array.from(groups.values());
  const shown = arr.slice(0, tableVisible);
  return { rows: shown.map(({ r, count }) => ({ r, count, g: _reviewGroups.push(((r.brand || '') + '|' + (r.namaItem || ''))) - 1 })), total };
}
$('btnTableMore').addEventListener('click', () => { tableVisible += TABLE_PAGE; renderTable(); });


async function reviewAction(mode, btn, key) {
  if (!currentResultId) return;
  btn.disabled = true;
  const old = btn.textContent;
  btn.textContent = '...';
  try {
    const r = await fetch(mode === 'reject' ? '/api/mc/reject-review' : '/api/mc/confirm-review', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ resultId: currentResultId, key })
    });
    const d = await r.json();
    if (!r.ok) throw new Error(d.error);
    const applied = d.applied || 1;
    const patch = {
      flag: d.row.flag, needsReview: d.row.needsReview,
      masterCode: d.row.masterCode, masterDesc: d.row.masterDesc, note: d.row.note,
      candCode: d.row.candCode !== undefined ? d.row.candCode : '', candDesc: d.row.candDesc !== undefined ? d.row.candDesc : ''
    };
    const nk = key.toLowerCase();
    for (const x of compareResults) {
      if (((x.brand || '') + '|' + (x.namaItem || '')).toLowerCase() === nk) Object.assign(x, patch);
    }
    $('status').className = 'ok';
    $('status').textContent = compareResults.length + ' baris \u2014 ' + d.exist + ' EXISTING, ' + d.new + ' NEW ITEM, ' + d.review + ' REVIEW';
    toast(mode === 'reject' ? applied + ' baris divalidasi sebagai item baru.' : applied + ' baris dikonfirmasi EXISTING.', 'ok');
    tableVisible = TABLE_PAGE;
    renderTable();
    setupReview();
  } catch (err) {
    toast('Gagal: ' + err.message, 'fail');
    btn.disabled = false; btn.textContent = old;
  }
}
$('tbody').addEventListener('click', (e) => {
  const btnC = e.target.closest('button[data-gconfirm]');
  if (btnC) { reviewAction('confirm', btnC, _reviewGroups[parseInt(btnC.getAttribute('data-gconfirm'), 10)]); return; }
  const btnR = e.target.closest('button[data-greject]');
  if (btnR) { reviewAction('reject', btnR, _reviewGroups[parseInt(btnR.getAttribute('data-greject'), 10)]); return; }
});


$('filterChips').addEventListener('click', (e) => {
  const chip = e.target.closest('.chip'); if (!chip) return;
  currentFilter = chip.dataset.filter;
  $('filterChips').querySelectorAll('.chip').forEach((c) => c.classList.remove('active'));
  chip.classList.add('active');
  tableVisible = TABLE_PAGE;
  renderTable();
});


const MC_MONTHS = ['', 'januari', 'februari', 'maret', 'april', 'mei', 'juni', 'juli', 'agustus', 'september', 'oktober', 'november', 'desember'];
const MC_MONTH_KEY = { jan: 1, feb: 2, mar: 3, apr: 4, mei: 5, jun: 6, jul: 7, agu: 8, sep: 9, okt: 10, nov: 11, des: 12 };
function mcDateFromFilename(fn) {
  const s = String(fn || '').replace(/\.(xlsx?|csv)$/i, '');
  const m = s.match(/(\d{1,2})\s*(jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|mei|jun(?:e)?|jul(?:y)?|agu(?:stus)?|sep(?:tember)?|okt(?:ober)?|nov(?:ember)?|des(?:ember)?)/i);
  if (m) {
    const day = parseInt(m[1], 10);
    const mo = MC_MONTH_KEY[m[2].toLowerCase().slice(0, 3)];
    if (day >= 1 && day <= 31 && mo) return day + ' ' + MC_MONTHS[mo];
  }
  const n = s.match(/(\d{1,2})[-/](\d{1,2})[-/](\d{2,4})/);
  if (n) {
    const day = parseInt(n[1], 10), mo = parseInt(n[2], 10);
    if (day >= 1 && day <= 31 && mo >= 1 && mo <= 12) return day + ' ' + MC_MONTHS[mo];
  }
  return '';
}
async function mcOutputName() {
  try {
    const r = await fetch('/api/mc/masterlists', { cache: 'no-store' });
    const d = await r.json();
    if (d.uploads && d.uploads.length) {
      for (const u of d.uploads) {
        const label = mcDateFromFilename(u.filename);
        if (label) return 'Output Hasil MC ' + label + '.xlsx';
      }
    }
  } catch (_) {}
  return 'Output Hasil MC.xlsx';
}
$('btnExport').addEventListener('click', async () => {
  if (!compareResults.length) { toast('Compare dulu!', 'fail'); return; }
  const b = $('btnExport');
  b.disabled = true; b.textContent = 'Export...';
  try {
    const body = currentResultId ? { resultId: currentResultId } : { results: compareResults };
    const r = await fetch('/api/mc/export', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body)
    });
    if (!r.ok) { let e = 'Export gagal.'; try { e = (await r.json()).error || e; } catch (_) {} throw new Error(e); }
    const blob = await r.blob();
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = await mcOutputName();
    document.body.appendChild(a); a.click(); a.remove();
    toast('Export Excel berhasil!', 'ok');
  } catch (e) {
    toast('Gagal export: ' + e.message, 'fail');
  } finally {
    b.disabled = false; b.textContent = 'Export';
  }
});


let newItems = [];
let newVisible = 200;

const NEW_PAGE = 300;
function setupReview() {
  const uniq = new Map();
  for (const r of compareResults) {
    if (r.flag !== 'NEW ITEM' || r.needsReview) continue; 
    const key = (r.brand + '|' + r.namaItem).toLowerCase();
    if (!uniq.has(key)) uniq.set(key, r);
  }
  newItems = Array.from(uniq.values());
  newVisible = NEW_PAGE;
  $('reviewSec').style.display = newItems.length ? 'block' : 'none';
  if (newItems.length) renderNewItems();
}
function renderNewItems() {
  const tb = $('tbodyNew');
  const shown = newItems.slice(0, newVisible);
  let h = '';
  for (let i = 0; i < shown.length; i++) {
    const it = shown[i];
    h += '<tr data-idx="' + i + '">' +
      '<td><input type="checkbox"' + (it._sel === false ? '' : ' checked') + ' onchange="updateNewSelection()"></td>' +
      '<td style="white-space:nowrap">' + esc(it.brand) + '</td>' +
      '<td>' + esc(it.namaItem) + '</td>' +
      '<td>' + esc(it.eta) + '</td>' +
      '<td><input type="text" class="new-code" value="' + esc(it._code !== undefined ? it._code : (it.masterCode === '-' ? (it.candCode || '') : it.masterCode)) + '" placeholder="Kode item baru" oninput="updateNewSelection()"></td>' +
      '<td><input type="text" class="new-desc" value="' + esc(it._desc !== undefined ? it._desc : (it.masterDesc === '-' ? (it.candDesc || it.namaItem) : it.masterDesc)) + '" placeholder="Deskripsi standar" oninput="updateNewSelection()"></td>' +
      '</tr>';
  }
  tb.innerHTML = h;
  const hasMore = newItems.length > newVisible;
  $('newLoadMore').style.display = hasMore ? 'block' : 'none';
  if (hasMore) $('btnNewMore').textContent = 'Muat ' + Math.min(NEW_PAGE, newItems.length - newVisible) + ' lagi (' + shown.length + '/' + newItems.length + ' ditampilkan)';
  updateNewSelection();
}
$('btnNewMore').addEventListener('click', () => { newVisible += NEW_PAGE; renderNewItems(); });
function updateNewSelection() {
  const rows = $('tbodyNew').querySelectorAll('tr[data-idx]');
  rows.forEach((tr) => {
    const idx = parseInt(tr.dataset.idx);
    const box = tr.querySelector('input[type=checkbox]');
    const code = tr.querySelector('.new-code').value.trim();
    const desc = tr.querySelector('.new-desc').value.trim();
    Object.assign(newItems[idx], { _sel: box.checked, _code: code, _desc: desc });
  });
  const selected = newItems.filter((it) => it._sel).length;
  $('reviewCount').textContent = selected + ' / ' + newItems.length + ' item dipilih';
  $('btnSaveMasterItems').disabled = selected === 0;
}
$('btnSelectAllNew').addEventListener('click', () => { newItems.forEach((it) => it._sel = true); $('tbodyNew').querySelectorAll('input[type=checkbox]').forEach((b) => b.checked = true); updateNewSelection(); });
$('btnUnselectAllNew').addEventListener('click', () => { newItems.forEach((it) => it._sel = false); $('tbodyNew').querySelectorAll('input[type=checkbox]').forEach((b) => b.checked = false); updateNewSelection(); });
$('btnSaveMasterItems').addEventListener('click', async () => {
  const toSave = newItems.filter((it) => it._sel);
  if (!toSave.length) { toast('Pilih minimal 1 item.', 'fail'); return; }
  const payload = toSave.map((it) => ({
    brand: it.brand,
    namaItem: it.namaItem,
    itemCode: it._code || '',
    itemDesc: it._desc || it.namaItem
  }));
  $('btnSaveMasterItems').disabled = true;
  $('reviewStatus').textContent = 'Menyimpan...';
  try {
    const r = await fetch('/api/mc/save-master-items', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ items: payload })
    });
    const d = await r.json();
    if (!r.ok) throw new Error(d.error);
    const savedKeys = new Set(toSave.map((it) => (it.brand + '|' + it.namaItem).toLowerCase()));
    $('reviewStatus').textContent = d.saved + ' item disimpan ke Masterlist.';
    toast(d.saved + ' item berhasil jadi Master Data (EXISTING untuk compare berikutnya)', 'ok');
    for (const r of compareResults) {
      if (r.flag === 'NEW ITEM' && savedKeys.has((r.brand + '|' + r.namaItem).toLowerCase())) { r.flag = 'EXISTING'; }
    }
    tableVisible = TABLE_PAGE;
    renderTable();
    loadMasterlists();
    setTimeout(() => { $('reviewStatus').textContent = ''; }, 3500);
  } catch (e) { toast('Gagal simpan: ' + e.message, 'fail'); $('reviewStatus').textContent = 'Gagal: ' + e.message; }
  finally { setupReview(); $('btnSaveMasterItems').disabled = false; }
});


async function loadHistory(attempt = 1, quiet = false) {
  const el = $('historyList');
  const MAX = 6;
  if (!quiet) el.innerHTML = 'Memuat riwayat...';
  try {
    const r = await fetch('/api/mc/results', { cache: 'no-store' });
    if (!r.ok) throw new Error('HTTP ' + r.status);
    const d = await r.json();
    if (!Array.isArray(d.results)) throw new Error('Respons tidak valid');
    if (d.results.length) {
      el.innerHTML = '';
      for (const r of d.results) {
        const cls = r.approved ? 'approved' : 'pending';
        const label = r.approved ? 'APPROVED' : 'READY';
        el.innerHTML += '<div class="ml-item">' +
          '<span class="badge ' + cls + '">' + label + '</span>' +
          '<span class="ml-name">#' + r.id + ' — ' + esc(r.filename || 'Compare') + '</span>' +
          '<span class="ml-date">' + esc(r.created_at) + '</span>' +
          '<span class="ml-sheet">' + r.total + ' baris (' + r.exist_count + ' E / ' + r.new_count + ' N' + (r.review_count ? ' / ' + r.review_count + ' R' : '') + ')</span>' +
          '<button class="ghost" data-act="lihat" data-id="' + r.id + '">Lihat</button>' +
          (r.approved
            ? ''
            : '<button class="primary" style="font-size:11px;padding:6px 12px" data-act="approve" data-id="' + r.id + '">Approve &amp; Simpan</button>') +
          '<button class="ghost danger" data-act="hapus" data-id="' + r.id + '" data-approved="' + (r.approved ? 1 : 0) + '">Hapus</button>' +
          '</div>';
      }
      return;
    }
    
    if (attempt < MAX) { el.innerHTML = 'Belum ada riwayat. (cek ulang percobaan ' + attempt + '..)'; setTimeout(() => loadHistory(attempt + 1, true), 1200 * attempt); return; }
    el.innerHTML = 'Belum ada riwayat.';
  } catch (e) {
    if (attempt < MAX) { el.innerHTML = 'Riwayat gagal dimuat (' + e.message + '). Coba lagi...'; setTimeout(() => loadHistory(attempt + 1, true), 1200 * attempt); return; }
    el.innerHTML = 'Riwayat gagal dimuat (' + e.message + '). Muat ulang halaman (F5).';
  }
}

const deleteArm = {};
$('historyList').addEventListener('click', (e) => {
  const btn = e.target.closest('button[data-id]');
  if (!btn) return;
  const id = parseInt(btn.getAttribute('data-id'), 10);
  const act = btn.getAttribute('data-act');
  if (act === 'hapus') handleDelete(btn, id);
  else if (act === 'lihat') loadResult(id);
  else if (act === 'approve') approveResultFromHistory(id);
});

function handleDelete(btn, id) {
  if (btn.getAttribute('data-approved') === '1') {
    if (deleteArm[id]) {
      delete deleteArm[id];
      reallyDelete(id);
      return;
    }
    deleteArm[id] = true;
    btn.textContent = 'Yakin?';
    btn.classList.add('armed');
    toast('Hasil APPROVED — klik sekali lagi untuk hapus #' + id, 'fail');
    setTimeout(() => { delete deleteArm[id]; btn.textContent = 'Hapus'; btn.classList.remove('armed'); }, 4000);
    return;
  }
  reallyDelete(id);
}

async function reallyDelete(id) {
  try {
    const r = await fetch('/api/mc/results/' + id, { method: 'DELETE' });
    const d = await r.json();
    if (!r.ok || !d.ok) throw new Error(d.error || 'Gagal hapus');
    toast('Compare #' + id + ' dihapus', 'ok');
    if (currentResultId === id) {
      compareResults = [];
      currentResultId = null;
      renderTable();
    }
    loadHistory();
  } catch (e) { toast('Gagal hapus: ' + e.message, 'fail'); }
}

async function loadResult(id) {
  try {
    const r = await fetch('/api/mc/results/' + id);
    const d = await r.json();
    compareResults = d.results || [];
    currentResultId = id;
    currentResultApproved = !!d.approved;
    tableVisible = TABLE_PAGE;
    renderTable();
    $('filterChips').style.display = 'flex';
    $('btnExport').disabled = false;
    updateApproveButton(true);
    $('status').className = 'ok';
    $('status').textContent = '#' + id + ' — ' + d.total + ' baris, ' + d.exist_count + ' EXISTING, ' + d.new_count + ' NEW ITEM' + (d.review_count ? ', ' + d.review_count + ' REVIEW' : '') + (d.approved ? ' \u2014 APPROVED' : '');
    setupReview();
    window.scrollTo({ top: $('tbl').offsetTop - 80, behavior: 'smooth' });
  } catch (e) { toast('Gagal load: ' + e.message, 'fail'); }
}
window.loadResult = loadResult;

async function approveResultFromHistory(id) {
  currentResultId = id;
  currentResultApproved = false;
  $('btnApproveResult').click();
}

async function deleteResult(id) {
  if (!confirm('Hapus riwayat compare #' + id + '? Data item di Masterlist tidak terpengaruh.')) return;
  try {
    const r = await fetch('/api/mc/results/' + id, { method: 'DELETE' });
    const d = await r.json();
    if (!r.ok || !d.ok) throw new Error(d.error || 'Gagal hapus');
    toast('Compare #' + id + ' dihapus', 'ok');
    if (currentResultId === id) {
      compareResults = [];
      currentResultId = null;
      renderTable();
    }
    loadHistory();
  } catch (e) { toast('Gagal hapus: ' + e.message, 'fail'); }
}
window.deleteResult = deleteResult;
window.approveResultFromHistory = approveResultFromHistory;


function fmtBytes(b) { return b < 1024 ? b + ' B' : b < 1048576 ? (b / 1024).toFixed(1) + ' KB' : (b / 1048576).toFixed(1) + ' MB'; }
function toast(msg, type) {
  const t = $('toast'); t.textContent = msg;
  t.className = 'show' + (type === 'ok' ? ' ok' : type === 'fail' ? ' fail' : '');
  clearTimeout(t._t); t._t = setTimeout(() => { t.className = ''; }, 3000);
}


$('navSettingsBtn').addEventListener('click', (e) => { e.stopPropagation(); $('navSettingsDD').classList.toggle('show'); });
document.addEventListener('click', () => $('navSettingsDD').classList.remove('show'));
$('navSettingsDD').addEventListener('click', (e) => e.stopPropagation());

async function checkSuperAdmin() {
  try {
    const d = await (await fetch('/api/auth/me')).json();
    const isSuper = d.loggedIn && d.session && d.session.role === 'superadmin';
    const wrapper = $('navSettingsWrapper');
    if (wrapper) wrapper.style.display = isSuper ? '' : 'none';
  } catch (_) {
    const wrapper = $('navSettingsWrapper');
    if (wrapper) wrapper.style.display = 'none';
  }
}
checkSuperAdmin();


$('revokeBtn').addEventListener('click', async (e) => {
  e.stopPropagation();
  if (!confirm('Putus koneksi Google Drive?\n\nIni akan menghapus token tersimpan.\nKemudian bisa hubungkan akun Google lain lewat menu Ekspor.')) return;
  try {
    const d = await (await fetch('/api/gdrive/revoke', { method: 'POST' })).json();
    if (d.error) throw new Error(d.error);
    toast(d.msg || 'Token Drive dihapus', 'ok');
    $('navSettingsDD').classList.remove('show');
  } catch (err) { toast('Gagal: ' + err.message, 'fail'); }
});

const clearAllOrig = $('clearAllBtn').innerHTML;
let resetArm = false;
$('clearAllBtn').addEventListener('click', async (e) => {
  e.stopPropagation();
  if (!resetArm) {
    resetArm = true;
    $('clearAllBtn').textContent = '⚠ KLIK LAGI untuk RESET';
    $('clearAllBtn').style.cssText = 'background:var(--red);color:#fff;width:100%';
    toast('Konfirmasi: klik tombol Reset lagi', 'fail');
    setTimeout(() => { resetArm = false; $('clearAllBtn').innerHTML = clearAllOrig; $('clearAllBtn').style.cssText = 'color:var(--red)'; }, 4000);
    return;
  }
  resetArm = false;
  $('clearAllBtn').innerHTML = clearAllOrig;
  $('clearAllBtn').style.cssText = 'color:var(--red)';
  $('navSettingsDD').classList.remove('show');
  try {
    const r = await fetch('/api/mc/clear-all', { method: 'POST' });
    const d = await r.json();
    if (!r.ok) throw new Error(d.error);
    plUploadIds = [];
    currentPlUploadId = null;
    currentResultId = null;
    currentResultApproved = false;
    compareResults = [];
    newItems = [];
    $('tbody').innerHTML = '';
    $('tbl').style.display = 'none';
    $('reviewSec').style.display = 'none';
    $('btnCompare').disabled = true;
    $('btnExport').disabled = true;
    $('btnApproveResult').disabled = true;
    $('btnApproveResult').textContent = 'Approve & Simpan';
    $('btnApproveResult').style.opacity = '';
    $('approveStatus').style.display = 'none';
    $('filterChips').style.display = 'none';
    searchQuery = '';
    const si = $('searchInput'); if (si) si.value = '';
    $('searchCount').textContent = '';
    $('status').className = ''; $('status').textContent = '';
    toast('Semua data MC di-reset.', 'ok');
    loadPlUploads();
    loadMasterlists();
    loadHistory();
  } catch (e) { toast('Gagal reset: ' + e.message, 'fail'); }
});


loadPlUploads();
loadMasterlists();
loadHistory();



window.addEventListener('pageshow', (e) => {
  if (e.persisted) { loadPlUploads(); loadMasterlists(); loadHistory(); }
});
