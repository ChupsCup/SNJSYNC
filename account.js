





(function () {
  const SESSION = '/api/auth/me';
  const ROLE_LABEL = { admin: 'Admin', superadmin: 'Super Admin' };



  
  
  
  
  const CSS =
    
    
    
    '#accRoot{position:fixed;z-index:45;display:flex;align-items:center;justify-content:flex-end;' +
    'padding-right:12px;pointer-events:none}' +
    '#accRoot button.acc-pill{display:flex;align-items:center;justify-content:center;width:40px;height:40px;' +
    'border-radius:10px;border:1px solid var(--border);background:var(--card);color:var(--text2);cursor:pointer;' +
    'pointer-events:auto;flex:none}' +
    '#accRoot button.acc-pill:hover{color:var(--text)}' +
    '#accRoot button.acc-pill svg{width:18px;height:18px;display:block}' +
    '#accModal{position:fixed;inset:0;z-index:300;display:none;background:rgba(0,0,0,.6);padding:18px;' +
    'align-items:center;justify-content:center}' +
    '#accModal.open{display:flex}' +
    '#accModal{backdrop-filter:blur(6px);-webkit-backdrop-filter:blur(6px)}' +
    
    
    '#accCard{background:var(--card);border:1px solid var(--border);border-radius:20px;' +
    'padding:22px;width:100%;max-width:400px;position:relative;' +
    'box-shadow:0 24px 60px rgba(0,0,0,.55),0 2px 0 0 rgba(255,255,255,.04) inset;' +
    'animation:accRise .22s cubic-bezier(.22,.61,.36,1) both}' +
    '@keyframes accRise{from{opacity:0;transform:translateY(8px) scale(.98)}' +
    'to{opacity:1;transform:none}}' +
    
    '#accCard h3{font-weight:700;font-size:15px;margin:0;letter-spacing:-.01em}' +
    '#accHead{display:flex;align-items:center;gap:12px;padding-right:34px}' +
    '#accAvatar{width:42px;height:42px;flex:none;border-radius:13px;display:grid;place-items:center;' +
    'background:var(--accent);color:#fff;font-weight:700;font-size:16px;' +
    'box-shadow:0 4px 14px color-mix(in srgb, var(--accent) 40%, transparent)}' +
    '#accHeadText{min-width:0}' +
    '#accName{font-weight:700;font-size:14px;color:var(--text);white-space:nowrap;' +
    'overflow:hidden;text-overflow:ellipsis;line-height:1.3}' +
    '#accRole{display:inline-block;margin-top:3px;font-size:10.5px;font-weight:700;letter-spacing:.04em;' +
    'text-transform:uppercase;padding:2.5px 8px;border-radius:999px;' +
    'background:color-mix(in srgb, var(--accent) 16%, transparent);color:var(--accent)}' +
    
    '#accWho{display:none}' +
    
    '#accForm{display:none;margin:16px 0 0;padding:14px;border-radius:14px;' +
    'background:color-mix(in srgb, var(--bg) 55%, transparent);' +
    'border:1px solid var(--border)}' +
    '#accForm label{display:block;font-size:11px;font-weight:600;color:var(--text2);' +
    'margin-bottom:5px;letter-spacing:.02em}' +
    '#accForm .accField{margin-bottom:10px}' +
    '#accForm .accField:last-child{margin-bottom:0}' +
    '#accForm input{width:100%;padding:9px 12px;background:var(--bg);' +
    'border:1px solid var(--border);border-radius:10px;color:var(--text);font-size:13px;' +
    'outline:none;transition:border-color .15s,box-shadow .15s;font-family:inherit}' +
    '#accForm input::placeholder{color:var(--text2);opacity:.65}' +
    '#accForm input:focus{border-color:var(--accent);' +
    'box-shadow:0 0 0 3px color-mix(in srgb, var(--accent) 22%, transparent)}' +
    '#accMsg{font-size:11.5px;margin:14px 0 0;display:none;padding:9px 11px;border-radius:10px;' +
    'font-weight:600;line-height:1.45}' +
    
    '#accRow{display:flex;gap:8px;flex-wrap:wrap;margin-top:18px}' +
    '#accRow button{padding:10px 14px;border-radius:11px;font-weight:600;font-size:12.5px;' +
    'cursor:pointer;font-family:inherit;transition:background-color .15s,border-color .15s,color .15s}' +
    '#accRow button:focus-visible{outline:2px solid var(--accent);outline-offset:2px}' +
    '#accSave{flex:1 1 100%;background:var(--accent);border:1px solid var(--accent);color:#fff}' +
    '#accSave:hover{background:color-mix(in srgb, var(--accent) 86%, #000)}' +
    '#accOut{flex:1;background:transparent;border:1px solid var(--border);color:var(--text2)}' +
    '#accOut:hover{color:var(--red);border-color:var(--red);' +
    'background:color-mix(in srgb, var(--red) 10%, transparent)}' +
    '#accOut:disabled{opacity:.6;cursor:default}' +
    '#accCancel{flex:1;background:transparent;border:1px solid var(--border);color:var(--text2)}' +
    '#accCancel:hover{color:var(--text);border-color:var(--text2)}' +
    '#accRow button:disabled{opacity:.65;cursor:default}' +
    '#accRow button:disabled:hover{background:var(--accent);border-color:var(--accent);color:#fff}' +
    
    '#accX{position:absolute;top:14px;right:14px;width:30px;height:30px;border-radius:9px;' +
    'background:none;border:1px solid transparent;font-size:19px;display:grid;place-items:center;' +
    'color:var(--text2);cursor:pointer;line-height:1;font-family:inherit;transition:all .15s}' +
    '#accX:hover{color:var(--text);background:var(--bg);border-color:var(--border)}' +
    '@media(max-width:420px){#accCard{padding:18px}#accAvatar{width:38px;height:38px;font-size:15px}}' +
    '@media(prefers-reduced-motion:reduce){#accCard{animation:none}}';

  function addStyle() {
    if (document.getElementById('accStyle')) return;
    const s = document.createElement('style');
    s.id = 'accStyle';
    s.textContent = CSS;
    document.head.appendChild(s);
  }

  
  
  function say(msg, ok) {
    const m = document.getElementById('accMsg');
    m.style.display = 'block';
    m.style.color = ok ? 'var(--green)' : 'var(--red)';
    m.style.background = ok
      ? 'color-mix(in srgb, var(--green) 12%, transparent)'
      : 'color-mix(in srgb, var(--red) 12%, transparent)';
    m.textContent = msg;
  }

  function buildModal(roleTxt, username) {
    const modal = document.createElement('div');
    modal.id = 'accModal';
    modal.innerHTML =
      '<div id="accCard">' +
      '<button id="accX" aria-label="Tutup">&times;</button>' +
      '<div id="accHead">' +
      '<div id="accAvatar"></div>' +
      '<div id="accHeadText">' +
      '<h3 id="accName"></h3>' +
      '<span id="accRole">' + roleTxt + '</span>' +
      '</div>' +
      '</div>' +
      '<div id="accWho"></div>' +
      '<form id="accForm" autocomplete="off">' +
      '<div class="accField"><label for="accOld">Password lama</label>' +
      '<input id="accOld" type="password" placeholder="Masukkan password lama" autocomplete="current-password"></div>' +
      '<div class="accField"><label for="accNew">Password baru</label>' +
      '<input id="accNew" type="password" placeholder="Minimal 6 karakter" autocomplete="new-password"></div>' +
      '<div class="accField"><label for="accNew2">Ulangi password baru</label>' +
      '<input id="accNew2" type="password" placeholder="Ulangi password baru" autocomplete="new-password"></div>' +
      '</form>' +
      '<div id="accMsg" role="status"></div>' +
      '<div id="accRow">' +
      '<button id="accSave">Ganti Password</button>' +
      '<button id="accCancel" hidden>Batal</button>' +
      '<button id="accOut">Keluar</button>' +
      '</div>' +
      '</div>';
    document.body.appendChild(modal);
    document.getElementById('accName').textContent = username;
    document.getElementById('accAvatar').textContent =
      username.trim().charAt(0).toUpperCase() || '?';
    return modal;
  }

  function boot() {
    addStyle();
    fetch(SESSION)
      .then((r) => r.json())
      .then((d) => {
        if (!d || !d.loggedIn) return;

        const role = (d.session && d.session.role) || 'admin';
        const username = (d.session && d.session.username) || '';
        const roleTxt = ROLE_LABEL[role] || role;

        
        const root = document.createElement('div');
        root.id = 'accRoot';
        const pill = document.createElement('button');
        pill.className = 'acc-pill';
        pill.title = username + ' - ' + roleTxt;
        pill.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">' +
          '<circle cx="12" cy="8" r="4"/><path d="M4 21a8 8 0 0 1 16 0"/></svg>';
        root.appendChild(pill);
        document.body.appendChild(root);

        
        
        
        const align = () => {
          const bar = document.querySelector('.topbar');
          if (!bar) { root.style.top = '14px'; root.style.right = '16px'; return; }
          const r = bar.getBoundingClientRect();
          root.style.top = r.top + 'px';
          root.style.left = r.left + 'px';
          root.style.width = r.width + 'px';
          root.style.height = r.height + 'px';
          root.style.paddingRight =
            Math.max(10, parseFloat(getComputedStyle(bar).paddingRight) || 12) + 'px';
        };
        align();
        window.addEventListener('resize', align);
        window.addEventListener('scroll', align, { passive: true });

        const modal = buildModal(roleTxt, username);

        
        
        let mode = 'idle';
        let formOpen = false;
        
        
        
        
        const setSaveLabel = (on) => {
          document.getElementById('accSave').textContent = on ? 'Simpan' : 'Ganti Password';
          document.getElementById('accCancel').hidden = !on;
        };
        const showForm = () => {
          formOpen = true;
          document.getElementById('accForm').style.display = 'block';
          document.getElementById('accMsg').style.display = 'none';
          ['accOld', 'accNew', 'accNew2'].forEach((id) => { document.getElementById(id).value = ''; });
          setSaveLabel(true);
          document.getElementById('accOld').focus();
        };
        const hideForm = () => {
          formOpen = false;
          mode = 'idle';
          document.getElementById('accForm').style.display = 'none';
          document.getElementById('accMsg').style.display = 'none';
          setSaveLabel(false);
        };
        const close = () => {
          modal.classList.remove('open');
          hideForm();
        };

        pill.addEventListener('click', () => {
          modal.classList.add('open');
          if (!formOpen) hideForm();
        });
        document.getElementById('accX').addEventListener('click', close);
        modal.addEventListener('click', (e) => { if (e.target === modal) close(); });

        



        document.getElementById('accOut').addEventListener('click', async () => {
          const out = document.getElementById('accOut');
          out.disabled = true;
          out.textContent = 'Keluar...';
          try {
            await fetch('/api/auth/logout', {
              method: 'POST',
              credentials: 'same-origin',
              cache: 'no-store',
            });
          } catch (e) {
            
            console.warn('[account.js] logout request gagal, tetap ke /', e);
          }
          location.replace('/');
        });

        
        document.getElementById('accForm').addEventListener('submit', (e) => {
          e.preventDefault();
          document.getElementById('accSave').click();
        });

        
        document.getElementById('accSave').addEventListener('click', async () => {
          if (!formOpen) { showForm(); mode = 'save'; return; }
          const oldPw = document.getElementById('accOld').value;
          const newPw = document.getElementById('accNew').value;
          const newPw2 = document.getElementById('accNew2').value;
          if (!oldPw) return say('Masukkan password lama.', false);
          if (newPw !== newPw2) return say('Password baru tidak cocok.', false);
          if (newPw.length < 6) return say('Password baru minimal 6 karakter.', false);
          const btn = document.getElementById('accSave');
          const label = btn.textContent;
          btn.disabled = true;
          btn.textContent = 'Menyimpan...';
          let r;
          try {
            const fd = new URLSearchParams({ oldPassword: oldPw, newPassword: newPw });
            r = await fetch('/api/auth/change-password', {
              method: 'POST', body: fd,
              headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            });
          } catch (e) {
            btn.disabled = false;
            btn.textContent = label;
            return say('Jaringan bermasalah. Coba lagi.', false);
          }
          let res = {};
          try { res = await r.json(); } catch (e) {  }
          btn.disabled = false;
          if (!r.ok) { btn.textContent = label; return say(res.error || 'Gagal ganti password.', false); }
          
          
          hideForm();
          say('Password diubah. Sesi lain sudah keluar.', true);
        });

        document.getElementById('accCancel').addEventListener('click', () => hideForm());

        
        document.addEventListener('keydown', (e) => {
          if (e.key === 'Escape' && modal.classList.contains('open')) close();
        });
      })
      .catch((e) => { console.error('[account.js]', e); });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', boot);
  } else {
    boot();
  }
})();
