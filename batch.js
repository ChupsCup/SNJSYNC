const $ = (id) => document.getElementById(id);
const csvBox = $('csvBox'), btnCheck = $('btnCheck'), btnExp = $('btnExp');
const tbody = $('tbody'), sumChips = $('sumChips'), gdot = $('gdot');

let results = [];
let picks = {}; 

function esc(s) { const d = document.createElement('div'); d.textContent = s; return d.innerHTML; }
function photoUrl(id) { return '/api/photo?id=' + encodeURIComponent(id) + '&v=20261007'; }
function toast(msg, cls) {
  const t = $('toast');
  t.textContent = msg; t.className = 'show' + (cls ? ' ' + cls : '');
  setTimeout(() => { t.className = ''; }, 4000);
}
function openBatchModal(c) {
  $('bmTitle').textContent = c.name || '';
  $('bmImg').src = photoUrl(c.id);
  $('bmInfo').textContent = 'Folder: ' + (c.dir || '') + (c.text ? '\nOCR: ' + c.text.slice(0, 200) : '');
  $('batchModal').style.display = 'flex';
}
$('bmClose').onclick = () => { $('batchModal').style.display = 'none'; };
$('batchModal').onclick = (e) => { if (e.target === $('batchModal')) $('batchModal').style.display = 'none'; };
function openFullscreen(c) {
  $('fsImg').src = photoUrl(c.id);
  const info = 'Nama: ' + (c.name || '') + '\nFolder: ' + (c.dir || '') + (c.text ? '\nOCR: ' + c.text.slice(0, 300) : '');
  $('fsInfo').textContent = info;
  $('fsOverlay').style.display = 'flex';
  const img = $('fsImg');
  if (img.complete && img.naturalWidth) fsCenter();
}
$('fsBack').onclick = () => { $('fsOverlay').style.display = 'none'; };
$('fsOverlay').onclick = (e) => { if (e.target === $('fsOverlay')) $('fsOverlay').style.display = 'none'; };
let fsScale = 1, fsOx = 0, fsOy = 0, fsDrag = false, fsLastX = 0, fsLastY = 0;
function fsCenter() {
  const ov = $('fsOverlay'), img = $('fsImg');
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
function parseCsv(text) {
  const rows = [];
  for (const line of String(text || '').split(/\r?\n/)) {
    const cells = [];
    let cur = '', q = false;
    for (let i = 0; i < line.length; i++) {
      const ch = line[i];
      if (q) { if (ch === '"') { if (line[i + 1] === '"') { cur += '"'; i++; } else q = false; } else cur += ch; }
      else if (ch === '"') q = true;
      else if (ch === ',' || ch === ';' || ch === '\t') { cells.push(cur); cur = ''; }
      else cur += ch;
    }
    cells.push(cur);
    const name = (cells[0] || '').trim(), desc = (cells[1] || '').trim();
    if (name || desc) rows.push({ name, desc });
  }
  return rows;
}

function render() {
  let ada = 0, tidak = 0, picked = 0;
  const usedIds = new Set();
  for (const k in picks) if (picks[k]) usedIds.add(picks[k].id);
  tbody.innerHTML = '';
  results.forEach((r, i) => {
    const hasPick = !!picks[i];
    if (r.status === 'ada') ada++; else tidak++;
    if (hasPick) picked++;
    const pick = picks[i] || null;
    const tr = document.createElement('tr');
    tr.innerHTML =
      '<td>' + (i + 1) + '</td>' +
      '<td><b>' + esc(r.row.name) + '</b>' + (r.row.desc ? '<br><span style="color:var(--text2)">' + esc(r.row.desc) + '</span>' : '') + '</td>' +
      '<td>' + (r.ests.length ? r.ests.map((e) => '<span class="estchip">EST ' + e + '</span>').join('') : '&mdash;') + '</td>' +
      '<td><span class="status ' + r.status + '">' + (r.status === 'ada' ? 'ADA' : 'TIDAK') + '</span></td>' +
      '<td style="color:var(--text2)">' + esc(r.note) + '</td>' +
      '<td><div style="display:flex;gap:6px;flex-wrap:wrap;max-width:420px">' +
      (r.cands.length
        ? r.cands.map((c, ci) => ({ c, ci, sort: c.labelScore * 100 + c.score })).sort((a, b) => b.sort - a.sort).map(({ c, ci }) => {
            const sel = pick && pick.id === c.id;
            const blk = usedIds.has(c.id) && !sel;
            const qBadge = c.labelScore >= 70 ? '<span style="position:absolute;top:2px;right:2px;background:#1a9649;color:#fff;font-size:8px;padding:1px 3px;border-radius:3px">Label</span>'
              : c.labelScore > 0 ? '<span style="position:absolute;top:2px;right:2px;background:#e6a817;color:#fff;font-size:8px;padding:1px 3px;border-radius:3px">' + c.labelScore + '%</span>'
              : '<span style="position:absolute;top:2px;right:2px;background:#7f8c8d;color:#fff;font-size:8px;padding:1px 3px;border-radius:3px">?</span>';
            return '<div style="text-align:center;position:relative" title="' + esc(c.folder) + (c.text ? '\n' + c.text.slice(0, 100) : '') + '">' +
              '<img class="cand-thumb' + (sel ? ' sel' : '') + '" src="' + photoUrl(c.id) + '" data-i="' + i + '" data-ci="' + ci + '" style="' + (blk ? 'opacity:.3' : '') + '">' +
              qBadge +
              '<div style="font-size:9px;color:var(--text2);max-width:56px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">' + esc(c.folder) + '</div>' +
              '</div>';
          }).join('')
        : '<span style="color:var(--text2)">Tidak ada kandidat</span>') +
      '</div>' + (hasPick ? '<div style="color:var(--green);font-weight:700;margin-top:4px">\u2714 Terpilih: ' + esc(pick.name) + '</div>' : '') +
      '</td>';
    tr.querySelectorAll('.cand-thumb').forEach((img) => {
      img.onclick = (e) => {
        const i2 = +img.dataset.i, ci2 = +img.dataset.ci;
        const c = results[i2].cands[ci2];
        if (e.shiftKey || e.ctrlKey || e.metaKey) {
          openFullscreen(c);
          return;
        }
        if (pick && pick.id === c.id) { delete picks[i2]; }
        else {
          const conflict = Object.keys(picks).find((k) => picks[k] && picks[k].id === c.id);
          if (conflict) { toast('Foto itu sudah dipakai baris ' + (+conflict + 1) + '.', 'fail'); return; }
          const rowName = results[i2].row.name || results[i2].row.desc || 'foto';
          picks[i2] = { id: c.id, name: c.name, path: c.path, exportName: rowName };
        }
        render();
      };
      img.ondblclick = () => {
        const i2 = +img.dataset.i, ci2 = +img.dataset.ci;
        openFullscreen(results[i2].cands[ci2]);
      };
    });
    tbody.appendChild(tr);
  });
  sumChips.innerHTML =
    '<span class="chip g"><span class="v">' + ada + '</span> ADA (cocok persis)</span>' +
    '<span class="chip r"><span class="v">' + tidak + '</span> TIDAK</span>' +
    '<span class="chip b"><span class="v">' + picked + '</span> Foto terpilih</span>';
  btnExp.disabled = picked === 0;
  btnExp.textContent = 'Ekspor ' + picked + ' Foto Terpilih ke Drive';
}

btnCheck.onclick = async () => {
  const rows = parseCsv(csvBox.value);
  if (!rows.length) { toast('Kotak kosong — paste dari Excel dulu.', 'fail'); return; }
  const g = await (await fetch('/api/gdrive/status')).json();
  if (!g.configured || !g.authorized) {
    if (!g.admin) { toast('Google belum dikonfigurasi admin. Hubungi pemilik web.', 'fail'); return; }
    openGdriveModal();
    return;
  }
  btnCheck.disabled = true;
  btnCheck.innerHTML = '<span class="spinner"></span>Memeriksa...';
  picks = {};
  try {
    const d = await (await fetch('/api/batch/check', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ rows })
    })).json();
    if (d.error) throw new Error(d.error);
    results = d.results;
    $('secResult').style.display = 'block';
    render();
    toast('Selesai: ' + d.ada + ' ada, ' + d.tidak + ' tidak (' + d.ms + ' ms). Review dulu, baru ekspor!', 'ok');
  } catch (err) {
    toast('Error: ' + err.message, 'fail');
  }
  btnCheck.disabled = false;
  btnCheck.textContent = 'Periksa ke Index';
};


