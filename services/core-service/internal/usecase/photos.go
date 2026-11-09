package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/google/uuid"
	"github.com/titipku/titipku-backend/internal/platform/apperror"
	"github.com/titipku/titipku-backend/internal/platform/imageupload"
	"github.com/titipku/titipku-backend/services/core-service/internal/domain"
)

// ImageStore menyimpan dan menghapus file foto (mis. direktori volume Docker).
type ImageStore interface {
	Save(img *imageupload.Image) (string, error)
	Delete(publicPath string) error
}

// WithImageStore memasang penyimpanan foto. Tanpa ini, endpoint foto mengembalikan 503.
func (a *App) WithImageStore(images ImageStore, maxBytes int64) *App {
	a.images = images
	a.maxImageBytes = maxBytes
	return a
}

// photoValidationError memetakan error pemrosesan gambar ke error validasi field "file".
func photoValidationError(err error) error {
	switch {
	case errors.Is(err, imageupload.ErrTooLarge):
		return apperror.Validation(map[string]string{"file": "must not exceed 5242880 bytes"})
	case errors.Is(err, imageupload.ErrUnsupportedType):
		return apperror.Validation(map[string]string{"file": "must be one of jpg, png, webp"})
	case errors.Is(err, imageupload.ErrTooManyPixels):
		return apperror.Validation(map[string]string{"file": "must be between 1 and 8000 pixels per side"})
	case errors.Is(err, imageupload.ErrInvalidImage):
		return apperror.Validation(map[string]string{"file": "must be a valid image"})
	default:
		return apperror.Internal("PHOTO_PROCESS_FAILED", "unable to process photo").WithCause(err)
	}
}

// savePhoto memvalidasi, membersihkan metadata, dan menyimpan foto.
func (a *App) savePhoto(r io.Reader) (string, error) {
	if a.images == nil {
		return "", apperror.Unavailable("UPLOADS_DISABLED", "photo uploads are not configured")
	}
	img, err := imageupload.Process(r, a.maxImageBytes)
	if err != nil {
		return "", photoValidationError(err)
	}
	url, err := a.images.Save(img)
	if err != nil {
		return "", apperror.Internal("PHOTO_SAVE_FAILED", "unable to save photo").WithCause(err)
	}
	return url, nil
}

func (a *App) deletePhoto(path *string) {
	if a.images == nil || path == nil {
		return
	}
	if err := a.images.Delete(*path); err != nil {
		// File yatim tidak memengaruhi data; cukup dicatat.
		slog.Warn("unable to delete old photo", "path", *path, "error", err)
	}
}

// SetMyStorePhoto mengganti foto toko milik pemilik toko yang sedang login.
// r nil berarti menghapus foto.
func (a *App) SetMyStorePhoto(ctx context.Context, userID uuid.UUID, r io.Reader) (*domain.Store, error) {
	var newPath *string
	if r != nil {
		url, err := a.savePhoto(r)
		if err != nil {
			return nil, err
		}
		newPath = &url
	}

	store, previous, err := a.store.SetStorePhoto(ctx, userID, newPath)
	if err != nil {
		a.deletePhoto(newPath)
		if errors.Is(err, domain.ErrNotFound) {
			return nil, apperror.NotFound("STORE_NOT_FOUND", "this account does not have a store yet")
		}
		return nil, mapStoreError(err, "STORE_PHOTO_FAILED", "unable to update store photo")
	}
	a.deletePhoto(previous)
	return store, nil
}

// SetProductPhoto mengganti foto produk milik bisnis penitip. r nil berarti menghapus foto.
func (a *App) SetProductPhoto(ctx context.Context, businessID, productID uuid.UUID, r io.Reader) (*domain.Product, error) {
	var newPath *string
	if r != nil {
		url, err := a.savePhoto(r)
		if err != nil {
			return nil, err
		}
		newPath = &url
	}

	product, previous, err := a.store.SetProductPhoto(ctx, businessID, productID, newPath)
	if err != nil {
		a.deletePhoto(newPath)
		return nil, mapStoreError(err, "PRODUCT_PHOTO_FAILED", "unable to update product photo")
	}
	a.deletePhoto(previous)
	return product, nil
}
