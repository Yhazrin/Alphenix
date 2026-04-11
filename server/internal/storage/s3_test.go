package storage

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// sanitizeFilename
// ---------------------------------------------------------------------------

func TestSanitizeFilename_Clean(t *testing.T) {
	got := sanitizeFilename("report.pdf")
	if got != "report.pdf" {
		t.Errorf("sanitizeFilename() = %q, want %q", got, "report.pdf")
	}
}

func TestSanitizeFilename_ControlChars(t *testing.T) {
	got := sanitizeFilename("file\x00name\x01.txt")
	if strings.Contains(got, "\x00") || strings.Contains(got, "\x01") {
		t.Errorf("sanitizeFilename() should strip control chars, got %q", got)
	}
	if got != "file_name_.txt" {
		t.Errorf("sanitizeFilename() = %q, want %q", got, "file_name_.txt")
	}
}

func TestSanitizeFilename_QuotesAndSemicolons(t *testing.T) {
	got := sanitizeFilename(`file";name\;.txt`)
	if strings.Contains(got, `"`) || strings.Contains(got, ";") || strings.Contains(got, "\\") {
		t.Errorf("sanitizeFilename() should strip quotes/semicolons/backslashes, got %q", got)
	}
}

func TestSanitizeFilename_Newlines(t *testing.T) {
	got := sanitizeFilename("file\nname\r.txt")
	if strings.Contains(got, "\n") || strings.Contains(got, "\r") {
		t.Errorf("sanitizeFilename() should strip newlines, got %q", got)
	}
}

func TestSanitizeFilename_Empty(t *testing.T) {
	got := sanitizeFilename("")
	if got != "" {
		t.Errorf("sanitizeFilename(\"\") = %q, want empty", got)
	}
}

func TestSanitizeFilename_UnicodePreserved(t *testing.T) {
	got := sanitizeFilename("文件名.pdf")
	if got != "文件名.pdf" {
		t.Errorf("sanitizeFilename() should preserve unicode, got %q", got)
	}
}

func TestSanitizeFilename_DeleteChar(t *testing.T) {
	got := sanitizeFilename("file\x7fname.txt")
	if strings.Contains(got, "\x7f") {
		t.Errorf("sanitizeFilename() should strip DEL char, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// KeyFromURL
// ---------------------------------------------------------------------------

func TestKeyFromURL_CDNDomain(t *testing.T) {
	s := &S3Storage{cdnDomain: "cdn.example.com", bucket: "my-bucket"}
	got := s.KeyFromURL("https://cdn.example.com/uploads/file.png")
	if got != "uploads/file.png" {
		t.Errorf("KeyFromURL() = %q, want %q", got, "uploads/file.png")
	}
}

func TestKeyFromURL_BucketDomain(t *testing.T) {
	s := &S3Storage{cdnDomain: "", bucket: "my-bucket"}
	got := s.KeyFromURL("https://my-bucket/path/to/file.jpg")
	if got != "path/to/file.jpg" {
		t.Errorf("KeyFromURL() = %q, want %q", got, "path/to/file.jpg")
	}
}

func TestKeyFromURL_Fallback(t *testing.T) {
	s := &S3Storage{cdnDomain: "", bucket: "my-bucket"}
	got := s.KeyFromURL("https://unknown-domain.com/some/path/file.txt")
	if got != "file.txt" {
		t.Errorf("KeyFromURL() = %q, want %q", got, "file.txt")
	}
}

func TestKeyFromURL_NoSlash(t *testing.T) {
	s := &S3Storage{cdnDomain: "", bucket: "my-bucket"}
	got := s.KeyFromURL("just-a-filename.txt")
	if got != "just-a-filename.txt" {
		t.Errorf("KeyFromURL() = %q, want %q", got, "just-a-filename.txt")
	}
}
