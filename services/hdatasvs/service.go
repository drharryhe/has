/**********************************
	Service 关系数据管理服务
	为了保证Service可以对所有注册到GormPlugin的对象进行管理，Service需要在其他服务注册之后，最后进行注册
 **********************************/

package hdatasvs

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"

	"github.com/jinzhu/gorm"
	jsoniter "github.com/json-iterator/go"

	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
	"github.com/drharryhe/has/plugins/hgormplugin"
	"github.com/drharryhe/has/utils/hconverter"
	"github.com/drharryhe/has/utils/hdatetime"
	"github.com/drharryhe/has/utils/hruntime"
)

const (
	opCreate = "create"
	opUpdate = "update"
	opDelete = "del"
	opQuery  = "query"
)

type FieldFunc func(param interface{}) (interface{}, *herrors.Error)
type FieldFuncMap map[string]FieldFunc

type Service struct {
	core.Service

	conf DataService

	db                      *gorm.DB
	objectsWithKey          map[string]*object
	objectsWithName         map[string]*object
	instancesWithName       map[string]interface{}
	tableNamesOfObj         map[string]string
	afterUpdateHookCallers  map[string]*core.MethodCaller
	afterUpdateHookNames    map[string]string
	beforeUpdateHookCallers map[string]*core.MethodCaller
	beforeUpdateHookNames   map[string]string
	afterCreateHookCallers  map[string]*core.MethodCaller
	afterCreateHookNames    map[string]string
	beforeCreateHookCallers map[string]*core.MethodCaller
	beforeCreateHookNames   map[string]string
	afterQueryHookCallers   map[string]*core.MethodCaller
	afterQueryHookNames     map[string]string
	beforeQueryHookCallers  map[string]*core.MethodCaller
	beforeQueryHookNames    map[string]string
	afterDelHookCallers     map[string]*core.MethodCaller
	afterDelHookNames       map[string]string
	beforeDelHookCallers    map[string]*core.MethodCaller
	beforeDelHookNames      map[string]string
	fieldFuncMap            map[string]FieldFunc
}

func (this *Service) Open(s core.IServer, instance core.IService, args ...htypes.Any) *herrors.Error {
	err := this.Service.Open(s, instance, args)
	if err != nil {
		return err
	}

	plugin := this.UsePlugin("GormPlugin").(*hgormplugin.Plugin)
	this.db = plugin.Capability().(*gorm.DB)
	objs := plugin.Objects()

	this.instancesWithName = make(map[string]interface{})
	this.tableNamesOfObj = make(map[string]string)
	for _, o := range objs {
		n := hruntime.GetObjectName(o)
		this.instancesWithName[n] = o
		this.tableNamesOfObj[n] = gorm.ToTableName(n)
	}
	this.objectsWithKey = make(map[string]*object)
	this.objectsWithName = make(map[string]*object)

	this.beforeCreateHookNames = make(map[string]string)
	this.afterCreateHookNames = make(map[string]string)
	this.beforeUpdateHookNames = make(map[string]string)
	this.afterUpdateHookNames = make(map[string]string)
	this.beforeQueryHookNames = make(map[string]string)
	this.afterQueryHookNames = make(map[string]string)
	this.beforeDelHookNames = make(map[string]string)
	this.afterDelHookNames = make(map[string]string)
	for _, o := range objs {
		obj := this.parseObject(o)
		if obj != nil {
			this.objectsWithKey[obj.key] = obj
			this.objectsWithName[hruntime.GetObjectName(o)] = obj
		}
	}

	if len(args) > 0 {
		go this.mountHookCallers(args[0])
	}
	this.fieldFuncMap = make(map[string]FieldFunc)
	if len(args) > 1 {
		fm, ok := args[1].(map[string]FieldFunc)
		if ok {
			this.fieldFuncMap = fm
		}
	}
	return nil
}

func (this *Service) EntityStub() *core.EntityStub {
	return core.NewEntityStub(
		&core.EntityStubOptions{
			Owner:       this,
			Ping:        nil,
			GetLoad:     nil,
			ResetConfig: nil,
		})
}

func (this *Service) Config() core.IEntityConf {
	return &this.conf
}

func (this *Service) CreateM(ps htypes.Map, res *core.SlotResponse) {
	key := ps["key"].(string)
	o := this.objectsWithKey[key]
	if o == nil {
		this.Response(res, nil, herrors.ErrSysInternal.New("key [%s]not found:", key).D("failed to create data"))
		return
	}

	vs := make(map[string]interface{})
	vs["key"] = key
	var ids []interface{}
	for _, vals := range ps["objects"].([]interface{}) {
		vs["object"] = vals
		this.Create(vs, res)
		if res.Error == nil {
			ids = append(ids, res.Data)
		} else {
			this.Response(res, nil, res.Error)
			return
		}
	}

	this.Response(res, ids, nil)
}

func (this *Service) Create(ps htypes.Map, res *core.SlotResponse) {
	key := ps["key"].(string)
	o := this.objectsWithKey[key]
	if o == nil {
		this.Response(res, nil, herrors.ErrSysInternal.New("key [%s] not found", key).D("failed to create data"))
		return
	}
	if o.deniedOperations[opCreate] {
		this.Response(res, nil, herrors.ErrUserUnauthorizedAct.New("object [%s] cannot be created", key).D("failed to create data"))
		return
	}

	vals := ps["object"].(map[string]interface{})
	if vals["id"] != nil {
		this.Response(res, nil, herrors.ErrCallerInvalidRequest.New("object parameter [id] is forbidden in create()", key).D("failed to create data"))
		return
	}

	for k := range vals {
		f := o.fieldMap[k]
		if f != nil {
			if f.opDenies[opCreate] {
				this.Response(res, nil, herrors.ErrUserUnauthorizedAct.New("field [%s] of object [%s] cannot be created", k, key).D("failed to create data"))
				return
			}
		}
	}

	vs, err := this.shapeObjectFieldValues(opCreate, o.name, vals)
	if err != nil {
		this.Response(res, nil, err)
		return
	}

	if hook := this.beforeCreateHookNames[key]; hook != "" {
		reply := core.CallerResponse{}
		stop := false
		this.callBeforeCreateHook(hook, vs, &reply, &stop)
		if reply.Error != nil {
			this.Response(res, nil, reply.Error)
			return
		}
		if stop {
			this.Response(res, reply.Data, reply.Error)
			return
		}
	}

	ins, err := this.createObject(o.instance, vs)
	if err != nil {
		this.Response(res, nil, err)
		return
	}
	if vs["ID"] == nil {
		vs["ID"] = this.getObjectFieldValue(ins, "ID")
	}

	if hook := this.afterCreateHookNames[key]; hook != "" {
		reply := core.CallerResponse{}
		this.callAfterCreateHook(hook, vs, &reply)
		this.Response(res, reply.Data, reply.Error)
	} else {
		this.Response(res, vs["ID"], nil)
	}
}

func (this *Service) delRelations(tab string, relatedField string, val interface{}) *herrors.Error {
	err := this.db.Table(tab).Where(fmt.Sprintf("%s = ?", gorm.ToColumnName(relatedField)), val).Delete(nil).Error
	if err != nil {
		return herrors.ErrSysInternal.New(err.Error())
	}
	return nil
}

