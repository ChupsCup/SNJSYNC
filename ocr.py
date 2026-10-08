
"""
CTF OCR - run LOCAL (bukan Colab).
Download foto langsung dari Google Drive (urllib, tanpa googleapiclient),
OCR pakai RapidOCR (8 worker max), auto-resume dari data/index.json,
auto-save setelah tiap chunk, upload index.json ke Drive di akhir.

Cara pakai:
  python -u ocr.py            
  python -u ocr.py --test     
  python -u ocr.py --workers 6
"""
import os, sys, json, time, io, argparse, uuid

FOLDER_NAME = 'CTF'
OUTPUT_FILE = 'index.json'


FALLBACK_PHOTOS_FOLDER_ID = '1uHoZ6rxUjpMfoEPydc611qdH94Vf-_Tq'
FALLBACK_DATA_FOLDER_ID    = '1d70WG4bhfe-ClLTmVq7nF3I3wdYPaCwU'

FILELIST_CACHE = 'filelist.json'  



INCLUDE_PREFIXES = ()

_ROOT = os.path.dirname(os.path.abspath(__file__))

def _load_client():
    """Baca client_id/client_secret dari client_secret.json (tidak ada secret di source)."""
    with open(os.path.join(_ROOT, 'client_secret.json'), 'r', encoding='utf-8') as fh:
        c = json.load(fh)
    sec = c.get('installed') or c.get('web') or {}
    return sec.get('client_id', ''), sec.get('client_secret', '')

def _load_refresh_token():
    """Baca refresh token dari data/gdrive_token.json (satu-satunya sumber kebenaran)."""
    p = os.path.join(_ROOT, 'data', 'gdrive_token.json')
    with open(p, 'r', encoding='utf-8') as fh:
        return json.load(fh).get('refresh_token', '')

CLIENT_ID, CLIENT_SECRET = _load_client()
REFRESH_TOKEN = _load_refresh_token()
if not REFRESH_TOKEN:
    sys.exit('data/gdrive_token.json tidak punya refresh_token. Hubungkan Drive dulu lewat web.')

import urllib.request, urllib.parse

_tok = [0, None]  

def access_token():
    now = time.time()
    if _tok[1] and _tok[0] > now + 60:
        return _tok[1]
    body = urllib.parse.urlencode({
        'client_id': CLIENT_ID, 'client_secret': CLIENT_SECRET,
        'refresh_token': REFRESH_TOKEN, 'grant_type': 'refresh_token'
    }).encode()
    req = urllib.request.Request('https://oauth2.googleapis.com/token', data=body, method='POST')
    with urllib.request.urlopen(req, timeout=60) as r:
        d = json.load(r)
    if not d.get('access_token'):
        raise RuntimeError('refresh token ditolak: %s' % str(d)[:300])
    _tok[0] = now + int(d.get('expires_in', 3600))
    _tok[1] = d['access_token']
    return _tok[1]

def _http(url, timeout=90):
    """GET Bearer -> (status, bytes). Retry tipis untuk 401/429/5xx."""
    for attempt in range(3):
        req = urllib.request.Request(url, headers={'Authorization': 'Bearer ' + access_token()})
        try:
            with urllib.request.urlopen(req, timeout=timeout) as r:
                return r.status, r.read()
        except urllib.error.HTTPError as e:
            if e.code == 401 or e.code == 429 or e.code >= 500:
                _tok[1] = None if e.code == 401 else _tok[1]
                time.sleep(1 + attempt * 2)
                continue
            return e.code, e.read()
        except Exception:
            time.sleep(1 + attempt * 2)
    return 0, b''

