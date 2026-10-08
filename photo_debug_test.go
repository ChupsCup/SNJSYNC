package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDebugPhotoAccess(t *testing.T) {
	if os.Getenv("CTF_DEBUG") == "" {
		t.Skip("audit data asli (set CTF_DEBUG=1 untuk menjalankan)")
	}
	root := "D:/Website/SNJSYNC"
	if v := os.Getenv("CTF_ROOT"); v != "" {
		root = v
	}
	app := &App{root: root, dataDir: root + "/data", ready: make(chan struct{})}
	close(app.ready)

	tok, err := app.gdriveAccessToken()
	if err != nil || tok == "" {
		t.Fatalf("token kosong: len=%d err=%v", len(tok), err)
	}
	if lt := app.loadToken(); lt != nil {
		t.Logf("token siap (len=%d, expiry sisa %d detik)", len(tok),
			(lt.Expiry-time.Now().UnixMilli())/1000)
	}

	ids := []struct {
		label string
		id    string
	}{
		{"OK  (RUSTAM)", "1TKTUGRimrBRlODynUrOXBNKVYa2tF9JV"},
		{"BAD (BULK 95CL)", "1-p-HUxGVuQkF9DM65zf2IbXeKd4wta7m"},
		{"BAD (BULK 95CL)", "1EWnjtcvgUvxv1cez7tHFtHWPScxHeTB8"},
		{"BAD (BULK 95CL)", "1ULsK8TWGwK1pbkt8xBffrvb9hpigSROt"},
	}
	client := &http.Client{Timeout: 60 * time.Second}

	for _, it := range ids {
		t.Run(it.label+" "+it.id[:8], func(t *testing.T) {
			probeMetadata(t, client, tok, it.id)
			probeAPIMedia(t, client, tok, it.id)
			probeUcDownload(t, client, tok, it.id)
			probeDriveDownloadURL(t, app, it.id)
		})
	}
}

func probeDriveDownloadURL(t *testing.T, app *App, id string) {
	resp, err := app.driveDownloadURL(id)
	if err != nil {
		t.Logf("driveDownloadURL: error %v", err)
		return
	}
	defer resp.Body.Close()
	head := make([]byte, 300)
	n, _ := io.ReadFull(resp.Body, head)
	t.Logf("driveDownloadURL: HTTP %d ct=%q awal=%s",
		resp.StatusCode, resp.Header.Get("Content-Type"), oneLine(string(head[:n])))
}

func probeMetadata(t *testing.T, client *http.Client, tok, id string) {
	u := "https://www.googleapis.com/drive/v3/files/" + url.PathEscape(id) +
		"?fields=id,name,mimeType,parents,driveId,ownedByMe,shared,size,explicitTrashed&supportsAllDrives=true"
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := client.Do(req)
	if err != nil {
		t.Logf("metadata: gagal %v", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 800))
	t.Logf("metadata: HTTP %d %s", resp.StatusCode, oneLine(string(body)))
}

func probeAPIMedia(t *testing.T, client *http.Client, tok, id string) {
	u := "https://www.googleapis.com/drive/v3/files/" + url.PathEscape(id) +
		"?alt=media&supportsAllDrives=true"
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := client.Do(req)
	if err != nil {
		t.Logf("api media: gagal %v", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 400))
	t.Logf("api media: HTTP %d ct=%q len(baca)=%d awal=%s",
		resp.StatusCode, resp.Header.Get("Content-Type"), len(body), oneLine(string(body)))
}

func probeUcDownload(t *testing.T, client *http.Client, tok, id string) {
	redir := client
	cc := *client
	cc.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 8 {
			return fmt.Errorf("terlalu banyak redirect")
		}
		t.Logf("  redirect #%d -> %s", len(via), hostOnly(req.URL))
		return nil
	}
	redir = &cc

	u := "https://drive.google.com/uc?export=download&confirm=t&id=" + url.QueryEscape(id)
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := redir.Do(req)
	if err != nil {
		t.Logf("uc download: gagal %v", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
	t.Logf("uc download: HTTP %d ct=%q akhir=%s awal=%s",
		resp.StatusCode, resp.Header.Get("Content-Type"),
		hostOnly(resp.Request.URL), oneLine(string(body)))
}

func hostOnly(u *url.URL) string {
	if u == nil {
		return "-"
	}
	return u.Scheme + "://" + u.Host
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}
