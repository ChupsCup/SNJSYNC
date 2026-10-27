function esc(s) { const d = document.createElement('div'); d.textContent = s; return d.innerHTML; }
const $ = (id) => document.getElementById(id);

let plUploadIds = [];
let compareResults = [];
let currentFilter = 'all';
let currentPlUploadId = null;
let currentResultId = null;
let currentResultApproved = false;
let _reviewGroups = [];

let plPmCtl = null;

function pmBarBuild(progId, barId) {
  const prog = document.getElementById(progId);
  if (!prog) return null;
  const barEl = document.getElementById(barId);
  const wrap = barEl ? barEl.closest('.prog-bar') : null;
  if (wrap) wrap.style.display = 'none';
  const w = document.createElement('div');
  w.style.cssText = 'width:100%;max-width:420px;margin:4px 0 2px';
  if (wrap) wrap.parentNode.insertBefore(w, wrap);
  else prog.appendChild(w);
  return Pm.create(w, { height: 34 });
}

function plPmUpd(p) { if (!plPmCtl) plPmCtl = pmBarBuild('plProg', 'plBar'); if (plPmCtl) plPmCtl.set(p / 100); }
function plPmDone() { if (plPmCtl) plPmCtl.done(); }
function plPmRestart() { if (plPmCtl) plPmCtl.restart(); }


const plDrop = $('plDropZone'), plInput = $('plFileInput');
plDrop.addEventListener('click', () => plInput.click());
plDrop.addEventListener('dragover', (e) => { e.preventDefault(); plDrop.classList.add('dragover'); });
plDrop.addEventListener('dragleave', () => plDrop.classList.remove('dragover'));
plDrop.addEventListener('drop', (e) => { e.preventDefault(); plDrop.classList.remove('dragover'); if (e.dataTransfer.files.length) uploadPL(Array.from(e.dataTransfer.files)); });
plInput.addEventListener('change', (e) => { if (e.target.files.length) uploadPL(Array.from(e.target.files)); });

async function uploadPL(files) {
  $('plProg').style.display = 'block';
  $('plInfo').style.display = 'none';
  plPmRestart();
  let totalAll = 0;
  let sheetsAll = [];

  for (let fi = 0; fi < files.length; fi++) {
    const file = files[fi];
    const ext = file.name.split('.').pop().toLowerCase();
    if (!['xlsx', 'xls', 'csv'].includes(ext)) { toast(file.name + ': bukan file Excel', 'fail'); continue; }

    $('plPct').textContent = (fi + 1) + '/' + files.length;
    $('plBar').style.width = '0%'; $('plBar').classList.remove('ok');
    plPmRestart(); plPmUpd(0);
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
            plPmUpd(p * 0.6);
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
        plPmUpd(60);
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
  plPmDone();
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
          plPmUpd(Math.min(94, 60 + tries * 3));
        }
      } catch (e) {  }
    }, 1000);
  });
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

