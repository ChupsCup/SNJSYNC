package main

import (
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)



func TestPhotoKnownWhitelist(t *testing.T) {
	invalidatePhotoIDs()
	a := &App{
		universe: []PhotoEntry{
			{ID: "UNIVERSE_1", Name: "a.jpg"},
			{ID: "UNIVERSE_2", Name: "b.jpg"},
		},
		idx: []PhotoEntry{
			{ID: "IDX_ONLY_1", Name: "c.jpg"},
		},
	}

	cases := []struct {
		id   string
		want bool
	}{
		{"UNIVERSE_1", true},
		{"UNIVERSE_2", true},
		{"IDX_ONLY_1", true},
		{"", false},
		{"aca1b2c3d4e5f6g7h8i9j0", false},
		{"../.env", false},
		{"1Pnv3_LnzWS8vG6iL9lpeC0Qct--Lcy4v", false},
	}
	for _, c := range cases {
		if got := a.photoKnown(c.id); got != c.want {
			t.Errorf("photoKnown(%q) = %v, want %v", c.id, got, c.want)
		}
	}
}


func TestLoginRateLimit(t *testing.T) {
	k := loginKey("203.0.113.9", "Korban")
	loginReset(k)
	defer loginReset(k)

	if until := loginLockedUntil(k); until.After(time.Now()) {
		t.Fatalf("seharusnya belum dikunci, sampai %v", until)
	}
	for i := 0; i < loginMaxFails; i++ {
		loginRecordFail(k)
	}
	until := loginLockedUntil(k)
	if !until.After(time.Now()) {
		t.Fatalf("seharusnya dikunci setelah %d gagal, dapat %v", loginMaxFails, until)
	}


	other := loginKey("203.0.113.9", "OrangLain")
	loginReset(other)
	defer loginReset(other)
	if until := loginLockedUntil(other); until.After(time.Now()) {
		t.Errorf("username lain seharusnya tidak ikut terkunci")
	}


	loginReset(k)
	if until := loginLockedUntil(k); until.After(time.Now()) {
		t.Errorf("kunci seharusnya lepas setelah login sukses, dapat %v", until)
	}
}


func TestClientIPTrustedProxy(t *testing.T) {
	t.Setenv("CTF_TRUSTED_PROXY", "")
	trustedProxyOnce = sync.Once{}
	trustedProxyNets = nil
	defer func() {
		trustedProxyOnce = sync.Once{}
		trustedProxyNets = nil
	}()

	req := httptest.NewRequest("GET", "/api/auth/login", nil)
	req.RemoteAddr = "203.0.113.50:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.7")
	if got := clientIP(req); got != "203.0.113.50" {
		t.Errorf("tanpa trusted proxy, XFF harus diabaikan: got %q", got)
	}

	t.Setenv("CTF_TRUSTED_PROXY", "203.0.113.0/24")
	trustedProxyOnce = sync.Once{}
	trustedProxyNets = nil
	if got := clientIP(req); got != "198.51.100.7" {
		t.Errorf("dengan trusted proxy, XFF harus dipakai: got %q", got)
	}
}
