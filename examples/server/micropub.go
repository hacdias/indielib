package main

import (
	"errors"
	"fmt"
	"net/http"
	urlpkg "net/url"
	"reflect"
	"time"

	"go.hacdias.com/indielib/micropub"
)

type micropubImplementation struct {
	*server
}

func (s *micropubImplementation) HasScope(r *http.Request, scope string) bool {
	v := r.Context().Value(scopesContextKey)
	if scopes, ok := v.([]string); ok {
		for _, sc := range scopes {
			if sc == scope {
				return true
			}
		}
	}

	return false
}

func (s *micropubImplementation) Source(urlStr string) (map[string]any, error) {
	url, err := urlpkg.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}

	s.postsMu.RLock()
	defer s.postsMu.RUnlock()

	if post, ok := s.posts[url.Path]; ok {
		return map[string]any{
			"type":       []string{post.Type},
			"properties": post.Properties,
		}, nil
	}

	return nil, micropub.ErrNotFound
}

func (s *micropubImplementation) SourceMany(limit, offset int) ([]map[string]any, error) {
	return nil, micropub.ErrNotImplemented
}

func (s *micropubImplementation) Create(req *micropub.Request) (string, error) {
	newPath := "/" + time.Now().Format(time.RFC3339)

	s.posts[newPath] = post{
		Type:       req.Type,
		Properties: req.Properties,
	}

	return s.profileURL + newPath, nil
}

func (s *micropubImplementation) Update(req *micropub.Request) (string, error) {
	url, err := urlpkg.Parse(req.URL)
	if err != nil {
		return "", fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}

	s.postsMu.Lock()
	defer s.postsMu.Unlock()
	post, ok := s.posts[url.Path]
	if !ok {
		return "", fmt.Errorf("%w does not exist", micropub.ErrBadRequest)
	}

	post.Properties, err = updateProperties(post.Properties, req)
	if err != nil {
		return "", fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}

	return s.profileURL + url.Path, nil
}

func (s *micropubImplementation) Delete(urlStr string) error {
	url, err := urlpkg.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}

	s.postsMu.Lock()
	defer s.postsMu.Unlock()

	delete(s.posts, url.Path)
	return nil
}

func (s *micropubImplementation) Undelete(url string) error {
	return micropub.ErrNotImplemented
}

// updateProperties applies the updates (additions, deletions, replacements)
// in the given [micropub.Request] to a set of existing microformats properties.
func updateProperties(properties map[string][]any, req *micropub.Request) (map[string][]any, error) {
	if req.Updates.Replace != nil {
		for key, value := range req.Updates.Replace {
			properties[key] = value
		}
	}

	if req.Updates.Add != nil {
		for key, value := range req.Updates.Add {
			switch key {
			case "name":
				return nil, errors.New("cannot add a new name")
			case "content":
				return nil, errors.New("cannot add content")
			default:
				if key == "published" {
					if _, ok := properties["published"]; ok {
						return nil, errors.New("cannot replace published through add method")
					}
				}

				if _, ok := properties[key]; !ok {
					properties[key] = []any{}
				}

				properties[key] = append(properties[key], value...)
			}
		}
	}

	if req.Updates.Delete != nil {
		if reflect.TypeOf(req.Updates.Delete).Kind() == reflect.Slice {
			toDelete, ok := req.Updates.Delete.([]any)
			if !ok {
				return nil, errors.New("invalid delete array")
			}

			for _, key := range toDelete {
				delete(properties, fmt.Sprint(key))
			}
		} else {
			toDelete, ok := req.Updates.Delete.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid delete object: expected map[string]any, got: %s", reflect.TypeOf(req.Updates.Delete))
			}

			for key, v := range toDelete {
				value, ok := v.([]any)
				if !ok {
					return nil, fmt.Errorf("invalid value: expected []any, got: %s", reflect.TypeOf(value))
				}

				if _, ok := properties[key]; !ok {
					properties[key] = []any{}
				}

				properties[key] = filter(properties[key], func(ss any, _ int) bool {
					for _, s := range value {
						if s == ss {
							return false
						}
					}
					return true
				})
			}
		}
	}

	return properties, nil
}

// From https://github.com/samber/lo
func filter[V any](collection []V, predicate func(item V, index int) bool) []V {
	result := make([]V, 0, len(collection))

	for i, item := range collection {
		if predicate(item, i) {
			result = append(result, item)
		}
	}

	return result
}