const expModal = $('expModal'), expStatus = $('expStatus');
const expFolder = $('expFolder'), expRun = $('expRun'), expResult = $('expResult');
let authPoller = null;
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
    expFolder.innerHTML = '<option value="">Pilih folder tujuan...</option>' + d.files.map((f) => '<option value="' + esc(f.id) + '">' + esc(f.name) + '</option>').join('');
  } catch (err) { expStatus.textContent = 'Error: ' + err.message; }
}
async function checkGdrive() {
  expStatus.textContent = 'Memeriksa koneksi Google Drive...';
  try {
    const d = await (await fetch('/api/gdrive/status')).json();
    if (!d.configured) { showStep(1); expStatus.textContent = 'Langkah 1: pilih client_secret.json.'; return; }
    if (!d.authorized) { showStep(2); expStatus.textContent = 'Langkah 2: hubungkan akun Google.'; return; }
    if (authPoller) { clearInterval(authPoller); authPoller = null; }
    showStep(3);
    expStatus.textContent = 'Terhubung sebagai ' + (d.email || 'akun') + '.';
    $('authBanner').style.display = 'none';
    await loadFolders();
  } catch (err) { expStatus.textContent = 'Error: ' + err.message; }
}
function openGdriveModal() {
  expResult.innerHTML = ''; expResult.className = '';
  expModal.style.display = 'flex';
  checkGdrive();
}
btnExp.onclick = () => {
  expResult.innerHTML = ''; expResult.className = '';
  expModal.style.display = 'flex';
  checkGdrive();
};
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
expRun.onclick = async () => {
  const folderId = expFolder.value;
  if (!folderId) { expStatus.textContent = 'Pilih folder tujuan dulu.'; return; }
  const items = Object.keys(picks).map((k) => picks[k]);
  expRun.disabled = true;
  expRun.innerHTML = '<span class="spinner"></span>Mengirim...';
  expResult.innerHTML = ''; expResult.className = '';
  try {
    const d = await (await fetch('/api/batch/export', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ folderId, items })
    })).json();
    if (d.error) throw new Error(d.error);
    expResult.className = d.fails.length ? 'fail' : 'ok';
    expResult.innerHTML = '\u2714 ' + d.done.length + ' foto terkirim ke Google Drive.' +
      (d.fails.length ? '<br>\u2716 Gagal: ' + esc(d.fails.join(' | ')) : '') +
      (d.done.length ? ' <a href="' + esc(d.done[0].link) + '" target="_blank">Buka foto pertama</a>' : '');
    expRun.disabled = false;
    expRun.textContent = 'Ekspor ' + items.length + ' Foto';
  } catch (err) {
    expResult.className = 'fail';
    expResult.innerHTML = 'Error: ' + esc(err.message);
    expRun.disabled = false;
    expRun.textContent = 'Ekspor ' + items.length + ' Foto';
  }
};

