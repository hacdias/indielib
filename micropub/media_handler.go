package micropub

import (
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
)

const (
	// DefaultMaxMediaSize is the default max media size, which is 20 MiB.
	DefaultMaxMediaSize = 20 << 20
)

// MediaUploader is the media upload function. Must return the location (e.g., URL)
// of the uploaded file.
type MediaUploader func(file multipart.File, header *multipart.FileHeader) (string, error)

// ScopeChecker is a function that checks if the user has the required scope to
// handle the given request.
type ScopeChecker func(r *http.Request, scope string) bool

// MediaConfiguration is the configuration for a media handler.
type MediaConfiguration struct {
	MaxMediaSize int64
}

// MediaOption is an option that configures [MediaConfiguration].
type MediaOption func(*MediaConfiguration)

// WithMaxMediaSize configures the maximum size of media uploads, in bytes. By
// default it is 20 MiB.
func WithMaxMediaSize(size int64) MediaOption {
	return func(conf *MediaConfiguration) {
		conf.MaxMediaSize = size
	}
}

// NewMediaHandler creates a Micropub [media endpoint] handler with the given
// configuration.
//
// [media endpoint]: https://micropub.spec.indieweb.org/#x3-6-media-endpoint
func NewMediaHandler(mediaUploader MediaUploader, scopeChecker ScopeChecker, options ...MediaOption) http.Handler {
	conf := &MediaConfiguration{
		MaxMediaSize: DefaultMaxMediaSize,
	}

	for _, option := range options {
		option(conf)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !scopeChecker(r, "media") {
			serveErrorJSON(w, http.StatusForbidden, "insufficient_scope", "Insufficient scope.")
			return
		}

		if conf.MaxMediaSize != 0 {
			r.Body = http.MaxBytesReader(w, r.Body, conf.MaxMediaSize)
		}

		err := r.ParseMultipartForm(conf.MaxMediaSize)
		if err != nil {
			serveError(w, fmt.Errorf("%w: %w", ErrBadRequest, err))
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			serveError(w, errors.Join(ErrBadRequest, err))
			return
		}
		defer file.Close()

		redirect, err := mediaUploader(file, header)
		if err != nil {
			serveError(w, err)
			return
		}

		http.Redirect(w, r, redirect, http.StatusCreated)
	})
}
