package v1

import (
	"encoding/json"
	"fmt"
)

// UnmarshalJSON implements json.Unmarshaler for AnyValue's oneof field.
func (m *AnyValue) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if v, ok := raw["stringValue"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return err
		}
		m.Value = &AnyValue_StringValue{StringValue: s}
	} else if v, ok := raw["boolValue"]; ok {
		var b bool
		if err := json.Unmarshal(v, &b); err != nil {
			return err
		}
		m.Value = &AnyValue_BoolValue{BoolValue: b}
	} else if v, ok := raw["intValue"]; ok {
		// JSON may encode int64 as string
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			var i int64
			if _, err := json.Number(s).Int64(); err == nil {
				i, _ = json.Number(s).Int64()
				m.Value = &AnyValue_IntValue{IntValue: i}
				return nil
			}
		}
		var i int64
		if err := json.Unmarshal(v, &i); err != nil {
			return err
		}
		m.Value = &AnyValue_IntValue{IntValue: i}
	} else if v, ok := raw["doubleValue"]; ok {
		var d float64
		if err := json.Unmarshal(v, &d); err != nil {
			return err
		}
		m.Value = &AnyValue_DoubleValue{DoubleValue: d}
	} else if v, ok := raw["arrayValue"]; ok {
		var arr ArrayValue
		if err := json.Unmarshal(v, &arr); err != nil {
			return err
		}
		m.Value = &AnyValue_ArrayValue{ArrayValue: arr}
	} else if v, ok := raw["kvlistValue"]; ok {
		var kvl KeyValueList
		if err := json.Unmarshal(v, &kvl); err != nil {
			return err
		}
		m.Value = &AnyValue_KvlistValue{KvlistValue: kvl}
	} else if v, ok := raw["bytesValue"]; ok {
		var b []byte
		if err := json.Unmarshal(v, &b); err != nil {
			return err
		}
		m.Value = &AnyValue_BytesValue{BytesValue: b}
	}
	return nil
}

// MarshalJSON implements json.Marshaler for AnyValue's oneof field.
func (m AnyValue) MarshalJSON() ([]byte, error) {
	switch v := m.Value.(type) {
	case *AnyValue_StringValue:
		return json.Marshal(map[string]any{"stringValue": v.StringValue})
	case *AnyValue_BoolValue:
		return json.Marshal(map[string]any{"boolValue": v.BoolValue})
	case *AnyValue_IntValue:
		// Proto JSON spec: int64 values are encoded as strings
		return json.Marshal(map[string]any{"intValue": fmt.Sprintf("%d", v.IntValue)})
	case *AnyValue_DoubleValue:
		return json.Marshal(map[string]any{"doubleValue": v.DoubleValue})
	case *AnyValue_ArrayValue:
		return json.Marshal(map[string]any{"arrayValue": v.ArrayValue})
	case *AnyValue_KvlistValue:
		return json.Marshal(map[string]any{"kvlistValue": v.KvlistValue})
	case *AnyValue_BytesValue:
		return json.Marshal(map[string]any{"bytesValue": v.BytesValue})
	default:
		return []byte("{}"), nil
	}
}
