package vparquet6

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/golang/protobuf/jsonpb" //nolint:all //deprecated
	"github.com/parquet-go/parquet-go"

	"github.com/grafana/tempo/pkg/tempopb"
	v1 "github.com/grafana/tempo/pkg/tempopb/common/v1"
	v1_resource "github.com/grafana/tempo/pkg/tempopb/resource/v1"
	v1_trace "github.com/grafana/tempo/pkg/tempopb/trace/v1"
	"github.com/grafana/tempo/pkg/util"
	"github.com/grafana/tempo/tempodb/backend"
	"github.com/grafana/tempo/tempodb/encoding/common"
)

// Label names for conversion b/n Proto <> Parquet
const (
	LabelRootSpanName    = "root.name"
	LabelRootServiceName = "root.service.name"

	LabelServiceName = "service.name"
	LabelCluster     = "cluster"
	LabelNamespace   = "namespace"
	LabelPod         = "pod"
	LabelContainer   = "container"

	LabelK8sClusterName   = "k8s.cluster.name"
	LabelK8sNamespaceName = "k8s.namespace.name"
	LabelK8sPodName       = "k8s.pod.name"
	LabelK8sContainerName = "k8s.container.name"

	LabelName                   = "name"
	LabelHTTPMethod             = "http.method"
	LabelHTTPUrl                = "http.url"
	LabelHTTPStatusCode         = "http.status_code"
	LabelStatusCode             = "status.code"
	LabelStatus                 = "status"
	LabelKind                   = "kind"
	LabelTraceQLRootServiceName = "rootServiceName"
	LabelTraceQLRootName        = "rootName"
	LabelTraceID                = "trace:id"
	LabelSpanID                 = "span:id"
)

// Definition levels for the flat schema (one row per span).
// Events and Links are the only nested lists remaining.
const (
	DefinitionLevelSpan       = 0
	DefinitionLevelSpanAttrs  = 1
	DefinitionLevelEvent      = 1
	DefinitionLevelLink       = 1
	DefinitionLevelEventAttrs = 2
	DefinitionLevelLinkAttrs  = 2

	// Field paths for resource attributes (flat)
	FieldResourceAttrKey       = "ResourceAttrs.Key"
	FieldResourceAttrIsArray   = "ResourceAttrs.IsArray"
	FieldResourceAttrVal       = "ResourceAttrs.Value"
	FieldResourceAttrValInt    = "ResourceAttrs.ValueInt"
	FieldResourceAttrValDouble = "ResourceAttrs.ValueDouble"
	FieldResourceAttrValBool   = "ResourceAttrs.ValueBool"

	// Field paths for span attributes (flat)
	FieldSpanAttrKey       = "Attrs.Key"
	FieldSpanAttrIsArray   = "Attrs.IsArray"
	FieldSpanAttrVal       = "Attrs.Value"
	FieldSpanAttrValInt    = "Attrs.ValueInt"
	FieldSpanAttrValDouble = "Attrs.ValueDouble"
	FieldSpanAttrValBool   = "Attrs.ValueBool"

	// Field paths for event attributes (flat)
	FieldEventAttrKey        = "Events.Attrs.Key"
	FieldEventAttrIsArray    = "Events.Attrs.IsArray"
	FieldEventAttrVal        = "Events.Attrs.Value"
	FieldEventAttrValInt     = "Events.Attrs.ValueInt"
	FieldEventAttrValDouble  = "Events.Attrs.ValueDouble"
	FieldEventAttrValBool    = "Events.Attrs.ValueBool"
	FieldEventDedicatedAttrs = "Events.DedicatedAttributes"
)

const (
	// These are used to round span start times into smaller integers that are highly compressible.
	roundingStart = uint64(0)
	roundingEnd   = uint64(0xF000000000000000)
)

var (
	jsonMarshaler = new(jsonpb.Marshaler)

	// todo: remove this when support for tag based search is removed
	labelMappings = map[string]string{
		LabelRootSpanName:    "RootSpanName",
		LabelRootServiceName: "RootServiceName",
		LabelServiceName:     "ResourceServiceName",
		LabelName:            "Name",
		LabelStatusCode:      "StatusCode",
	}
	traceqlResourceLabelMappings = map[string]string{
		LabelServiceName: "ResourceServiceName",
	}
)

// Attribute stores a key-value pair. Struct tags have NO compression -- compression is applied via writer options.
type Attribute struct {
	Key string `parquet:",dict"`

	IsArray          bool      `parquet:""`
	Value            []string  `parquet:",dict,"`
	ValueInt         []int64   `parquet:","`
	ValueDouble      []float64 `parquet:","`
	ValueBool        []bool    `parquet:","`
	ValueUnsupported *string   `parquet:",optional"`
}

// DedicatedAttributes add spare columns to the schema that can be assigned to attributes at runtime.
// No compression in struct tags -- compression is applied via writer options.
type DedicatedAttributes struct {
	String01 []string `parquet:",optional,dict"`
	String02 []string `parquet:",optional,dict"`
	String03 []string `parquet:",optional,dict"`
	String04 []string `parquet:",optional,dict"`
	String05 []string `parquet:",optional,dict"`
	String06 []string `parquet:",optional,dict"`
	String07 []string `parquet:",optional,dict"`
	String08 []string `parquet:",optional,dict"`
	String09 []string `parquet:",optional,dict"`
	String10 []string `parquet:",optional,dict"`
	String11 []string `parquet:",optional,dict"`
	String12 []string `parquet:",optional,dict"`
	String13 []string `parquet:",optional,dict"`
	String14 []string `parquet:",optional,dict"`
	String15 []string `parquet:",optional,dict"`
	String16 []string `parquet:",optional,dict"`
	String17 []string `parquet:",optional,dict"`
	String18 []string `parquet:",optional,dict"`
	String19 []string `parquet:",optional,dict"`
	String20 []string `parquet:",optional,dict"`
	Int01    []int64  `parquet:",optional"`
	Int02    []int64  `parquet:",optional"`
	Int03    []int64  `parquet:",optional"`
	Int04    []int64  `parquet:",optional"`
	Int05    []int64  `parquet:",optional"`
}

