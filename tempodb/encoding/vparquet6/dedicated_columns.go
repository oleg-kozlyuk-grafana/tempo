package vparquet6

import (
	"iter"

	v1 "github.com/grafana/tempo/pkg/tempopb/common/v1"
	"github.com/grafana/tempo/tempodb/backend"
)

// DedicatedResourceColumnPaths makes paths for spare dedicated attribute columns available.
// Paths are flat (no rs.Resource or rs.ss.Spans prefix).
var DedicatedResourceColumnPaths = map[backend.DedicatedColumnScope]map[backend.DedicatedColumnType][]string{
	backend.DedicatedColumnScopeResource: {
		backend.DedicatedColumnTypeString: {
			"ResourceDedicatedString01",
			"ResourceDedicatedString02",
			"ResourceDedicatedString03",
			"ResourceDedicatedString04",
			"ResourceDedicatedString05",
			"ResourceDedicatedString06",
			"ResourceDedicatedString07",
			"ResourceDedicatedString08",
			"ResourceDedicatedString09",
			"ResourceDedicatedString10",
			"ResourceDedicatedString11",
			"ResourceDedicatedString12",
			"ResourceDedicatedString13",
			"ResourceDedicatedString14",
			"ResourceDedicatedString15",
			"ResourceDedicatedString16",
			"ResourceDedicatedString17",
			"ResourceDedicatedString18",
			"ResourceDedicatedString19",
			"ResourceDedicatedString20",
		},
		backend.DedicatedColumnTypeInt: {
			"ResourceDedicatedInt01",
			"ResourceDedicatedInt02",
			"ResourceDedicatedInt03",
			"ResourceDedicatedInt04",
			"ResourceDedicatedInt05",
		},
	},
	backend.DedicatedColumnScopeSpan: {
		backend.DedicatedColumnTypeString: {
			"DedicatedString01",
			"DedicatedString02",
			"DedicatedString03",
			"DedicatedString04",
			"DedicatedString05",
			"DedicatedString06",
			"DedicatedString07",
			"DedicatedString08",
			"DedicatedString09",
			"DedicatedString10",
			"DedicatedString11",
			"DedicatedString12",
			"DedicatedString13",
			"DedicatedString14",
			"DedicatedString15",
			"DedicatedString16",
			"DedicatedString17",
			"DedicatedString18",
			"DedicatedString19",
			"DedicatedString20",
		},
		backend.DedicatedColumnTypeInt: {
			"DedicatedInt01",
			"DedicatedInt02",
			"DedicatedInt03",
			"DedicatedInt04",
			"DedicatedInt05",
		},
	},
	backend.DedicatedColumnScopeEvent: {
		backend.DedicatedColumnTypeString: {
			"Events.DedicatedAttributes.String01",
			"Events.DedicatedAttributes.String02",
			"Events.DedicatedAttributes.String03",
			"Events.DedicatedAttributes.String04",
			"Events.DedicatedAttributes.String05",
			"Events.DedicatedAttributes.String06",
			"Events.DedicatedAttributes.String07",
			"Events.DedicatedAttributes.String08",
			"Events.DedicatedAttributes.String09",
			"Events.DedicatedAttributes.String10",
			"Events.DedicatedAttributes.String11",
			"Events.DedicatedAttributes.String12",
			"Events.DedicatedAttributes.String13",
			"Events.DedicatedAttributes.String14",
			"Events.DedicatedAttributes.String15",
			"Events.DedicatedAttributes.String16",
			"Events.DedicatedAttributes.String17",
			"Events.DedicatedAttributes.String18",
			"Events.DedicatedAttributes.String19",
			"Events.DedicatedAttributes.String20",
		},
		backend.DedicatedColumnTypeInt: {
			"Events.DedicatedAttributes.Int01",
			"Events.DedicatedAttributes.Int02",
			"Events.DedicatedAttributes.Int03",
			"Events.DedicatedAttributes.Int04",
			"Events.DedicatedAttributes.Int05",
		},
	},
}

