package micropub

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseRequest(t *testing.T) {
	t.Parallel()

	t.Run("application/x-www-form-urlencoded", func(t *testing.T) {
		for _, testCase := range []struct {
			body             string
			expectedError    error
			expectedResponse *Request
		}{
			{
				"", ErrNoData, nil,
			},
			{
				"h=entry&content=hello+world&category[]=foo&category[]=bar",
				nil,
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
				"h=entry&action=delete&content=hello+world&category[]=foo&category[]=bar",
				ErrNoActionCreate,
				nil,
			},
			{
				"h=entry&content=hello+world&photo=https%3A%2F%2Fphotos.example.com%2F592829482876343254.jpg",
				nil,
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
				nil,
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
				nil,
				&Request{
					Action: ActionDelete,
					URL:    "https://example.com/test",
				},
			},
			{
				"action=undelete&url=https://example.com/test",
				nil,
				&Request{
					Action: ActionUndelete,
					URL:    "https://example.com/test",
				},
			},
			{
				"action=delete",
				ErrNoURL, nil,
			},
			{
				"action=undelete",
				ErrNoURL, nil,
			},
			{
				"action=update&url=https://example.com/test",
				ErrNoFormUpdate,
				nil,
			},
		} {
			httpReq := httptest.NewRequest(http.MethodPost, "/micropub", bytes.NewReader([]byte(testCase.body)))
			httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			req, err := ParseRequest(httpReq)
			require.ErrorIs(t, err, testCase.expectedError)
			require.EqualValues(t, testCase.expectedResponse, req)
		}
	})

	t.Run("application/json", func(t *testing.T) {
		for _, testCase := range []struct {
			body             string
			expectedError    error
			expectedResponse *Request
		}{
			{
				"{}", ErrNoData, nil,
			},
			{
				`{"type":["h-entry"],"properties":{"category":["foo","bar"],"content":["hello world"]}}`,
				nil,
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
				nil,
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
				`{"type":["h-entry", "h-review"],"properties":{"category":["foo","bar"],"content":["hello world"],"mp-command":["blah"]}}`,
				ErrMultipleTypes,
				nil,
			},
			{
				`{"action":"delete","url":"https://example.com/test"}`,
				nil,
				&Request{
					Action: ActionDelete,
					URL:    "https://example.com/test",
				},
			},
			{
				`{"action":"undelete","url":"https://example.com/test"}`,
				nil,
				&Request{
					Action: ActionUndelete,
					URL:    "https://example.com/test",
				},
			},
			{
				`{"action":"delete"}`,
				ErrNoURL, nil,
			},
			{
				`{"action":"undelete"}`,
				ErrNoURL, nil,
			},
			{
				`{"action":"update"}`,
				ErrNoURL, nil,
			},
			{
				`{"action":"update","url":"https://example.com/test","delete":["category"]}`,
				nil,
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
				nil,
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
		} {
			httpReq := httptest.NewRequest(http.MethodPost, "/micropub", bytes.NewReader([]byte(testCase.body)))
			httpReq.Header.Set("Content-Type", "application/json")

			req, err := ParseRequest(httpReq)
			require.ErrorIs(t, err, testCase.expectedError)
			require.EqualValues(t, testCase.expectedResponse, req)
		}
	})
}
