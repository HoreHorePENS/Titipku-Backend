package imageupload

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 16, 12))
	for x := 0; x < 16; x++ {
		for y := 0; y < 12; y++ {
			img.Set(x, y, color.RGBA{uint8(x * 15), uint8(y * 20), 100, 255})
		}
	}
	return img
}

// jpegWithEXIF menyisipkan segmen APP1 EXIF berisi penanda GPS tepat setelah SOI.
func jpegWithEXIF(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, testImage(), nil); err != nil {
		t.Fatal(err)
	}
	payload := []byte("Exif\x00\x00GPSLatitude-7.2575-SECRET")
	segment := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(segment[2:], uint16(len(payload)+2))
	segment = append(segment, payload...)
	raw := buf.Bytes()
	return append(append(append([]byte{}, raw[:2]...), segment...), raw[2:]...)
}

// 1x1 WebP lossless minimal (chunk VP8L).
const tinyWebPLossless = "UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA=="

func chunk(fourCC string, data []byte) []byte {
	out := append([]byte(fourCC), 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(out[4:], uint32(len(data)))
	out = append(out, data...)
	if len(data)%2 == 1 {
		out = append(out, 0)
	}
	return out
}

// webpWithEXIF membangun container VP8X (flag EXIF+XMP) berisi gambar 1x1 dan metadata.
func webpWithEXIF(t *testing.T) []byte {
	t.Helper()
	tiny, err := base64.StdEncoding.DecodeString(tinyWebPLossless)
	if err != nil {
		t.Fatal(err)
	}
	vp8l := tiny[12:] // chunk VP8L lengkap (header + data)

	vp8x := make([]byte, 10)
	vp8x[0] = vp8xFlagEXIF | vp8xFlagXMP
	// lebar-1 dan tinggi-1 = 0 (24-bit little endian) -> kanvas 1x1

	body := []byte("WEBP")
	body = append(body, chunk("VP8X", vp8x)...)
	body = append(body, vp8l...)
	body = append(body, chunk("EXIF", []byte("GPSLatitude-7.2575-SECRET"))...)
	body = append(body, chunk("XMP ", []byte("<x:xmpmeta>SECRET</x:xmpmeta>"))...)

	out := append([]byte("RIFF"), 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(out[4:], uint32(len(body)))
	return append(out, body...)
}

func TestProcessJPEGStripsEXIF(t *testing.T) {
	raw := jpegWithEXIF(t)
	if !bytes.Contains(raw, []byte("SECRET")) {
		t.Fatal("fixture should contain metadata")
	}
	img, err := Process(bytes.NewReader(raw), DefaultMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	if img.Ext != "jpg" || img.Width != 16 || img.Height != 12 {
		t.Fatalf("unexpected result: %s %dx%d", img.Ext, img.Width, img.Height)
	}
	if bytes.Contains(img.Data, []byte("SECRET")) || bytes.Contains(img.Data, []byte("Exif")) {
		t.Fatal("EXIF/GPS metadata was not removed")
	}
}

func TestProcessPNG(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, testImage()); err != nil {
		t.Fatal(err)
	}
	img, err := Process(&buf, DefaultMaxBytes)
	if err != nil || img.Ext != "png" {
		t.Fatalf("png: %v %+v", err, img)
	}
}

func TestProcessWebPStripsMetadata(t *testing.T) {
	raw := webpWithEXIF(t)
	img, err := Process(bytes.NewReader(raw), DefaultMaxBytes)
	if err != nil {
		t.Fatalf("webp: %v", err)
	}
	if img.Ext != "webp" || img.Width != 1 || img.Height != 1 {
		t.Fatalf("unexpected result: %s %dx%d", img.Ext, img.Width, img.Height)
	}
	if bytes.Contains(img.Data, []byte("SECRET")) {
		t.Fatal("EXIF/XMP chunk was not removed")
	}
	if got := binary.LittleEndian.Uint32(img.Data[4:8]); int(got) != len(img.Data)-8 {
		t.Fatalf("RIFF size %d does not match data length %d", got, len(img.Data)-8)
	}
	// flag EXIF/XMP pada VP8X harus dimatikan; hasil tetap bisa di-decode.
	if flags := img.Data[12+8]; flags&(vp8xFlagEXIF|vp8xFlagXMP) != 0 {
		t.Fatalf("VP8X flags still set: %08b", flags)
	}
	if _, err := Process(bytes.NewReader(img.Data), DefaultMaxBytes); err != nil {
		t.Fatalf("cleaned webp no longer decodes: %v", err)
	}
}

func TestProcessRejectsByContent(t *testing.T) {
	cases := map[string][]byte{
		"text pretending to be jpg": []byte("hello, this is not an image"),
		"gif":                       []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\xff\xff\xff\x00\x00\x00!\xf9\x04\x00\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;"),
		"svg":                       []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
	}
	for name, raw := range cases {
		if _, err := Process(bytes.NewReader(raw), DefaultMaxBytes); !errors.Is(err, ErrUnsupportedType) {
			t.Errorf("%s: expected ErrUnsupportedType, got %v", name, err)
		}
	}

	truncated := jpegWithEXIF(t)[:200]
	if _, err := Process(bytes.NewReader(truncated), DefaultMaxBytes); !errors.Is(err, ErrInvalidImage) {
		t.Errorf("truncated jpeg: expected ErrInvalidImage, got %v", err)
	}
}

func TestProcessRejectsTooLarge(t *testing.T) {
	raw := jpegWithEXIF(t)
	if _, err := Process(bytes.NewReader(raw), int64(len(raw)-1)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected ErrTooLarge, got %v", err)
	}
}

func TestStorageSaveServeDelete(t *testing.T) {
	storage, err := NewStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	img, err := Process(bytes.NewReader(jpegWithEXIF(t)), DefaultMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	url, err := storage.Save(img)
	if err != nil {
		t.Fatal(err)
	}
	if !IsUploadedPath(url) {
		t.Fatalf("saved url %q does not match pattern", url)
	}

	handler := storage.Handler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, url, nil))
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("serve: %d %q", recorder.Code, recorder.Header().Get("Content-Type"))
	}
	if recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing nosniff header")
	}

	// Path di luar pola (traversal, listing, file lain) tidak boleh tersaji.
	secret := filepath.Join(filepath.Dir(storage.Dir()), "secret.txt")
	_ = os.WriteFile(secret, []byte("secret"), 0o600)
	for _, path := range []string{"/uploads/", "/uploads/../secret.txt", "/uploads/..%2fsecret.txt", "/uploads/x.webp"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code == http.StatusOK || strings.Contains(recorder.Body.String(), "secret") {
			t.Errorf("%s should not be served (got %d)", path, recorder.Code)
		}
	}

	if err := storage.Delete(url); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(storage.Dir(), strings.TrimPrefix(url, PublicPrefix))); !os.IsNotExist(err) {
		t.Fatal("file should be deleted")
	}
	// URL eksternal lama diabaikan, bukan error.
	if err := storage.Delete("https://example.com/foto.jpg"); err != nil {
		t.Fatal(err)
	}
}
