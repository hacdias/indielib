package microformats

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPropertyToType(t *testing.T) {
	t.Parallel()

	t.Run("Known Types", func(t *testing.T) {
		for _, propType := range propertyToType {
			assert.Equal(t, propType.typ, PropertyToType(propType.prop))
		}
	})

	t.Run("Unknown Types", func(t *testing.T) {
		for _, prop := range []string{"blah", "listen-", "bookmark"} {
			assert.Equal(t, TypeUnknown, PropertyToType(prop))
		}
	})
}

func TestDiscoverType(t *testing.T) {
	t.Parallel()

	t.Run("Edge Cases", func(t *testing.T) {
		testCases := []struct {
			properties       string
			expectedType     Type
			expectedProperty string
		}{
			{`{"type":["h-recipe"],"properties":{"name":["Microformats are amazing"],"summary":["In which I extoll the virtues of using microformats."],"content":[{"value":"Blah blah blah","html":"<p>Blah blah blah</p>"}]}}`, TypeRecipe, ""},
			{`{"type":["h-event"],"properties":{"name":["Microformats are amazing"],"summary":["In which I extoll the virtues of using microformats."],"content":[{"value":"Blah blah blah","html":"<p>Blah blah blah</p>"}]}}`, TypeEvent, ""},
			{`{"type":["h-review"],"properties":{"name":["Microformats are amazing"],"summary":["In which I extoll the virtues of using microformats."],"content":[{"value":"Blah blah blah","html":"<p>Blah blah blah</p>"}]}}`, TypeReview, ""},
			{`{"type":["h-entry"],"properties":{"name":["Microformats are amazing"],"summary":["In which I extoll the virtues of using microformats."],"content":[{"value":"Blah blah blah","html":"<p>Blah blah blah</p>"}]}}`, TypeArticle, ""},
			{`{"type":["h-entry"],"properties":{"summary":["In which I extoll the virtues of using microformats."],"content":[{"value":"Blah blah blah","html":"<p>Blah blah blah</p>"}]}}`, TypeNote, ""},
			{`{"type":["h-entry"],"properties":{"name":["Microformats are amazing"],"summary":["In which I extoll the virtues of using microformats."],"content":[{"value":"Microformats are amazing","html":"<p>Microformats are amazing</p>"}]}}`, TypeNote, ""},
			{`{"type":["h-entry"],"properties":{"name":["Microformats are amazing"],"summary":["In which I extoll the virtues of using microformats."],"content":[{"value":"Microformats are amazing and blah blah","html":"<p>Microformats are amazing</p>"}]}}`, TypeNote, ""},
			{`{"type":["h-entry"],"properties":{"name":["Microformats are amazing"],"summary":["In which I extoll the virtues of using microformats."],"content":[{"value":"      Microformats are amazing and blah blah","html":"<p>Microformats are amazing</p>"}]}}`, TypeNote, ""},
			{`{"type":["h-entry"],"properties":{"name":["Microformats are amazing"],"summary":["Microformats are amazing"]}}`, TypeNote, ""},
			{`{"type":["h-entry"],"properties":{"name":["Microformats are amazing"],"summary":["Microformats are amazing and blah blah"]}}`, TypeNote, ""},
			{`{"type":["h-entry"],"properties":{"name":["Microformats are amazing"],"summary":["     Microformats are amazing and blah blah"]}}`, TypeNote, ""},
			{`{"type":["h-entry"],"properties":{"name":["    Microformats are amazing"],"summary":["In which I extoll the virtues of using microformats."],"content":[{"value":"      Microformats are amazing and blah blah","html":"<p>Microformats are amazing</p>"}]}}`, TypeNote, ""},
			{`{"type":["h-entry"],"properties":{"name":["    Microformats are amazing"],"summary":["Microformats are amazing"],"content":[{"value":"Article","html":"<p>Article</p>"}]}}`, TypeArticle, ""},
			{`{"type":["h-entry"],"properties":{"name":["Microformats are amazing"],"summary":["Microformats are amazing"],"content":[{"value":"Article","html":"<p>Article</p>"}]}}`, TypeArticle, ""},

			{`{"type":["h-entry"],"properties":{}}`, TypeNote, ""},
			{`{"type":["h-entry"]}`, TypeNote, ""},
			{`{}`, TypeNote, ""},
		}

		for _, testCase := range testCases {
			var properties map[string]any
			err := json.Unmarshal([]byte(testCase.properties), &properties)
			assert.NoError(t, err)

			typ, prop := DiscoverType(properties)
			assert.Equal(t, testCase.expectedType, typ)
			assert.Equal(t, testCase.expectedProperty, prop)
		}
	})

	t.Run("From Property", func(t *testing.T) {
		for _, propType := range propertyToType {
			data := fmt.Sprintf(`{"type":["h-entry"],"properties":{"name":["Microformats are amazing"],"%s":[{}]}}`, propType.prop)

			var properties map[string]any
			err := json.Unmarshal([]byte(data), &properties)
			assert.NoError(t, err)

			typ, prop := DiscoverType(properties)
			assert.Equal(t, propType.typ, typ)
			assert.Equal(t, propType.prop, prop)
		}
	})
}
