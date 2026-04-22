package tempopb

import (
	"encoding/json"
	"strings"

	"github.com/gogo/protobuf/jsonpb"
)

// MarshalToJSONV1 marshals a Trace to OTEL-compatible JSON.
// Replaces "resourceSpans" with "batches" for backward compatibility.
func MarshalToJSONV1(t *Trace) ([]byte, error) {
	marshaler := &jsonpb.Marshaler{}
	jsonStr, err := marshaler.MarshalToString(t)
	if err != nil {
		return nil, err
	}
	jsonStr = strings.Replace(jsonStr, `"resourceSpans":`, `"batches":`, 1)
	return []byte(jsonStr), nil
}

// UnmarshalFromJSONV1 unmarshals OTEL-compatible JSON into a Trace.
// Replaces "batches" with "resourceSpans" for backward compatibility.
func UnmarshalFromJSONV1(data []byte, t *Trace) error {
	jsonStr := strings.Replace(string(data), `"batches":`, `"resourceSpans":`, 1)
	return json.Unmarshal([]byte(jsonStr), t)
}