func (da *DedicatedAttributes) Reset() {
	da.String01 = da.String01[:0]
	da.String02 = da.String02[:0]
	da.String03 = da.String03[:0]
	da.String04 = da.String04[:0]
	da.String05 = da.String05[:0]
	da.String06 = da.String06[:0]
	da.String07 = da.String07[:0]
	da.String08 = da.String08[:0]
	da.String09 = da.String09[:0]
	da.String10 = da.String10[:0]
	da.String11 = da.String11[:0]
	da.String12 = da.String12[:0]
	da.String13 = da.String13[:0]
	da.String14 = da.String14[:0]
	da.String15 = da.String15[:0]
	da.String16 = da.String16[:0]
	da.String17 = da.String17[:0]
	da.String18 = da.String18[:0]
	da.String19 = da.String19[:0]
	da.String20 = da.String20[:0]
	da.Int01 = da.Int01[:0]
	da.Int02 = da.Int02[:0]
	da.Int03 = da.Int03[:0]
	da.Int04 = da.Int04[:0]
	da.Int05 = da.Int05[:0]
}

// Event represents a span event. No compression in struct tags.
type Event struct {
	TimeSinceStartNano     uint64      `parquet:""`
	Name                   string      `parquet:",dict"`
	Attrs                  []Attribute `parquet:","`
	DroppedAttributesCount int32       `parquet:""`

	DedicatedAttributes DedicatedAttributes `parquet:""`
}

// ServiceStats stores per-service span count and error count for a trace (denormalized to each span).
type ServiceStats struct {
	ServiceName string `parquet:",dict"`
	SpanCount   uint32 `parquet:",delta"`
	ErrorCount  uint32 `parquet:",delta"`
}

// Link represents a span link. No compression in struct tags.
type Link struct {
	TraceID                []byte      `parquet:","`
	SpanID                 []byte      `parquet:","`
	TraceState             string      `parquet:",optional"`
	Attrs                  []Attribute `parquet:","`
	DroppedAttributesCount int32       `parquet:""`
}

// FlatSpan is the root Parquet row type for vParquet6. One row per span.
// All trace-level, resource-level, and scope-level data is denormalized.
// Compression is specified via writer options, NOT struct tags.
//
//nolint:revive
type FlatSpan struct {
	// Trace-level (denormalized, same for all spans in a trace)
	TraceID                []byte         `parquet:",plain"`
	TraceIDText            string         `parquet:",optional"`
	TraceStartTimeUnixNano uint64         `parquet:""`
	TraceEndTimeUnixNano   uint64         `parquet:""`
	TraceDurationNano      uint64         `parquet:""`
	RootServiceName        string         `parquet:",optional,dict"`
	RootSpanName           string         `parquet:",optional,dict"`
	ServiceStats           []ServiceStats `parquet:""`

	// Resource-level (denormalized from parent resource)
	ResourceServiceName            string      `parquet:",optional,dict"`
	ResourceAttrs                  []Attribute `parquet:","`
	ResourceDroppedAttributesCount int32       `parquet:""`

	ResourceDedicatedString01 []string `parquet:",optional,dict"`
	ResourceDedicatedString02 []string `parquet:",optional,dict"`
	ResourceDedicatedString03 []string `parquet:",optional,dict"`
	ResourceDedicatedString04 []string `parquet:",optional,dict"`
	ResourceDedicatedString05 []string `parquet:",optional,dict"`
	ResourceDedicatedString06 []string `parquet:",optional,dict"`
	ResourceDedicatedString07 []string `parquet:",optional,dict"`
	ResourceDedicatedString08 []string `parquet:",optional,dict"`
	ResourceDedicatedString09 []string `parquet:",optional,dict"`
	ResourceDedicatedString10 []string `parquet:",optional,dict"`
	ResourceDedicatedString11 []string `parquet:",optional,dict"`
	ResourceDedicatedString12 []string `parquet:",optional,dict"`
	ResourceDedicatedString13 []string `parquet:",optional,dict"`
	ResourceDedicatedString14 []string `parquet:",optional,dict"`
	ResourceDedicatedString15 []string `parquet:",optional,dict"`
	ResourceDedicatedString16 []string `parquet:",optional,dict"`
	ResourceDedicatedString17 []string `parquet:",optional,dict"`
	ResourceDedicatedString18 []string `parquet:",optional,dict"`
	ResourceDedicatedString19 []string `parquet:",optional,dict"`
	ResourceDedicatedString20 []string `parquet:",optional,dict"`
	ResourceDedicatedInt01    []int64  `parquet:",optional"`
	ResourceDedicatedInt02    []int64  `parquet:",optional"`
	ResourceDedicatedInt03    []int64  `parquet:",optional"`
	ResourceDedicatedInt04    []int64  `parquet:",optional"`
	ResourceDedicatedInt05    []int64  `parquet:",optional"`

	// Scope-level (denormalized from parent scope)
	ScopeName                   string      `parquet:",optional,dict"`
	ScopeVersion                string      `parquet:",optional,dict"`
	ScopeAttrs                  []Attribute `parquet:","`
	ScopeDroppedAttributesCount int32       `parquet:""`

	// Span-level
	SpanID                 []byte      `parquet:",plain"`
	ParentSpanID           []byte      `parquet:",plain"`
	ParentID               int32       `parquet:""`
	NestedSetLeft          int32       `parquet:""`
	NestedSetRight         int32       `parquet:""`
	ChildCount             int32       `parquet:""`
	Name                   string      `parquet:",optional,dict"`
	Kind                   int         `parquet:""`
	TraceState             string      `parquet:",optional"`
	StartTimeUnixNano      uint64      `parquet:""`
	DurationNano           uint64      `parquet:""`
	StatusCode             int         `parquet:""`
	StatusMessage          string      `parquet:",optional"`
	Attrs                  []Attribute `parquet:","`
	DroppedAttributesCount int32       `parquet:""`
	Events                 []Event     `parquet:","`
	DroppedEventsCount     int32       `parquet:""`
	Links                  []Link      `parquet:","`
	DroppedLinksCount      int32       `parquet:""`

	// Dynamically assignable dedicated attribute columns (flat)
	DedicatedString01 []string `parquet:",optional,dict"`
	DedicatedString02 []string `parquet:",optional,dict"`
	DedicatedString03 []string `parquet:",optional,dict"`
	DedicatedString04 []string `parquet:",optional,dict"`
	DedicatedString05 []string `parquet:",optional,dict"`
	DedicatedString06 []string `parquet:",optional,dict"`
	DedicatedString07 []string `parquet:",optional,dict"`
	DedicatedString08 []string `parquet:",optional,dict"`
	DedicatedString09 []string `parquet:",optional,dict"`
	DedicatedString10 []string `parquet:",optional,dict"`
	DedicatedString11 []string `parquet:",optional,dict"`
	DedicatedString12 []string `parquet:",optional,dict"`
	DedicatedString13 []string `parquet:",optional,dict"`
	DedicatedString14 []string `parquet:",optional,dict"`
	DedicatedString15 []string `parquet:",optional,dict"`
	DedicatedString16 []string `parquet:",optional,dict"`
	DedicatedString17 []string `parquet:",optional,dict"`
	DedicatedString18 []string `parquet:",optional,dict"`
	DedicatedString19 []string `parquet:",optional,dict"`
	DedicatedString20 []string `parquet:",optional,dict"`
	DedicatedInt01    []int64  `parquet:",optional"`
	DedicatedInt02    []int64  `parquet:",optional"`
	DedicatedInt03    []int64  `parquet:",optional"`
	DedicatedInt04    []int64  `parquet:",optional"`
	DedicatedInt05    []int64  `parquet:",optional"`

	// Precomputed/Optimized values for metrics
	StartTimeRounded15   uint32 `parquet:""`
	StartTimeRounded60   uint32 `parquet:""`
	StartTimeRounded300  uint32 `parquet:""`
	StartTimeRounded3600 uint32 `parquet:""`
}