func (this *Service) createObject(o interface{}, vals map[string]interface{}) (interface{}, *herrors.Error) {
	ins := hruntime.CloneObject(o)
	err := hruntime.SetObjectValues(ins, vals)
	if err != nil {
		return nil, herrors.ErrSysInternal.New(err.Error())
	}
	err = this.db.Save(ins).Error
	if err != nil {
		if strings.Index(err.Error(), "Error 1062") >= 0 {
			return nil, herrors.ErrCallerInvalidRequest.New("object duplicated")
		}
		return nil, herrors.ErrSysInternal.New(err.Error())
	}

	return ins, nil
}

func (this *Service) Update(ps htypes.Map, res *core.SlotResponse) {
	key := ps["key"].(string)
	o := this.objectsWithKey[key]
	if o == nil {
		this.Response(res, nil, herrors.ErrCallerInvalidRequest.New("key %s not found", key))
		return
	}
	if o.deniedOperations[opUpdate] {
		this.Response(res, nil, herrors.ErrCallerUnauthorizedAccess.New("object %s cannot be updated", key))
		return
	}

	vals := ps["value"].(map[string]interface{})
	if vals["id"] == nil {
		this.Response(res, nil, herrors.ErrSysInternal.New("parameter id required"))
		return
	}
	for k := range vals {
		f := o.fieldMap[k]
		if f != nil {
			if f.opDenies[opUpdate] {
				this.Response(res, nil, herrors.ErrCallerUnauthorizedAccess.New("field %s of object %s cannot be updated", k, key))
				return
			}
		}
	}

	vs, err := this.shapeObjectFieldValues(opUpdate, o.name, vals)
	if err != nil {
		this.Response(res, nil, err)
		return
	}

	if hook := this.beforeUpdateHookNames[key]; hook != "" {
		reply := core.CallerResponse{}
		stop := false
		this.callBeforeUpdateHook(hook, int64(vals["id"].(float64)), vs, &reply, &stop)
		if reply.Error != nil {
			this.Response(res, nil, reply.Error)
			return
		}
		if stop {
			this.Response(res, reply.Data, reply.Error)
			return
		}
	}

	//获取老对象
	ins := hruntime.CloneObject(o.instance)
	if err := this.db.Where("id = ?", vals["id"]).First(ins).Error; err != nil {
		this.Response(res, nil, herrors.ErrCallerInvalidRequest.New(err.Error()).D("object not found"))
		return
	}

	if err := this.db.Model(ins).Updates(vs).Error; err != nil {
		this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
		return
	}

	if hook := this.afterUpdateHookNames[key]; hook != "" {
		reply := core.CallerResponse{}
		this.callAfterUpdateHook(hook, int64(vs["ID"].(float64)), vs, &reply)
		this.Response(res, reply.Data, reply.Error)
	} else {
		this.Response(res, nil, nil)
	}
}

func (this *Service) UpdateM(ps htypes.Map, res *core.SlotResponse) {
	key := ps["key"].(string)
	o := this.objectsWithKey[key]
	if o == nil {
		this.Response(res, nil, herrors.ErrSysInternal.New("key [%s] not found:", key))
		return
	}

	vs := make(map[string]interface{})
	vs["key"] = key
	for _, vals := range ps["values"].([]interface{}) {
		vs["values"] = vals
		this.Update(vs, res)
		if res.Error.Code != herrors.ECodeOK {
			return
		}
	}
}

func (this *Service) Delete(ps htypes.Map, res *core.SlotResponse) {
	key := ps["key"].(string)
	o := this.objectsWithKey[key]
	if o == nil {
		this.Response(res, nil, herrors.ErrSysInternal.New("key [%s] not found:", key))
		return
	}
	if o.deniedOperations[opDelete] {
		this.Response(res, nil, herrors.ErrCallerUnauthorizedAccess.New("object %s cannot be deleted", key))
		return
	}
	vs := make(map[string]interface{})
	vs["key"] = key

	var ids []int64
	for _, id := range ps["ids"].([]interface{}) {
		ids = append(ids, int64(id.(float64)))
	}

	if hook := this.beforeDelHookNames[key]; hook != "" {
		reply := core.CallerResponse{}
		stop := false
		this.callBeforeDelHook(hook, ids, &reply, &stop)
		if reply.Error != nil {
			this.Response(res, nil, reply.Error)
			return
		}
		if stop {
			this.Response(res, reply.Data, reply.Error)
			return
		}
	}

	for _, id := range ids {
		ins := hruntime.CloneObject(o.instance)
		if err := this.db.Where("id = ?", id).First(ins).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				continue
			}
			this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
			return
		}
		_ = this.db.Delete(ins).Error
	}

	if hook := this.afterDelHookNames[key]; hook != "" {
		reply := core.CallerResponse{}
		this.callAfterDelHook(hook, ids, &reply)
		this.Response(res, reply.Data, reply.Error)
	} else {
		this.Response(res, nil, nil)
	}
}