def download(fid):
    """Download foto -> io.BytesIO. Retry 401(refresh)/403/429/5xx/network. 400 = gagal permanen."""
    for attempt in range(5):
        try:
            url = 'https://www.googleapis.com/drive/v3/files/%s?alt=media' % fid
            req = urllib.request.Request(url, headers={'Authorization': 'Bearer ' + access_token()})
            with urllib.request.urlopen(req, timeout=180) as r:
                return io.BytesIO(r.read())
        except urllib.error.HTTPError as e:
            code = e.code
            body = e.read().decode('utf-8', 'ignore')[:400]
            if code == 401:
                _tok[1] = None
                if attempt < 4:
                    time.sleep(1 + attempt * 2); continue
                raise RuntimeError('HTTP 401 habis retry: %s' % body)
            if code in (403, 429) or code >= 500:
                if attempt < 4:
                    time.sleep(3 + attempt * 4); continue
                raise RuntimeError('HTTP %d habis retry: %s' % (code, body))
            raise RuntimeError('HTTP %d: %s' % (code, body))
        except (urllib.error.URLError, OSError) as e:
            if attempt < 4:
                time.sleep(2 + attempt * 3); continue
            raise RuntimeError('network: %s' % e)
    raise RuntimeError('download tidak jalan (unreachable)')

def drive_list(q):
    out, token = [], None
    while True:
        url = 'https://www.googleapis.com/drive/v3/files?pageSize=1000&fields=nextPageToken,files(id,name,mimeType,size)&q=' + urllib.parse.quote(q)
        if token: url += '&pageToken=' + urllib.parse.quote(token)
        code, body = _http(url)
        if code != 200:
            raise RuntimeError('list gagal HTTP %d: %s' % (code, body[:200]))
        j = json.loads(body)
        out += j.get('files', [])
        token = j.get('nextPageToken')
        if not token: break
    return out

def _in_scope(rel):
    
    if not INCLUDE_PREFIXES:
        return True
    for p in INCLUDE_PREFIXES:
        if rel == p or rel.startswith(p + '/'):
            return True
    return False

walk_req = [0]

def walk(root_id, prefix=''):
    out = []
    children = drive_list("'%s' in parents and trashed=false" % root_id)
    for f in children:
        rel = prefix + '/' + f['name'] if prefix else f['name']
        if f['mimeType'] == 'application/vnd.google-apps.folder':
            if _in_scope(rel):
                walk_req[0] += 1
                if walk_req[0] % 25 == 0:
                    print('  scanning... %d folder (sudah %d foto)' % (walk_req[0], len(out)))
                out += walk(f['id'], rel)
        elif f['mimeType'].startswith('image/'):
            if _in_scope(rel):
                out.append({'id': f['id'], 'name': f['name'], 'dir': prefix,
                            'path': rel, 'size': int(f['size'] or 0)})
    return out

def _is_good(e):
    """Entri dianggap selesai: punya teks ASLI (bukan kosong, bukan '[OCR GAGAL]').
    Entri text '' (kosong) TIDAK dianggap selesai -> di-OCR ulang."""
    if not e:
        return False
    t = e.get('text') or ''
    return bool(t.strip()) and not t.startswith('[OCR GAGAL]') and (e.get('ocrConf') or 0) > 0




MAX_DIM = 1600
_ocr = None

def _wc_init():
    from rapidocr_onnxruntime import RapidOCR
    return RapidOCR(params={'Global': {'cpu_threads': 1}})

def _wc_run(ocr, img):
    """Ambil teks + confidence. FIX bug int+str: confidence kadang string/np —
    di-coerce ke float, item rusak dilewati, bukan di-sum mentah."""
    result, _ = ocr(img)
    lines, confs = [], []
    for item in (result or []):
        try:
            txt = item[1]
            c = float(item[2])
        except Exception:
            continue
        lines.append(txt if isinstance(txt, str) else str(txt))
        confs.append(c)
    return lines, confs

def decode_barcodes(img_bytes):
    try:
        import numpy as np
        import cv2
        from pyzbar.pyzbar import decode as pyzdecode
        g = cv2.imdecode(np.frombuffer(img_bytes, np.uint8), cv2.IMREAD_GRAYSCALE)
        bcs = []
        for b in pyzdecode(g):
            bcs.append({'text': b.data.decode('utf-8', 'ignore'), 'format': b.type})
        return bcs
    except Exception:
        return []

def _worker_init():
    global _ocr
    _ocr = _wc_init()