func (s *FlatSpan) IsRoot() bool {
	return len(s.ParentSpanID) == 0
}

func attrToParquet(a *v1.KeyValue, p *Attribute) {
	p.Key = a.Key
	p.IsArray = false
	p.Value = p.Value[:0]
	p.ValueInt = p.ValueInt[:0]
	p.ValueDouble = p.ValueDouble[:0]
	p.ValueBool = p.ValueBool[:0]
	p.ValueUnsupported = nil

	switch v := a.GetValue().Value.(type) {
	case *v1.AnyValue_StringValue:
		p.Value = append(p.Value, v.StringValue)
	case *v1.AnyValue_IntValue:
		p.ValueInt = append(p.ValueInt, v.IntValue)
	case *v1.AnyValue_DoubleValue:
		p.ValueDouble = append(p.ValueDouble, v.DoubleValue)
	case *v1.AnyValue_BoolValue:
		p.ValueBool = append(p.ValueBool, v.BoolValue)
	case *v1.AnyValue_ArrayValue:
		p.IsArray = true
		if v.ArrayValue == nil || len(v.ArrayValue.Values) == 0 {
			return
		}
		switch v.ArrayValue.Values[0].Value.(type) {
		case *v1.AnyValue_StringValue:
			for _, e := range v.ArrayValue.Values {
				ev, ok := e.Value.(*v1.AnyValue_StringValue)
				if !ok {
					p.Value = p.Value[:0]
					attrToParquetTypeUnsupported(a, p)
					return
				}

				p.Value = append(p.Value, ev.StringValue)
			}
		case *v1.AnyValue_IntValue:
			for _, e := range v.ArrayValue.Values {
				ev, ok := e.Value.(*v1.AnyValue_IntValue)
				if !ok {
					p.ValueInt = p.ValueInt[:0]
					attrToParquetTypeUnsupported(a, p)
					return
				}

				p.ValueInt = append(p.ValueInt, ev.IntValue)
			}
		case *v1.AnyValue_DoubleValue:
			for _, e := range v.ArrayValue.Values {
				ev, ok := e.Value.(*v1.AnyValue_DoubleValue)
				if !ok {
					p.ValueDouble = p.ValueDouble[:0]
					attrToParquetTypeUnsupported(a, p)
					return
				}

				p.ValueDouble = append(p.ValueDouble, ev.DoubleValue)
			}
		case *v1.AnyValue_BoolValue:
			for _, e := range v.ArrayValue.Values {
				ev, ok := e.Value.(*v1.AnyValue_BoolValue)
				if !ok {
					p.ValueBool = p.ValueBool[:0]
					attrToParquetTypeUnsupported(a, p)
					return
				}

				p.ValueBool = append(p.ValueBool, ev.BoolValue)
			}
		default:
			attrToParquetTypeUnsupported(a, p)
		}
	default:
		attrToParquetTypeUnsupported(a, p)
	}
}