func (this *Service) Query(ps htypes.Map, res *core.SlotResponse) {
	key := ps["key"].(string)
	o := this.objectsWithKey[key]
	if o == nil {
		this.Response(res, nil, herrors.ErrSysInternal.New("key [%s] not found", key))
		return
	}
	if o.deniedOperations[opQuery] {
		this.Response(res, nil, herrors.ErrCallerUnauthorizedAccess.New("object %s cannot be queried", key))
		return
	}

	//处理where 子句
	var rawFilters RawFilters
	bs, _ := jsoniter.Marshal(ps["filters"])
	_ = jsoniter.Unmarshal(bs, &rawFilters)
	filters, err := this.ParserRawFilters(o, &rawFilters)
	if err != nil {
		this.Response(res, nil, err)
		return
	}
	var where string
	var vals []interface{}
	if filters != nil {
		where, vals, err = this.BuildWhereClause(filters)
		if err != nil {
			this.Response(res, nil, err)
			return
		}
	}

	//处理select 子句
	var dims []string
	var selectFieldNames []string
	dimMap := make(map[string]bool)

	for _, d := range ps["dims"].([]interface{}) {
		f := o.fieldMap[d.(string)]
		if f == nil {
			continue
		}
		if f.opDenies[opQuery] {
			this.Response(res, nil, herrors.ErrCallerUnauthorizedAccess.New("field %s of object %s cannot be queried", key, f.key))
			return
		}

		if f.kind == reflect.Map || f.kind == reflect.Ptr {
			continue
		}

		if f.kind == reflect.Slice || f.kind == reflect.Struct {
			//associations = append(associations, f)
			continue
		} else {
			c := gorm.ToColumnName(f.name)
			dimMap[c] = true
			dims = append(dims, c)
			selectFieldNames = append(selectFieldNames, fmt.Sprintf("`%s`", c))
		}
	}

	if len(dims) == 0 {
		this.Response(res, nil, herrors.ErrCallerInvalidRequest.New("parameter [dims] not found"))
		return
	}

	//处理order子句
	var orderBys []string
	if ps["order_by"] != nil {
		var order orderBy
		for _, s := range ps["order_by"].([]interface{}) {
			ss := strings.Split(s.(string), ",")
			if len(ss) != 2 {
				this.Response(res, nil, herrors.ErrCallerInvalidRequest.New("invalid parameter order_by [%s]", s))
				continue
			}
			f := o.fieldMap[strings.Trim(ss[0], " ")]
			if f == nil {
				continue
			}
			order.column = gorm.ToColumnName(f.name)
			if len(ss) == 1 || strings.ToUpper(strings.Trim(ss[1], " ")) == "ASC" {
				order.direction = "ASC"
			} else {
				order.direction = "DESC"
			}
			orderBys = append(orderBys, fmt.Sprintf("%s %s", order.column, order.direction))
		}
	}

	//处理limit 子句
	limit := limit{}
	computeTotal := false
	if ps["paging"] != nil {
		paging := ps["paging"].([]interface{})
		if len(paging) == 2 {
			computeTotal = true
			limit.offset = int((paging[0].(float64) - 1) * paging[1].(float64))
			limit.count = int(paging[1].(float64))
		}
	}

	var group groupBy
	if ps["group"] != nil {
		gs := ps["group"].([]interface{})
		f := o.fieldMap[gs[0].(string)]
		if f != nil {
			group.column = gorm.ToColumnName(f.name)
			if len(gs) > 1 {
				ss, err := this.parseCondition(gs[1].(string))
				if err != nil {
					this.Response(res, nil, err)
					return
				}
				group.having, err = this.convCondition2Filter(o, ss)
				if err != nil {
					this.Response(res, nil, err)
					return
				}
			}
		}
	}

	if hook := this.beforeQueryHookNames[key]; hook != "" {
		reply := core.CallerResponse{}
		stop := false
		this.callBeforeQueryHook(hook, ps, &reply, &stop)
		if reply.Error != nil {
			this.Response(res, nil, reply.Error)
			return
		}
		if stop {
			this.Response(res, reply.Data, reply.Error)
			return
		}
	}

	//查询数据
	var data []interface{}
	scope := this.db.Model(o.instance)

	if strings.Trim(strings.Trim(where, ")"), "(") != "" {
		scope = scope.Where(where, vals...)
	}
	scope = scope.Select(selectFieldNames)
	if group.column != "" {
		scope = scope.Group(group.column)
	}
	if group.having != nil {
		scope = scope.Having(fmt.Sprintf("%s %s ?", gorm.ToColumnName(group.having.field.name), group.having.compare), group.having.value)
	}
	if len(orderBys) > 0 {
		scope = scope.Order(strings.Join(orderBys, ","))
	}

	if limit.count > 0 {
		scope = scope.Limit(limit.count).Offset(limit.offset)
	}
	if rows, err := scope.Rows(); err != nil {
		this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
		return
	} else {
		defer rows.Close()
		for rows.Next() {
			vals := this.buildDimValues(o, dims)
			err = rows.Scan(vals...)
			if err != nil {
				this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
				return
			}
			data = append(data, this.bindDimValues(o, dims, vals))
		}
	}

	if hook := this.afterQueryHookNames[key]; hook != "" {
		var err error
		reply := core.CallerResponse{}
		this.callAfterQueryHook(hook, ps, data, &reply)
		if computeTotal {
			scope := this.db.Model(o.instance)
			var total int
			if where != "" {
				err = scope.Where(where, vals...).Count(&total).Error
			} else {
				err = scope.Count(&total).Error
			}
			if err != nil {
				this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
				return
			} else {
				resData := map[string]interface{}{
					"total": total,
					"data":  reply.Data,
				}
				this.Response(res, resData, reply.Error)
			}
		} else {
			this.Response(res, data, nil)
		}
	} else {
		var err error
		if computeTotal {
			scope := this.db.Model(o.instance)
			var total int
			if where != "" && where != "()" {
				err = scope.Where(where, vals...).Count(&total).Error
			} else {
				err = scope.Count(&total).Error
			}
			if err != nil {
				this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
				return
			} else {
				resData := map[string]interface{}{
					"total": total,
				}
				if data == nil || reflect.ValueOf(data).IsNil() {
					resData["data"] = []interface{}{}
				} else {
					resData["data"] = data
				}
				this.Response(res, resData, nil)
			}
		} else {
			this.Response(res, data, nil)
		}
	}
}

func (this *Service) BuildWhereClause(fs *Filters) (string, []interface{}, *herrors.Error) {
	logic := "AND"
	var (
		where string
		vals  []interface{}
		w     string
		v     interface{}
		ss    []string
		ns    []float64
		ok    bool
	)

	for i, f := range fs.atomFilters {
		if f.field.kind == reflect.Slice || f.field.kind == reflect.Map || f.field.kind == reflect.Struct || f.field.kind == reflect.Ptr {
			continue
		}
		com := strings.ToUpper(f.compare)
		switch com {
		case "BETWEEN", "NOT BETWEEN":
			w = fmt.Sprintf("`%s` BETWEEN ? AND ?", gorm.ToColumnName(f.field.name))
			if f.field.kind == reflect.String {
				ss, ok = hconverter.String2StringArray(f.value.(string))
				if !ok {
					return "", nil, herrors.ErrCallerInvalidRequest.New("invalid query condition %s", f.value.(string))
				}
				for _, t := range ss {
					vals = append(vals, t)
				}

			} else {
				ns, ok = hconverter.String2NumberArray(f.value.(string))
				if !ok {
					return "", nil, herrors.ErrCallerInvalidRequest.New("invalid query condition %s", f.value.(string))
				}
				for _, t := range ns {
					vals = append(vals, t)
				}
			}
		case "IN", "NOT IN":
			if com == "IN" {
				w = fmt.Sprintf("`%s` IN (", gorm.ToColumnName(f.field.name))
			} else {
				w = fmt.Sprintf("`%s` NOT IN (", gorm.ToColumnName(f.field.name))
			}

			if f.field.kind == reflect.String {
				ss, ok = hconverter.String2StringArray(f.value.(string))
				if !ok {
					return "", nil, herrors.ErrCallerInvalidRequest.New("invalid query condition %s", f.value.(string))
				}
				for i, t := range ss {
					vals = append(vals, t)
					if i == len(ss)-1 {
						w += "?"
					} else {
						w += "?,"
					}
				}
			} else {
				ns, ok = hconverter.String2NumberArray(f.value.(string))
				if !ok {
					return "", nil, herrors.ErrCallerInvalidRequest.New("invalid query condition %s", f.value.(string))
				}
				for i, t := range ns {
					vals = append(vals, t)
					if i == len(ns)-1 {
						w += "?"
					} else {
						w += "?,"
					}
				}
			}
			w += ")"
		case "LIKE", "NOT LIKE":
			w = fmt.Sprintf("`%s` %s ?", gorm.ToColumnName(f.field.name), f.compare)
			v = fmt.Sprintf("%%%v%%", f.value)
			vals = append(vals, v)
		case "HAS_PREFIX", "NOT HAS_PREFIX":
			if com == "HAS_PREFIX" {
				w = fmt.Sprintf("`%s` %s ?", gorm.ToColumnName(f.field.name), "LIKE")
			} else {
				w = fmt.Sprintf("`%s` %s ?", gorm.ToColumnName(f.field.name), "NOT LIKE")
			}
			v = fmt.Sprintf("%v%%", f.value)
			vals = append(vals, v)
		case "HAS_SUFFIX", "NOT HAS_SUFFIX":
			if com == "HAS_SUFFIX" {
				w = fmt.Sprintf("`%s` %s ?", gorm.ToColumnName(f.field.name), "LIKE")
			} else {
				w = fmt.Sprintf("`%s` %s ?", gorm.ToColumnName(f.field.name), "NOT LIKE")
			}
			v = fmt.Sprintf("%%%v", f.value)
			vals = append(vals, v)
		case "IS", "NOT IS":
			w = fmt.Sprintf("`%s` %s ?", gorm.ToColumnName(f.field.name), f.compare)
			v = f.value
			vals = append(vals, v)
		default:
			w = fmt.Sprintf("`%s` %s ?", gorm.ToColumnName(f.field.name), f.compare)
			vals = append(vals, f.value)
		}
		if i == 0 {
			where = w
		} else {
			where = fmt.Sprintf("%s %s %s", where, logic, w)
		}

	}

	if len(fs.atomFilters) > 0 {
		where = fmt.Sprintf("(%s)", where)
	}

	if fs.or {
		logic = "OR"
	}
	for _, c := range fs.combinedFilters {
		if c == nil {
			continue
		}
		w, vs, err := this.BuildWhereClause(c)
		if err != nil {
			return "", nil, err
		}
		if where == "" {
			where = fmt.Sprintf("%s %s", where, w)
		} else {
			where = fmt.Sprintf("%s %s %s", where, logic, w)
		}
		vals = append(vals, vs...)
	}

	if len(where) > 0 {
		return "(" + where + ")", vals, nil
	} else {
		return "", vals, nil
	}
}