def process_one(f):
    import numpy as np, cv2
    entry = dict(f)
    entry['text'] = ''
    entry['ocrConf'] = 0.0
    entry['barcodes'] = []
    try:
        buf = download(f['id'])
        img_bytes = buf.getvalue()
        arr = np.frombuffer(img_bytes, np.uint8)
        img = cv2.imdecode(arr, cv2.IMREAD_COLOR)
        if img is None:
            raise ValueError('cv2.imdecode gagal')
        h, w = img.shape[:2]
        if max(h, w) > MAX_DIM:
            r = MAX_DIM / float(max(h, w))
            img = cv2.resize(img, (int(w * r), int(h * r)), interpolation=cv2.INTER_AREA)
        lines, confs = _wc_run(_ocr, img)
        entry['text'] = '\n'.join(lines).strip()
        entry['ocrConf'] = round(sum(confs) / len(confs), 3) if confs else 0.0
        entry['barcodes'] = decode_barcodes(img_bytes)
    except Exception as e:
        entry['text'] = '[OCR GAGAL] ' + str(e)
    return entry

def _reason(txt):
    return txt.split('[OCR GAGAL] ', 1)[1].split('|')[0][:220] if '[OCR GAGAL] ' in txt else '?'




def _multipart(url, method, meta, data):
    boundary = 'x' + uuid.uuid4().hex
    head = ('--%s\r\nContent-Type: application/json; charset=UTF-8\r\n\r\n%s\r\n' % (boundary, json.dumps(meta, ensure_ascii=False))).encode('utf-8')
    head += ('--%s\r\nContent-Type: application/octet-stream\r\n\r\n' % boundary).encode('utf-8')
    body = head + data + ('\r\n--%s--\r\n' % boundary).encode('utf-8')
    req = urllib.request.Request(url + '?uploadType=multipart&fields=id', data=body, method=method)
    req.add_header('Authorization', 'Bearer ' + access_token())
    req.add_header('Content-Type', 'multipart/related; boundary=' + boundary)
    with urllib.request.urlopen(req, timeout=300) as r:
        return json.load(r)

def upload_index(local_path, parent):
    if not os.path.exists(local_path):
        print('index.json lokal tidak ada, skip upload.')
        return
    with open(local_path, 'rb') as fh:
        data = fh.read()
    q = urllib.parse.quote("'%s' in parents and name='%s' and trashed=false" % (parent, OUTPUT_FILE))
    code, res = _http('https://www.googleapis.com/drive/v3/files?q=%s&fields=files(id,name)' % q)
    files_ = json.loads(res).get('files', []) if code == 200 else []
    if files_:
        _multipart('https://www.googleapis.com/upload/drive/v3/files/%s' % files_[0]['id'], 'PATCH', {'name': OUTPUT_FILE}, data)
        print('Upload index.json -> Drive (update) OK.')
    else:
        _multipart('https://www.googleapis.com/upload/drive/v3/files', 'POST', {'name': OUTPUT_FILE, 'parents': [parent]}, data)
        print('Upload index.json -> Drive (create) OK.')




