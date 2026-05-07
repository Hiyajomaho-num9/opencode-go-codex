package jsonutil

import "encoding/json"

func Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func MustMarshalString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}