func attrToParquetTypeUnsupported(a *v1.KeyValue, p *Attribute) {
	jsonBytes := &bytes.Buffer{}
	_ = jsonMarshaler.Marshal(jsonBytes, a.Value) // deliberately marshalling a.Value because of AnyValue logic
	jsonStr := jsonBytes.String()
	p.ValueUnsupported = &jsonStr
	p.IsArray = false
}

// traceToParquet converts a tempopb.Trace to flat spans using block meta for dedicated column assignments
func traceToParquet(meta *backend.BlockMeta, id common.ID, tr *tempopb.Trace, buffer []FlatSpan) ([]FlatSpan, bool) {
	dedicatedResourceAttributes := dedicatedColumnsToColumnMapping(meta.DedicatedColumns, backend.DedicatedColumnScopeResource)
	dedicatedSpanAttributes := dedicatedColumnsToColumnMapping(meta.DedicatedColumns, backend.DedicatedColumnScopeSpan)
	dedicatedEventAttributes := dedicatedColumnsToColumnMapping(meta.DedicatedColumns, backend.DedicatedColumnScopeEvent)

	return traceToFlatSpans(id, tr, buffer, dedicatedResourceAttributes, dedicatedSpanAttributes, dedicatedEventAttributes)
}

// traceToFlatSpans converts a tempopb.Trace into a slice of FlatSpan (one per span),
// denormalizing trace/resource/scope data into each row.
func traceToFlatSpans(id common.ID, tr *tempopb.Trace, buffer []FlatSpan, dedicatedResourceAttributes, dedicatedSpanAttributes, dedicatedEventAttributes dedicatedColumnMapping) ([]FlatSpan, bool) {
	// First pass: count total spans and find root span/batch, compute trace timing
	totalSpans := 0
	traceStart := uint64(0)
	traceEnd := uint64(0)
	var rootSpan *v1_trace.Span
	var rootBatch *v1_trace.ResourceSpans

	for _, rs := range tr.ResourceSpans {
		for _, ss := range rs.ScopeSpans {
			totalSpans += len(ss.Spans)
			for _, s := range ss.Spans {
				if traceStart == 0 || s.StartTimeUnixNano < traceStart {
					traceStart = s.StartTimeUnixNano
				}
				if s.EndTimeUnixNano > traceEnd {
					traceEnd = s.EndTimeUnixNano
				}

				var hasChildOfLink bool
				for _, spanLink := range s.Links {
					if bytes.Equal(s.TraceId, spanLink.TraceId) {
						for _, attr := range spanLink.GetAttributes() {
							if attr.Key == "opentracing.ref_type" && attr.GetValue().GetStringValue() == "child_of" {
								hasChildOfLink = true
								break
							}
						}
						if hasChildOfLink {
							break
						}
					}
				}

				if len(s.ParentSpanId) == 0 && !hasChildOfLink {
					rootSpan = s
					rootBatch = rs
				}
			}
		}
	}

	rootSpanName := ""
	rootServiceName := ""
	if rootSpan != nil && rootBatch != nil && rootBatch.Resource != nil {
		rootSpanName = rootSpan.Name
		for _, a := range rootBatch.Resource.Attributes {
			if a.Key == LabelServiceName {
				rootServiceName = a.Value.GetStringValue()
				break
			}
		}
	}

	traceID := util.PadTraceIDTo16Bytes(id)
	traceIDText := util.TraceIDToHexString(id)
	traceDuration := traceEnd - traceStart

	// Reuse buffer
	buffer = extendReuseSlice(totalSpans, buffer)

	spanIdx := 0
	for _, rs := range tr.ResourceSpans {
		// Extract resource-level fields
		var resourceServiceName string
		var resourceDroppedAttrsCount int32
		var resourceDedicatedTemplate FlatSpan
		resetResourceDedicated(&resourceDedicatedTemplate)

		resourceAttrBuf := make([]Attribute, 0, len(rs.GetResource().GetAttributes()))
		if rs.Resource != nil {
			resourceDroppedAttrsCount = int32(rs.Resource.DroppedAttributesCount)
			attrCount := 0
			resourceAttrBuf = extendReuseSlice(len(rs.Resource.Attributes), resourceAttrBuf)
			for _, a := range rs.Resource.Attributes {
				var written bool
				if strVal, ok := a.Value.Value.(*v1.AnyValue_StringValue); ok && a.Key == LabelServiceName {
					resourceServiceName = strVal.StringValue
					written = true
				}
				if !written {
					if spareColumn, exists := dedicatedResourceAttributes.get(a.Key); exists {
						written = spareColumn.writeResourceValue(&resourceDedicatedTemplate, a.Value)
					}
				}
				if !written {
					attrToParquet(a, &resourceAttrBuf[attrCount])
					attrCount++
				}
			}
			resourceAttrBuf = resourceAttrBuf[:attrCount]
		}

		for _, ss := range rs.ScopeSpans {
			// Extract scope-level fields
			var scopeName, scopeVersion string
			var scopeDroppedAttrsCount int32
			var scopeAttrs []Attribute

			if ss.Scope != nil {
				scopeName = ss.Scope.Name
				scopeVersion = ss.Scope.Version
				scopeDroppedAttrsCount = int32(ss.Scope.DroppedAttributesCount)
				scopeAttrs = make([]Attribute, len(ss.Scope.Attributes))
				for i, a := range ss.Scope.Attributes {
					attrToParquet(a, &scopeAttrs[i])
				}
			}

			for _, s := range ss.Spans {
				fs := &buffer[spanIdx]
				spanIdx++

				// Trace-level
				fs.TraceID = traceID
				fs.TraceIDText = traceIDText
				fs.TraceStartTimeUnixNano = traceStart
				fs.TraceEndTimeUnixNano = traceEnd
				fs.TraceDurationNano = traceDuration
				fs.RootServiceName = rootServiceName
				fs.RootSpanName = rootSpanName

				// Resource-level
				fs.ResourceServiceName = resourceServiceName
				fs.ResourceAttrs = copyAttrs(resourceAttrBuf)
				fs.ResourceDroppedAttributesCount = resourceDroppedAttrsCount
				copyResourceDedicated(fs, &resourceDedicatedTemplate)

				// Scope-level
				fs.ScopeName = scopeName
				fs.ScopeVersion = scopeVersion
				fs.ScopeAttrs = copyAttrs(scopeAttrs)
				fs.ScopeDroppedAttributesCount = scopeDroppedAttrsCount

				// Span-level
				fs.SpanID = s.SpanId
				fs.ParentSpanID = s.ParentSpanId
				fs.Name = s.Name
				fs.Kind = int(s.Kind)
				fs.TraceState = s.TraceState
				if s.Status != nil {
					fs.StatusCode = int(s.Status.Code)
					fs.StatusMessage = s.Status.Message
				} else {
					fs.StatusCode = 0
					fs.StatusMessage = ""
				}
				fs.StartTimeUnixNano = s.StartTimeUnixNano
				if s.StartTimeUnixNano == 0 {
					fs.StartTimeRounded15 = 0
					fs.StartTimeRounded60 = 0
					fs.StartTimeRounded300 = 0
					fs.StartTimeRounded3600 = 0
				} else {
					fs.StartTimeRounded15 = uint32(intervalMapper15Seconds.Interval(s.StartTimeUnixNano))
					fs.StartTimeRounded60 = uint32(intervalMapper60Seconds.Interval(s.StartTimeUnixNano))
					fs.StartTimeRounded300 = uint32(intervalMapper300Seconds.Interval(s.StartTimeUnixNano))
					fs.StartTimeRounded3600 = uint32(intervalMapper3600Seconds.Interval(s.StartTimeUnixNano))
				}
				fs.DurationNano = s.EndTimeUnixNano - s.StartTimeUnixNano
				fs.DroppedAttributesCount = int32(s.DroppedAttributesCount)
				fs.DroppedEventsCount = int32(s.DroppedEventsCount)
				fs.DroppedLinksCount = int32(s.DroppedLinksCount)

				// Nested set values and service stats (set by assignNestedSetModelBounds)
				fs.NestedSetLeft = 0
				fs.NestedSetRight = 0
				fs.ParentID = 0
				fs.ChildCount = 0
				fs.ServiceStats = nil

				// Events
				fs.Events = extendReuseSlice(len(s.Events), fs.Events)
				for ie, e := range s.Events {
					eventToParquet(e, &fs.Events[ie], s.StartTimeUnixNano, dedicatedEventAttributes)
				}

				// Links
				fs.Links = extendReuseSlice(len(s.Links), fs.Links)
				for il, l := range s.Links {
					linkToParquet(l, &fs.Links[il])
				}

				// Span attributes
				resetSpanDedicated(fs)
				writeFlatSpanAttrs(s.Attributes, &fs.Attrs, fs, dedicatedSpanAttributes)
			}
		}
	}

	// Compute nested set model bounds directly on the flat spans
	result := buffer[:spanIdx]
	connected := assignNestedSetModelBounds(result)

	return result, connected
}