$('btnCompare').addEventListener('click', () => runCompareNow());
async function runCompareNow() {
  if (!plUploadIds.length && !currentPlUploadId) { toast('Upload packing list dulu!', 'fail'); return; }
  $('btnCompare').disabled = true; $('btnCompare').textContent = 'Compare...';
  $('status').className = '';
  $('status').style.display = 'flex';
$('status').style.alignItems = 'center';
  $('status').style.gap = '9px';
  $('status').style.flexWrap = 'wrap';
  $('status').innerHTML = '<span id="cmpStatusText" style="flex:0 1 auto">Compare berjalan...</span><div id="cmpPmWrap" style="width:190px;height:38px;flex:0 0 auto;line-height:0"></div>';
  if (cmpPmCtl) cmpPmCtl.destroy();
  cmpPmCtl = Pm.create($('cmpPmWrap'), { height: 38, spin: true });
  let cmpSecs = 0;
  clearInterval(compareTimer);
  compareTimer = setInterval(() => {
    cmpSecs++;
    $('cmpStatusText').textContent = 'Compare berjalan... (' + cmpSecs + ' dtk) — matching packing list vs masterlist';
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
    $('status').style.display = '';
    $('status').className = 'ok';
    $('status').textContent = d.total + ' baris \u2014 ' + d.exist + ' EXISTING, ' + d.new + ' NEW ITEM' + (d.review ? ', ' + d.review + ' REVIEW' : '') + (d.skipped ? ' \u2014 ' + d.skipped + ' baris TOTAL di-skip' : '');
    toast('Compare: ' + d.exist + ' EXISTING, ' + d.new + ' NEW ITEM' + (d.review ? ', ' + d.review + ' REVIEW' : '') + (d.skipped ? ' (' + d.skipped + ' baris TOTAL di-skip)' : ''), 'ok');
    loadHistory();
  } catch (e) {
    toast('Gagal: ' + e.message, 'fail');
    $('status').className = 'error'; $('status').textContent = e.message;
  } finally { clearInterval(compareTimer); $('status').style.display = ''; if (cmpPmCtl) { cmpPmCtl.destroy(); cmpPmCtl = null; } $('btnCompare').disabled = false; $('btnCompare').textContent = 'Compare'; }
}
let compareTimer = null;
let cmpPmCtl = null;
let expPmCtl = null;


let tableVisible = 300;

const TABLE_PAGE = 300;
let searchQuery = '';
function rowMatchesSearch(r, toks) {
  const fields = [r.no, r.noContainer, r.brand, r.namaItem, r.itemCode, r.itemDesc,
    r.importir, r.masterCode, r.masterDesc, r.note, r.flag];
  return toks.every((t) => fields.some((f) => String(f || '').toLowerCase().includes(t)));
}
$('searchInput').addEventListener('input', (e) => {
  searchQuery = e.target.value.trim();
  tableVisible = TABLE_PAGE;
  renderTable();
});
let renderToken = 0;
function rowHtml(r, count, g) {
  const isReview = !!r.needsReview;
  const c = isReview ? 'review' : (r.flag === 'NEW ITEM' ? 'new' : 'existing');
  const label = isReview ? 'REVIEW' : r.flag;
  const gk = encodeURIComponent((r.brand || '') + '|' + (r.namaItem || ''));
  const cntTxt = count > 1 ? ' (diterapkan ke ' + count + ' baris produk sama)' : '';
  const confirmBtn = isReview && !currentResultApproved
    ? '<button class="confirm-btn" data-gconfirm="' + gk + '" title="Setujui kandidat: ' + esc(r.candDesc || r.candCode || '') + cntTxt + '">\u2713 Cocok</button>'
      + '<button class="confirm-btn" data-greject="' + gk + '" style="color:var(--red);border-color:color-mix(in srgb, var(--red) 40%, transparent)" title="Tolak kandidat: barang ini memang baru' + cntTxt + '">\u2715 Bukan</button>' : '';
  const gbadge = count > 1 ? ' <span class="gcount" title="' + count + ' baris dengan produk sama (beda no. container)">\u00d7' + count + '</span>' : '';
  return '<tr><td>' + r.no + '</td><td style="font-weight:600">' + esc(r.noContainer) + gbadge + '</td><td>' + esc(r.brand) + '</td><td>' + esc(r.namaItem) + '</td><td>' + esc(r.eta) + '</td><td>' + esc(r.importir || '') + '</td><td><span class="flag ' + c + '">' + label + '</span>' + confirmBtn + '</td><td>' + esc(r.masterCode) + '</td><td>' + esc(r.masterDesc) + '</td><td class="note-cell" title="' + esc(r.note || '') + '">' + esc(r.note || '') + '</td></tr>';
}

function renderTable() {
  let base = compareResults;
  if (searchQuery) {
    const toks = searchQuery.toLowerCase().split(/\s+/).filter(Boolean);
    base = base.filter((r) => rowMatchesSearch(r, toks));
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
  } catch (err) {
    toast('Gagal: ' + err.message, 'fail');
    btn.disabled = false; btn.textContent = old;
  }
}
$('tbody').addEventListener('click', (e) => {
  const btnC = e.target.closest('button[data-gconfirm]');
  if (btnC) { reviewAction('confirm', btnC, decodeURIComponent(btnC.getAttribute('data-gconfirm'))); return; }
  const btnR = e.target.closest('button[data-greject]');
  if (btnR) { reviewAction('reject', btnR, decodeURIComponent(btnR.getAttribute('data-greject'))); return; }
});


$('filterChips').addEventListener('click', (e) => {
  const chip = e.target.closest('.chip'); if (!chip) return;
  currentFilter = chip.dataset.filter;
  $('filterChips').querySelectorAll('.chip').forEach((c) => c.classList.remove('active'));
  chip.classList.add('active');
  tableVisible = TABLE_PAGE;
  renderTable();
});


async function mcOutputName() {
  try {
    const r = await fetch('/api/user/mmd-meta', { cache: 'no-store' });
    const d = await r.json();
    if (d && d.cutoff) return 'Output Hasil MC ' + d.cutoff + '.xlsx';
  } catch (_) {}
  return 'Output Hasil MC.xlsx';
}
$('btnExport').addEventListener('click', async () => {
  if (!compareResults.length) { toast('Compare dulu!', 'fail'); return; }
  const b = $('btnExport');
  b.disabled = true; b.textContent = 'Export...';
  const as = $('actionStatus');
  as.style.display = 'flex'; as.style.alignItems = 'center'; as.style.gap = '9px'; as.style.flexWrap = 'wrap';
  as.innerHTML = '<span style="flex:0 1 auto">Membuat file Excel export...</span><div id="exPmWrap" style="width:190px;height:38px;flex:0 0 auto;line-height:0"></div>';
  if (expPmCtl) expPmCtl.destroy();
  expPmCtl = Pm.create($('exPmWrap'), { height: 38, spin: true });
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
    if (expPmCtl) { expPmCtl.destroy(); expPmCtl = null; }
    as.style.display = 'none'; as.innerHTML = '';
    b.disabled = false; b.textContent = 'Export';
  }
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
  else if (act === 'lihat') loadResult(id, btn);
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

async function loadResult(id, btn) {
  if (btn) {
    btn.disabled = true;
    btn.classList.add('loading');
    btn.innerHTML = '<span class="spin"></span>Memuat';
  }
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
    $('status').className = 'ok';
    $('status').textContent = '#' + id + ' — ' + d.total + ' baris, ' + d.exist_count + ' EXISTING, ' + d.new_count + ' NEW ITEM' + (d.review_count ? ', ' + d.review_count + ' REVIEW' : '') + (d.approved ? ' \u2014 APPROVED' : '');
    window.scrollTo({ top: $('tbl').offsetTop - 80, behavior: 'smooth' });
  } catch (e) { toast('Gagal load: ' + e.message, 'fail'); }
  finally {
    if (btn) {
      btn.disabled = false;
      btn.classList.remove('loading');
      btn.textContent = 'Lihat';
    }
  }
}
window.loadResult = loadResult;

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
    $('tbody').innerHTML = '';
    $('tbl').style.display = 'none';
    $('btnCompare').disabled = true;
    $('btnExport').disabled = true;
    $('filterChips').style.display = 'none';
    searchQuery = '';
    const si = $('searchInput'); if (si) si.value = '';
    $('searchCount').textContent = '';
    $('status').className = ''; $('status').textContent = '';
    toast('Semua data MC di-reset.', 'ok');
    loadPlUploads();
    loadHistory();
  } catch (e) { toast('Gagal reset: ' + e.message, 'fail'); }
});