type dedicatedColumn struct {
	Type        backend.DedicatedColumnType
	ColumnPath  string
	ColumnIndex int
	// IsArray     bool
	IsBlob bool
}

func (dc *dedicatedColumn) readValue(attrs *DedicatedAttributes) *v1.AnyValue {
	var val *v1.AnyValue

	switch dc.Type {
	case backend.DedicatedColumnTypeString:
		switch dc.ColumnIndex {
		case 0:
			val = dedicatedColStrToAnyValue(attrs.String01)
		case 1:
			val = dedicatedColStrToAnyValue(attrs.String02)
		case 2:
			val = dedicatedColStrToAnyValue(attrs.String03)
		case 3:
			val = dedicatedColStrToAnyValue(attrs.String04)
		case 4:
			val = dedicatedColStrToAnyValue(attrs.String05)
		case 5:
			val = dedicatedColStrToAnyValue(attrs.String06)
		case 6:
			val = dedicatedColStrToAnyValue(attrs.String07)
		case 7:
			val = dedicatedColStrToAnyValue(attrs.String08)
		case 8:
			val = dedicatedColStrToAnyValue(attrs.String09)
		case 9:
			val = dedicatedColStrToAnyValue(attrs.String10)
		case 10:
			val = dedicatedColStrToAnyValue(attrs.String11)
		case 11:
			val = dedicatedColStrToAnyValue(attrs.String12)
		case 12:
			val = dedicatedColStrToAnyValue(attrs.String13)
		case 13:
			val = dedicatedColStrToAnyValue(attrs.String14)
		case 14:
			val = dedicatedColStrToAnyValue(attrs.String15)
		case 15:
			val = dedicatedColStrToAnyValue(attrs.String16)
		case 16:
			val = dedicatedColStrToAnyValue(attrs.String17)
		case 17:
			val = dedicatedColStrToAnyValue(attrs.String18)
		case 18:
			val = dedicatedColStrToAnyValue(attrs.String19)
		case 19:
			val = dedicatedColStrToAnyValue(attrs.String20)
		}
	case backend.DedicatedColumnTypeInt:
		switch dc.ColumnIndex {
		case 0:
			val = dedicatedColIntToAnyValue(attrs.Int01)
		case 1:
			val = dedicatedColIntToAnyValue(attrs.Int02)
		case 2:
			val = dedicatedColIntToAnyValue(attrs.Int03)
		case 3:
			val = dedicatedColIntToAnyValue(attrs.Int04)
		case 4:
			val = dedicatedColIntToAnyValue(attrs.Int05)
		}
	}

	return val
}

func (dc *dedicatedColumn) writeValue(attrs *DedicatedAttributes, value *v1.AnyValue) bool {
	var written bool

	switch dc.Type {
	case backend.DedicatedColumnTypeString:
		switch dc.ColumnIndex {
		case 0:
			attrs.String01, written = anyValueToDedicatedColStr(value, attrs.String01)
		case 1:
			attrs.String02, written = anyValueToDedicatedColStr(value, attrs.String02)
		case 2:
			attrs.String03, written = anyValueToDedicatedColStr(value, attrs.String03)
		case 3:
			attrs.String04, written = anyValueToDedicatedColStr(value, attrs.String04)
		case 4:
			attrs.String05, written = anyValueToDedicatedColStr(value, attrs.String05)
		case 5:
			attrs.String06, written = anyValueToDedicatedColStr(value, attrs.String06)
		case 6:
			attrs.String07, written = anyValueToDedicatedColStr(value, attrs.String07)
		case 7:
			attrs.String08, written = anyValueToDedicatedColStr(value, attrs.String08)
		case 8:
			attrs.String09, written = anyValueToDedicatedColStr(value, attrs.String09)
		case 9:
			attrs.String10, written = anyValueToDedicatedColStr(value, attrs.String10)
		case 10:
			attrs.String11, written = anyValueToDedicatedColStr(value, attrs.String11)
		case 11:
			attrs.String12, written = anyValueToDedicatedColStr(value, attrs.String12)
		case 12:
			attrs.String13, written = anyValueToDedicatedColStr(value, attrs.String13)
		case 13:
			attrs.String14, written = anyValueToDedicatedColStr(value, attrs.String14)
		case 14:
			attrs.String15, written = anyValueToDedicatedColStr(value, attrs.String15)
		case 15:
			attrs.String16, written = anyValueToDedicatedColStr(value, attrs.String16)
		case 16:
			attrs.String17, written = anyValueToDedicatedColStr(value, attrs.String17)
		case 17:
			attrs.String18, written = anyValueToDedicatedColStr(value, attrs.String18)
		case 18:
			attrs.String19, written = anyValueToDedicatedColStr(value, attrs.String19)
		case 19:
			attrs.String20, written = anyValueToDedicatedColStr(value, attrs.String20)
		}
	case backend.DedicatedColumnTypeInt:
		switch dc.ColumnIndex {
		case 0:
			attrs.Int01, written = anyValueToDedicatedColInt(value, attrs.Int01)
		case 1:
			attrs.Int02, written = anyValueToDedicatedColInt(value, attrs.Int02)
		case 2:
			attrs.Int03, written = anyValueToDedicatedColInt(value, attrs.Int03)
		case 3:
			attrs.Int04, written = anyValueToDedicatedColInt(value, attrs.Int04)
		case 4:
			attrs.Int05, written = anyValueToDedicatedColInt(value, attrs.Int05)
		}
	}
	return written
}

