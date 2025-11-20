package rag

import (
	"encoding/json"
	"fmt"
)

// encodeMetadata serializes a metadata map to JSON string
func encodeMetadata(meta map[string]interface{}) (string, error) {
	if meta == nil {
		return "{}", nil
	}

	bytes, err := json.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("failed to encode metadata: %w", err)
	}

	return string(bytes), nil
}

// decodeMetadata deserializes a JSON string to metadata map
func decodeMetadata(jsonStr string) (map[string]interface{}, error) {
	if jsonStr == "" {
		return make(map[string]interface{}), nil
	}

	var meta map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &meta); err != nil {
		return nil, fmt.Errorf("failed to decode metadata: %w", err)
	}

	return meta, nil
}