loadPlUploads();
loadHistory();



window.addEventListener('pageshow', (e) => {
  if (e.persisted) { loadPlUploads(); loadHistory(); }
});

$('aliasBtn').addEventListener('click', async () => {
  $('navSettingsDD').classList.remove('show');
  $('aliasModal').classList.add('show');
  const inp = $('aliasInput');
  inp.value = '';
  inp.focus();
  await loadAliases();
});

$('aliasClose').addEventListener('click', () => $('aliasModal').classList.remove('show'));

$('aliasModal').addEventListener('click', (e) => {
  if (e.target === $('aliasModal')) $('aliasModal').classList.remove('show');
});

async function loadAliases() {
  const box = $('aliasList');
  try {
    const r = await fetch('/api/mc/aliases', { cache: 'no-store' });
    const d = await r.json();
    if (!r.ok) throw new Error(d.error);
    const list = d.aliases || [];
    if (!list.length) { box.innerHTML = '<div class="alias-empty">Belum ada alias.</div>'; return; }
    box.innerHTML = list.map((a) => '<div class="alias-item"><span class="alias-txt">' + esc(a.a) + '</span><span class="alias-eq">=</span><span class="alias-txt">' + esc(a.b) + '</span><button class="alias-del" data-id="' + a.id + '" title="Hapus">\u2715</button></div>').join('');
  } catch (e) {
    box.innerHTML = '<div class="alias-empty">Gagal load: ' + esc(e.message) + '</div>';
  }
}

$('aliasList').addEventListener('click', async (e) => {
  const btn = e.target.closest('.alias-del');
  if (!btn) return;
  btn.disabled = true;
  try {
    const r = await fetch('/api/mc/aliases/delete', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ id: Number(btn.dataset.id) }) });
    const d = await r.json();
    if (!r.ok) throw new Error(d.error);
    toast('Alias dihapus.', 'ok');
    await loadAliases();
  } catch (err) { btn.disabled = false; toast('Gagal hapus: ' + err.message, 'fail'); }
});