func (dc *dedicatedColumn) readSpanValue(fs *FlatSpan) *v1.AnyValue {
	var val *v1.AnyValue

	switch dc.Type {
	case backend.DedicatedColumnTypeString:
		switch dc.ColumnIndex {
		case 0:
			val = dedicatedColStrToAnyValue(fs.DedicatedString01)
		case 1:
			val = dedicatedColStrToAnyValue(fs.DedicatedString02)
		case 2:
			val = dedicatedColStrToAnyValue(fs.DedicatedString03)
		case 3:
			val = dedicatedColStrToAnyValue(fs.DedicatedString04)
		case 4:
			val = dedicatedColStrToAnyValue(fs.DedicatedString05)
		case 5:
			val = dedicatedColStrToAnyValue(fs.DedicatedString06)
		case 6:
			val = dedicatedColStrToAnyValue(fs.DedicatedString07)
		case 7:
			val = dedicatedColStrToAnyValue(fs.DedicatedString08)
		case 8:
			val = dedicatedColStrToAnyValue(fs.DedicatedString09)
		case 9:
			val = dedicatedColStrToAnyValue(fs.DedicatedString10)
		case 10:
			val = dedicatedColStrToAnyValue(fs.DedicatedString11)
		case 11:
			val = dedicatedColStrToAnyValue(fs.DedicatedString12)
		case 12:
			val = dedicatedColStrToAnyValue(fs.DedicatedString13)
		case 13:
			val = dedicatedColStrToAnyValue(fs.DedicatedString14)
		case 14:
			val = dedicatedColStrToAnyValue(fs.DedicatedString15)
		case 15:
			val = dedicatedColStrToAnyValue(fs.DedicatedString16)
		case 16:
			val = dedicatedColStrToAnyValue(fs.DedicatedString17)
		case 17:
			val = dedicatedColStrToAnyValue(fs.DedicatedString18)
		case 18:
			val = dedicatedColStrToAnyValue(fs.DedicatedString19)
		case 19:
			val = dedicatedColStrToAnyValue(fs.DedicatedString20)
		}
	case backend.DedicatedColumnTypeInt:
		switch dc.ColumnIndex {
		case 0:
			val = dedicatedColIntToAnyValue(fs.DedicatedInt01)
		case 1:
			val = dedicatedColIntToAnyValue(fs.DedicatedInt02)
		case 2:
			val = dedicatedColIntToAnyValue(fs.DedicatedInt03)
		case 3:
			val = dedicatedColIntToAnyValue(fs.DedicatedInt04)
		case 4:
			val = dedicatedColIntToAnyValue(fs.DedicatedInt05)
		}
	}

	return val
}

