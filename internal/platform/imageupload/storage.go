package imageupload

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// PublicPrefix adalah prefix URL publik untuk file unggahan.
const PublicPrefix = "/uploads/"

// Nama file yang sah: 32 hex acak + ekstensi yang didukung.
var fileNamePattern = regexp.MustCompile(`^[0-9a-f]{32}\.(webp|jpg|png)$`)

// IsUploadedPath melaporkan apakah value adalah URL publik file unggahan yang sah,
// mis. "/uploads/3f2a…c9.webp".
func IsUploadedPath(value string) bool {
	name, ok := strings.CutPrefix(value, PublicPrefix)
	return ok && fileNamePattern.MatchString(name)
}

// Storage menyimpan file di direktori lokal (mis. volume Docker).
type Storage struct {
	dir string
}

// NewStorage memastikan direktori ada dan dapat ditulisi.
func NewStorage(dir string) (*Storage, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("upload directory is empty")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve upload directory: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("create upload directory %s: %w", abs, err)
	}
	probe, err := os.CreateTemp(abs, ".write-check-*")
	if err != nil {
		return nil, fmt.Errorf("upload directory %s is not writable: %w", abs, err)
	}
	_ = probe.Close()
	_ = os.Remove(probe.Name())
	return &Storage{dir: abs}, nil
}

// Dir mengembalikan path absolut direktori unggahan.
func (s *Storage) Dir() string { return s.dir }

// Save menulis gambar dengan nama acak dan mengembalikan URL publiknya.
func (s *Storage) Save(img *Image) (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate file name: %w", err)
	}
	name := hex.EncodeToString(random[:]) + "." + img.Ext

	// Tulis ke file sementara lalu rename agar tidak ada file setengah jadi yang tersaji.
	tmp, err := os.CreateTemp(s.dir, ".upload-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(img.Data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("write upload: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("close upload: %w", err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("chmod upload: %w", err)
	}
	if err := os.Rename(tmpName, filepath.Join(s.dir, name)); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("store upload: %w", err)
	}
	return PublicPrefix + name, nil
}

// Delete menghapus file unggahan berdasarkan URL publiknya.
// URL selain file unggahan yang sah diabaikan (mis. URL eksternal lama).
func (s *Storage) Delete(publicPath string) error {
	if !IsUploadedPath(publicPath) {
		return nil
	}
	name := strings.TrimPrefix(publicPath, PublicPrefix)
	err := os.Remove(filepath.Join(s.dir, name))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Handler menyajikan file unggahan sebagai file statis di bawah PublicPrefix.
// Hanya nama file yang cocok pola yang dilayani: tanpa daftar direktori,
// tanpa path traversal, dan dengan header yang aman.
func (s *Storage) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, PublicPrefix)
		if !fileNamePattern.MatchString(name) {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
		w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
		// Nama file acak dan tidak pernah ditimpa, jadi aman di-cache lama.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		switch filepath.Ext(name) {
		case ".webp":
			w.Header().Set("Content-Type", "image/webp")
		case ".jpg":
			w.Header().Set("Content-Type", "image/jpeg")
		case ".png":
			w.Header().Set("Content-Type", "image/png")
		}
		http.ServeFile(w, r, filepath.Join(s.dir, name))
	})
}
