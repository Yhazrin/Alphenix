package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"
)

func TestExtractBinaryFromTarGz_Found(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	content := []byte("fake-binary-data")
	hdr := &tar.Header{Name: "alphenix", Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}
	tw.WriteHeader(hdr)
	tw.Write(content)
	tw.Close()
	gw.Close()

	data, err := extractBinaryFromTarGz(&buf, "alphenix")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "fake-binary-data" {
		t.Errorf("got %q, want %q", data, "fake-binary-data")
	}
}

func TestExtractBinaryFromTarGz_NotFound(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{Name: "other-file", Mode: 0o644, Size: 0, Typeflag: tar.TypeReg}
	tw.WriteHeader(hdr)
	tw.Close()
	gw.Close()

	_, err := extractBinaryFromTarGz(&buf, "alphenix")
	if err == nil {
		t.Error("expected error for missing binary")
	}
}

func TestExtractBinaryFromTarGz_InSubdirectory(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	content := []byte("nested-binary")
	hdr := &tar.Header{Name: "subdir/alphenix", Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}
	tw.WriteHeader(hdr)
	tw.Write(content)
	tw.Close()
	gw.Close()

	data, err := extractBinaryFromTarGz(&buf, "alphenix")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "nested-binary" {
		t.Errorf("got %q, want %q", data, "nested-binary")
	}
}
