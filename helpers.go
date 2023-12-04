package httplog

import (
	"encoding/json"
	"log/slog"
	"reflect"
)

// StructValue will convert a struct or slice of structs to a slog.Value
func StructValue(v interface{}) slog.Value {
	var out interface{}

	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice {
		// assume slice of objects
		out = []map[string]interface{}{}
	} else {
		// assume single object
		out = map[string]interface{}{}
	}

	b, _ := json.Marshal(v)
	json.Unmarshal(b, &out)

	return slog.AnyValue(out)
}