func (dc *dedicatedColumn) writeSpanValue(fs *FlatSpan, value *v1.AnyValue) bool {
	var written bool

	switch dc.Type {
	case backend.DedicatedColumnTypeString:
		switch dc.ColumnIndex {
		case 0:
			fs.DedicatedString01, written = anyValueToDedicatedColStr(value, fs.DedicatedString01)
		case 1:
			fs.DedicatedString02, written = anyValueToDedicatedColStr(value, fs.DedicatedString02)
		case 2:
			fs.DedicatedString03, written = anyValueToDedicatedColStr(value, fs.DedicatedString03)
		case 3:
			fs.DedicatedString04, written = anyValueToDedicatedColStr(value, fs.DedicatedString04)
		case 4:
			fs.DedicatedString05, written = anyValueToDedicatedColStr(value, fs.DedicatedString05)
		case 5:
			fs.DedicatedString06, written = anyValueToDedicatedColStr(value, fs.DedicatedString06)
		case 6:
			fs.DedicatedString07, written = anyValueToDedicatedColStr(value, fs.DedicatedString07)
		case 7:
			fs.DedicatedString08, written = anyValueToDedicatedColStr(value, fs.DedicatedString08)
		case 8:
			fs.DedicatedString09, written = anyValueToDedicatedColStr(value, fs.DedicatedString09)
		case 9:
			fs.DedicatedString10, written = anyValueToDedicatedColStr(value, fs.DedicatedString10)
		case 10:
			fs.DedicatedString11, written = anyValueToDedicatedColStr(value, fs.DedicatedString11)
		case 11:
			fs.DedicatedString12, written = anyValueToDedicatedColStr(value, fs.DedicatedString12)
		case 12:
			fs.DedicatedString13, written = anyValueToDedicatedColStr(value, fs.DedicatedString13)
		case 13:
			fs.DedicatedString14, written = anyValueToDedicatedColStr(value, fs.DedicatedString14)
		case 14:
			fs.DedicatedString15, written = anyValueToDedicatedColStr(value, fs.DedicatedString15)
		case 15:
			fs.DedicatedString16, written = anyValueToDedicatedColStr(value, fs.DedicatedString16)
		case 16:
			fs.DedicatedString17, written = anyValueToDedicatedColStr(value, fs.DedicatedString17)
		case 17:
			fs.DedicatedString18, written = anyValueToDedicatedColStr(value, fs.DedicatedString18)
		case 18:
			fs.DedicatedString19, written = anyValueToDedicatedColStr(value, fs.DedicatedString19)
		case 19:
			fs.DedicatedString20, written = anyValueToDedicatedColStr(value, fs.DedicatedString20)
		}
	case backend.DedicatedColumnTypeInt:
		switch dc.ColumnIndex {
		case 0:
			fs.DedicatedInt01, written = anyValueToDedicatedColInt(value, fs.DedicatedInt01)
		case 1:
			fs.DedicatedInt02, written = anyValueToDedicatedColInt(value, fs.DedicatedInt02)
		case 2:
			fs.DedicatedInt03, written = anyValueToDedicatedColInt(value, fs.DedicatedInt03)
		case 3:
			fs.DedicatedInt04, written = anyValueToDedicatedColInt(value, fs.DedicatedInt04)
		case 4:
			fs.DedicatedInt05, written = anyValueToDedicatedColInt(value, fs.DedicatedInt05)
		}
	}
	return written
}

