package micropub

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockRouterImplementation struct{ mock.Mock }

var _ Implementation = &mockRouterImplementation{}

func (m *mockRouterImplementation) HasScope(r *http.Request, scope string) bool {
	return m.Called(r, scope).Get(0).(bool)
}

func (m *mockRouterImplementation) Source(url string) (map[string]any, error) {
	args := m.Called(url)
	return args.Get(0).(map[string]any), args.Error(1)
}

func (m *mockRouterImplementation) SourceMany(limit, offset int) ([]map[string]any, error) {
	args := m.Called(limit, offset)
	return args.Get(0).([]map[string]any), args.Error(1)
}

func (m *mockRouterImplementation) Create(req *Request) (string, error) {
	args := m.Called(req)
	return args.Get(0).(string), args.Error(1)
}

func (m *mockRouterImplementation) Update(req *Request) (string, error) {
	args := m.Called(req)
	return args.Get(0).(string), args.Error(1)
}

func (m *mockRouterImplementation) Delete(url string) error {
	return m.Called(url).Error(0)
}

func (m *mockRouterImplementation) Undelete(url string) error {
	return m.Called(url).Error(0)
}

func TestRouterGet(t *testing.T) {
	t.Parallel()

	t.Run("?q=source (list posts, default params)", func(t *testing.T) {
		impl := &mockRouterImplementation{}
		impl.Mock.On("SourceMany", -1, 0).Return([]map[string]any{
			{"type": "h-entry", "properties": map[string][]any{"name": {"A"}}},
			{"type": "h-entry", "properties": map[string][]any{"name": {"B"}}},
			{"type": "h-entry", "properties": map[string][]any{"name": {"C"}}},
		}, nil)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/micropub?q=source", nil)

		handler := NewHandler(impl)
		handler.ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Result().StatusCode)

		body, err := io.ReadAll(w.Result().Body)
		assert.NoError(t, err)
		assert.EqualValues(t, `{"items":[{"properties":{"name":["A"]},"type":"h-entry"},{"properties":{"name":["B"]},"type":"h-entry"},{"properties":{"name":["C"]},"type":"h-entry"}]}`+"\n", string(body))
	})

	t.Run("?q=source&limit&offset (list posts, good params)", func(t *testing.T) {
		impl := &mockRouterImplementation{}
		impl.Mock.On("SourceMany", 3, 10).Return([]map[string]any{
			{"type": "h-entry", "properties": map[string][]any{"name": {"A"}}},
			{"type": "h-entry", "properties": map[string][]any{"name": {"B"}}},
			{"type": "h-entry", "properties": map[string][]any{"name": {"C"}}},
		}, nil)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/micropub?q=source&limit=3&offset=10", nil)

		handler := NewHandler(impl)
		handler.ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Result().StatusCode)

		body, err := io.ReadAll(w.Result().Body)
		assert.NoError(t, err)
		assert.EqualValues(t, `{"items":[{"properties":{"name":["A"]},"type":"h-entry"},{"properties":{"name":["B"]},"type":"h-entry"},{"properties":{"name":["C"]},"type":"h-entry"}]}`+"\n", string(body))
	})

	t.Run("?q=source&limit&offset (list posts, bad limit)", func(t *testing.T) {
		impl := &mockRouterImplementation{}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/micropub?q=source&limit=badLimit&offset=10", nil)

		handler := NewHandler(impl)
		handler.ServeHTTP(w, r)
		assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
	})

	t.Run("?q=source&limit&offset (list posts, bad offset)", func(t *testing.T) {
		impl := &mockRouterImplementation{}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/micropub?q=source&limit=10&offset=badOffset", nil)

		handler := NewHandler(impl)
		handler.ServeHTTP(w, r)
		assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
	})

	t.Run("?q=source&url=", func(t *testing.T) {
		impl := &mockRouterImplementation{}
		impl.Mock.On("Source", "https://example.com/1").Return(map[string]any{"type": "h-entry", "properties": map[string][]any{}}, nil)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/micropub?q=source&url=https://example.com/1", nil)

		handler := NewHandler(impl)
		handler.ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Result().StatusCode)
		body, err := io.ReadAll(w.Result().Body)
		assert.NoError(t, err)
		assert.EqualValues(t, `{"properties":{},"type":"h-entry"}`+"\n", string(body))
	})

	t.Run("?q=config", func(t *testing.T) {
		impl := &mockRouterImplementation{}
		options := []Option{
			WithMediaEndpoint("https://example.com/media"),
			WithGetCategories(func() []string {
				return []string{"a", "b"}
			}),
			WithGetVisibility(func() []string {
				return []string{"public"}
			}),
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/micropub?q=config", nil)

		handler := NewHandler(impl, options...)
		handler.ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Result().StatusCode)
		body, err := io.ReadAll(w.Result().Body)
		assert.NoError(t, err)
		assert.EqualValues(t, `{"categories":["a","b"],"media-endpoint":"https://example.com/media","visibility":["public"]}`+"\n", string(body))
	})

	t.Run("?q=category, syndicate-to, channel", func(t *testing.T) {
		for _, testCase := range []struct {
			options        []Option
			query          string
			expectedStatus int
			expectedBody   []byte
		}{
			{[]Option{WithGetSyndicateTo(func() []Syndication { return []Syndication{{UID: "art-tree", Name: "Art Tree"}} })}, "?q=syndicate-to", http.StatusOK, []byte(`{"syndicate-to":[{"uid":"art-tree","name":"Art Tree"}]}` + "\n")},
			{[]Option{}, "?q=syndicate-to", http.StatusNotFound, nil},
			{[]Option{WithGetCategories(func() []string { return []string{"a", "b"} })}, "?q=category", http.StatusOK, []byte(`{"categories":["a","b"]}` + "\n")},
			{[]Option{}, "?q=category", http.StatusNotFound, nil},
			{[]Option{WithGetChannels(func() []Channel { return []Channel{{UID: "art-tree", Name: "Art Tree"}} })}, "?q=channel", http.StatusOK, []byte(`{"channels":[{"uid":"art-tree","name":"Art Tree"}]}` + "\n")},
			{[]Option{}, "?q=channel", http.StatusNotFound, nil},
		} {
			impl := &mockRouterImplementation{}

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/micropub"+testCase.query, nil)

			handler := NewHandler(impl, testCase.options...)
			handler.ServeHTTP(w, r)
			assert.Equal(t, testCase.expectedStatus, w.Result().StatusCode)

			if testCase.expectedBody != nil {
				body, err := io.ReadAll(w.Result().Body)
				assert.NoError(t, err)
				assert.EqualValues(t, testCase.expectedBody, string(body))
			}
		}
	})

	t.Run("Missing/Invalid Query", func(t *testing.T) {
		impl := &mockRouterImplementation{}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/micropub?q=blah", nil)

		handler := NewHandler(impl)
		handler.ServeHTTP(w, r)

		assert.Equal(t, http.StatusNotFound, w.Result().StatusCode)
	})
}

