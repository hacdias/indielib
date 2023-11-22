package micropub

import (
	"bytes"
	"crypto/rand"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeRandomBytes(t *testing.T, n int64) []byte {
	data := make([]byte, n)
	_, err := rand.Read(data)
	require.NoError(t, err)
	return data
}

func TestMediaHandler(t *testing.T) {
	makeFormFile := func(t *testing.T, data []byte) (io.Reader, *multipart.Writer) {
		bodyBuf := &bytes.Buffer{}
		bodyWriter := multipart.NewWriter(bodyBuf)

		part, err := bodyWriter.CreateFormFile("file", "text.dat")
		require.NoError(t, err)

		_, err = part.Write(data)
		require.NoError(t, err)

		err = bodyWriter.Close()
		require.NoError(t, err)

		return bodyBuf, bodyWriter
	}

	scopeChecker := func(r *http.Request, scope string) bool {
		return scope == "media"
	}

	t.Run("OK Request", func(t *testing.T) {
		data := makeRandomBytes(t, 1024)

		uploader := func(file multipart.File, header *multipart.FileHeader) (string, error) {
			received := make([]byte, 1024)
			_, err := file.Read(received)
			require.NoError(t, err)
			require.True(t, bytes.Equal(data, received))
			return "https://example.com/text.dat", nil
		}

		body, mp := makeFormFile(t, data)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", body)
		r.Header.Set("Content-Type", mp.FormDataContentType())

		handler := NewMediaHandler(uploader, scopeChecker)
		handler.ServeHTTP(w, r)
		assert.Equal(t, http.StatusCreated, w.Result().StatusCode)
		assert.Equal(t, "https://example.com/text.dat", w.Result().Header.Get("Location"))
	})

	t.Run("Max Size", func(t *testing.T) {
		data := makeRandomBytes(t, 1024)

		uploader := func(file multipart.File, header *multipart.FileHeader) (string, error) {
			return "", ErrNotImplemented
		}

		body, mp := makeFormFile(t, data)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", body)
		r.Header.Set("Content-Type", mp.FormDataContentType())

		handler := NewMediaHandler(uploader, scopeChecker, WithMaxMediaSize(512))
		handler.ServeHTTP(w, r)
		assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)

		requestBody, err := io.ReadAll(w.Result().Body)
		assert.NoError(t, err)
		assert.EqualValues(t, `{"error":"invalid_request","error_description":"invalid request: http: request body too large"}`+"\n", string(requestBody))
	})
}
