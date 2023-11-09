package micropub

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type validRequest struct {
	body        string
	contentType string
	response    *Request
}

type invalidRequest struct {
	body        string
	contentType string
	err         error
}

var (
	validRequests = []validRequest{
		{
			"h=entry&content=hello+world&category[]=foo&category[]=bar",
			"application/x-www-form-urlencoded",
			&Request{
				Action:   ActionCreate,
				Type:     "h-entry",
				Commands: map[string][]any{},
				Properties: map[string][]any{
					"category": {"foo", "bar"},
					"content":  {"hello world"},
				},
			},
		},
		{
			"h=entry&content=hello+world&photo=https%3A%2F%2Fphotos.example.com%2F592829482876343254.jpg",
			"application/x-www-form-urlencoded",
			&Request{
				Action:   ActionCreate,
				Type:     "h-entry",
				Commands: map[string][]any{},
				Properties: map[string][]any{
					"content": {"hello world"},
					"photo":   {"https://photos.example.com/592829482876343254.jpg"},
				},
			},
		},
		{
			"h=entry&content=hello+world&mp-command=blah",
			"application/x-www-form-urlencoded",
			&Request{
				Action: ActionCreate,
				Type:   "h-entry",
				Commands: map[string][]any{
					"mp-command": {"blah"},
				},
				Properties: map[string][]any{
					"content": {"hello world"},
				},
			},
		},
		{
			"action=delete&url=https://example.com/test",
			"application/x-www-form-urlencoded",
			&Request{
				Action: ActionDelete,
				URL:    "https://example.com/test",
			},
		},
		{
			"action=undelete&url=https://example.com/test",
			"application/x-www-form-urlencoded",
			&Request{
				Action: ActionUndelete,
				URL:    "https://example.com/test",
			},
		},
		{
			`{"type":["h-entry"],"properties":{"category":["foo","bar"],"content":["hello world"]}}`,
			"application/json",
			&Request{
				Action:   ActionCreate,
				Type:     "h-entry",
				Commands: map[string][]any{},
				Properties: map[string][]any{
					"category": {"foo", "bar"},
					"content":  {"hello world"},
				},
			},
		},
		{
			`{"type":["h-entry"],"properties":{"category":["foo","bar"],"content":["hello world"],"mp-command":["blah"]}}`,
			"application/json",
			&Request{
				Action: ActionCreate,
				Type:   "h-entry",
				Commands: map[string][]any{
					"mp-command": {"blah"},
				},
				Properties: map[string][]any{
					"category": {"foo", "bar"},
					"content":  {"hello world"},
				},
			},
		},
		{
			`{"action":"delete","url":"https://example.com/test"}`,
			"application/json",
			&Request{
				Action: ActionDelete,
				URL:    "https://example.com/test",
			},
		},
		{
			`{"action":"undelete","url":"https://example.com/test"}`,
			"application/json",
			&Request{
				Action: ActionUndelete,
				URL:    "https://example.com/test",
			},
		},
		{
			`{"action":"update","url":"https://example.com/test","delete":["category"]}`,
			"application/json",
			&Request{
				Action: ActionUpdate,
				URL:    "https://example.com/test",
				Updates: RequestUpdate{
					Delete: []any{"category"},
				},
			},
		},
		{
			`{"action": "update","url":"https://example.com/test","delete":{"category": ["indieweb"]}}`,
			"application/json",
			&Request{
				Action: ActionUpdate,
				URL:    "https://example.com/test",
				Updates: RequestUpdate{
					Delete: map[string]any{
						"category": []any{"indieweb"},
					},
				},
			},
		},
	}

	invalidRequests = []invalidRequest{
		{"", "application/x-www-form-urlencoded", ErrNoData},
		{"h=entry&action=delete&content=hello+world&category[]=foo&category[]=bar", "application/x-www-form-urlencoded", ErrNoActionCreate},
		{"action=delete", "application/x-www-form-urlencoded", ErrNoURL},
		{"action=undelete", "application/x-www-form-urlencoded", ErrNoURL},
		{"action=update&url=https://example.com/test", "application/x-www-form-urlencoded", ErrNoFormUpdate},
		{"{}", "application/json", ErrNoData},
		{`{"type":["h-entry", "h-review"],"properties":{"category":["foo","bar"],"content":["hello world"],"mp-command":["blah"]}}`, "application/json", ErrMultipleTypes},
		{`{"action":"delete"}`, "application/json", ErrNoURL},
		{`{"action":"undelete"}`, "application/json", ErrNoURL},
		{`{"action":"update"}`, "application/json", ErrNoURL},
	}
)

func TestParseRequest(t *testing.T) {
	t.Parallel()

	t.Run("Valid Requests", func(t *testing.T) {
		for _, request := range validRequests {
			r := httptest.NewRequest(http.MethodPost, "/micropub", bytes.NewReader([]byte(request.body)))
			r.Header.Set("Content-Type", request.contentType)
			req, err := ParseRequest(r)
			require.NoError(t, err)
			require.EqualValues(t, request.response, req)
		}
	})

	t.Run("Invalid Requests", func(t *testing.T) {
		for _, request := range invalidRequests {
			r := httptest.NewRequest(http.MethodPost, "/micropub", bytes.NewReader([]byte(request.body)))
			r.Header.Set("Content-Type", request.contentType)
			req, err := ParseRequest(r)
			require.ErrorIs(t, err, request.err)
			require.Nil(t, req)
		}
	})
}
