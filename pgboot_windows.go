//go:build windows

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

func hiddenProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}

const pgContainer = "ctf-postgres"
const dockerExe = `C:\Program Files\Docker\Docker\resources\bin\docker.exe`

var (
	pgBootMu    sync.Mutex
	pgBootState string
)

func dockerPath() string {
	cands := []string{
		dockerExe,
		filepath.Join(os.Getenv("LOCALAPPDATA"), `Programs\DockerDesktop\resources\bin\docker.exe`),
		`C:\Program Files\Docker\Docker\resources\bin\docker.exe`,
	}
	for _, c := range cands {
		if c != "" {
			if st, err := os.Stat(c); err == nil && !st.IsDir() {
				return c
			}
		}
	}
	if p, err := exec.LookPath("docker"); err == nil {
		return p
	}
	return ""
}

func pgStatus() string {
	d := dockerPath()
	if d == "" {
		return "docker-tidak-ada"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, d, "ps", "--filter", "name="+pgContainer,
		"--format", "{{.Status}}").Output()
	if err != nil {
		return "docker-mati"
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return "container-berhenti"
	}
	if strings.HasPrefix(strings.ToLower(s), "up") {
		return "ready"
	}
	return "container-berhenti"
}

func ensurePostgres() string {
	pgBootMu.Lock()
	if pgBootState == "starting" {
		pgBootMu.Unlock()
		return "starting"
	}
	pgBootState = "starting"
	pgBootMu.Unlock()

	st := pgStatus()
	if st != "ready" {
		logf("[MC] PostgreSQL belum siap (%s) - menyalakan Docker Desktop...", st)
		d := dockerPath()
		if d == "" {
			pgBootMu.Lock()
			pgBootState = ""
			pgBootMu.Unlock()
			return "docker-tidak-ada"
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, d, "desktop", "start")
		cmd.SysProcAttr = hiddenProcAttr()
		if out, err := cmd.CombinedOutput(); err != nil {
			logf("[MC] docker desktop start: %v (%s)", err, strings.TrimSpace(string(out)))
		}

		waitContainer(d)
		st = pgStatus()
		logf("[MC] PostgreSQL setelah start: %s", st)
	}

	pgBootMu.Lock()
	pgBootState = ""
	pgBootMu.Unlock()
	return st
}

func waitContainer(d string) {
	deadline := time.Now().Add(3 * time.Minute)
	started := false
	for time.Now().Before(deadline) {
		if pgStatus() == "docker-mati" {
			time.Sleep(2 * time.Second)
			continue
		}
		if !started {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			cmd := exec.CommandContext(ctx, d, "start", pgContainer)
			cmd.SysProcAttr = hiddenProcAttr()
			_, _ = cmd.CombinedOutput()
			cancel()
			started = true
		}
		if pgStatus() == "ready" {
			return
		}
		time.Sleep(2 * time.Second)
	}
}

func (a *App) warmPostgres() {
	if dockerPath() == "" {
		return
	}
	if pgStatus() == "ready" {
		a.waitDBUp(30 * time.Second)
		return
	}
	logf("[MC] menyiapkan PostgreSQL di background (bisa saja perlu ~1 menit)...")
	ensurePostgres()
	if a.waitDBUp(60*time.Second) == nil {
		logf("[MC] PostgreSQL belum siap setelah 60 detik, akan dicoba lagi saat dipakai")
		return
	}
	logf("[MC] PostgreSQL siap di background")
}

func pgInfoMsg(st string) string {
	switch st {
	case "ready":
		return "PostgreSQL sudah jalan tapi belum mau menerima koneksi. " +
			"Coba lagi dalam beberapa detik."
	case "starting":
		return "PostgreSQL sedang dinyalakan, tunggu sebentar lalu coba lagi."
	case "docker-tidak-ada":
		return "Docker Desktop tidak ditemukan di komputer ini. Modul MMD butuh PostgreSQL. " +
			"Pasang Docker Desktop, atau ganti MMD ke SQLite."
	case "docker-mati":
		return "Docker Desktop belum jalan. Menyalakan otomatis, coba lagi dalam ~1 menit."
	default:
		return "Container " + pgContainer + " belum jalan. Menyalakan otomatis, coba lagi sebentar."
	}
}