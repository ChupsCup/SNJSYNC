








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

async function load() {
  const res = await api('/api/auth/accounts');
  if (!res.ok) {
    $('rows').innerHTML =
      '<tr><td colspan="6" class="empty">' +
      esc(res.d.error || 'Gagal memuat daftar akun.') + '</td></tr>';
    say($('lMsg'), res.d.error || 'Gagal memuat daftar akun.', false);
    $('btnCreate').disabled = true;
    return;
  }
  cache = res.d.accounts || [];
  if (!cache.length) {
    $('rows').innerHTML = '<tr><td colspan="6" class="empty">Belum ada akun.</td></tr>';
    return;
  }
  $('rows').innerHTML = cache.map(row).join('');
}

function row(a) {
  const live = a.sesiAktif > 0
    ? '<span class="badge live">' + a.sesiAktif + ' aktif</span>'
    : '<span class="muted">tidak aktif</span>';
  const you = a.isYou ? ' <span class="badge you">kamu</span>' : '';
  
  
  const superCount = cache.filter(x => x.role === 'superadmin').length;
  const lastSuper = a.role === 'superadmin' && superCount <= 1;
  const delDisabled = a.isYou || lastSuper ? ' disabled' : '';
  const delTitle = a.isYou
    ? 'Tidak bisa menghapus akun sendiri.'
    : (lastSuper ? 'Harus ada minimal satu Super Admin.' : 'Hapus akun ini.');

  return '<tr data-id="' + a.id + '">' +
    '<td class="u">' + esc(a.username) + you + '</td>' +
    '<td>' + roleBadge(a) + '</td>' +
    '<td class="muted">' + esc(a.createdAt) + '</td>' +
    '<td class="muted">' + (a.lastLogin ? esc(a.lastLogin) : 'belum pernah') + '</td>' +
    '<td>' + live + '</td>' +
    '<td><div class="row-actions">' +
      '<button class="ghost" data-act="edit">Ubah</button>' +
      '<button class="ghost" data-act="out"' + (a.sesiAktif > 0 ? '' : ' disabled') +
        ' title="Keluarkan akun ini dari semua perangkat/browser sekaligus.">Keluar semua</button>' +
      '<button class="ghost danger" data-act="del" title="' + esc(delTitle) + '"' + delDisabled + '>Hapus</button>' +
    '</div></td>' +
  '</tr>';
}

function roleBadge(a) {
  return a.role === 'superadmin'
    ? '<span class="badge super">Super Admin</span>'
    : '<span class="badge admin">Admin</span>';
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
  btn.disabled = true;
  btn.textContent = 'Membuat...';
  const res = await api('/api/auth/accounts/create', { username, password, role });
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
    await load();
});





$('btnSave').addEventListener('click', async () => {
  if (editId == null) return;
  const btn = $('btnSave');
  const username = $('eUser').value.trim();
  const password = $('ePass').value;
  const role = $('eRole').value;

  btn.disabled = true;
  btn.textContent = 'Menyimpan...';
  const res = await api('/api/auth/accounts/update', {
    id: editId,
    username: username,
    password: password,
    role: $('eRole').disabled ? '-' : role,
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





load();