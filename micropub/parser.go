package micropub

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var (
	ErrNoFormUpdate   = errors.New("micropub update actions require using the JSON syntax")
	ErrNoURL          = errors.New("micropub actions require a URL property")
	ErrNoData         = errors.New("no micropub data was found in the request")
	ErrNoActionCreate = errors.New("cannot specify an action when creating a post")
	ErrMultipleTypes  = errors.New("type must have a single value")
)

// Action represents a Micropub action.
type Action string

const (
	ActionCreate   Action = "create"
	ActionUpdate   Action = "update"
	ActionDelete   Action = "delete"
	ActionUndelete Action = "undelete"
)

type RequestUpdate struct {
	Replace map[string][]any
	Add     map[string][]any
	Delete  any
}

// Request describes a Micropub request.
type Request struct {
	Action     Action
	URL        string
	Type       string
	Properties map[string][]any
	Commands   map[string][]any
	Updates    RequestUpdate
}

// ParseRequest parses a Micropub POST [http.Request] into a [Request] object.
// Supports both JSON and form-encoded requests.
func ParseRequest(r *http.Request) (*Request, error) {
	contentType := r.Header.Get("Content-type")
	if strings.Contains(contentType, "application/json") {
		return parseJSON(r.Body)
	}

	err := r.ParseForm()
	if err != nil {
		return nil, err
	}

	return parseFormEncoded(r.Form)
}

func parseFormEncoded(body url.Values) (*Request, error) {
	req := &Request{}

	if typ := body.Get("h"); typ != "" {
		req.Properties = map[string][]interface{}{}
		req.Commands = map[string][]interface{}{}
		req.Action = ActionCreate
		req.Type = "h-" + typ

		delete(body, "h")
		delete(body, "access_token")

		if _, ok := body["action"]; ok {
			return nil, ErrNoActionCreate
		}

		for key, val := range body {
			if len(val) == 0 {
				continue
			}

			// TODO: some wild micropub clients seem to be posting stuff
			// such as properties[checkin][location]. It'd be great to have
			// a way to parse that easily. Look into libraries.
			key = strings.TrimSuffix(key, "[]")

			if strings.HasPrefix(key, "mp-") {
				req.Commands[strings.TrimPrefix(key, "mp-")] = asAnySlice(val)
			} else {
				req.Properties[key] = asAnySlice(val)
			}
		}

		return req, nil
	}

	if action := body.Get("action"); action != "" {
		if action == string(ActionUpdate) {
			return nil, ErrNoFormUpdate
		}

		if url := body.Get("url"); url != "" {
			req.URL = url
		} else {
			return nil, ErrNoURL
		}

		req.Action = Action(action)
		return req, nil
	}

	return nil, ErrNoData
}

type requestJSON struct {
	Type       []string         `json:"type,omitempty"`
	URL        string           `json:"url,omitempty"`
	Action     Action           `json:"action,omitempty"`
	Properties map[string][]any `json:"properties,omitempty"`
	Replace    map[string][]any `json:"replace,omitempty"`
	Add        map[string][]any `json:"add,omitempty"`
	Delete     interface{}      `json:"delete,omitempty"`
}

func parseJSON(r io.Reader) (*Request, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	body := requestJSON{}
	err = json.Unmarshal(data, &body)
	if err != nil {
		return nil, err
	}

	req := &Request{}

	if body.Type != nil {
		if len(body.Type) != 1 {
			return nil, ErrMultipleTypes
		}

		req.Properties = map[string][]interface{}{}
		req.Commands = map[string][]interface{}{}
		req.Action = ActionCreate
		req.Type = body.Type[0]

		for key, value := range body.Properties {
			if strings.HasPrefix(key, "mp-") {
				req.Commands[strings.TrimPrefix(key, "mp-")] = value
			} else {
				req.Properties[key] = value
			}
		}

		return req, nil
	}

	if body.Action != "" {
		if body.URL == "" {
			return nil, ErrNoURL
		}

		req.Action = Action(body.Action)
		req.URL = body.URL

		if body.Action == ActionUpdate {
			req.Updates.Add = body.Add
			req.Updates.Replace = body.Replace
			req.Updates.Delete = body.Delete

			// Best effort to get all commands by unmarshaling one more time
			other := map[string]any{}
			err = json.Unmarshal(data, &other)
			if err != nil {
				return nil, err
			}
			req.Commands = map[string][]interface{}{}
			for key, value := range other {
				if strings.HasPrefix(key, "mp-") {
					if arr, ok := value.([]any); ok {
						req.Commands[strings.TrimPrefix(key, "mp-")] = arr
					}
				}
			}
		}

		return req, nil
	}

	return nil, ErrNoData
}

func asAnySlice[T any](str []T) []interface{} {
	arr := []interface{}{}
	for _, s := range str {
		arr = append(arr, s)
	}
	return arr
}
