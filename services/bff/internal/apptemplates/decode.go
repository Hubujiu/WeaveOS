package apptemplates

import (
	"bytes"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appfields"
	"io"
	"reflect"
	"strings"
	"unicode/utf8"
)

// Walk tokens before decoding to reject duplicate keys at every depth, including
// conditions/config/default RawMessages, without relying on last-key-wins JSON.
func tokens(d *json.Decoder, depth int) error {
	if depth > 32 {
		return ErrInvalid
	}
	t, e := d.Token()
	if e != nil {
		return ErrInvalid
	}
	v, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch v {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			k, e := d.Token()
			s, ok := k.(string)
			if e != nil || !ok || seen[s] {
				return ErrInvalid
			}
			seen[s] = true
			if tokens(d, depth+1) != nil {
				return ErrInvalid
			}
		}
		t, e = d.Token()
		if e != nil || t != json.Delim('}') {
			return ErrInvalid
		}
	case '[':
		for d.More() {
			if tokens(d, depth+1) != nil {
				return ErrInvalid
			}
		}
		t, e = d.Token()
		if e != nil || t != json.Delim(']') {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

var rawType = reflect.TypeOf(json.RawMessage{})
var layoutType = reflect.TypeOf(appfields.LayoutNode{})

func shape(raw json.RawMessage, typ reflect.Type, depth int) bool {
	if depth > 32 {
		return false
	}
	if typ == rawType {
		return true
	}
	if typ.Kind() == reflect.Pointer {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return true
		}
		return shape(raw, typ.Elem(), depth+1)
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return false
	}
	switch typ.Kind() {
	case reflect.Struct:
		var object map[string]json.RawMessage
		if json.Unmarshal(raw, &object) != nil || object == nil {
			return false
		}
		if typ == layoutType {
			return layoutShape(object, depth)
		}
		fields := map[string]reflect.StructField{}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			tag := strings.Split(f.Tag.Get("json"), ",")
			if tag[0] == "-" {
				continue
			}
			key := tag[0]
			if key == "" {
				return false
			}
			fields[key] = f
			_, present := object[key]
			optional := len(tag) > 1 && tag[1] == "omitempty"
			if !present && !optional {
				return false
			}
		}
		for key, value := range object {
			f, ok := fields[key]
			if !ok || !shape(value, f.Type, depth+1) {
				return false
			}
		}
	case reflect.Slice:
		var items []json.RawMessage
		if json.Unmarshal(raw, &items) != nil || items == nil {
			return false
		}
		for _, item := range items {
			if !shape(item, typ.Elem(), depth+1) {
				return false
			}
		}
	}
	return true
}
func layoutShape(object map[string]json.RawMessage, depth int) bool {
	var kind string
	if json.Unmarshal(object["kind"], &kind) != nil {
		return false
	}
	required := []string{"id", "kind"}
	optional := []string{}
	switch kind {
	case "field", "system_field":
		required = append(required, "fieldId")
		optional = []string{"span"}
	case "group":
		required = append(required, "title", "children")
		optional = []string{"span"}
	case "description":
		required = append(required, "text")
	case "divider":
	default:
		return false
	}
	allowed := map[string]bool{}
	for _, key := range required {
		if _, ok := object[key]; !ok {
			return false
		}
		allowed[key] = true
	}
	for _, key := range optional {
		allowed[key] = true
	}
	for key, v := range object {
		if !allowed[key] || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return false
		}
		if key == "children" && !shape(v, reflect.TypeOf([]appfields.LayoutNode{}), depth+1) {
			return false
		}
	}
	return true
}
func DecodeManifest(raw []byte) (Manifest, error) {
	if len(raw) == 0 || len(raw) > 1048576 || !utf8.Valid(raw) {
		return Manifest{}, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if tokens(d, 0) != nil {
		return Manifest{}, ErrInvalid
	}
	if _, e := d.Token(); e != io.EOF {
		return Manifest{}, ErrInvalid
	}
	if !shape(raw, reflect.TypeOf(Manifest{}), 0) {
		return Manifest{}, ErrInvalid
	}
	var m Manifest
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&m) != nil {
		return Manifest{}, ErrInvalid
	}
	return NormalizeManifest(m)
}