func (this *Service) convertGo2SqlType(v interface{}) interface{} {
	switch reflect.TypeOf(v).Kind() {
	case reflect.Float64, reflect.Float32:
		return &sql.NullFloat64{}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &sql.NullInt64{}
	case reflect.Bool:
		return &sql.NullBool{}
	case reflect.String:
		return &sql.NullString{}
	default:
		return v
	}
}

func (this *Service) convertSql2GoValue(v interface{}, kind reflect.Kind) interface{} {
	switch kind {
	case reflect.Float64, reflect.Float32:
		return v.(*sql.NullFloat64).Float64
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.(*sql.NullInt64).Int64
	case reflect.Bool:
		return v.(*sql.NullBool).Bool
	case reflect.String:
		return v.(*sql.NullString).String
	default:
		return v
	}
}

func (this *Service) buildDimValues(o *object, dims []string) []interface{} {
	var ret []interface{}
	v := reflect.ValueOf(hruntime.CloneObject(o.instance))
	if dims == nil || len(dims) == 0 {
		for i := 0; i < v.Elem().NumField(); i++ {
			k := v.Elem().Field(i).Kind()
			if k == reflect.Map || k == reflect.Slice || k == reflect.Struct || k == reflect.Ptr {
				continue
			}
			f := this.convertGo2SqlType(v.Elem().Field(i).Interface())
			ret = append(ret, f)
		}
	} else {
		for _, s := range dims {
			k := v.Elem().FieldByName(o.fieldMap[s].name).Kind()
			if k == reflect.Map || k == reflect.Slice || k == reflect.Struct || k == reflect.Ptr {
				continue
			}
			f := this.convertGo2SqlType(v.Elem().FieldByName(o.fieldMap[s].name).Interface())
			ret = append(ret, f)
		}
	}

	return ret
}

func (this *Service) bindDimValues(o *object, dims []string, values []interface{}) map[string]interface{} {
	ret := make(map[string]interface{})
	for i, d := range dims {
		if o.fieldMap[d].kind == reflect.Struct || o.fieldMap[d].kind == reflect.Slice {
			continue
		}
		ret[d] = this.convertSql2GoValue(values[i], o.fieldMap[d].kind)
		//if o.fieldMap[d].kind == reflect.String {
		//	ret[d] = string(reflect.ValueOf(values[i]).Elem().Interface().([]byte))
		//} else {
		//	ret[d] = reflect.ValueOf(values[i]).Elem().Interface()
		//}
	}
	return ret
}

func (this *Service) GetObject(key string) *object {
	return this.objectsWithKey[key]
}

func (this *Service) ParserRawFilters(o *object, fs *RawFilters) (*Filters, *herrors.Error) {
	var fls *Filters
	if len(fs.Conditions) > 0 {
		fls = &Filters{
			or: fs.Or,
		}

		for _, f := range fs.Conditions {
			ss, err := this.parseCondition(f)
			if err != nil {
				return nil, err
			}
			af, err := this.convCondition2Filter(o, ss)
			if err != nil {
				return nil, err
			}
			fls.atomFilters = append(fls.atomFilters, af)
		}

		for _, rcf := range fs.SubFilters {
			v, err := this.ParserRawFilters(o, &rcf)
			if v != nil {
				fls.combinedFilters = append(fls.combinedFilters, v)
			} else {
				return nil, err
			}
		}
	}
	return fls, nil
}

//func (this *Service) handleGormField(fields map[string]bool, f *reflect.StructField, kv map[string]string, of *field) *field {
//	if kv["-"] != "" {
//		return nil
//	}
//
//	many2many := kv["many2many"]
//	if many2many != "" {
//		if f.Type.Kind() != reflect.Slice {
//			panic(herrors.ErrSysInternal.New("invalid many2many tag:", f.Type.Elem().Name()))
//		}
//
//		k := f.Type.Elem().Kind()
//		if k != reflect.Struct {
//			panic(herrors.ErrSysInternal.New("related object is not data source:", f.Type.Elem().Name()))
//		}
//
//		ins := this.instancesWithName[f.Type.Elem().Name()]
//		if ins == nil {
//			panic(herrors.ErrSysInternal.New("related object is not added to GormPlugin:", f.Type.Name()))
//		}
//
//		of.relation = &relationship{
//			relType:        relTypeToMany,
//			relatedObject:  ins,
//			relatedObjName: hruntime.GetObjectName(ins),
//			relatedField:   "ID",
//			selfField:      "ID",
//			relTable:       many2many,
//		}
//	} else {
//		fk := kv["foreignkey"]
//		rk := kv["references"]
//		if rk == "" {
//			rk = "ID"
//		}
//
//		if fk != "" {
//			if f.Type.Kind() == reflect.Struct {
//				ins := this.instancesWithName[f.Type.Name()]
//				if ins == nil {
//					panic(herrors.ErrSysInternal.New("related object is not added to GormPlugin:", f.Type.Name()))
//					return nil
//				}
//
//				if fields[rk] && !fields[fk] {
//					of.relation = &relationship{
//						relType:        relTypeHasOne,
//						relatedObject:  ins,
//						relatedObjName: hruntime.GetObjectName(ins),
//						relatedField:   fk,
//						selfField:      rk,
//					}
//				} else if fields[fk] {
//					of.relation = &relationship{
//						relType:        relTypeBelongTo,
//						relatedObject:  ins,
//						relatedObjName: hruntime.GetObjectName(ins),
//						relatedField:   fk,
//						selfField:      rk,
//					}
//				}
//				return of
//			}
//
//			if f.Type.Kind() == reflect.Slice {
//				k := f.Type.Elem().Kind()
//				if k != reflect.Struct {
//					panic(herrors.ErrSysInternal.New("related object is not data source:", f.Type.Elem().Name()))
//				}
//				ins := this.instancesWithName[f.Type.Elem().Name()]
//				if ins == nil {
//					panic(herrors.ErrSysInternal.New("related object is not added to GormPlugin:", f.Type.Name()))
//				}
//
//				if fields[rk] && !fields[fk] {
//					of.relation = &relationship{
//						relType:        relTypeHasMany,
//						relatedObject:  ins,
//						relatedObjName: hruntime.GetObjectName(ins),
//						relatedField:   fk,
//						selfField:      rk,
//					}
//					return of
//				}
//			}
//
//			panic(herrors.ErrSysInternal.New("gorm foreignkey not assigned to a struct:", f.Name))
//			return nil
//		}
//	}
//
//	return of
//}