func TestRouterPost(t *testing.T) {
	t.Parallel()

	t.Run("Valid Request", func(t *testing.T) {
		for _, request := range validRequests {
			impl := &mockRouterImplementation{}

			switch request.response.Action {
			case ActionCreate:
				impl.Mock.On("HasScope", mock.Anything, "create").Return(true)
				impl.Mock.On("Create", request.response).Return("https://example.org/1", nil)
			case ActionUpdate:
				impl.Mock.On("HasScope", mock.Anything, "update").Return(true)
				impl.Mock.On("Update", request.response).Return(request.response.URL, nil)
			case ActionDelete:
				impl.Mock.On("HasScope", mock.Anything, "delete").Return(true)
				impl.Mock.On("Delete", request.response.URL).Return(nil)
			case ActionUndelete:
				impl.Mock.On("HasScope", mock.Anything, "undelete").Return(true)
				impl.Mock.On("Undelete", request.response.URL).Return(nil)
			}

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/micropub", bytes.NewReader([]byte(request.body)))
			r.Header.Set("Content-Type", request.contentType)

			handler := NewHandler(impl)
			handler.ServeHTTP(w, r)

			switch request.response.Action {
			case ActionCreate:
				assert.Equal(t, "https://example.org/1", w.Result().Header.Get("Location"))
				assert.Equal(t, http.StatusAccepted, w.Result().StatusCode)
			case ActionUpdate:
				assert.Equal(t, request.response.URL, w.Result().Header.Get("Location"))
				assert.Equal(t, http.StatusOK, w.Result().StatusCode)
			case ActionDelete:
				assert.Equal(t, http.StatusOK, w.Result().StatusCode)
			case ActionUndelete:
				assert.Equal(t, http.StatusOK, w.Result().StatusCode)
			}
		}
	})

	t.Run("Valid Request, No Scope Permission", func(t *testing.T) {
		for _, request := range validRequests {
			impl := &mockRouterImplementation{}

			switch request.response.Action {
			case ActionCreate:
				impl.Mock.On("HasScope", mock.Anything, "create").Return(false)
			case ActionUpdate:
				impl.Mock.On("HasScope", mock.Anything, "update").Return(false)
			case ActionDelete:
				impl.Mock.On("HasScope", mock.Anything, "delete").Return(false)
			case ActionUndelete:
				impl.Mock.On("HasScope", mock.Anything, "undelete").Return(false)
			}

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/micropub", bytes.NewReader([]byte(request.body)))
			r.Header.Set("Content-Type", request.contentType)

			handler := NewHandler(impl)
			handler.ServeHTTP(w, r)

			body, err := io.ReadAll(w.Result().Body)
			assert.NoError(t, err)

			assert.Equal(t, http.StatusForbidden, w.Result().StatusCode)
			assert.Equal(t, `{"error":"insufficient_scope","error_description":"Insufficient scope."}`+"\n", string(body))
		}
	})

	t.Run("Valid Request, Implementation Errored", func(t *testing.T) {
		for _, request := range validRequests {
			impl := &mockRouterImplementation{}

			magicError := errors.New("magic error")

			switch request.response.Action {
			case ActionCreate:
				impl.Mock.On("HasScope", mock.Anything, "create").Return(true)
				impl.Mock.On("Create", request.response).Return("", magicError)
			case ActionUpdate:
				impl.Mock.On("HasScope", mock.Anything, "update").Return(true)
				impl.Mock.On("Update", request.response).Return("", magicError)
			case ActionDelete:
				impl.Mock.On("HasScope", mock.Anything, "delete").Return(true)
				impl.Mock.On("Delete", request.response.URL).Return(magicError)
			case ActionUndelete:
				impl.Mock.On("HasScope", mock.Anything, "undelete").Return(true)
				impl.Mock.On("Undelete", request.response.URL).Return(magicError)
			}

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/micropub", bytes.NewReader([]byte(request.body)))
			r.Header.Set("Content-Type", request.contentType)

			handler := NewHandler(impl)
			handler.ServeHTTP(w, r)

			body, err := io.ReadAll(w.Result().Body)
			assert.NoError(t, err)

			assert.Equal(t, http.StatusInternalServerError, w.Result().StatusCode)
			assert.Contains(t, string(body), "server_error")
		}
	})

	t.Run("Valid Requests, Custom Errors", func(t *testing.T) {
		for _, testCase := range []struct {
			err    error
			status int
		}{
			{ErrBadRequest, http.StatusBadRequest},
			{ErrNotFound, http.StatusNotFound},
			{ErrNotImplemented, http.StatusNotImplemented},
			{errors.New("something else"), http.StatusInternalServerError},
		} {
			body := "h=entry&content=hello+world&category[]=foo&category[]=bar"
			request := &Request{
				Action:   ActionCreate,
				Type:     "h-entry",
				Commands: map[string][]any{},
				Properties: map[string][]any{
					"category": {"foo", "bar"},
					"content":  {"hello world"},
				},
			}

			impl := &mockRouterImplementation{}
			impl.Mock.On("HasScope", mock.Anything, "create").Return(true)
			impl.Mock.On("Create", request).Return("", testCase.err)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/micropub", bytes.NewReader([]byte(body)))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			handler := NewHandler(impl)
			handler.ServeHTTP(w, r)
			assert.Equal(t, testCase.status, w.Result().StatusCode)
		}
	})

	t.Run("Invalid Requests", func(t *testing.T) {
		for _, request := range invalidRequests {
			impl := &mockRouterImplementation{}

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/micropub", bytes.NewReader([]byte(request.body)))
			r.Header.Set("Content-Type", request.contentType)

			handler := NewHandler(impl)
			handler.ServeHTTP(w, r)

			body, err := io.ReadAll(w.Result().Body)
			assert.NoError(t, err)

			assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
			assert.Contains(t, string(body), "invalid_request")
		}
	})
}