func copyAttrs(src []Attribute) []Attribute {
	if len(src) == 0 {
		return nil
	}
	dst := make([]Attribute, len(src))
	copy(dst, src)
	return dst
}

func writeAttrs(input []*v1.KeyValue, generic *[]Attribute, dedicated *DedicatedAttributes, mapping dedicatedColumnMapping) {
	*generic = extendReuseSlice(len(input), *generic)

	attrCount := 0
	for _, a := range input {
		written := false

		if spareColumn, exists := mapping.get(a.Key); exists {
			written = spareColumn.writeValue(dedicated, a.Value)
		}

		if !written {
			attrToParquet(a, &(*generic)[attrCount])
			attrCount++
		}
	}
	*generic = (*generic)[:attrCount]
}

func writeFlatSpanAttrs(input []*v1.KeyValue, generic *[]Attribute, fs *FlatSpan, mapping dedicatedColumnMapping) {
	*generic = extendReuseSlice(len(input), *generic)

	attrCount := 0
	for _, a := range input {
		written := false

		if spareColumn, exists := mapping.get(a.Key); exists {
			written = spareColumn.writeSpanValue(fs, a.Value)
		}

		if !written {
			attrToParquet(a, &(*generic)[attrCount])
			attrCount++
		}
	}
	*generic = (*generic)[:attrCount]
}

func copyResourceDedicated(dst, src *FlatSpan) {
	dst.ResourceDedicatedString01 = src.ResourceDedicatedString01
	dst.ResourceDedicatedString02 = src.ResourceDedicatedString02
	dst.ResourceDedicatedString03 = src.ResourceDedicatedString03
	dst.ResourceDedicatedString04 = src.ResourceDedicatedString04
	dst.ResourceDedicatedString05 = src.ResourceDedicatedString05
	dst.ResourceDedicatedString06 = src.ResourceDedicatedString06
	dst.ResourceDedicatedString07 = src.ResourceDedicatedString07
	dst.ResourceDedicatedString08 = src.ResourceDedicatedString08
	dst.ResourceDedicatedString09 = src.ResourceDedicatedString09
	dst.ResourceDedicatedString10 = src.ResourceDedicatedString10
	dst.ResourceDedicatedString11 = src.ResourceDedicatedString11
	dst.ResourceDedicatedString12 = src.ResourceDedicatedString12
	dst.ResourceDedicatedString13 = src.ResourceDedicatedString13
	dst.ResourceDedicatedString14 = src.ResourceDedicatedString14
	dst.ResourceDedicatedString15 = src.ResourceDedicatedString15
	dst.ResourceDedicatedString16 = src.ResourceDedicatedString16
	dst.ResourceDedicatedString17 = src.ResourceDedicatedString17
	dst.ResourceDedicatedString18 = src.ResourceDedicatedString18
	dst.ResourceDedicatedString19 = src.ResourceDedicatedString19
	dst.ResourceDedicatedString20 = src.ResourceDedicatedString20
	dst.ResourceDedicatedInt01 = src.ResourceDedicatedInt01
	dst.ResourceDedicatedInt02 = src.ResourceDedicatedInt02
	dst.ResourceDedicatedInt03 = src.ResourceDedicatedInt03
	dst.ResourceDedicatedInt04 = src.ResourceDedicatedInt04
	dst.ResourceDedicatedInt05 = src.ResourceDedicatedInt05
}