func (this *Service) formatGormTags(kv map[string]string) map[string]string {
	tags := make(map[string]string)
	for k, v := range kv {
		k2 := strings.ToLower(k)
		tags[k2] = v
	}
	return tags
}

func (this *Service) parseCondition(s string) ([]string, *herrors.Error) {
	ss := strings.ToUpper(s)
	var res []string

	i := strings.Index(ss, ">=")
	if i >= 0 {
		res = []string{strings.TrimSpace(s[:i]), strings.TrimSpace(s[i : i+2]), strings.TrimSpace(s[i+2:])}
		goto out
	}

	i = strings.Index(ss, "<=")
	if i >= 0 {
		res = []string{strings.TrimSpace(s[:i]), strings.TrimSpace(s[i : i+2]), strings.TrimSpace(s[i+2:])}
		goto out
	}

	i = strings.Index(ss, "!=")
	if i >= 0 {
		res = []string{strings.TrimSpace(s[:i]), strings.TrimSpace(s[i : i+2]), strings.TrimSpace(s[i+2:])}
		goto out
	}

	i = strings.Index(ss, "<")
	if i >= 0 {
		res = []string{strings.TrimSpace(s[:i]), strings.TrimSpace(s[i : i+1]), strings.TrimSpace(s[i+1:])}
		goto out
	}

	i = strings.Index(ss, ">")
	if i >= 0 {
		res = []string{strings.TrimSpace(s[:i]), strings.TrimSpace(s[i : i+1]), strings.TrimSpace(s[i+1:])}
		goto out
	}

	i = strings.Index(ss, "==")
	if i >= 0 {
		res = []string{strings.TrimSpace(s[:i]), "=", strings.TrimSpace(s[i+2:])}
		goto out
	}

	if i = strings.Index(ss, " NOT IN "); i >= 0 {
		res = []string{strings.TrimSpace(s[:i]), strings.TrimSpace(s[i : i+8]), strings.TrimSpace(s[i+8:])}
		goto out
	} else if i = strings.Index(ss, " IN "); i >= 0 {
		res = []string{strings.TrimSpace(s[:i]), strings.TrimSpace(s[i : i+4]), strings.TrimSpace(s[i+4:])}
		goto out
	}

	if i = strings.Index(ss, " NOT BETWEEN "); i >= 0 {
		res = []string{strings.TrimSpace(s[:i]), strings.TrimSpace(s[i : i+13]), strings.TrimSpace(s[i+13:])}
		goto out
	} else if i = strings.Index(ss, " BETWEEN "); i >= 0 {
		res = []string{strings.TrimSpace(s[:i]), strings.TrimSpace(s[i : i+9]), strings.TrimSpace(s[i+9:])}
		goto out
	}

	if i = strings.Index(ss, " NOT LIKE "); i >= 0 {
		res = []string{strings.TrimSpace(s[:i]), strings.TrimSpace(s[i : i+10]), strings.TrimSpace(s[i+10:])}
		goto out
	} else if i = strings.Index(ss, " LIKE "); i >= 0 {
		res = []string{strings.TrimSpace(s[:i]), strings.TrimSpace(s[i : i+6]), strings.TrimSpace(s[i+6:])}
		goto out
	}

	if i = strings.Index(ss, " NOT HAS_PREFIX "); i >= 0 {
		res = []string{strings.TrimSpace(s[:i]), strings.TrimSpace(s[i : i+16]), strings.TrimSpace(s[i+16:])}
		goto out
	} else if i = strings.Index(ss, " HAS_PREFIX "); i >= 0 {
		res = []string{strings.TrimSpace(s[:i]), strings.TrimSpace(s[i : i+12]), strings.TrimSpace(s[i+12:])}
		goto out
	}

	if i = strings.Index(ss, " NOT HAS_SUFFIX "); i >= 0 {
		res = []string{strings.TrimSpace(s[:i]), strings.TrimSpace(s[i : i+16]), strings.TrimSpace(s[i+16:])}
		goto out
	} else if i = strings.Index(ss, " HAS_SUFFIX "); i >= 0 {
		res = []string{strings.TrimSpace(s[:i]), strings.TrimSpace(s[i : i+12]), strings.TrimSpace(s[i+12:])}
		goto out
	}

	if i = strings.Index(ss, " NOT IS "); i >= 0 {
		res = []string{strings.TrimSpace(s[:i]), strings.TrimSpace(s[i : i+8]), strings.TrimSpace(s[i+8:])}
		goto out
	} else if i = strings.Index(ss, " IS "); i >= 0 {
		res = []string{strings.TrimSpace(s[:i]), strings.TrimSpace(s[i : i+4]), strings.TrimSpace(s[i+4:])}
		goto out
	}

	return nil, herrors.ErrSysInternal.New("invalid condition operator in condition parameter [%s]", s)

out:
	if len(res) != 3 {
		return nil, herrors.ErrSysInternal.New("invalid condition format %s", s)
	}

	return res, nil
}

func (this *Service) convCondition2Filter(o *object, cons []string) (*atomFilter, *herrors.Error) {
	if len(cons) != 3 {
		return nil, herrors.ErrCallerInvalidRequest.New("invalid condition [%s]", strings.Join(cons, ""))
	}

	v := o.fieldMap[cons[0]]
	if v != nil {
		af := &atomFilter{
			field:   v,
			compare: cons[1],
		}
		if v.kind == reflect.Bool {
			if strings.ToLower(strings.TrimSpace(cons[2])) == "true" {
				af.value = true
			} else {
				af.value = false
			}
		} else {
			af.value = cons[2]
		}
		return af, nil
	} else {
		return nil, herrors.ErrCallerInvalidRequest.New("invalid field [%s]", cons[0])
	}

}

func (this *Service) mountHookCallers(args ...interface{}) {
	for i := 0; i < len(args); i++ {
		hookCallers := args[0].([]htypes.Any)
		for _, h := range hookCallers {
			this.mountAfterDelHook(h)
			this.mountBeforeDelHook(h)
			this.mountAfterQueryHook(h)
			this.mountBeforeQueryHook(h)
			this.mountAfterCreateHook(h)
			this.mountBeforeCreateHook(h)
			this.mountAfterUpdateHook(h)
			this.mountBeforeUpdateHook(h)
		}
	}
}

func (this *Service) mountBeforeCreateHook(anchor htypes.Any) {
	this.beforeCreateHookCallers = make(map[string]*core.MethodCaller)
	typ := reflect.TypeOf(anchor)
	val := reflect.ValueOf(anchor)
	n := val.NumMethod()
	for i := 0; i < n; i++ {
		method := typ.Method(i)
		mtype := method.Type
		if method.PkgPath != "" {
			continue
		}

		if mtype.NumOut() != 0 {
			continue
		}

		if mtype.NumIn() != 5 {
			continue
		}

		ctxType := mtype.In(1)
		if !ctxType.Implements(reflect.TypeOf((*core.IService)(nil)).Elem()) {
			continue
		}

		ctxType = mtype.In(2)
		if ctxType.Name() != "Map" {
			continue
		}

		ctxType = mtype.In(3)
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Name() != "CallerResponse" {
			continue
		}

		ctxType = mtype.In(4)
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Kind() != reflect.Bool {
			continue
		}

		this.beforeCreateHookCallers[method.Name] = &core.MethodCaller{val, method.Func}
	}
}

