package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// maxAssetSize caps release downloads (the binary is a few MB).
const maxAssetSize = 200 << 20

// AssetName returns the release asset for this platform, e.g.
// "claude-profile_0.4.0_darwin_arm64.tar.gz" (zip on Windows).
func AssetName(version string) string {
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("claude-profile_%s_%s_%s.%s", Canonical(version), runtime.GOOS, runtime.GOARCH, ext)
}

// Download fetches the platform asset for rel, verifies its SHA-256 against
// the release's checksums.txt, and returns the extracted binary.
func Download(client *http.Client, rel Release) ([]byte, error) {
	name := AssetName(rel.Version)
	assetURL, ok := rel.Assets[name]
	if !ok {
		return nil, fmt.Errorf("release %s has no asset %s for this platform", rel.Version, name)
	}
	sumsURL, ok := rel.Assets["checksums.txt"]
	if !ok {
		return nil, fmt.Errorf("release %s has no checksums.txt — refusing to update unverified", rel.Version)
	}
	sums, err := fetch(client, sumsURL)
	if err != nil {
		return nil, fmt.Errorf("downloading checksums: %w", err)
	}
	asset, err := fetch(client, assetURL)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", name, err)
	}
	if err := verifyChecksum(sums, name, asset); err != nil {
		return nil, err
	}
	return extractBinary(name, asset)
}

func fetch(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxAssetSize))
}

// verifyChecksum checks data against the "hash  filename" lines GoReleaser
// writes into checksums.txt.
func verifyChecksum(sums []byte, name string, data []byte) error {
	want := ""
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == name {
			want = strings.ToLower(fields[0])
			break
		}
	}
	if want == "" {
		return fmt.Errorf("checksums.txt has no entry for %s", name)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("checksum mismatch for %s — download corrupted or tampered with, aborting", name)
	}
	return nil
}

// extractBinary pulls the claude-profile binary out of a tar.gz or zip asset.
func extractBinary(name string, asset []byte) ([]byte, error) {
	binName := "claude-profile"
	if strings.HasSuffix(name, ".zip") {
		binName += ".exe"
		zr, err := zip.NewReader(bytes.NewReader(asset), int64(len(asset)))
		if err != nil {
			return nil, fmt.Errorf("opening %s: %w", name, err)
		}
		for _, f := range zr.File {
			if filepath.Base(f.Name) != binName {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer func() { _ = rc.Close() }()
			return io.ReadAll(io.LimitReader(rc, maxAssetSize))
		}
		return nil, fmt.Errorf("%s does not contain %s", name, binName)
	}
	gz, err := gzip.NewReader(bytes.NewReader(asset))
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", name, err)
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}
		if hdr.Typeflag == tar.TypeReg && filepath.Base(hdr.Name) == binName {
			return io.ReadAll(io.LimitReader(tr, maxAssetSize))
		}
	}
	return nil, fmt.Errorf("%s does not contain %s", name, binName)
}

// Apply replaces the binary at exePath with newBinary. The write goes to a
// sibling temp file first, so the swap is a rename and a half-written binary
// can never end up in place.
func Apply(exePath string, newBinary []byte) error {
	dir := filepath.Dir(exePath)
	tmp := filepath.Join(dir, filepath.Base(exePath)+".new")
	if err := os.WriteFile(tmp, newBinary, 0o755); err != nil {
		return permissionHint(fmt.Errorf("writing %s: %w", tmp, err))
	}
	if err := swap(exePath, tmp); err != nil {
		_ = os.Remove(tmp)
		return permissionHint(err)
	}
	return nil
}

// permissionHint keeps "you can't write there" failures actionable.
func permissionHint(err error) error {
	if os.IsPermission(err) {
		return fmt.Errorf("%w\nThe install directory is not writable — re-run the installer instead:\n  curl -fsSL https://raw.githubusercontent.com/%s/main/install.sh | sh", err, Repo)
	}
	return err
}
