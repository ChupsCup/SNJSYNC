








const $ = id => document.getElementById(id);

function esc(s) {
  return String(s == null ? '' : s).replace(/[&<>"']/g, c => (
    { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]
  ));
}

  function say(el, text, ok) {
    el.className = 'msg ' + (ok ? 'ok' : 'err');
    el.textContent = text;
  }

  
  const EYE = '<path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7Z"/><circle cx="12" cy="12" r="3"/>';
  const EYE_OFF = '<path d="M10.7 5.1A10.9 10.9 0 0 1 12 5c6.5 0 10 7 10 7a17.7 17.7 0 0 1-3.2 4.3M6.6 6.6A17.4 17.4 0 0 0 2 12s3.5 7 10 7a10.7 10.7 0 0 0 4.5-.9"/><path d="M3 3l18 18"/><path d="M9.9 9.9a3 3 0 0 0 4.2 4.2"/>';

  function setEye(btn, shown) {
    const input = $(btn.dataset.eye);
    const icon = btn.querySelector('svg');
    input.type = shown ? 'text' : 'password';
    icon.innerHTML = shown ? EYE_OFF : EYE;
    btn.setAttribute('aria-pressed', String(shown));
    btn.setAttribute('aria-label', shown ? 'Sembunyikan password' : 'Tampilkan password');
    btn.title = shown ? 'Sembunyikan password' : 'Tampilkan password';
  }

  document.querySelectorAll('.pw-eye').forEach(btn => {
    btn.addEventListener('click', () => {
      const input = $(btn.dataset.eye);
      const shown = input.type === 'password';
      setEye(btn, shown);
      if (shown) {
        input.focus();
        input.setSelectionRange(input.value.length, input.value.length);
      }
    });
  });

async function api(path, params) {
  const fd = new URLSearchParams(params || {});
  const r = await fetch(path, {
    method: params ? 'POST' : 'GET',
    body: params ? fd : undefined,
    cache: 'no-store',
  });
  return { ok: r.ok, status: r.status, d: await r.json().catch(() => ({})) };
}





let cache = [];
let filterStatus = '';
let sortMode = 'role';

async function load() {
  const res = await api('/api/auth/accounts');
  if (!res.ok) {
    $('rows').innerHTML =
      '<tr><td colspan="7" class="empty">' +
      esc(res.d.error || 'Gagal memuat daftar akun.') + '</td></tr>';
    say($('lMsg'), res.d.error || 'Gagal memuat daftar akun.', false);
    $('btnCreate').disabled = true;
    return;
  }
  cache = res.d.accounts || [];
  if (!cache.length) {
    $('rows').innerHTML = '<tr><td colspan="7" class="empty">Belum ada akun.</td></tr>';
    updateStat();
    return;
  }
  render();
}

function render() {
  let list = cache.slice();
  if (filterStatus) list = list.filter(a => a.status === filterStatus);
  if (sortMode === 'idle') {
    list.sort((a, b) => {
      const x = a.idleDays < 0 ? Number.MAX_SAFE_INTEGER : a.idleDays;
      const y = b.idleDays < 0 ? Number.MAX_SAFE_INTEGER : b.idleDays;
      return y - x;
    });
  }
  $('rows').innerHTML = list.length
    ? list.map(row).join('')
    : '<tr><td colspan="7" class="empty">Tidak ada akun pada filter ini.</td></tr>';
  updateStat();
}

function updateStat() {
  const c = { aktif: 0, jarang: 0, mati: 0, belum: 0 };
  cache.forEach(a => { c[a.status] = (c[a.status] || 0) + 1; });
  $('fStat').innerHTML =
    '<span class="badge aktif">Aktif ' + c.aktif + '</span>' +
    '<span class="badge jarang">Jarang ' + c.jarang + '</span>' +
    '<span class="badge mati">Tidak aktif ' + c.mati + '</span>' +
    (c.belum ? '<span class="badge belum">Belum ' + c.belum + '</span>' : '');
}