func (dc *dedicatedColumn) readResourceValue(fs *FlatSpan) *v1.AnyValue {
	var val *v1.AnyValue

	switch dc.Type {
	case backend.DedicatedColumnTypeString:
		switch dc.ColumnIndex {
		case 0:
			val = dedicatedColStrToAnyValue(fs.ResourceDedicatedString01)
		case 1:
			val = dedicatedColStrToAnyValue(fs.ResourceDedicatedString02)
		case 2:
			val = dedicatedColStrToAnyValue(fs.ResourceDedicatedString03)
		case 3:
			val = dedicatedColStrToAnyValue(fs.ResourceDedicatedString04)
		case 4:
			val = dedicatedColStrToAnyValue(fs.ResourceDedicatedString05)
		case 5:
			val = dedicatedColStrToAnyValue(fs.ResourceDedicatedString06)
		case 6:
			val = dedicatedColStrToAnyValue(fs.ResourceDedicatedString07)
		case 7:
			val = dedicatedColStrToAnyValue(fs.ResourceDedicatedString08)
		case 8:
			val = dedicatedColStrToAnyValue(fs.ResourceDedicatedString09)
		case 9:
			val = dedicatedColStrToAnyValue(fs.ResourceDedicatedString10)
		case 10:
			val = dedicatedColStrToAnyValue(fs.ResourceDedicatedString11)
		case 11:
			val = dedicatedColStrToAnyValue(fs.ResourceDedicatedString12)
		case 12:
			val = dedicatedColStrToAnyValue(fs.ResourceDedicatedString13)
		case 13:
			val = dedicatedColStrToAnyValue(fs.ResourceDedicatedString14)
		case 14:
			val = dedicatedColStrToAnyValue(fs.ResourceDedicatedString15)
		case 15:
			val = dedicatedColStrToAnyValue(fs.ResourceDedicatedString16)
		case 16:
			val = dedicatedColStrToAnyValue(fs.ResourceDedicatedString17)
		case 17:
			val = dedicatedColStrToAnyValue(fs.ResourceDedicatedString18)
		case 18:
			val = dedicatedColStrToAnyValue(fs.ResourceDedicatedString19)
		case 19:
			val = dedicatedColStrToAnyValue(fs.ResourceDedicatedString20)
		}
	case backend.DedicatedColumnTypeInt:
		switch dc.ColumnIndex {
		case 0:
			val = dedicatedColIntToAnyValue(fs.ResourceDedicatedInt01)
		case 1:
			val = dedicatedColIntToAnyValue(fs.ResourceDedicatedInt02)
		case 2:
			val = dedicatedColIntToAnyValue(fs.ResourceDedicatedInt03)
		case 3:
			val = dedicatedColIntToAnyValue(fs.ResourceDedicatedInt04)
		case 4:
			val = dedicatedColIntToAnyValue(fs.ResourceDedicatedInt05)
		}
	}

	return val
}

