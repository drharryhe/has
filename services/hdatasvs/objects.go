package hdatasvs

import (
	"fmt"
	"reflect"
)

type DataObject struct {
}

type DataView struct {
}

//type fieldOwner struct {
//	object *object
//	name   string
//	key    string
//}

type iField interface {
	Name() string
	Key() string
	Kind() reflect.Kind
	Owner() *fieldOwner
	SQL() string
	Column() string
}

type fieldOwner struct {
	object   *object //字段所有者对象
	fieldKey string  //字段在所有者对象中的key
	fieldCol string  //字段在所属对象实例中的列名
}

type baseField struct {
	key  string //字段key
	name string //字段名
	//col    string                 //字段
	kind  reflect.Kind //字段的kind
	owner *fieldOwner  //字段所属所有者对象
	//stored bool //是否存入数据库
}

func (this *baseField) Name() string {
	return this.name
}

func (this *baseField) Key() string {
	return this.key
}

func (this *baseField) Kind() reflect.Kind {
	return this.kind
}

func (this *baseField) SQL() string {
	return fmt.Sprintf("`%s`.`%s` AS `%s`", this.owner.object.key, this.owner.fieldCol, this.key)
}

func (this *baseField) Column() string {
	return fmt.Sprintf("`%s`.`%s`", this.owner.object.key, this.owner.fieldCol)
}

func (this *baseField) Owner() *fieldOwner {
	return this.owner
}

type viewField struct {
	baseField
}

func newViewField() *viewField {
	f := &viewField{}
	return f
}

//视图，用于连表查询。主要在定义视图实例时，全部使用key，而不是对象/字段名或列名
type view struct {
	from       *object                        //From 连接的对象
	joins      []*join                        //LEFT JOIN 的对象
	key        string                         //视图Key
	instance   interface{}                    //视图实例对象
	name       string                         //视图名称
	fieldMap   map[string] /*key*/ *viewField //视图所有字段
	fieldSlice []*viewField                   //视图所有字段数组
	iFieldMap  map[string] /*key*/ iField     //字段key到iField的映射
}

type join struct {
	object *object
	on     on
}

type on struct {
	leftObj    *object
	leftField  *objField
	rightObj   *object
	rightField *objField
}

func (this *view) AddField(f *viewField) {
	if this.fieldMap == nil {
		this.fieldMap = make(map[string]*viewField)
	}
	if this.iFieldMap == nil {
		this.iFieldMap = make(map[string]iField)
	}
	this.fieldMap[f.key] = f
	this.iFieldMap[f.key] = f
	this.fieldSlice = append(this.fieldSlice, f)
}

//对象
type object struct {
	database           string                              //对象保存到的数据库的key
	tableName          string                              //表名（分表时作为前缀）
	tabNamingFieldKeys []string                            //指定以哪个字段Key的值作为分表的依据
	name               string                              //对象名称，即实例的struct name
	key                string                              //对象key
	instance           interface{}                         //具体业务对象实例
	primaryField       *objField                           //主键所在列
	fieldMap           map[string] /*field key*/ *objField //实例所有字段
	fieldSlice         []*objField                         //实例所有字段数组
	fieldMapByName     map[string] /*name*/ *objField      /*key*/ //实例字段名到字段的映射
	iFieldMap          map[string] /*field key*/ iField    //实例字段key到iField接口的映射
	deniedOperations   map[string]bool                     //实例禁止的操作
}

func (this *object) AddField(f *objField) {
	if this.fieldMap == nil {
		this.fieldMap = make(map[string]*objField)
	}
	if (this.iFieldMap) == nil {
		this.iFieldMap = make(map[string]iField)
	}
	if this.fieldMapByName == nil {
		this.fieldMapByName = make(map[string]*objField)
	}
	this.fieldMap[f.key] = f
	this.iFieldMap[f.key] = f
	this.fieldMapByName[f.name] = f
	this.fieldSlice = append(this.fieldSlice, f)
}

type objField struct {
	baseField

	col          string          //对应表的列名
	autoInitFunc string          //自动初始化函数名
	autoFillFunc string          //自动填充函数名
	customFunc   string          //自定义参数
	opDenies     map[string]bool //字段禁止的操作
	opRequired   map[string]bool //字段必须的操作
}

func newObjectField() *objField {
	f := &objField{}
	f.opDenies = make(map[string]bool)
	f.opRequired = make(map[string]bool)
	return f
}

type filter struct {
	or         bool
	conditions []*condition
	filters    []*filter
}

type condition struct {
	field   iField
	compare string
	value   interface{} //已经根据field的类型，解析成了正确的数据
}

type rawFilter struct {
	Or         bool        `json:"or"`
	Conditions []string    `json:"conditions"`
	Filters    []rawFilter `json:"filters"`
}

type ordering struct {
	object    string
	column    string
	direction string
}

type limit struct {
	offset int
	count  int
}

//
//type groupBy struct {
//	column string
//	having *condition
//}