function statusInfo(a) {
  switch (a.status) {
    case 'aktif': return { cls: 'aktif', label: 'Aktif' };
    case 'jarang': return { cls: 'jarang', label: 'Jarang' };
    case 'mati': return { cls: 'mati', label: 'Tidak aktif' };
    default: return { cls: 'belum', label: 'Belum pernah' };
  }
}

function agoText(a) {
  if (a.idleDays == null || a.idleDays < 0) return 'belum pernah';
  if (a.idleDays === 0) return 'hari ini';
  if (a.idleDays === 1) return 'kemarin';
  return a.idleDays + ' hari lalu';
}

function row(a) {
  const live = a.sesiAktif > 0
    ? '<span class="badge live">' + a.sesiAktif + ' aktif</span>'
    : '<span class="muted">tidak aktif</span>';
  const you = a.isYou ? ' <span class="badge you">kamu</span>' : '';
  const st = statusInfo(a);
  const keaktifan =
    '<span class="badge ' + st.cls + '">' + st.label + '</span>' +
    '<div class="muted" style="margin-top:3px">' + a.loginTotal + ' login' +
      (a.loginTotal ? ' \u00b7 30hr: ' + a.login30 : '') + '</div>';
  const seenTip = a.lastSeen
    ? ' title="Terakhir dipakai: ' + esc(a.lastSeen) + '"'
    : (a.lastLogin ? ' title="Login terakhir: ' + esc(a.lastLogin) + '"' : '');

  const superCount = cache.filter(x => x.role === 'superadmin').length;
  const lastSuper = a.role === 'superadmin' && superCount <= 1;
  const delDisabled = a.isYou || lastSuper ? ' disabled' : '';
  const delTitle = a.isYou
    ? 'Tidak bisa menghapus akun sendiri.'
    : (lastSuper ? 'Harus ada minimal satu Super Admin.' : 'Hapus akun ini.');

  return '<tr data-id="' + a.id + '">' +
    '<td class="u">' + esc(a.username) + you + '</td>' +
    '<td>' + roleBadge(a) + '</td>' +
    '<td>' + moduleBadges(a) + '</td>' +
    '<td>' + keaktifan + '</td>' +
    '<td class="muted"' + seenTip + '>' + esc(agoText(a)) + '</td>' +
    '<td>' + live + '</td>' +
    '<td><div class="row-actions">' +
      '<button class="ghost" data-act="edit">Ubah</button>' +
      '<button class="ghost" data-act="act">Aktivitas</button>' +
      '<button class="ghost" data-act="out"' + (a.sesiAktif > 0 ? '' : ' disabled') +
        ' title="Keluarkan akun ini dari semua perangkat/browser sekaligus.">Keluar semua</button>' +
      '<button class="ghost danger" data-act="del" title="' + esc(delTitle) + '"' + delDisabled + '>Hapus</button>' +
    '</div></td>' +
  '</tr>';
}

function roleBadge(a) {
  if (a.role === 'superadmin') return '<span class="badge super">Super Admin</span>';
  if (a.role === 'user') return '<span class="badge admin">User</span>';
  return '<span class="badge admin">Admin</span>';
}

const MOD_NAMES = { ctf: 'Cari Foto', mc: 'Master Item', mmd: 'Masterlist' };

function moduleBadges(a) {
  if (a.role !== 'user') return '<span class="muted">semua</span>';
  const mods = a.modules || [];
  if (!mods.length) return '<span class="muted">-</span>';
  return '<div class="mod-badges">' + mods.map(m =>
    '<span class="badge">' + esc(MOD_NAMES[m] || m) + '</span>').join('') + '</div>';
}

function checkedMods(prefix) {
  return Array.from(document.querySelectorAll('.' + prefix + 'Mod:checked')).map(x => x.value);
}
function setModChecks(prefix, mods) {
  const set = new Set(mods || []);
  document.querySelectorAll('.' + prefix + 'Mod').forEach(x => { x.checked = set.has(x.value); });
}
function setModWrap(role, prefix) {
  const wrap = $(prefix + 'ModWrap');
  if (wrap) wrap.style.display = role === 'user' ? '' : 'none';
}





