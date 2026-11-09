package httpapi

import (
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/internal/platform/httpx"
)

// multipartOverhead memberi ruang untuk header multipart di atas batas ukuran file.
const multipartOverhead = 64 << 10

// photoPart membuka bagian "file" dari body multipart/form-data secara streaming.
// Body dibatasi agar klien tidak dapat mengirim data tanpa batas.
func (h *Handler) photoPart(w http.ResponseWriter, r *http.Request) (io.Reader, func(), error) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		return nil, nil, apperror.Validation(map[string]string{"file": "is required"})
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadBytes+multipartOverhead)
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, nil, apperror.Validation(map[string]string{"file": "is required"})
	}

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return nil, nil, apperror.Validation(map[string]string{"file": "is required"})
		}
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				return nil, nil, apperror.Validation(map[string]string{"file": "must not exceed 5242880 bytes"})
			}
			return nil, nil, apperror.Validation(map[string]string{"file": "is required"})
		}
		if part.FormName() == "file" {
			return part, func() { _ = part.Close() }, nil
		}
		_ = part.Close()
	}
}

// PutMyStorePhoto: PUT /v1/stores/mine/photo (multipart, field "file")
func (h *Handler) PutMyStorePhoto(w http.ResponseWriter, r *http.Request) error {
	principal, err := httpx.PrincipalFromRequest(r)
	if err != nil {
		return err
	}
	file, done, err := h.photoPart(w, r)
	if err != nil {
		return err
	}
	defer done()

	store, err := h.app.SetMyStorePhoto(r.Context(), principal.UserID, file)
	if err != nil {
		return err
	}
	httpx.WriteData(w, r, http.StatusOK, newStoreResponse(*store))
	return nil
}

// DeleteMyStorePhoto: DELETE /v1/stores/mine/photo
func (h *Handler) DeleteMyStorePhoto(w http.ResponseWriter, r *http.Request) error {
	principal, err := httpx.PrincipalFromRequest(r)
	if err != nil {
		return err
	}
	store, err := h.app.SetMyStorePhoto(r.Context(), principal.UserID, nil)
	if err != nil {
		return err
	}
	httpx.WriteData(w, r, http.StatusOK, newStoreResponse(*store))
	return nil
}

// PutProductPhoto: PUT /v1/products/{productID}/photo (multipart, field "file")
func (h *Handler) PutProductPhoto(w http.ResponseWriter, r *http.Request) error {
	businessID, _, err := h.scope(r)
	if err != nil {
		return err
	}
	productID, err := pathUUID(r, "productID")
	if err != nil {
		return err
	}
	file, done, err := h.photoPart(w, r)
	if err != nil {
		return err
	}
	defer done()

	product, err := h.app.SetProductPhoto(r.Context(), businessID, productID, file)
	if err != nil {
		return err
	}
	httpx.WriteData(w, r, http.StatusOK, newProductResponse(*product))
	return nil
}

// DeleteProductPhoto: DELETE /v1/products/{productID}/photo
func (h *Handler) DeleteProductPhoto(w http.ResponseWriter, r *http.Request) error {
	businessID, _, err := h.scope(r)
	if err != nil {
		return err
	}
	productID, err := pathUUID(r, "productID")
	if err != nil {
		return err
	}
	product, err := h.app.SetProductPhoto(r.Context(), businessID, productID, nil)
	if err != nil {
		return err
	}
	httpx.WriteData(w, r, http.StatusOK, newProductResponse(*product))
	return nil
}
