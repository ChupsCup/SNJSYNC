//go:build !windows

package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const pgContainer = "ctf-postgres"

var (
	pgBootMu    sync.Mutex
	pgBootState string
)

func dockerPath() string {
	if p, err := exec.LookPath("docker"); err == nil {
		return p
	}
	for _, c := range []string{"/usr/bin/docker", "/usr/local/bin/docker", "/snap/bin/docker"} {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
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
	if st != "ready" && st != "docker-tidak-ada" {
		logf("[MC] PostgreSQL belum siap (%s) - mencoba docker start %s...", st, pgContainer)
		d := dockerPath()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, d, "start", pgContainer)
		if out, err := cmd.CombinedOutput(); err != nil {
			logf("[MC] docker start %s: %v (%s)", pgContainer, err, strings.TrimSpace(string(out)))
		}
		if st = pgStatus(); st == "ready" {
			logf("[MC] PostgreSQL setelah start: %s", st)
		}
	}

	pgBootMu.Lock()
	pgBootState = ""
	pgBootMu.Unlock()
	return st
}

func waitContainer(d string) {
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		if pgStatus() == "docker-mati" {
			time.Sleep(2 * time.Second)
			continue
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
		return "Docker tidak ditemukan di server ini. Modul MMD butuh PostgreSQL. " +
			"Pastikan DB server (PostgreSQL) bisa diakses dari sini."
	case "docker-mati":
		return "Docker daemon belum jalan. Nyalakan dulu, coba lagi dalam ~1 menit."
	default:
		return "Container " + pgContainer + " belum jalan. Menyalakan otomatis, coba lagi sebentar."
	}
}