func (dc *dedicatedColumn) writeResourceValue(fs *FlatSpan, value *v1.AnyValue) bool {
	var written bool

	switch dc.Type {
	case backend.DedicatedColumnTypeString:
		switch dc.ColumnIndex {
		case 0:
			fs.ResourceDedicatedString01, written = anyValueToDedicatedColStr(value, fs.ResourceDedicatedString01)
		case 1:
			fs.ResourceDedicatedString02, written = anyValueToDedicatedColStr(value, fs.ResourceDedicatedString02)
		case 2:
			fs.ResourceDedicatedString03, written = anyValueToDedicatedColStr(value, fs.ResourceDedicatedString03)
		case 3:
			fs.ResourceDedicatedString04, written = anyValueToDedicatedColStr(value, fs.ResourceDedicatedString04)
		case 4:
			fs.ResourceDedicatedString05, written = anyValueToDedicatedColStr(value, fs.ResourceDedicatedString05)
		case 5:
			fs.ResourceDedicatedString06, written = anyValueToDedicatedColStr(value, fs.ResourceDedicatedString06)
		case 6:
			fs.ResourceDedicatedString07, written = anyValueToDedicatedColStr(value, fs.ResourceDedicatedString07)
		case 7:
			fs.ResourceDedicatedString08, written = anyValueToDedicatedColStr(value, fs.ResourceDedicatedString08)
		case 8:
			fs.ResourceDedicatedString09, written = anyValueToDedicatedColStr(value, fs.ResourceDedicatedString09)
		case 9:
			fs.ResourceDedicatedString10, written = anyValueToDedicatedColStr(value, fs.ResourceDedicatedString10)
		case 10:
			fs.ResourceDedicatedString11, written = anyValueToDedicatedColStr(value, fs.ResourceDedicatedString11)
		case 11:
			fs.ResourceDedicatedString12, written = anyValueToDedicatedColStr(value, fs.ResourceDedicatedString12)
		case 12:
			fs.ResourceDedicatedString13, written = anyValueToDedicatedColStr(value, fs.ResourceDedicatedString13)
		case 13:
			fs.ResourceDedicatedString14, written = anyValueToDedicatedColStr(value, fs.ResourceDedicatedString14)
		case 14:
			fs.ResourceDedicatedString15, written = anyValueToDedicatedColStr(value, fs.ResourceDedicatedString15)
		case 15:
			fs.ResourceDedicatedString16, written = anyValueToDedicatedColStr(value, fs.ResourceDedicatedString16)
		case 16:
			fs.ResourceDedicatedString17, written = anyValueToDedicatedColStr(value, fs.ResourceDedicatedString17)
		case 17:
			fs.ResourceDedicatedString18, written = anyValueToDedicatedColStr(value, fs.ResourceDedicatedString18)
		case 18:
			fs.ResourceDedicatedString19, written = anyValueToDedicatedColStr(value, fs.ResourceDedicatedString19)
		case 19:
			fs.ResourceDedicatedString20, written = anyValueToDedicatedColStr(value, fs.ResourceDedicatedString20)
		}
	case backend.DedicatedColumnTypeInt:
		switch dc.ColumnIndex {
		case 0:
			fs.ResourceDedicatedInt01, written = anyValueToDedicatedColInt(value, fs.ResourceDedicatedInt01)
		case 1:
			fs.ResourceDedicatedInt02, written = anyValueToDedicatedColInt(value, fs.ResourceDedicatedInt02)
		case 2:
			fs.ResourceDedicatedInt03, written = anyValueToDedicatedColInt(value, fs.ResourceDedicatedInt03)
		case 3:
			fs.ResourceDedicatedInt04, written = anyValueToDedicatedColInt(value, fs.ResourceDedicatedInt04)
		case 4:
			fs.ResourceDedicatedInt05, written = anyValueToDedicatedColInt(value, fs.ResourceDedicatedInt05)
		}
	}
	return written
}

func resetSpanDedicated(fs *FlatSpan) {
	fs.DedicatedString01 = fs.DedicatedString01[:0]
	fs.DedicatedString02 = fs.DedicatedString02[:0]
	fs.DedicatedString03 = fs.DedicatedString03[:0]
	fs.DedicatedString04 = fs.DedicatedString04[:0]
	fs.DedicatedString05 = fs.DedicatedString05[:0]
	fs.DedicatedString06 = fs.DedicatedString06[:0]
	fs.DedicatedString07 = fs.DedicatedString07[:0]
	fs.DedicatedString08 = fs.DedicatedString08[:0]
	fs.DedicatedString09 = fs.DedicatedString09[:0]
	fs.DedicatedString10 = fs.DedicatedString10[:0]
	fs.DedicatedString11 = fs.DedicatedString11[:0]
	fs.DedicatedString12 = fs.DedicatedString12[:0]
	fs.DedicatedString13 = fs.DedicatedString13[:0]
	fs.DedicatedString14 = fs.DedicatedString14[:0]
	fs.DedicatedString15 = fs.DedicatedString15[:0]
	fs.DedicatedString16 = fs.DedicatedString16[:0]
	fs.DedicatedString17 = fs.DedicatedString17[:0]
	fs.DedicatedString18 = fs.DedicatedString18[:0]
	fs.DedicatedString19 = fs.DedicatedString19[:0]
	fs.DedicatedString20 = fs.DedicatedString20[:0]
	fs.DedicatedInt01 = fs.DedicatedInt01[:0]
	fs.DedicatedInt02 = fs.DedicatedInt02[:0]
	fs.DedicatedInt03 = fs.DedicatedInt03[:0]
	fs.DedicatedInt04 = fs.DedicatedInt04[:0]
	fs.DedicatedInt05 = fs.DedicatedInt05[:0]
}

