package config

import "encoding/json"

// reencode rewrites src (arbitrary map/slice tree from interface{}
// deserialization) into dst via a JSON round-trip. Silently ignores
// errors — callers treat a failed reencode the same as missing data.
func reencode(src, dst interface{}) {
	b, err := json.Marshal(src)
	if err != nil {
		return
	}
	_ = json.Unmarshal(b, dst)
}

// parseJSONString unmarshals the raw JSON string into dst in place.
// Silently ignores parse errors.
func parseJSONString(s string, dst interface{}) {
	_ = json.Unmarshal([]byte(s), dst)
}