let editId = null;

  function openEdit(id) {
    const a = cache.find(x => x.id === id);
    if (!a) return;
    editId = id;
    $('eUser').value = a.username;
    $('ePass').value = '';
    
    setEye(document.querySelector('.pw-eye[data-eye="ePass"]'), false);
  $('eRole').value = a.role;
  $('eRole').disabled = a.isYou;
  setModChecks('e', a.modules);
  setModWrap(a.role, 'e');
  $('eNote').textContent = a.isYou
    ? 'Ini akun kamu. Ganti username / password boleh; role terkunci agar kamu tidak mengunci diri sendiri dari halaman ini.'
    : 'Kosongkan password kalau tidak ingin mengubahnya. Mengubah akun ini akan mengeluarkan semua sesinya di browser lain.';
  $('btnSave').disabled = false;
  $('btnCancel').hidden = false;
  $('editSec').scrollIntoView({ behavior: 'smooth', block: 'center' });
  $('eUser').focus();
}

  function closeEdit() {
    editId = null;
    $('eUser').value = '';
    $('ePass').value = '';
    setEye(document.querySelector('.pw-eye[data-eye="ePass"]'), false);
    $('btnSave').disabled = true;
  $('btnCancel').hidden = true;
  say($('eMsg'), '', true);
  $('eMsg').className = 'msg';
}





$('rows').addEventListener('click', async e => {
  const btn = e.target.closest('button[data-act]');
  if (!btn) return;
  const tr = btn.closest('tr');
  const id = +tr.dataset.id;
  const a = cache.find(x => x.id === id);
  if (!a) return;

  if (btn.dataset.act === 'edit') { openEdit(id); return; }

  if (btn.dataset.act === 'act') { openActivity(id); return; }

  if (btn.dataset.act === 'out') {
    if (btn.disabled) return;
    if (!confirm(
      'Keluarkan akun "' + a.username + '" dari semua perangkat/browser?\n\n' +
      'Sesi aktifnya (' + a.sesiAktif + ') langsung berakhir; yang tidak '
      + 'diperbolehkan harus login ulang.')) return;
    btn.disabled = true;
    const res = await api('/api/auth/accounts/logout-all', { id });
    if (!res.ok) {
      btn.disabled = false;
      say($('lMsg'), res.d.error || 'Gagal mengeluarkan.', false);
      return;
    }
    if (editId === id) closeEdit();
    say($('lMsg'), res.d.pesan || 'Sesi dikeluarkan.', true);
    await load();
  }

  if (btn.dataset.act === 'del') {
    if (btn.disabled) return;
    if (!confirm(
      'Hapus akun "' + a.username + '"?\n\n' +
      'Akun ini langsung tidak bisa login lagi dan semua sesinya langsung ' +
      'berakhir, termasuk sesi di browser lain.')) return;
    btn.disabled = true;
    const res = await api('/api/auth/accounts/delete', { id });
    if (!res.ok) {
      btn.disabled = false;
      say($('lMsg'), res.d.error || 'Gagal menghapus.', false);
      return;
    }
    if (editId === id) closeEdit();
    say($('lMsg'), res.d.pesan || 'Akun dihapus.', true);
    await load();
  }
});





$('btnCreate').addEventListener('click', async () => {
  const btn = $('btnCreate');
  const username = $('cUser').value.trim();
  const password = $('cPass').value;
  const role = $('cRole').value;
  if (!username || !password) {
    say($('cMsg'), 'Username dan password wajib diisi.', false);
    return;
  }
  if (role === 'user' && !checkedMods('c').length) {
    say($('cMsg'), 'Pilih minimal satu modul untuk akun User.', false);
    return;
  }
  btn.disabled = true;
  btn.textContent = 'Membuat...';
  const res = await api('/api/auth/accounts/create', {
    username, password, role,
    modules: role === 'user' ? checkedMods('c').join(',') : '',
  });
  btn.disabled = false;
  btn.textContent = 'Buat Akun';
  if (!res.ok) {
    say($('cMsg'), res.d.error || 'Gagal membuat akun.', false);
    return;
  }
    say($('cMsg'), res.d.pesan || 'Akun dibuat.', true);
    $('cUser').value = '';
    $('cPass').value = '';
    setEye(document.querySelector('.pw-eye[data-eye="cPass"]'), false);
    setModChecks('c', []);
    await load();
});