func resetResourceDedicated(fs *FlatSpan) {
	fs.ResourceDedicatedString01 = fs.ResourceDedicatedString01[:0]
	fs.ResourceDedicatedString02 = fs.ResourceDedicatedString02[:0]
	fs.ResourceDedicatedString03 = fs.ResourceDedicatedString03[:0]
	fs.ResourceDedicatedString04 = fs.ResourceDedicatedString04[:0]
	fs.ResourceDedicatedString05 = fs.ResourceDedicatedString05[:0]
	fs.ResourceDedicatedString06 = fs.ResourceDedicatedString06[:0]
	fs.ResourceDedicatedString07 = fs.ResourceDedicatedString07[:0]
	fs.ResourceDedicatedString08 = fs.ResourceDedicatedString08[:0]
	fs.ResourceDedicatedString09 = fs.ResourceDedicatedString09[:0]
	fs.ResourceDedicatedString10 = fs.ResourceDedicatedString10[:0]
	fs.ResourceDedicatedString11 = fs.ResourceDedicatedString11[:0]
	fs.ResourceDedicatedString12 = fs.ResourceDedicatedString12[:0]
	fs.ResourceDedicatedString13 = fs.ResourceDedicatedString13[:0]
	fs.ResourceDedicatedString14 = fs.ResourceDedicatedString14[:0]
	fs.ResourceDedicatedString15 = fs.ResourceDedicatedString15[:0]
	fs.ResourceDedicatedString16 = fs.ResourceDedicatedString16[:0]
	fs.ResourceDedicatedString17 = fs.ResourceDedicatedString17[:0]
	fs.ResourceDedicatedString18 = fs.ResourceDedicatedString18[:0]
	fs.ResourceDedicatedString19 = fs.ResourceDedicatedString19[:0]
	fs.ResourceDedicatedString20 = fs.ResourceDedicatedString20[:0]
	fs.ResourceDedicatedInt01 = fs.ResourceDedicatedInt01[:0]
	fs.ResourceDedicatedInt02 = fs.ResourceDedicatedInt02[:0]
	fs.ResourceDedicatedInt03 = fs.ResourceDedicatedInt03[:0]
	fs.ResourceDedicatedInt04 = fs.ResourceDedicatedInt04[:0]
	fs.ResourceDedicatedInt05 = fs.ResourceDedicatedInt05[:0]
}

func newDedicatedColumnMapping(size int) dedicatedColumnMapping {
	return dedicatedColumnMapping{
		mapping: make(map[string]dedicatedColumn, size),
		keys:    make([]string, 0, size),
	}
}

type dedicatedColumnMapping struct {
	mapping map[string]dedicatedColumn
	keys    []string
}

func (dm *dedicatedColumnMapping) put(attr string, col dedicatedColumn) {
	dm.mapping[attr] = col
	dm.keys = append(dm.keys, attr)
}

func (dm *dedicatedColumnMapping) get(attr string) (dedicatedColumn, bool) {
	col, ok := dm.mapping[attr]
	return col, ok
}

func (dm *dedicatedColumnMapping) usesPath(path string) bool {
	for _, col := range dm.mapping {
		if col.ColumnPath == path {
			return true
		}
	}
	return false
}

func (dm *dedicatedColumnMapping) items() iter.Seq2[string, dedicatedColumn] {
	return func(yield func(string, dedicatedColumn) bool) {
		for _, k := range dm.keys {
			if !yield(k, dm.mapping[k]) {
				return
			}
		}
	}
}

func (dm *dedicatedColumnMapping) len() int {
	return len(dm.keys)
}

var allScopes = []backend.DedicatedColumnScope{backend.DedicatedColumnScopeResource, backend.DedicatedColumnScopeSpan}

