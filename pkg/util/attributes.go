package util

import (
	"strconv"

	v1common "github.com/grafana/tempo/pkg/tempopb/common/v1"
)

func StringifyAnyValue(anyValue *v1common.AnyValue) string {
	if anyValue == nil {
		return ""
	}
	switch anyValue.Value.(type) {
	case *v1common.AnyValue_BoolValue:
		return strconv.FormatBool(anyValue.GetBoolValue())
	case *v1common.AnyValue_IntValue:
		return strconv.FormatInt(anyValue.GetIntValue(), 10)
	case *v1common.AnyValue_ArrayValue:
		arrStr := "["
		av := anyValue.GetArrayValue()
		for i := range av.Values {
			arrStr += StringifyAnyValue(&av.Values[i])
		}
		arrStr += "]"
		return arrStr
	case *v1common.AnyValue_DoubleValue:
		return strconv.FormatFloat(anyValue.GetDoubleValue(), 'f', -1, 64)
	case *v1common.AnyValue_KvlistValue:
		mapStr := "{"
		kvl := anyValue.GetKvlistValue()
		for i := range kvl.Values {
			mapStr += kvl.Values[i].Key + ":" + StringifyAnyValue(&kvl.Values[i].Value)
		}
		mapStr += "}"
		return mapStr
	case *v1common.AnyValue_StringValue:
		return anyValue.GetStringValue()
	}

	return ""
}