func eventToParquet(e *v1_trace.Span_Event, ee *Event, spanStartTime uint64, dedicatedEventAttributes dedicatedColumnMapping) {
	ee.Name = e.Name
	ee.TimeSinceStartNano = e.TimeUnixNano - spanStartTime
	ee.DroppedAttributesCount = int32(e.DroppedAttributesCount)
	writeAttrs(e.Attributes, &ee.Attrs, &ee.DedicatedAttributes, dedicatedEventAttributes)
}

func linkToParquet(l *v1_trace.Span_Link, ll *Link) {
	ll.TraceID = l.TraceId
	ll.SpanID = l.SpanId
	ll.TraceState = l.TraceState
	ll.DroppedAttributesCount = int32(l.DroppedAttributesCount)

	ll.Attrs = extendReuseSlice(len(l.Attributes), ll.Attrs)
	for i, a := range l.Attributes {
		attrToParquet(a, &ll.Attrs[i])
	}
}

func parquetToProtoAttrs(parquetAttrs []Attribute) []*v1.KeyValue {
	var protoAttrs []*v1.KeyValue

	for _, attr := range parquetAttrs {
		var protoVal v1.AnyValue

		if !attr.IsArray {
			switch {
			case len(attr.Value) > 0:
				protoVal.Value = &v1.AnyValue_StringValue{StringValue: attr.Value[0]}
			case len(attr.ValueInt) > 0:
				protoVal.Value = &v1.AnyValue_IntValue{IntValue: attr.ValueInt[0]}
			case len(attr.ValueDouble) > 0:
				protoVal.Value = &v1.AnyValue_DoubleValue{DoubleValue: attr.ValueDouble[0]}
			case len(attr.ValueBool) > 0:
				protoVal.Value = &v1.AnyValue_BoolValue{BoolValue: attr.ValueBool[0]}
			case attr.ValueUnsupported != nil:
				_ = jsonpb.Unmarshal(bytes.NewBufferString(*attr.ValueUnsupported), &protoVal)
			default:
				continue
			}
		} else {
			switch {
			case len(attr.Value) > 0:
				values := make([]*v1.AnyValue, len(attr.Value))
				anyValues := make([]v1.AnyValue, len(values))
				strValues := make([]v1.AnyValue_StringValue, len(values))
				for i, v := range attr.Value {
					s := &strValues[i]
					s.StringValue = v
					values[i] = &anyValues[i]
					values[i].Value = s
				}
				protoVal.Value = &v1.AnyValue_ArrayValue{ArrayValue: &v1.ArrayValue{Values: values}}
			case len(attr.ValueInt) > 0:
				values := make([]*v1.AnyValue, len(attr.ValueInt))
				anyValues := make([]v1.AnyValue, len(values))
				intValues := make([]v1.AnyValue_IntValue, len(values))
				for i, v := range attr.ValueInt {
					n := &intValues[i]
					n.IntValue = v
					values[i] = &anyValues[i]
					values[i].Value = n
				}
				protoVal.Value = &v1.AnyValue_ArrayValue{ArrayValue: &v1.ArrayValue{Values: values}}
			case len(attr.ValueDouble) > 0:
				values := make([]*v1.AnyValue, len(attr.ValueDouble))
				anyValues := make([]v1.AnyValue, len(values))
				doubleValues := make([]v1.AnyValue_DoubleValue, len(values))
				for i, v := range attr.ValueDouble {
					n := &doubleValues[i]
					n.DoubleValue = v
					values[i] = &anyValues[i]
					values[i].Value = n
				}
				protoVal.Value = &v1.AnyValue_ArrayValue{ArrayValue: &v1.ArrayValue{Values: values}}
			case len(attr.ValueBool) > 0:
				values := make([]*v1.AnyValue, len(attr.ValueBool))
				anyValues := make([]v1.AnyValue, len(values))
				boolValues := make([]v1.AnyValue_BoolValue, len(values))
				for i, v := range attr.ValueBool {
					n := &boolValues[i]
					n.BoolValue = v
					values[i] = &anyValues[i]
					values[i].Value = n
				}
				protoVal.Value = &v1.AnyValue_ArrayValue{ArrayValue: &v1.ArrayValue{Values: values}}
			default:
				protoVal.Value = &v1.AnyValue_ArrayValue{ArrayValue: &v1.ArrayValue{Values: []*v1.AnyValue{}}}
			}
		}

		protoAttrs = append(protoAttrs, &v1.KeyValue{
			Key:   attr.Key,
			Value: &protoVal,
		})
	}

	return protoAttrs
}