async function applyAlias() {
  const raw = $('aliasInput').value.trim();
  if (!raw) { toast('Isi dulu: kata lama = kata baru', 'fail'); return; }
  const i = raw.indexOf('=');
  if (i < 1) { toast('Format: kiri = kanan', 'fail'); return; }
  const a = raw.slice(0, i).trim();
  const b = raw.slice(i + 1).trim();
  if (!a || !b) { toast('Dua sisi harus terisi', 'fail'); return; }
  const btn = $('aliasApply');
  btn.disabled = true; btn.textContent = 'Menyimpan...';
  try {
    const r = await fetch('/api/mc/aliases', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ a: a, b: b }) });
    const d = await r.json();
    if (!r.ok) throw new Error(d.error);
    $('aliasInput').value = '';
    toast('Alias tersimpan: ' + d.a + ' = ' + d.b, 'ok');
    await loadAliases();
    if ($('aliasAutoCompare').checked && (plUploadIds.length || currentPlUploadId)) {
      $('aliasModal').classList.remove('show');
      setTimeout(() => runCompareNow(), 250);
    }
  } catch (e) { toast('Gagal: ' + e.message, 'fail'); }
  finally { btn.disabled = false; btn.textContent = 'Apply'; }
}

$('aliasApply').addEventListener('click', applyAlias);

$('aliasInput').addEventListener('keydown', (e) => {
  if (e.key === 'Enter') { e.preventDefault(); applyAlias(); }
});

$('ignoreBtn').addEventListener('click', async () => {
  $('navSettingsDD').classList.remove('show');
  $('ignoreModal').classList.add('show');
  const inp = $('ignoreInput');
  inp.value = '';
  inp.focus();
  await loadIgnores();
});

$('ignoreClose').addEventListener('click', () => $('ignoreModal').classList.remove('show'));

$('ignoreModal').addEventListener('click', (e) => {
  if (e.target === $('ignoreModal')) $('ignoreModal').classList.remove('show');
});

async function loadIgnores() {
  const box = $('ignoreList');
  try {
    const r = await fetch('/api/mc/ignores', { cache: 'no-store' });
    const d = await r.json();
    if (!r.ok) throw new Error(d.error);
    const list = d.words || [];
    if (!list.length) { box.innerHTML = '<div class="alias-empty">Belum ada kata diabaikan.</div>'; return; }
    box.innerHTML = list.map((a) => '<div class="alias-item"><span class="alias-txt">' + esc(a.word) + '</span><button class="alias-del" data-id="' + a.id + '" title="Hapus">\u2715</button></div>').join('');
  } catch (e) {
    box.innerHTML = '<div class="alias-empty">Gagal load: ' + esc(e.message) + '</div>';
  }
}

$('ignoreList').addEventListener('click', async (e) => {
  const btn = e.target.closest('.alias-del');
  if (!btn) return;
  btn.disabled = true;
  try {
    const r = await fetch('/api/mc/ignores/delete', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ id: Number(btn.dataset.id) }) });
    const d = await r.json();
    if (!r.ok) throw new Error(d.error);
    toast('Kata dihapus.', 'ok');
    await loadIgnores();
  } catch (err) { btn.disabled = false; toast('Gagal hapus: ' + err.message, 'fail'); }
});

async function applyIgnore() {
  const word = $('ignoreInput').value.trim();
  if (!word) { toast('Isi satu kata', 'fail'); return; }
  const btn = $('ignoreApply');
  btn.disabled = true; btn.textContent = 'Menyimpan...';
  try {
    const r = await fetch('/api/mc/ignores', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ word: word }) });
    const d = await r.json();
    if (!r.ok) throw new Error(d.error);
    $('ignoreInput').value = '';
    toast('Kata diabaikan: ' + d.word, 'ok');
    await loadIgnores();
    if ($('ignoreAutoCompare').checked && (plUploadIds.length || currentPlUploadId)) {
      $('ignoreModal').classList.remove('show');
      setTimeout(() => runCompareNow(), 250);
    }
  } catch (e) { toast('Gagal: ' + e.message, 'fail'); }
  finally { btn.disabled = false; btn.textContent = 'Apply'; }
}

$('ignoreApply').addEventListener('click', applyIgnore);

$('ignoreInput').addEventListener('keydown', (e) => {
  if (e.key === 'Enter') { e.preventDefault(); applyIgnore(); }
});

