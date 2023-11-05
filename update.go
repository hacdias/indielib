package micropub

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/samber/lo"
)

// UpdateProperties applies the updates (additions, deletions, replacements)
// in the given [Request] to a set of existing microformats properties.
func UpdateProperties(properties map[string][]interface{}, req *Request) (map[string][]interface{}, error) {
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
					properties[key] = []interface{}{}
				}

				properties[key] = append(properties[key], value...)
			}
		}
	}

	if req.Updates.Delete != nil {
		if reflect.TypeOf(req.Updates.Delete).Kind() == reflect.Slice {
			toDelete, ok := req.Updates.Delete.([]interface{})
			if !ok {
				return nil, errors.New("invalid delete array")
			}

			for _, key := range toDelete {
				delete(properties, fmt.Sprint(key))
			}
		} else {
			toDelete, ok := req.Updates.Delete.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid delete object: expected map[string]interface{}, got: %s", reflect.TypeOf(req.Updates.Delete))
			}

			for key, v := range toDelete {
				value, ok := v.([]interface{})
				if !ok {
					return nil, fmt.Errorf("invalid value: expected []interface{}, got: %s", reflect.TypeOf(value))
				}

				if _, ok := properties[key]; !ok {
					properties[key] = []interface{}{}
				}

				properties[key] = lo.Filter(properties[key], func(ss interface{}, _ int) bool {
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