func (this *Service) mountAfterCreateHook(anchor htypes.Any) {
	this.afterCreateHookCallers = make(map[string]*core.MethodCaller)
	typ := reflect.TypeOf(anchor)
	val := reflect.ValueOf(anchor)
	n := val.NumMethod()
	for i := 0; i < n; i++ {
		method := typ.Method(i)
		mtype := method.Type
		if method.PkgPath != "" {
			continue
		}

		if mtype.NumOut() != 0 {
			continue
		}

		if mtype.NumIn() != 4 {
			continue
		}

		ctxType := mtype.In(1)
		if !ctxType.Implements(reflect.TypeOf((*core.IService)(nil)).Elem()) {
			continue
		}

		ctxType = mtype.In(2)
		if ctxType.Name() != "Map" {
			continue
		}

		ctxType = mtype.In(3)
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Name() != "CallerResponse" {
			continue
		}

		this.afterCreateHookCallers[method.Name] = &core.MethodCaller{val, method.Func}
	}
}

func (this *Service) mountBeforeUpdateHook(anchor htypes.Any) {
	this.beforeUpdateHookCallers = make(map[string]*core.MethodCaller)
	typ := reflect.TypeOf(anchor)
	val := reflect.ValueOf(anchor)
	n := val.NumMethod()
	for i := 0; i < n; i++ {
		method := typ.Method(i)
		mtype := method.Type
		if method.PkgPath != "" {
			continue
		}

		if mtype.NumOut() != 0 {
			continue
		}

		if mtype.NumIn() != 6 {
			continue
		}

		ctxType := mtype.In(1)
		if !ctxType.Implements(reflect.TypeOf((*core.IService)(nil)).Elem()) {
			continue
		}

		ctxType = mtype.In(2)
		if ctxType.Kind() != reflect.Int64 {
			continue
		}

		ctxType = mtype.In(3)
		if ctxType.Name() != "Map" {
			continue
		}

		ctxType = mtype.In(4)
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Name() != "CallerResponse" {
			continue
		}

		ctxType = mtype.In(5)
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Kind() != reflect.Bool {
			continue
		}

		this.beforeUpdateHookCallers[method.Name] = &core.MethodCaller{val, method.Func}
	}
}

func (this *Service) mountAfterUpdateHook(anchor htypes.Any) {
	this.afterUpdateHookCallers = make(map[string]*core.MethodCaller)
	typ := reflect.TypeOf(anchor)
	val := reflect.ValueOf(anchor)
	n := val.NumMethod()
	for i := 0; i < n; i++ {
		method := typ.Method(i)
		mtype := method.Type
		if method.PkgPath != "" {
			continue
		}

		if mtype.NumOut() != 0 {
			continue
		}

		if mtype.NumIn() != 5 {
			continue
		}

		ctxType := mtype.In(1)
		if !ctxType.Implements(reflect.TypeOf((*core.IService)(nil)).Elem()) {
			continue
		}

		ctxType = mtype.In(2)
		if ctxType.Kind() != reflect.Int64 {
			continue
		}

		ctxType = mtype.In(3)
		if ctxType.Name() != "Map" {
			continue
		}

		ctxType = mtype.In(4)
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Name() != "CallerResponse" {
			continue
		}

		this.afterUpdateHookCallers[method.Name] = &core.MethodCaller{val, method.Func}
	}
}

func (this *Service) mountBeforeDelHook(anchor htypes.Any) {
	this.beforeDelHookCallers = make(map[string]*core.MethodCaller)
	typ := reflect.TypeOf(anchor)
	val := reflect.ValueOf(anchor)
	n := val.NumMethod()
	for i := 0; i < n; i++ {
		method := typ.Method(i)
		mtype := method.Type
		if method.PkgPath != "" {
			continue
		}

		if mtype.NumOut() != 0 {
			continue
		}

		if mtype.NumIn() != 5 {
			continue
		}

		ctxType := mtype.In(1)
		if !ctxType.Implements(reflect.TypeOf((*core.IService)(nil)).Elem()) {
			continue
		}

		ctxType = mtype.In(2)
		if ctxType.Kind() != reflect.Slice {
			continue
		}

		ctxType = mtype.In(3)
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Name() != "CallerResponse" {
			continue
		}

		ctxType = mtype.In(4)
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Kind() != reflect.Bool {
			continue
		}

		this.beforeDelHookCallers[method.Name] = &core.MethodCaller{val, method.Func}
	}
}

func (this *Service) mountAfterDelHook(anchor htypes.Any) {
	this.afterDelHookCallers = make(map[string]*core.MethodCaller)
	typ := reflect.TypeOf(anchor)
	val := reflect.ValueOf(anchor)
	n := val.NumMethod()
	for i := 0; i < n; i++ {
		method := typ.Method(i)
		mtype := method.Type
		if method.PkgPath != "" {
			continue
		}

		if mtype.NumOut() != 0 {
			continue
		}

		if mtype.NumIn() != 4 {
			continue
		}

		ctxType := mtype.In(1)
		if !ctxType.Implements(reflect.TypeOf((*core.IService)(nil)).Elem()) {
			continue
		}

		ctxType = mtype.In(2)
		if ctxType.Kind() != reflect.Slice {
			continue
		}

		ctxType = mtype.In(3)
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Name() != "CallerResponse" {
			continue
		}

		this.afterDelHookCallers[method.Name] = &core.MethodCaller{val, method.Func}
	}
}

func (this *Service) mountBeforeQueryHook(anchor htypes.Any) {
	this.beforeQueryHookCallers = make(map[string]*core.MethodCaller)
	typ := reflect.TypeOf(anchor)
	val := reflect.ValueOf(anchor)
	n := val.NumMethod()
	for i := 0; i < n; i++ {
		method := typ.Method(i)
		mtype := method.Type
		if method.PkgPath != "" {
			continue
		}

		if mtype.NumOut() != 0 {
			continue
		}

		if mtype.NumIn() != 5 {
			continue
		}

		ctxType := mtype.In(1)
		if !ctxType.Implements(reflect.TypeOf((*core.IService)(nil)).Elem()) {
			continue
		}

		ctxType = mtype.In(2)
		if ctxType.Name() != "Map" {
			continue
		}

		ctxType = mtype.In(3)
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Name() != "CallerResponse" {
			continue
		}

		ctxType = mtype.In(4)
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Kind() != reflect.Bool {
			continue
		}

		this.beforeQueryHookCallers[method.Name] = &core.MethodCaller{val, method.Func}
	}
}

func (this *Service) mountAfterQueryHook(anchor htypes.Any) {
	this.afterQueryHookCallers = make(map[string]*core.MethodCaller)
	typ := reflect.TypeOf(anchor)
	val := reflect.ValueOf(anchor)
	n := val.NumMethod()
	for i := 0; i < n; i++ {
		method := typ.Method(i)
		mtype := method.Type
		if method.PkgPath != "" {
			continue
		}

		if mtype.NumOut() != 0 {
			continue
		}

		if mtype.NumIn() != 5 {
			continue
		}

		ctxType := mtype.In(1)
		if !ctxType.Implements(reflect.TypeOf((*core.IService)(nil)).Elem()) {
			continue
		}

		// parameters
		ctxType = mtype.In(2)
		if ctxType.Name() != "Map" {
			continue
		}

		//records
		ctxType = mtype.In(3)
		if ctxType.Kind() != reflect.Slice {
			continue
		}

		ctxType = mtype.In(4)
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Name() != "CallerResponse" {
			continue
		}

		this.afterQueryHookCallers[method.Name] = &core.MethodCaller{val, method.Func}
	}
}