$('btnSave').addEventListener('click', async () => {
  if (editId == null) return;
  const btn = $('btnSave');
  const username = $('eUser').value.trim();
  const password = $('ePass').value;
  const role = $('eRole').value;

  const effRole = $('eRole').disabled ? null : role;
  if (effRole === 'user' && !checkedMods('e').length) {
    say($('eMsg'), 'Pilih minimal satu modul untuk akun User.', false);
    return;
  }
  btn.disabled = true;
  btn.textContent = 'Menyimpan...';
  const res = await api('/api/auth/accounts/update', {
    id: editId,
    username: username,
    password: password,
    role: $('eRole').disabled ? '-' : role,
    modules: checkedMods('e').join(','),
  });
  btn.disabled = false;
  btn.textContent = 'Simpan Perubahan';
  if (!res.ok) {
    say($('eMsg'), res.d.error || 'Gagal menyimpan.', false);
    return;
  }
  say($('eMsg'), res.d.pesan || 'Tersimpan.', true);
  $('ePass').value = '';
  await load();
  
  openEdit(editId);
});

$('btnCancel').addEventListener('click', closeEdit);

$('cRole').addEventListener('change', () => setModWrap($('cRole').value, 'c'));
$('eRole').addEventListener('change', () => setModWrap($('eRole').value, 'e'));
setModWrap($('cRole').value, 'c');

function actStat(n, l) {
  return '<div class="act-stat"><div class="n">' + n + '</div><div class="l">' + esc(l) + '</div></div>';
}

async function openActivity(id) {
  const a = cache.find(x => x.id === id);
  if (!a) return;
  const st = statusInfo(a);
  $('actTitle').textContent = 'Aktivitas Login \u2014 ' + a.username;
  $('actSub').innerHTML =
    '<span class="badge ' + st.cls + '">' + st.label + '</span>' +
    '<span>Terakhir dipakai: ' + esc(a.lastSeen || a.lastLogin || 'belum pernah') + '</span>';
  $('actSum').innerHTML = '<div class="act-stat"><div class="n">\u2026</div><div class="l">memuat</div></div>';
  $('actChart').innerHTML = '';
  $('actOvl').hidden = false;

  const res = await api('/api/auth/accounts/activity?id=' + id);
  if (!res.ok) {
    $('actSum').innerHTML = '<div class="act-stat"><div class="n">!</div><div class="l">' +
      esc(res.d.error || 'Gagal memuat') + '</div></div>';
    return;
  }
  const d = res.d;
  const days = d.days || [];
  const max = days.reduce((m, x) => Math.max(m, x.count), 0);
  $('actSum').innerHTML =
    actStat(d.total30, 'Login 30 hari') +
    actStat(d.total, 'Total login') +
    actStat(a.sesiAktif, 'Sesi aktif') +
    actStat(max, 'Terbanyak/hari');
  $('actChart').innerHTML = days.map(x => {
    const h = x.count ? Math.max(5, Math.round(x.count / max * 100)) : 2;
    return '<div class="bar' + (x.count ? '' : ' zero') + '" style="height:' + h + '%" title="' +
      esc(x.date) + ': ' + x.count + ' login"></div>';
  }).join('');
}

$('actClose').addEventListener('click', () => { $('actOvl').hidden = true; });
$('actOvl').addEventListener('click', e => {
  if (e.target === $('actOvl')) $('actOvl').hidden = true;
});
document.addEventListener('keydown', e => {
  if (e.key === 'Escape' && !$('actOvl').hidden) $('actOvl').hidden = true;
});

$('fStatus').addEventListener('change', () => {
  filterStatus = $('fStatus').value;
  render();
});
$('fSort').addEventListener('change', () => {
  sortMode = $('fSort').value;
  render();
});





load();