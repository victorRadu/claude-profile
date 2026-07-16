package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"0.4.0", "0.3.1", true},
		{"v0.4.0", "v0.3.1", true},
		{"0.3.1", "0.3.1", false},
		{"0.3.0", "0.3.1", false},
		{"1.0.0", "0.9.9", true},
		{"0.3.10", "0.3.9", true},
		{"garbage", "0.3.1", false},
		{"0.4.0", "dev", false},
		{"0.4.0", "0.3.1-local", false},
	}
	for _, c := range cases {
		if got := Newer(c.latest, c.current); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestIsDevBuild(t *testing.T) {
	for v, want := range map[string]bool{
		"dev": true, "": true, "0.3.1-local": true, "v0.3.1-5-gabc123": true,
		"0.3.1": false, "v0.3.1": false,
	} {
		if got := IsDevBuild(v); got != want {
			t.Errorf("IsDevBuild(%q) = %v, want %v", v, got, want)
		}
	}
}

func testState(t *testing.T, at time.Time) State {
	t.Helper()
	return State{Root: t.TempDir(), Now: func() time.Time { return at }}
}

func TestStaleAndRefresh(t *testing.T) {
	srv := fakeRelease(t, "v0.5.0", nil)
	defer srv.Close()

	now := time.Now()
	st := testState(t, now)
	if !st.Stale() {
		t.Fatal("empty cache must be stale")
	}
	if err := st.Refresh(srv.Client()); err != nil {
		t.Fatal(err)
	}
	if st.Stale() {
		t.Fatal("cache must be fresh right after a refresh")
	}
	later := State{Root: st.Root, Now: func() time.Time { return now.Add(25 * time.Hour) }}
	if !later.Stale() {
		t.Fatal("cache must be stale after the interval")
	}
}

func TestRefreshFailureStillAdvancesTimestamp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	old := APIBase
	APIBase = srv.URL
	defer func() { APIBase = old }()

	st := testState(t, time.Now())
	if err := st.Refresh(srv.Client()); err == nil {
		t.Fatal("expected an error from a failing API")
	}
	if st.Stale() {
		t.Fatal("a failed refresh must still advance the timestamp (no hammering offline)")
	}
}

func TestNoticeShownOnceAndThrottled(t *testing.T) {
	srv := fakeRelease(t, "v9.9.9", nil)
	defer srv.Close()

	now := time.Now()
	st := testState(t, now)
	if err := st.Refresh(srv.Client()); err != nil {
		t.Fatal(err)
	}
	msg := st.Notice("0.3.1")
	if !strings.Contains(msg, "9.9.9") || !strings.Contains(msg, "0.3.1") || !strings.Contains(msg, "claude-profile update") {
		t.Fatalf("unexpected notice %q", msg)
	}
	if again := st.Notice("0.3.1"); again != "" {
		t.Fatalf("notice must be throttled, got %q", again)
	}
	later := State{Root: st.Root, Now: func() time.Time { return now.Add(25 * time.Hour) }}
	if later.Notice("0.3.1") == "" {
		t.Fatal("notice must reappear after the interval")
	}
	if cur := st.Notice("9.9.9"); cur != "" {
		t.Fatalf("no notice when current, got %q", cur)
	}
}

func TestMigratedVersionRoundTrip(t *testing.T) {
	st := testState(t, time.Now())
	if v := st.MigratedVersion(); v != "" {
		t.Fatalf("expected empty, got %q", v)
	}
	st.SetMigratedVersion("0.4.0")
	if v := st.MigratedVersion(); v != "0.4.0" {
		t.Fatalf("got %q", v)
	}
}

func TestStateDirIsNotAProfile(t *testing.T) {
	st := testState(t, time.Now())
	st.SetMigratedVersion("0.4.0")
	if base := filepath.Base(st.stateDir()); !strings.HasPrefix(base, ".") {
		t.Fatalf("state dir %q must be dot-prefixed so the profile store ignores it", base)
	}
}

func TestDownloadVerifiesChecksumAndExtracts(t *testing.T) {
	binary := []byte("#!/fake-binary v0.5.0\n")
	srv := fakeRelease(t, "v0.5.0", binary)
	defer srv.Close()

	rel, err := LatestRelease(srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	got, err := Download(srv.Client(), rel)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, binary) {
		t.Fatalf("extracted binary differs: %q", got)
	}
}

func TestDownloadRejectsBadChecksum(t *testing.T) {
	binary := []byte("legit")
	srv := fakeReleaseTampered(t, "v0.5.0", binary)
	defer srv.Close()

	rel, err := LatestRelease(srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Download(srv.Client(), rel); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
}

func TestDownloadRequiresChecksums(t *testing.T) {
	rel := Release{Version: "0.5.0", Assets: map[string]string{AssetName("0.5.0"): "http://unused"}}
	if _, err := Download(http.DefaultClient, rel); err == nil || !strings.Contains(err.Error(), "checksums.txt") {
		t.Fatalf("expected missing-checksums error, got %v", err)
	}
}

func TestApplySwapsBinary(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "claude-profile")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Apply(exe, []byte("new")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("binary content = %q", got)
	}
	if _, err := os.Stat(exe + ".new"); err == nil {
		t.Fatal("temp .new file left behind")
	}
}

// fakeRelease serves a GitHub-shaped latest release. With binary != nil it
// also serves the platform asset (tar.gz or zip) and matching checksums.
func fakeRelease(t *testing.T, tag string, binary []byte) *httptest.Server {
	return fakeReleaseWith(t, tag, binary, false)
}

func fakeReleaseTampered(t *testing.T, tag string, binary []byte) *httptest.Server {
	return fakeReleaseWith(t, tag, binary, true)
}

func fakeReleaseWith(t *testing.T, tag string, binary []byte, tamper bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)

	version := strings.TrimPrefix(tag, "v")
	asset := AssetName(version)
	var archive []byte
	if binary != nil {
		archive = makeArchive(t, asset, binary)
	}
	sum := sha256.Sum256(archive)
	digest := hex.EncodeToString(sum[:])
	if tamper {
		digest = strings.Repeat("0", 64)
	}

	mux.HandleFunc("/repos/"+Repo+"/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"tag_name":%q,"assets":[{"name":%q,"browser_download_url":%q},{"name":"checksums.txt","browser_download_url":%q}]}`,
			tag, asset, srv.URL+"/dl/"+asset, srv.URL+"/dl/checksums.txt")
	})
	mux.HandleFunc("/dl/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", digest, asset)
	})
	mux.HandleFunc("/dl/"+asset, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	})

	old := APIBase
	APIBase = srv.URL
	t.Cleanup(func() { APIBase = old })
	return srv
}

// makeArchive builds the same shape GoReleaser publishes for this platform.
func makeArchive(t *testing.T, assetName string, binary []byte) []byte {
	t.Helper()
	binName := "claude-profile"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	var buf bytes.Buffer
	if strings.HasSuffix(assetName, ".zip") {
		zw := zip.NewWriter(&buf)
		f, err := zw.Create(binName)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(binary); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: binName, Mode: 0o755, Size: int64(len(binary)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(binary); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