func (this *Service) callBeforeCreateHook(name string, ps htypes.Map, reply *core.CallerResponse, stop *bool) {
	if !hconf.IsDebug() {
		defer func() {
			e := recover()
			if e != nil {
				reply.Error = herrors.ErrSysInternal.New(e.(error).Error())
			}
		}()
	}

	caller := this.beforeCreateHookCallers[name]
	if caller == nil {
		reply.Error = herrors.ErrSysInternal.New("before create hook %s not found", name)
		return
	}

	caller.Handler.Call([]reflect.Value{caller.Object, reflect.ValueOf(this), reflect.ValueOf(ps), reflect.ValueOf(reply), reflect.ValueOf(stop)})
}

func (this *Service) callBeforeUpdateHook(name string, id int64, ps htypes.Map, reply *core.CallerResponse, stop *bool) {
	if !hconf.IsDebug() {
		defer func() {
			e := recover()
			if e != nil {
				reply.Error = herrors.ErrSysInternal.New(e.(error).Error())
			}
		}()
	}

	caller := this.beforeUpdateHookCallers[name]
	if caller == nil {
		reply.Error = herrors.ErrSysInternal.New("before create hook %s not found", name)
		return
	}

	caller.Handler.Call([]reflect.Value{caller.Object, reflect.ValueOf(this), reflect.ValueOf(id), reflect.ValueOf(ps), reflect.ValueOf(reply), reflect.ValueOf(stop)})
}

func (this *Service) callBeforeQueryHook(name string, ps htypes.Map, reply *core.CallerResponse, stop *bool) {
	if !hconf.IsDebug() {
		defer func() {
			e := recover()
			if e != nil {
				reply.Error = herrors.ErrSysInternal.New(e.(error).Error())
			}
		}()
	}

	caller := this.beforeQueryHookCallers[name]
	if caller == nil {
		reply.Error = herrors.ErrSysInternal.New("before query hook %s not found", name)
		return
	}

	caller.Handler.Call([]reflect.Value{caller.Object, reflect.ValueOf(this), reflect.ValueOf(ps), reflect.ValueOf(reply), reflect.ValueOf(stop)})
}

func (this *Service) callBeforeDelHook(name string, ids []int64, reply *core.CallerResponse, stop *bool) {
	if !hconf.IsDebug() {
		defer func() {
			e := recover()
			if e != nil {
				reply.Error = herrors.ErrSysInternal.New(e.(error).Error())
			}
		}()
	}

	caller := this.beforeDelHookCallers[name]
	if caller == nil {
		reply.Error = herrors.ErrSysInternal.New("before del hook %s not found", name)
		return
	}

	caller.Handler.Call([]reflect.Value{caller.Object, reflect.ValueOf(this), reflect.ValueOf(ids), reflect.ValueOf(reply), reflect.ValueOf(stop)})
}

func (this *Service) callAfterCreateHook(name string, ps htypes.Map, reply *core.CallerResponse) {
	if !hconf.IsDebug() {
		defer func() {
			e := recover()
			if e != nil {
				reply.Error = herrors.ErrSysInternal.New(e.(error).Error())
			}
		}()
	}

	caller := this.afterCreateHookCallers[name]
	if caller == nil {
		reply.Error = herrors.ErrSysInternal.New("after create hook %s not found", name)
		return
	}

	caller.Handler.Call([]reflect.Value{caller.Object, reflect.ValueOf(this), reflect.ValueOf(ps), reflect.ValueOf(reply)})
}

func (this *Service) callAfterUpdateHook(name string, id int64, ps htypes.Map, reply *core.CallerResponse) {
	if !hconf.IsDebug() {
		defer func() {
			e := recover()
			if e != nil {
				reply.Error = herrors.ErrSysInternal.New(e.(error).Error())
			}
		}()
	}

	caller := this.afterUpdateHookCallers[name]
	if caller == nil {
		reply.Error = herrors.ErrSysInternal.New("after create hook [%s] not found", name)
		return
	}

	caller.Handler.Call([]reflect.Value{caller.Object, reflect.ValueOf(this), reflect.ValueOf(id), reflect.ValueOf(ps), reflect.ValueOf(reply)})
}

func (this *Service) callAfterQueryHook(name string, ps htypes.Map, records []interface{}, reply *core.CallerResponse) {
	if !hconf.IsDebug() {
		defer func() {
			e := recover()
			if e != nil {
				reply.Error = herrors.ErrSysInternal.New(e.(error).Error())
			}
		}()
	}

	caller := this.afterQueryHookCallers[name]
	if caller == nil {
		reply.Error = herrors.ErrSysInternal.New("after query hook %s not found", name)
		return
	}

	caller.Handler.Call([]reflect.Value{caller.Object, reflect.ValueOf(this), reflect.ValueOf(ps), reflect.ValueOf(records), reflect.ValueOf(reply)})
}

func (this *Service) callAfterDelHook(name string, ids []int64, reply *core.CallerResponse) {
	if !hconf.IsDebug() {
		defer func() {
			e := recover()
			if e != nil {
				reply.Error = herrors.ErrSysInternal.New(e.(error).Error())
			}
		}()
	}

	caller := this.afterDelHookCallers[name]
	if caller == nil {
		reply.Error = herrors.ErrSysInternal.New("after del hook %s not found", name)
		return
	}

	caller.Handler.Call([]reflect.Value{caller.Object, reflect.ValueOf(this), reflect.ValueOf(ids), reflect.ValueOf(reply)})
}

func (this *Service) autoFill(funName string, ps htypes.Map) interface{} {
	switch funName {
	case "$now":
		return hdatetime.Now()
		//case "$user":
		//	return ps[core.VarUser]
	}
	return nil
}

func (this *Service) getObjectFieldValue(obj interface{}, f string) interface{} {
	o := reflect.ValueOf(obj)
	if o.Kind() != reflect.Ptr || o.Elem().Kind() != reflect.Struct {
		return herrors.ErrSysInternal.New("parameter object should be a pointer to struct")
	}
	var val reflect.Value
	if reflect.TypeOf(obj).Kind() == reflect.Ptr {
		val = reflect.ValueOf(obj).Elem().FieldByName(f)
	} else {
		val = reflect.ValueOf(obj).FieldByName(f)
	}
	if !val.IsValid() { //不属于该该对象的属性值
		return nil
	}
	switch val.Kind() {
	case reflect.Float64, reflect.Float32:
		return val.Float()
	case reflect.Uint, reflect.Uint16, reflect.Uint8, reflect.Uint32, reflect.Uint64:
		return val.Uint()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return val.Int()
	case reflect.String:
		return val.String()
		break
	case reflect.Bool:
		return val.Bool()
	}
	return nil
}

func (this *Service) parseStringArray(s string) ([]string, *herrors.Error) {
	if len(s) < 2 {
		return nil, herrors.ErrSysInternal.New("failed to parse StringArray :%s", s)
	}
	ss := strings.Split(s[1:len(s)-1], ",")
	if len(ss) != 2 {
		return nil, herrors.ErrSysInternal.New("failed to parse StringArray :%s", s)
	}

	return []string{
		fmt.Sprintf("%s", strings.TrimSpace(ss[0])),
		fmt.Sprintf("%s", strings.TrimSpace(ss[1])),
	}, nil

}