def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--test', action='store_true', help='uji 10 foto, tanpa upload')
    ap.add_argument('--limit', type=int, default=0, help='maksimal foto yang diproses (benchmark)')
    ap.add_argument('--scope', type=str, default=None, help="filter folder, misal 'SAPI' / 'KERBAU' / 'SAPI/SAPI 2026' (tanpa ini = SAPI+KERBAU semua)")
    ap.add_argument('--low', action='store_true', help='prioritas IDLE: laptop tetap lancar, OCR ambil sisa CPU (worker otomatis 4)')
    ap.add_argument('--workers', type=int, default=0)
    ap.add_argument('--refresh-list', action='store_true', help='scan ulang daftar file dari Drive')
    ap.add_argument('--upload', action='store_true', help='upload index.json ke Drive (default: lokal saja)')
    args = ap.parse_args()

    base = os.path.dirname(os.path.abspath(__file__))
    data_dir = os.path.join(base, 'data')
    os.makedirs(data_dir, exist_ok=True)
    out_path = os.path.join(data_dir, OUTPUT_FILE)
    cache_path = os.path.join(data_dir, FILELIST_CACHE)
    
    
    app_cache_path = os.path.join(data_dir, 'index_cache.json')

    def save_all(results):
        """Simpan ke index.json DAN index_cache.json supaya app langsung pakai."""
        for p in (out_path, app_cache_path):
            tmp = p + '.tmp'
            with open(tmp, 'w', encoding='utf-8') as fh:
                json.dump(results, fh, ensure_ascii=False)
            os.replace(tmp, p)

    
    photos_id = None
    data_folder_id = FALLBACK_DATA_FOLDER_ID
    cfgp = os.path.join(base, 'config.json')
    if os.path.exists(cfgp):
        try:
            with open(cfgp, 'r', encoding='utf-8') as fh:
                cfg = json.load(fh)
            photos_id = cfg.get('photosFolderId') or None
            data_folder_id = cfg.get('dataFolderId') or data_folder_id
        except Exception:
            pass
    photos_id = photos_id or FALLBACK_PHOTOS_FOLDER_ID
    print('Folder foto : %s' % photos_id)
    print('Folder data : %s (upload index di sini)' % data_folder_id)

    
    files = None
    if not args.refresh_list and os.path.exists(cache_path):
        try:
            with open(cache_path, 'r', encoding='utf-8') as fh:
                cached = json.load(fh)
            if cached.get('root') == photos_id and isinstance(cached.get('files'), list):
                files = cached['files']
                print('Filelist cache dipakai: %d foto (tanpa scan Drive).' % len(files))
        except Exception:
            files = None
    if files is None:
        
        
        pl_path = os.path.join(data_dir, 'photo_list.json')
        if os.path.exists(pl_path):
            try:
                with open(pl_path, 'r', encoding='utf-8') as fh:
                    pl = json.load(fh)
                if pl.get('root') == photos_id and isinstance(pl.get('files'), list) and pl['files']:
                    files = [f for f in pl['files'] if _in_scope(f.get('path', ''))]
                    with open(cache_path, 'w', encoding='utf-8') as fh:
                        json.dump({'root': photos_id, 'files': files, 'saved': time.time()}, fh, ensure_ascii=False)
                    print('Seed filelist dari photo_list.json: %d foto (0 API call).' % len(files))
            except Exception:
                files = None
    if files is None:
        print('Scanning folder (bisa lambat, ribuan folder)...')
        files = walk(photos_id)
        with open(cache_path, 'w', encoding='utf-8') as fh:
            json.dump({'root': photos_id, 'files': files, 'saved': time.time()}, fh, ensure_ascii=False)
        print('Scan selesai dan disimpan ke cache: %d foto.' % len(files))
    if args.scope:
        scope = args.scope.strip().rstrip('/').lower()
        files = [f for f in files if f['path'].lower() == scope or f['path'].lower().startswith(scope + '/')]
        print('Scope "%s" diterapkan -> %d foto.' % (args.scope, len(files)))
    if not files:
        raise SystemExit('Folder foto kosong / tidak terlihat (atau scope tidak cocok).')
    print('Ditemukan %d foto.' % len(files))

    
    
    existing = {}
    for src in (os.path.join(data_dir, 'index_cache.json'), out_path):
        if os.path.exists(src):
            try:
                with open(src, 'r', encoding='utf-8') as fh:
                    for e in json.load(fh):
                        if e.get('id') and _is_good(e):
                            existing.setdefault(e['id'], e)
            except Exception:
                pass
    print('Resume (entri berteks dari cache lokal): %d.' % len(existing))

    good_ids = {}
    todo = []
    for f in files:
        o = existing.get(f['id'])
        if _is_good(o):
            good_ids[f['id']] = o
        else:
            todo.append(f)
    print('Resume (selesai): %d | proses ulang/baru: %d' % (len(good_ids), len(todo)))

    if not todo:
        if args.scope:
            print('Scope "%s": tidak ada yang perlu di-OCR (semua sudah berteks).' % args.scope)
        else:
            upload_index(out_path, data_folder_id)
            print('Tidak ada yang perlu di-OCR. Upload index.json -> Drive.')
        return

    if args.low:
        try:
            import ctypes
            IDLE_PRIORITY_CLASS = 0x40
            ctypes.windll.kernel32.SetPriorityClass(
                ctypes.windll.kernel32.GetCurrentProcess(), IDLE_PRIORITY_CLASS)
            print('Prioritas: IDLE (laptop tetap lancar, OCR ambil sisa CPU).')
        except Exception as e:
            print('prioritas idle gagal: %s' % e)

    if args.test:
        todo = todo[:10]
        print('TEST MODE: cuma 10 foto.')
    if args.limit > 0:
        todo = todo[:args.limit]
        print('LIMIT vs benchmark: %d foto.' % args.limit)

    
    print('Health probe: uji download 5 foto dulu...')
    probe_ok = 0
    for f in todo[:5]:
        try:
            b = download(f['id'])
            probe_ok += 1 if len(b.getvalue()) > 0 else 0
        except Exception as e:
            print('  probe FAIL %s -> %s' % (f['path'], e))
    if probe_ok < 3:
        raise SystemExit('Health probe GAGAL (%d/5). Cek token/koneksi, jangan lanjut.' % probe_ok)
    print('Health probe OK (%d/5). Lanjut OCR.' % probe_ok)

    
    print('Menyiapkan model OCR (pertama kali download model... jangan tutup).')
    import multiprocessing as mp
    init_model = _wc_init()
    print('Model OCR siap. Mulai memproses foto...')

    nproc = args.workers or (4 if args.low else min(mp.cpu_count() or 2, 6))
    print('Worker: %d' % nproc)

    t0 = time.time()
    results = list(good_ids.values())
    reprocessed = 0
    chunk = max(50, min(500, len(todo))) if not args.test else 10
    reasons = {}

    try:
        with mp.Pool(processes=nproc, initializer=_worker_init) as pool:
            for si in range(0, len(todo), chunk):
                slice_ = todo[si:si + chunk]
                for f, entry in zip(slice_, pool.map(process_one, slice_)):
                    results.append(entry)
                    reprocessed += 1
                    if entry.get('text', '').startswith('[OCR GAGAL]'):
                        r = _reason(entry['text'])
                        reasons[r] = reasons.get(r, 0) + 1
                done_all = len(results)
                elapsed = time.time() - t0
                todo_left = len(todo) - reprocessed
                eta = (elapsed / reprocessed) * todo_left if reprocessed else 0
                print('%d/%d foto (%.0fs, reprocess %d, ETA %.0fm)' % (done_all, len(files), elapsed, reprocessed, eta / 60))
                save_all(results)
                if reasons:
                    top = sorted(reasons.items(), key=lambda kv: -kv[1])[:3]
                    print('  GAGAL saat ini: %s' % (' | '.join('%sx %s' % (v, k) for k, v in top)))
                if args.test:
                    print('\n=== TEST MODE selesai ===')
                    for j, r in enumerate(results[-10:], 1):
                        print('%d. conf=%.3f | %s' % (j, r.get('ocrConf', 0), r.get('text', '')[:80]))
                    break
    except KeyboardInterrupt:
        save_all(results)
        print('\nInterupsi. Progress disimpan.')

    ok = sum(1 for r in results if r.get('ocrConf', 0) > 0)
    fail = sum(1 for r in results if r.get('text', '').startswith('[OCR GAGAL]'))
    no_text = sum(1 for r in results if not r.get('text', '') and not r.get('ocrConf', 0))
    print('SELESAI: %d foto -> %s (%.0fs)' % (len(results), out_path, time.time() - t0))
    print('OCR berhasil: %d | Gagal: %d | tanpa teks: %d' % (ok, fail, no_text))
    if reasons:
        print('Rincian kegagalan:')
        for k, v in sorted(reasons.items(), key=lambda kv: -kv[1]):
            print('  %sx %s' % (v, k))

    if args.test:
        print('TEST OK. Jalankan tanpa --test untuk full scan.')
    elif args.scope:
        print('PARTIAL (--scope "%s"): index LOKAL tersimpan, TAPI tidak di-upload ke Drive.' % args.scope)
    elif args.upload:
        upload_index(out_path, data_folder_id)
        print('Index.json juga di-upload ke Drive.')
    else:
        print('Selesai LOKAL. index_cache.json sudah diperbarui -> langsung dipakai web SNJSYNC.')
        print('Restart snjsync.exe (atau klik "Ambil Foto dari Drive") untuk memuat hasil baru.')
        print('Butuh salinan di Drive juga? Jalankan ulang dengan --upload.')

if __name__ == '__main__':
    main()