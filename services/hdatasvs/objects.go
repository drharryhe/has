package hdatasvs

import (
	"reflect"
)

const (
	relTypeHasOne   = 1
	relTypeHasMany  = 2
	relTypeToMany   = 3
	relTypeBelongTo = 4
)

type DataSource struct {
}

type object struct {
	key              string
	desc             string
	instance         interface{}
	name             string
	primaryField     *field //主键所在列
	fieldMap         map[string]*field
	fieldSlice       []*field
	fieldKeysByName  map[string]string
	deniedOperations map[string]bool
}

func (this *object) AddField(f *field) {
	if this.fieldMap == nil {
		this.fieldMap = make(map[string]*field)
	}
	this.fieldMap[f.key] = f
	this.fieldSlice = append(this.fieldSlice, f)
}

func (this *object) GetInstance() interface{} {
	return this.instance
}

func (this *object) GetName() string {
	return this.name
}

func newField() *field {
	f := &field{}
	f.opDenies = make(map[string]bool)
	f.opRequired = make(map[string]bool)
	return f
}

type field struct {
	key          string
	desc         string
	name         string
	kind         reflect.Kind
	autoInitFunc string
	autoFillFunc string
	customFunc   string
	opDenies     map[string]bool
	opRequired   map[string]bool
	//immutable    bool //一旦设置值，就不可修改
	//relation *relationship
}

type relationship struct {
	relType        int
	relatedObject  interface{}
	relatedObjName string
	relatedField   string
	selfField      string
	relTable       string //仅用于many2many
}

type Filters struct {
	or              bool
	atomFilters     []*atomFilter
	combinedFilters []*Filters
}

type atomFilter struct {
	field   *field
	compare string
	value   interface{}
}

type RawFilters struct {
	Or         bool         `json:"or"`
	Conditions []string     `json:"conditions"`
	SubFilters []RawFilters `json:"filters"`
}

type orderBy struct {
	column    string
	direction string
}

type limit struct {
	offset int
	count  int
}

type groupBy struct {
	column string
	having *atomFilter
}
