// Package imageupload memvalidasi, membersihkan, dan menyimpan foto unggahan.
//
// Prinsip keamanan:
//   - Tipe ditentukan dari isi file (magic bytes), bukan dari nama/Content-Type klien.
//   - Ukuran byte dan dimensi piksel dibatasi sebelum decode penuh (anti decompression bomb).
//   - Seluruh metadata (EXIF, GPS, XMP, komentar) dibuang:
//     JPEG/PNG di-decode lalu di-encode ulang; WebP dibuang chunk EXIF/XMP-nya.
//   - Nama file acak 128-bit; nama asli dari klien tidak pernah dipakai.
package imageupload

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"

	"golang.org/x/image/webp"
)

const (
	// DefaultMaxBytes adalah batas ukuran file unggahan (5 MB).
	DefaultMaxBytes = 5 << 20
	// MaxPixels membatasi jumlah piksel (lebar*tinggi) agar decode tidak menghabiskan memori.
	MaxPixels = 40_000_000
	// MaxSide membatasi sisi terpanjang gambar.
	MaxSide = 8000
)

var (
	ErrTooLarge        = errors.New("image exceeds the maximum size")
	ErrUnsupportedType = errors.New("image type is not supported")
	ErrInvalidImage    = errors.New("image data is invalid")
	ErrTooManyPixels   = errors.New("image dimensions are too large")
)

// Image adalah gambar yang sudah divalidasi dan dibersihkan, siap disimpan.
type Image struct {
	Data   []byte
	Ext    string // "webp", "jpg", atau "png"
	Width  int
	Height int
}

// Process membaca paling banyak maxBytes dari r, memvalidasi, dan membuang metadata.
func Process(r io.Reader, maxBytes int64) (*Image, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}

	raw, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return nil, ErrTooLarge
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidImage, err)
	}
	if int64(len(raw)) > maxBytes {
		return nil, ErrTooLarge
	}
	if len(raw) == 0 {
		return nil, ErrInvalidImage
	}

	switch sniff(raw) {
	case "image/jpeg":
		return reencode(raw, "jpg", jpeg.DecodeConfig, jpeg.Decode, func(w io.Writer, img image.Image) error {
			return jpeg.Encode(w, img, &jpeg.Options{Quality: 85})
		})
	case "image/png":
		return reencode(raw, "png", png.DecodeConfig, png.Decode, func(w io.Writer, img image.Image) error {
			encoder := png.Encoder{CompressionLevel: png.BestCompression}
			return encoder.Encode(w, img)
		})
	case "image/webp":
		return processWebP(raw)
	default:
		return nil, ErrUnsupportedType
	}
}

func sniff(raw []byte) string {
	// http.DetectContentType mengenali jpeg, png, dan webp dari magic bytes.
	return http.DetectContentType(raw)
}

func checkDimensions(cfg image.Config) error {
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return ErrInvalidImage
	}
	if cfg.Width > MaxSide || cfg.Height > MaxSide || cfg.Width*cfg.Height > MaxPixels {
		return ErrTooManyPixels
	}
	return nil
}

func reencode(
	raw []byte,
	ext string,
	decodeConfig func(io.Reader) (image.Config, error),
	decode func(io.Reader) (image.Image, error),
	encode func(io.Writer, image.Image) error,
) (*Image, error) {
	cfg, err := decodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, ErrInvalidImage
	}
	if err := checkDimensions(cfg); err != nil {
		return nil, err
	}

	img, err := decode(bytes.NewReader(raw))
	if err != nil {
		return nil, ErrInvalidImage
	}

	var out bytes.Buffer
	if err := encode(&out, img); err != nil {
		return nil, fmt.Errorf("encode %s: %w", ext, err)
	}
	return &Image{Data: out.Bytes(), Ext: ext, Width: cfg.Width, Height: cfg.Height}, nil
}

func processWebP(raw []byte) (*Image, error) {
	cfg, err := webp.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, ErrInvalidImage
	}
	if err := checkDimensions(cfg); err != nil {
		return nil, err
	}
	// Decode penuh untuk memastikan data gambar benar-benar valid.
	if _, err := webp.Decode(bytes.NewReader(raw)); err != nil {
		return nil, ErrInvalidImage
	}

	cleaned, err := StripWebPMetadata(raw)
	if err != nil {
		return nil, ErrInvalidImage
	}
	return &Image{Data: cleaned, Ext: "webp", Width: cfg.Width, Height: cfg.Height}, nil
}

// VP8X feature flags (https://developers.google.com/speed/webp/docs/riff_container).
const (
	vp8xFlagXMP  = 1 << 2
	vp8xFlagEXIF = 1 << 3
)

// StripWebPMetadata menghapus chunk EXIF dan XMP dari container RIFF WebP
// dan menyesuaikan flag VP8X serta ukuran RIFF. Data gambar tidak diubah.
func StripWebPMetadata(raw []byte) ([]byte, error) {
	if len(raw) < 12 || string(raw[0:4]) != "RIFF" || string(raw[8:12]) != "WEBP" {
		return nil, ErrInvalidImage
	}

	out := make([]byte, 0, len(raw))
	out = append(out, raw[0:12]...)
	vp8xOffset := -1

	for offset := 12; offset < len(raw); {
		if offset+8 > len(raw) {
			return nil, ErrInvalidImage
		}
		fourCC := string(raw[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(raw[offset+4 : offset+8]))
		padded := size + size%2
		end := offset + 8 + padded
		if size < 0 || end > len(raw) {
			// Chunk terakhir boleh tanpa byte padding.
			if offset+8+size == len(raw) {
				end = len(raw)
			} else {
				return nil, ErrInvalidImage
			}
		}

		switch fourCC {
		case "EXIF", "XMP ":
			// dibuang
		default:
			if fourCC == "VP8X" {
				vp8xOffset = len(out)
			}
			out = append(out, raw[offset:end]...)
		}
		offset = end
	}

	if vp8xOffset >= 0 && vp8xOffset+8 < len(out) {
		out[vp8xOffset+8] &^= vp8xFlagEXIF | vp8xFlagXMP
	}
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8))
	return out, nil
}
