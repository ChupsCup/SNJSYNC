# Cara Menjalankan SNJSYNC

Ada 3 cara untuk menjalankan SNJSYNC secara lokal:

## 1. Klik 2x (Paling Mudah) - CMD Batch
Double klik file `run_snjsync.bat`

## 2. PowerShell
Klik kanan `start_snjsync.ps1` > Run with PowerShell
Atau jalankan di terminal:
```powershell
powershell -ExecutionPolicy Bypass -File "D:\Website\SNJSYNC\start_snjsync.ps1"
```

## 3. Langsung via Terminal (CMD/PowerShell)
```cmd
cd /d D:\Website\SNJSYNC
snjsync.exe
```

## 4. Build Ulang (jika perlu)
```cmd
cd /d D:\Website\SNJSYNC
go build -o snjsync.exe .
```

## URL Akses
Setelah server berjalan, buka browser ke: [http://localhost:8877](http://localhost:8877)

## Menghentikan Server
Tekan `Ctrl + C` di jendela terminal yang menjalankan snjsync.exe

## Jalankan di WSL / Linux
1. Install Go (min. 1.27), lalu:
   ```bash
   cd /path/ke/snjsync
   go build -o snjsync .
   ```
2. Buat `.env` (contoh: `.env.example`), isi `CTF_DB_HOST` dan password.
   - DB di mesin lain → `CTF_DB_HOST=<IP DB server>` dan `CTF_NO_PG_AUTOSTART=1`.
   - `.env`, `config.json`, `data/`, `client_secret.json` harus ada di folder saat run.
3. Jalankan `./snjsync` dari folder tersebut. Server: `http://<host>:8877`.

## Build untuk server Linux (upload binary, tanpa install Go di server)
```bash
./build-linux.sh   # hasil: snjsync_linux (static)
```
Upload `snjsync_linux` + `*.html` + `*.js` + aset, lalu:
- pm2: `pm2 start ecosystem.config.js`
- atau Docker: `docker build -t snjsync .`

## Deploy (DB server terpisah)
Di **DB server** (PostgreSQL):
```sql
CREATE USER ctf WITH PASSWORD 'ctf_password';
CREATE DATABASE ctf_db OWNER ctf;
```
+ `postgresql.conf`: `listen_addresses = '*'`
+ `pg_hba.conf`: `host all ctf <IP-web-server>/32 md5`
Di **web server**, `.env`:
```
CTF_DB_HOST=<IP DB server>
CTF_DB_PASSWORD=ctf_password
CTF_NO_PG_AUTOSTART=1
```

## Nahkah Catatan
- Server auto-check port `8877`, bind default `127.0.0.1`.
  Set `CTF_BIND=0.0.0.0` kalau mau expose langsung / tanpa reverse proxy.

## Catatan
- Server akan otomatis check apakah port `8877` sudah dipakai
- Jika `.exe` belum ada, script akan otomatis build menggunakan Go
- Aplikasi menggunakan PostgreSQL (ctf_db). Pastikan PostgreSQL sudah running jika pakai fitur MC (Masterlist Compare)

## Rahasia (.env) — WAJIB untuk mesin baru
Repo ini **publik**, jadi tidak ada satu pun password yang disimpan di kode.
Semua rahasia dibaca dari file `.env` (di-ignore git) atau environment variable:

1. Buat file baru bernama `.env` di folder aplikasi
2. Isi dengan kunci-kunci di bawah ini, lalu jalankan server

| Kunci | Fungsi |
|---|---|
| `CTF_DB_PASSWORD` | password PostgreSQL (pakai `docker-compose up -d`) |
| `POSTGRES_PASSWORD` | password container Postgres (harus sama dengan di atas) |
| `CTF_ADMIN_PASSWORD` | password akun `admin` saat **install baru** (tabel `accounts` kosong) |
| `CTF_SUPER_PASSWORD` | password akun `superadmin`, sama seperti di atas |

Catatan penting:
- Install baru yang **tidak** mengisi `CTF_ADMIN_PASSWORD` tidak akan membuat
  akun admin sama sekali — isi dulu sebelum server pertama kali jalan.
- Akun yang sudah ada tidak akan diganti saat server restart.
- Google Drive (`client_secret.json` dan `data/gdrive_token.json`) juga tidak
  ikut repo; salin manual ke mesin deploy lalu hubungkan Drive lewat web.

## Deploy (FileZilla ke server)
Yang di-upload cukup isi repo ini (kode + HTML/JS/aset). Setelah sampai:
1. Install Go & jalankan `go build -o snjsync.exe .` di server (atau build lokal lalu upload `.exe` yang **tidak** ikut repo)
2. Buat file `.env`, isi password sesuai tabel di atas
3. `docker-compose up -d` untuk PostgreSQL, buka port `8877`