func (this *Service) parseObject(o interface{}) *object {
	obj := &object{}
	obj.instance = o
	obj.name = hruntime.GetObjectName(o)

	manageable := false
	t := reflect.TypeOf(o)
	fields := make(map[string]bool)
	for i := 0; i < t.NumField(); i++ {
		fields[t.Field(i).Name] = true
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("data")
		if f.Type.Kind() == reflect.Struct && f.Type.Name() == "DataSource" {
			obj.deniedOperations = make(map[string]bool)
			manageable = true
			if tag == "" {
				obj.key = gorm.ToTableName(hruntime.GetObjectName(o))
			} else {
				kv := hruntime.ParseTag(tag)
				for k, v := range kv {
					if k == "key" {
						obj.key = v
						break
					} else if v == "" {
						obj.key = k
						break
					}
				}

				for k, v := range kv {
					switch k {
					case "desc":
						obj.desc = v
					case "deny":
						for _, s := range strings.Split(v, ",") {
							obj.deniedOperations[s] = true
						}
					case "afterUpdate":
						this.afterUpdateHookNames[obj.key] = v
					case "beforeUpdate":
						this.beforeUpdateHookNames[obj.key] = v
					case "beforeCreate":
						this.beforeCreateHookNames[obj.key] = v
					case "afterCreate":
						this.afterCreateHookNames[obj.key] = v
					case "beforeQuery":
						this.beforeQueryHookNames[obj.key] = v
					case "afterQuery":
						this.afterQueryHookNames[obj.key] = v
					case "beforeDel":
						this.beforeDelHookNames[obj.key] = v
					case "afterDel":
						this.afterDelHookNames[obj.key] = v
					}
				}
			}
			continue
		}

		if tag == "-" {
			continue
		}
		of := newField()

		of.kind = f.Type.Kind()
		if of.kind == reflect.Struct {
			of.kind = reflect.Map
		}
		of.name = f.Name

		if tag != "" {
			kv := hruntime.ParseTag(tag)
			for k, v := range kv {
				switch k {
				case "primary":
					obj.primaryField = of
				case "desc":
					of.desc = v
				case "require":
					for _, v := range strings.Split(v, ",") {
						switch strings.TrimSpace(v) {
						case opCreate:
							of.opRequired[opCreate] = true
						case opUpdate:
							of.opRequired[opUpdate] = true
						}
					}
				case "deny":
					for _, v := range strings.Split(v, ",") {
						switch strings.TrimSpace(v) {
						case opCreate:
							of.opDenies[opCreate] = true
						case opUpdate:
							of.opDenies[opUpdate] = true
						case opQuery:
							of.opDenies[opQuery] = true
						}
					}
				case opCreate:
					for _, v := range strings.Split(v, ",") {
						switch strings.TrimSpace(v) {
						case "deny":
							of.opDenies[opCreate] = true
						case "require":
							of.opRequired[opCreate] = true
						}
					}
				case opQuery:
					for _, v := range strings.Split(v, ",") {
						switch strings.TrimSpace(v) {
						case "deny":
							of.opDenies[opQuery] = true
						case "require":
							of.opRequired[opQuery] = true
						}
					}
				case opUpdate:
					for _, v := range strings.Split(v, ",") {
						switch strings.TrimSpace(v) {
						case "deny":
							of.opDenies[opUpdate] = true
						case "require":
							of.opRequired[opUpdate] = true
						}
					}
				case "key":
					of.key = v
				case "autoInit":
					of.autoInitFunc = v
				case "autoFill":
					of.autoFillFunc = v
				case "customFunc":
					of.customFunc = v
				case "":
					of.key = k
				}
			}
		}

		if of.key == "" {
			of.key = gorm.ToColumnName(of.name)
		}

		//t := f.Tag.Get("gorm")
		//if t != "" {
		//	kv := this.formatGormTags(hruntime.ParseTag(t))
		//	of = this.handleGormField(fields, &f, kv, of)
		//}
		if of != nil {
			obj.AddField(of)
		}
	}

	if manageable {
		obj.fieldKeysByName = make(map[string]string)
		for _, f := range obj.fieldMap {
			obj.fieldKeysByName[f.name] = f.key
		}
		return obj
	}
	return nil
}

func (this *Service) shapeObjectFieldValues(op string, objName string, vals map[string]interface{}) (map[string]interface{}, *herrors.Error) {
	obj := this.objectsWithName[objName]
	if obj == nil {
		return nil, herrors.ErrSysInternal.New("object is not declared as DataSource:", objName)
	}

	vs := make(map[string]interface{})
	for k, of := range obj.fieldMap {
		if of.autoInitFunc != "" {
			if vals["id"] == nil {
				vs[of.name] = this.autoFill(of.autoInitFunc, vals)
				continue
			}
		} else if of.autoFillFunc != "" {
			vs[of.name] = this.autoFill(of.autoFillFunc, vals)
			continue
		} else if of.customFunc != "" {
			ff := this.fieldFuncMap[of.customFunc[1:]]
			if ff == nil {
				return nil, herrors.ErrSysInternal.New("custom field [%s] not found", of.customFunc)
			}
			vv, err := ff(vals[k])
			if err != nil {
				return nil, err
			}
			vs[of.name] = vv
		}
		if vals[k] == nil {
			if of.opRequired[op] {
				return nil, herrors.ErrSysInternal.New("object field [%s] required:", of.name)
			}
		} else {
			v := vals[k]
			k := reflect.TypeOf(v).Kind()
			if k == reflect.Struct || k == reflect.Map || k == reflect.Slice {
				continue
			}
			if (k == reflect.Float64 || k == reflect.Float32 ||
				k == reflect.Uint || k == reflect.Uint64 ||
				k == reflect.Uint32 || k == reflect.Uint16 ||
				k == reflect.Uint8 || k == reflect.Int ||
				k == reflect.Int8 || k == reflect.Int16 ||
				k == reflect.Int32 || k == reflect.Int64) &&
				(of.kind == reflect.Float64 || of.kind == reflect.Float32 ||
					of.kind == reflect.Uint || of.kind == reflect.Uint64 ||
					of.kind == reflect.Uint32 || of.kind == reflect.Uint16 ||
					of.kind == reflect.Uint8 || of.kind == reflect.Int ||
					of.kind == reflect.Int8 || of.kind == reflect.Int16 ||
					of.kind == reflect.Int32 || of.kind == reflect.Int64) {
				vs[of.name] = v
			} else if of.kind != k {
				return nil, herrors.ErrSysInternal.New("object field [%s.%s] invalid kind", of.name, of.key)
			} else {
				vs[of.name] = v
			}
		}
	}

	return vs, nil
}

func (this *Service) findObjectFieldKey(objName string, fieldName string) (string, *herrors.Error) {
	obj := this.objectsWithName[objName]
	if obj == nil {
		return "", herrors.ErrSysInternal.New("object [%s] is not tagged as DataSource", objName)
	}

	f := obj.fieldKeysByName[fieldName]
	if f == "" {
		return "", herrors.ErrSysInternal.New("object field is not tagged as DataSource:%s.%s", objName, fieldName)
	}
	return f, nil
}
