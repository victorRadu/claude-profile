package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFindsMarkerInDir(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, MarkerName), "work\n")

	res, ok, err := Resolve(dir)
	if err != nil || !ok {
		t.Fatalf("Resolve = %v, %v, %v", res, ok, err)
	}
	if res.Name != "work" || res.Source != "marker" || res.MarkerDir != dir {
		t.Fatalf("Resolve = %+v, want work/marker/%s", res, dir)
	}
}

func TestResolveWalksUpward(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, MarkerName), "acme")
	nested := filepath.Join(root, "src", "deep", "pkg")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	res, ok, err := Resolve(nested)
	if err != nil || !ok {
		t.Fatalf("Resolve = %v, %v, %v", res, ok, err)
	}
	if res.Name != "acme" || res.MarkerDir != root {
		t.Fatalf("Resolve = %+v, want acme found in %s", res, root)
	}
}

func TestResolveNearestMarkerWins(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, MarkerName), "outer")
	inner := filepath.Join(root, "sub")
	mustWrite(t, filepath.Join(inner, MarkerName), "inner")

	res, ok, err := Resolve(inner)
	if err != nil || !ok || res.Name != "inner" {
		t.Fatalf("Resolve = %+v, %v, %v; want inner", res, ok, err)
	}
}

func TestResolveNoMarker(t *testing.T) {
	_, ok, err := Resolve(t.TempDir())
	if err != nil || ok {
		t.Fatalf("Resolve on unbound dir = ok=%v, err=%v; want false, nil", ok, err)
	}
}

func TestResolveRejectsInvalidMarker(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, MarkerName), "../evil")
	if _, _, err := Resolve(dir); err == nil {
		t.Fatal("Resolve should reject invalid profile name in marker")
	}
}

func TestResolveIgnoresTrailingContent(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, MarkerName), "work\n# comment on second line\n")
	res, ok, err := Resolve(dir)
	if err != nil || !ok || res.Name != "work" {
		t.Fatalf("Resolve = %+v, %v, %v; want work", res, ok, err)
	}
}

func TestWriteAndRemoveMarker(t *testing.T) {
	dir := t.TempDir()
	marker, err := WriteMarker(dir, "work")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(marker)
	if err != nil || string(b) != "work\n" {
		t.Fatalf("marker content = %q, %v; want \"work\\n\"", b, err)
	}

	if _, err := WriteMarker(dir, "bad name"); err == nil {
		t.Fatal("WriteMarker should reject invalid names")
	}

	removed, err := RemoveMarker(dir)
	if err != nil || !removed {
		t.Fatalf("RemoveMarker = %v, %v; want true, nil", removed, err)
	}
	removed, err = RemoveMarker(dir)
	if err != nil || removed {
		t.Fatalf("second RemoveMarker = %v, %v; want false, nil", removed, err)
	}
}