func parquetToProtoLinks(parquetLinks []Link) []*v1_trace.Span_Link {
	var protoLinks []*v1_trace.Span_Link

	if len(parquetLinks) > 0 {
		protoLinks = make([]*v1_trace.Span_Link, 0, len(parquetLinks))
		for _, l := range parquetLinks {
			protoLink := &v1_trace.Span_Link{
				TraceId:                l.TraceID,
				SpanId:                 l.SpanID,
				TraceState:             l.TraceState,
				DroppedAttributesCount: uint32(l.DroppedAttributesCount),
				Attributes:             nil,
			}

			if len(l.Attrs) > 0 {
				protoLink.Attributes = parquetToProtoAttrs(l.Attrs)
			}

			protoLinks = append(protoLinks, protoLink)
		}
	}

	return protoLinks
}

func parquetToProtoEvents(parquetEvents []Event, spanStartTimeNano uint64, dedicatedAttributes dedicatedColumnMapping) []*v1_trace.Span_Event {
	var protoEvents []*v1_trace.Span_Event

	if len(parquetEvents) > 0 {
		protoEvents = make([]*v1_trace.Span_Event, 0, len(parquetEvents))

		for _, e := range parquetEvents {
			protoEvent := &v1_trace.Span_Event{
				TimeUnixNano:           e.TimeSinceStartNano + spanStartTimeNano,
				Name:                   e.Name,
				Attributes:             nil,
				DroppedAttributesCount: uint32(e.DroppedAttributesCount),
			}

			if len(e.Attrs) > 0 {
				protoEvent.Attributes = parquetToProtoAttrs(e.Attrs)
			}

			for attr, col := range dedicatedAttributes.items() {
				val := col.readValue(&e.DedicatedAttributes)
				if val != nil {
					protoEvent.Attributes = append(protoEvent.Attributes, &v1.KeyValue{
						Key:   attr,
						Value: val,
					})
				}
			}

			protoEvents = append(protoEvents, protoEvent)
		}
	}

	return protoEvents
}

// resourceGroupKey builds a composite key for grouping flat spans into resource batches.
// Spans from the same original ResourceSpans share the same service name, attributes, and dedicated columns.
func resourceGroupKey(fs *FlatSpan, dedcols dedicatedColumnMapping) string {
	var b strings.Builder
	b.WriteString(fs.ResourceServiceName)
	fmt.Fprintf(&b, "\x00%d", fs.ResourceDroppedAttributesCount)
	for _, a := range fs.ResourceAttrs {
		b.WriteByte(0)
		b.WriteString(a.Key)
		fmt.Fprintf(&b, "\x00%v\x00%v\x00%v\x00%v\x00%v", a.Value, a.ValueInt, a.ValueDouble, a.ValueBool, a.IsArray)
	}
	for attr, col := range dedcols.items() {
		val := col.readResourceValue(fs)
		if val != nil {
			fmt.Fprintf(&b, "\x00%s=%v", attr, val)
		}
	}
	return b.String()
}

// FlatSpansToTempopbTrace reconstructs a hierarchical tempopb.Trace from flat spans.
// All spans must belong to the same trace (same TraceID).
func FlatSpansToTempopbTrace(meta *backend.BlockMeta, flatSpans []*FlatSpan) *tempopb.Trace {
	if len(flatSpans) == 0 {
		return &tempopb.Trace{ResourceSpans: make([]*v1_trace.ResourceSpans, 0)}
	}

	dedicatedResourceAttributes := dedicatedColumnsToColumnMapping(meta.DedicatedColumns, backend.DedicatedColumnScopeResource)
	dedicatedSpanAttributes := dedicatedColumnsToColumnMapping(meta.DedicatedColumns, backend.DedicatedColumnScopeSpan)
	dedicatedEventAttributes := dedicatedColumnsToColumnMapping(meta.DedicatedColumns, backend.DedicatedColumnScopeEvent)

	// Group flat spans by resource+scope identity to reconstruct hierarchy
	type scopeGroup struct {
		scope *v1_trace.ScopeSpans
	}
	type resourceGroup struct {
		batch  *v1_trace.ResourceSpans
		scopes map[string]*scopeGroup // key: scopeName+scopeVersion
	}

	// Use ordered slice to maintain deterministic output
	var resourceGroups []*resourceGroup
	resourceIndex := map[string]*resourceGroup{} // key: service name + resource attrs hash

	for _, fs := range flatSpans {
		resKey := resourceGroupKey(fs, dedicatedResourceAttributes)

		rg, ok := resourceIndex[resKey]
		if !ok {
			resAttrs := parquetToProtoAttrs(fs.ResourceAttrs)

			// Add dedicated resource attributes
			for attr, col := range dedicatedResourceAttributes.items() {
				val := col.readResourceValue(fs)
				if val != nil {
					resAttrs = append(resAttrs, &v1.KeyValue{Key: attr, Value: val})
				}
			}

			// Add service.name
			if fs.ResourceServiceName != "" {
				resAttrs = append(resAttrs, &v1.KeyValue{
					Key:   LabelServiceName,
					Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: fs.ResourceServiceName}},
				})
			}

			rg = &resourceGroup{
				batch: &v1_trace.ResourceSpans{
					Resource: &v1_resource.Resource{
						Attributes:             resAttrs,
						DroppedAttributesCount: uint32(fs.ResourceDroppedAttributesCount),
					},
				},
				scopes: map[string]*scopeGroup{},
			}
			resourceIndex[resKey] = rg
			resourceGroups = append(resourceGroups, rg)
		}

		scopeKey := fs.ScopeName + "\x00" + fs.ScopeVersion
		sg, ok := rg.scopes[scopeKey]
		if !ok {
			var scopeAttrs []*v1.KeyValue
			if len(fs.ScopeAttrs) > 0 {
				scopeAttrs = parquetToProtoAttrs(fs.ScopeAttrs)
			}
			sg = &scopeGroup{
				scope: &v1_trace.ScopeSpans{
					Scope: &v1.InstrumentationScope{
						Name:                   fs.ScopeName,
						Version:                fs.ScopeVersion,
						Attributes:             scopeAttrs,
						DroppedAttributesCount: uint32(fs.ScopeDroppedAttributesCount),
					},
				},
			}
			rg.scopes[scopeKey] = sg
			rg.batch.ScopeSpans = append(rg.batch.ScopeSpans, sg.scope)
		}

		// Build span proto
		spanAttrs := parquetToProtoAttrs(fs.Attrs)
		for attr, col := range dedicatedSpanAttributes.items() {
			val := col.readSpanValue(fs)
			if val != nil {
				spanAttrs = append(spanAttrs, &v1.KeyValue{Key: attr, Value: val})
			}
		}

		protoSpan := &v1_trace.Span{
			TraceId:                fs.TraceID,
			SpanId:                 fs.SpanID,
			TraceState:             fs.TraceState,
			Name:                   fs.Name,
			Kind:                   v1_trace.Span_SpanKind(fs.Kind),
			ParentSpanId:           fs.ParentSpanID,
			StartTimeUnixNano:      fs.StartTimeUnixNano,
			EndTimeUnixNano:        fs.StartTimeUnixNano + fs.DurationNano,
			Status:                 &v1_trace.Status{Message: fs.StatusMessage, Code: v1_trace.Status_StatusCode(fs.StatusCode)},
			Attributes:             spanAttrs,
			DroppedAttributesCount: uint32(fs.DroppedAttributesCount),
			Events:                 parquetToProtoEvents(fs.Events, fs.StartTimeUnixNano, dedicatedEventAttributes),
			DroppedEventsCount:     uint32(fs.DroppedEventsCount),
			Links:                  parquetToProtoLinks(fs.Links),
			DroppedLinksCount:      uint32(fs.DroppedLinksCount),
		}

		sg.scope.Spans = append(sg.scope.Spans, protoSpan)
	}

	protoTrace := &tempopb.Trace{}
	for _, rg := range resourceGroups {
		protoTrace.ResourceSpans = append(protoTrace.ResourceSpans, rg.batch)
	}

	return protoTrace
}