async function refreshGdriveDot() {
  try {
    const g = await (await fetch('/api/gdrive/status')).json();
    gdot.className = 'gdot ' + (g.authorized ? 'on' : (g.configured ? 'off' : ''));
    $('authBanner').style.display = (!g.configured || !g.authorized) ? 'flex' : 'none';
  } catch (_) {}
}
refreshGdriveDot();
setInterval(refreshGdriveDot, 30000);

const _idxBtn = $('idxBtn');
if (_idxBtn) _idxBtn.addEventListener('click', async () => {
  _idxBtn.disabled = true;
  try {
    const d = await (await fetch('/api/index/refresh?force=1')).json();
    if (d.error) throw new Error(d.error);
    let msg = 'Sinkron Foto: ' + d.total.toLocaleString('id-ID') + ' foto';
    if (d.withOcr > 0) msg += ', OCR ' + d.withOcr.toLocaleString('id-ID');
    if (d.indexError) msg += ' (' + d.indexError + ')';
    toast(msg, 'ok');
  } catch (err) {
    toast('Gagal: ' + err.message, 'fail');
  }
  _idxBtn.disabled = false;
});

const _reconnectBtn = $('authReconnect');
if (_reconnectBtn) {
  _reconnectBtn.addEventListener('click', async () => {
    const g = await (await fetch('/api/gdrive/status')).json();
    if (g.configured && g.authorized) { $('authBanner').style.display = 'none'; return; }
    openGdriveModal();
  });
}