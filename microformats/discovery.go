package microformats

import (
	"regexp"
	"strings"
)

// Type represents a post type.
type Type string

const (
	TypeRsvp      Type = "rsvp"
	TypeRepost    Type = "repost"
	TypeLike      Type = "like"
	TypeReply     Type = "reply"
	TypeBookmark  Type = "bookmark"
	TypeFollow    Type = "follow"
	TypeRead      Type = "read"
	TypeWatch     Type = "watch"
	TypeListen    Type = "listen"
	TypeCheckin   Type = "checkin"
	TypeVideo     Type = "video"
	TypeAudio     Type = "audio"
	TypePhoto     Type = "photo"
	TypeEvent     Type = "event"
	TypeRecipe    Type = "recipe"
	TypeReview    Type = "review"
	TypeNote      Type = "note"
	TypeArticle   Type = "article"
	TypeAte       Type = "ate"
	TypeDrank     Type = "drank"
	TypeItinerary Type = "itinerary"
	TypeUnknown   Type = "unknown"
)

type propTyp struct {
	prop string
	typ  Type
}

var propertyToType = []propTyp{
	{"rsvp", TypeRsvp},
	{"repost-of", TypeRepost},
	{"like-of", TypeLike},
	{"in-reply-to", TypeReply},
	{"bookmark-of", TypeBookmark},
	{"follow-of", TypeFollow},
	{"read-of", TypeRead},
	{"watch-of", TypeWatch},
	{"listen-of", TypeListen},
	{"checkin", TypeCheckin},
	{"ate", TypeAte},
	{"drank", TypeDrank},
	{"itinerary", TypeItinerary},

	// Most of the posts above can be accompanied by these,
	// so they are naturally the last ones.
	{"video", TypeVideo},
	{"audio", TypeAudio},
	{"photo", TypePhoto},
}

// PropertyToType retrieves the [Type] that corresponds to a given property.
// For example, given the property "listen-of", [TypeListen] would be returned.
// Return is [TypeUnknown] if no match was found.
func PropertyToType(prop string) Type {
	for _, v := range propertyToType {
		if v.prop == prop {
			return v.typ
		}
	}

	return TypeUnknown
}

// DiscoverType discovers the [Type] from a Microformat type, according to the
// [Post Type Discovery] algorithm. This is a slightly modified version that
// includes all other post types and checking for their properties.
//
// [Post Type Discovery]: https://www.w3.org/TR/post-type-discovery/
func DiscoverType(data map[string]any) (Type, string) {
	typ := getMf2Type(data)
	switch typ {
	case "event", "recipe", "review":
		return Type(typ), ""
	}

	properties := getMf2Properties(data)
	for _, v := range propertyToType {
		if _, ok := properties[v.prop]; ok {
			return v.typ, v.prop
		}
	}

	name, _ := getMf2String(properties, "name")
	if name == "" {
		return TypeNote, ""
	}

	// Get content (or summary), and collapse all sequences of internal whitespace
	// to a single space (0x20) character each.
	content := getMf2ContentOrSummary(properties)
	var re = regexp.MustCompile(`/\s+/`)
	name = re.ReplaceAllString(name, " ")
	content = re.ReplaceAllString(content, " ")

	// Trim whitespace.
	name = strings.TrimSpace(name)
	content = strings.TrimSpace(content)

	// If this processed "name" property value is NOT a prefix of the
	// processed "content" property, then it is an article post.
	if strings.Index(content, name) != 0 {
		return TypeArticle, ""
	}

	return TypeNote, ""
}

func getMf2Type(mf2 map[string]any) string {
	typeAny, ok := mf2["type"]
	if !ok {
		return ""
	}

	var typ string

	if typeSlice, ok := typeAny.([]string); ok {
		if len(typeSlice) != 0 {
			typ = typeSlice[0]
		}
	} else if typeSlice, ok := typeAny.([]any); ok {
		if len(typeSlice) != 0 {
			typ, _ = typeSlice[0].(string)
		}
	}

	return strings.TrimPrefix(typ, "h-")
}

func getMf2Properties(mf2 map[string]any) map[string][]any {
	propertiesAny, ok := mf2["properties"]
	if !ok {
		return nil
	}

	if properties, ok := propertiesAny.(map[string][]any); ok {
		return properties
	}

	propertiesMap, ok := propertiesAny.(map[string]any)
	if !ok {
		return nil
	}

	properties := map[string][]any{}
	for k, v := range propertiesMap {
		vSlice, ok := v.([]any)
		if ok {
			// Just ignore everything that does not comply with the specification.
			properties[k] = vSlice
		}
	}
	return properties
}

func getMf2ContentOrSummary(properties map[string][]any) string {
	if contentSlice, ok := properties["content"]; ok {
		if len(contentSlice) != 0 {
			contentMap, ok := contentSlice[0].(map[string]any)
			if ok {
				if content, ok := contentMap["text"].(string); ok && content != "" {
					return content
				}

				if content, ok := contentMap["value"].(string); ok && content != "" {
					return content
				}
			}
		}
	}

	content, _ := getMf2String(properties, "summary")
	return content
}

func getMf2String(properties map[string][]any, property string) (string, bool) {
	slice, ok := properties[property]
	if !ok {
		return "", false
	}

	if len(slice) == 0 {
		return "", false
	}

	v, ok := slice[0].(string)
	return v, ok
}