func extendReuseSlice[T any](sz int, in []T) []T {
	if cap(in) >= sz {
		return in[:sz]
	}
	in = in[:cap(in)]
	return append(in, make([]T, sz-len(in))...)
}

// SchemaWithDynamicChanges returns the Parquet schema and writer/reader options for vParquet6.
// Compression is specified via writer options (LZ4 default), not struct tags.
func SchemaWithDynamicChanges(dedicatedColumns backend.DedicatedColumns) (*parquet.Schema, []parquet.WriterOption, []parquet.ReaderOption) {
	var (
		resMapping   = dedicatedColumnsToColumnMapping(dedicatedColumns, backend.DedicatedColumnScopeResource)
		spanMapping  = dedicatedColumnsToColumnMapping(dedicatedColumns, backend.DedicatedColumnScopeSpan)
		eventMapping = dedicatedColumnsToColumnMapping(dedicatedColumns, backend.DedicatedColumnScopeEvent)
	)

	schemaOptions := []parquet.SchemaOption{}
	writerOptions := []parquet.WriterOption{}
	readerOptions := []parquet.ReaderOption{}

	// Snappy as default compression -- LZ4_RAW has a bug in parquet-go where
	// CompressBlock returns 0 bytes for incompressible data, silently dropping pages.
	writerOptions = append(writerOptions, parquet.Compression(&parquet.Snappy))

	// Larger page buffer to avoid silent data loss for repeated/nested fields
	writerOptions = append(writerOptions, parquet.PageBufferSize(10*1024*1024))

	// Built-in Parquet bloom filter on TraceID for efficient trace lookups
	writerOptions = append(writerOptions, parquet.BloomFilters(
		parquet.SplitBlockFilter(10, "TraceID"),
	))

	// Blobify dedicated columns that are marked as blobs
	blobify := func(col dedicatedColumn) {
		path := strings.Split(col.ColumnPath, ".")
		option := parquet.StructTag(`parquet:",zstd,optional"`, path...)
		schemaOptions = append(schemaOptions, option)
		readerOptions = append(readerOptions, option)
		writerOptions = append(writerOptions, option)
		writerOptions = append(writerOptions, parquet.SkipPageBounds(path...))
	}

	for _, col := range spanMapping.mapping {
		if col.IsBlob {
			blobify(col)
		}
	}
	for _, col := range resMapping.mapping {
		if col.IsBlob {
			blobify(col)
		}
	}
	for _, col := range eventMapping.mapping {
		if col.IsBlob {
			blobify(col)
		}
	}

	// Remove unused dedicated columns
	del := func(path string) {
		option := parquet.StructTag(`parquet:"-"`, strings.Split(path, ".")...)
		schemaOptions = append(schemaOptions, option)
		readerOptions = append(readerOptions, option)
		writerOptions = append(writerOptions, option)
	}

	for scope, m1 := range DedicatedResourceColumnPaths {
		for _, paths := range m1 {
			for _, path := range paths {
				switch scope {
				case backend.DedicatedColumnScopeResource:
					if !resMapping.usesPath(path) {
						del(path)
					}
				case backend.DedicatedColumnScopeSpan:
					if !spanMapping.usesPath(path) {
						del(path)
					}
				case backend.DedicatedColumnScopeEvent:
					if !eventMapping.usesPath(path) {
						del(path)
					}
				}
			}
		}

		if eventMapping.len() == 0 {
			del(FieldEventDedicatedAttrs)
		}
	}

	schema := parquet.SchemaOf(&FlatSpan{}, schemaOptions...)

	return schema, writerOptions, readerOptions
}