func dedicatedColumnsToColumnMapping(dedicatedColumns backend.DedicatedColumns, scopes ...backend.DedicatedColumnScope) dedicatedColumnMapping {
	if len(scopes) == 0 {
		scopes = allScopes
	}

	mapping := newDedicatedColumnMapping(len(dedicatedColumns))

	for _, scope := range scopes {
		spareColumnsByType, ok := DedicatedResourceColumnPaths[scope]
		if !ok {
			continue
		}

		indexByType := map[backend.DedicatedColumnType]int{}
		for _, c := range dedicatedColumns {
			if c.Scope != scope {
				continue
			}
			spareColumnPaths, exists := spareColumnsByType[c.Type]
			if !exists {
				continue
			}

			i := indexByType[c.Type]
			if i >= len(spareColumnPaths) {
				continue
			}

			dc := dedicatedColumn{
				Type:        c.Type,
				ColumnPath:  spareColumnPaths[i],
				ColumnIndex: i,
			}

			for _, opt := range c.Options {
				switch opt {
				case backend.DedicatedColumnOptionArray:
					// dc.IsArray = true
				case backend.DedicatedColumnOptionBlob:
					dc.IsBlob = true
				}
			}

			mapping.put(c.Name, dc)
			indexByType[c.Type]++
		}
	}

	return mapping
}

func filterDedicatedColumns(columns backend.DedicatedColumns) backend.DedicatedColumns {
	filtered := make(backend.DedicatedColumns, 0, len(columns))
	for _, c := range columns {
		if isIgnoredDedicatedColumn(&c) {
			continue
		}
		filtered = append(filtered, c)
	}
	return filtered
}

func isIgnoredDedicatedColumn(dc *backend.DedicatedColumn) bool {
	if _, found := DedicatedResourceColumnPaths[dc.Scope][dc.Type]; !found {
		return true
	}
	return false
}

func anyValueToDedicatedColStr(value *v1.AnyValue, buf []string) ([]string, bool) {
	buf = buf[:0]
	switch value := value.Value.(type) {
	case *v1.AnyValue_StringValue:
		buf = append(buf, value.StringValue)
	case *v1.AnyValue_ArrayValue:
		for _, v := range value.ArrayValue.Values {
			switch v := v.Value.(type) {
			case *v1.AnyValue_StringValue:
				buf = append(buf, v.StringValue)
			default:
				return nil, false
			}
		}
	default:
		return nil, false
	}

	return buf, true
}

func dedicatedColStrToAnyValue(v []string) *v1.AnyValue {
	switch len(v) {
	case 0:
		return nil
	case 1:
		return &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: v[0]}}
	default:
		var (
			values   = make([]*v1.AnyValue, 0, len(v))
			allocAny = make([]v1.AnyValue, len(v))
			allocStr = make([]v1.AnyValue_StringValue, len(v))
		)
		for i, s := range v {
			anyS := &allocStr[i]
			anyS.StringValue = s
			anyV := &allocAny[i]
			anyV.Value = anyS
			values = append(values, anyV)
		}
		return &v1.AnyValue{Value: &v1.AnyValue_ArrayValue{ArrayValue: &v1.ArrayValue{Values: values}}}
	}
}

func anyValueToDedicatedColInt(value *v1.AnyValue, buf []int64) ([]int64, bool) {
	buf = buf[:0]
	switch value := value.Value.(type) {
	case *v1.AnyValue_IntValue:
		buf = append(buf, value.IntValue)
	case *v1.AnyValue_ArrayValue:
		for _, v := range value.ArrayValue.Values {
			switch v := v.Value.(type) {
			case *v1.AnyValue_IntValue:
				buf = append(buf, v.IntValue)
			default:
				return nil, false
			}
		}
	default:
		return nil, false
	}

	return buf, true
}

func dedicatedColIntToAnyValue(v []int64) *v1.AnyValue {
	switch len(v) {
	case 0:
		return nil
	case 1:
		return &v1.AnyValue{Value: &v1.AnyValue_IntValue{IntValue: v[0]}}
	default:
		var (
			values   = make([]*v1.AnyValue, 0, len(v))
			allocAny = make([]v1.AnyValue, len(v))
			allocInt = make([]v1.AnyValue_IntValue, len(v))
		)
		for i, n := range v {
			anyS := &allocInt[i]
			anyS.IntValue = n
			anyV := &allocAny[i]
			anyV.Value = anyS
			values = append(values, anyV)
		}
		return &v1.AnyValue{Value: &v1.AnyValue_ArrayValue{ArrayValue: &v1.ArrayValue{Values: values}}}
	}
}
