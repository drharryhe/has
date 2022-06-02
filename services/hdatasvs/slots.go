package hdatasvs

import (
	"fmt"
	"reflect"
	"strings"

	"gorm.io/gorm"

	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
	"github.com/drharryhe/has/utils/hruntime"
)

type CreateRequest struct {
	core.SlotRequestBase

	Key    *string     `json:"key" param:"require"`
	Object *htypes.Map `json:"object" param:"require"`
}

func (this *Service) Create(req *CreateRequest, res *core.SlotResponse) {
	o := this.objectsByKey[*req.Key]
	if o == nil {
		this.Response(res, nil, herrors.ErrSysInternal.New("key [%s] not found", *req.Key).D("failed to create data"))
		return
	}

	tabName, herr := this.CheckTableName(o, *req.Object, true)
	if herr != nil {
		this.Response(res, nil, herr)
		return
	}

	if o.deniedOperations[opCreate] {
		this.Response(res, nil, herrors.ErrUserUnauthorizedAct.New("object [%s] cannot be created", *req.Key).D("failed to create data"))
		return
	}

	for k := range *req.Object {
		f := o.fieldMap[k]
		if f != nil {
			if f.opDenies[opCreate] {
				this.Response(res, nil, herrors.ErrUserUnauthorizedAct.New("field [%s] of object [%s] cannot be created", k, *req.Key).D("failed to create data"))
				return
			}
		}
	}

	vs, err := this.shapeObjectFieldValues(opCreate, o.name, *req.Object)
	if err != nil {
		this.Response(res, nil, err)
		return
	}

	if hook := this.beforeCreateHookNames[*req.Key]; hook != "" {
		reply := core.CallerResponse{}
		stop := false
		this.callBeforeCreateHook(hook, &CreateRequest{
			Key:    req.Key,
			Object: &vs,
		}, &reply, &stop)
		if reply.Error != nil && stop {
			this.Response(res, nil, reply.Error)
			return
		}
	}

	ins := hruntime.CloneObject(o.instance)
	if err := hruntime.SetObjectValues(ins, vs); err != nil {
		this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
		return
	}

	if err := this.getDB(o.database).Table(tabName).Create(ins).Error; err != nil {
		if strings.Index(err.Error(), "Error 1062") >= 0 {
			this.Response(res, nil, herrors.ErrCallerInvalidRequest.New("object duplicated"))
		} else {
			this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
		}
		return
	}

	vs[o.primaryField.name] = this.getObjectFieldValue(ins, o.primaryField.name)

	if hook := this.afterCreateHookNames[*req.Key]; hook != "" {
		reply := core.CallerResponse{}
		req.Object = &vs
		this.callAfterCreateHook(hook, req, &reply)
		this.Response(res, reply.Data, reply.Error)
	} else {
		this.Response(res, vs[o.primaryField.name], nil)
	}
}

type CreateMRequest struct {
	core.SlotRequestBase

	Key     *string       `json:"key" param:"require"`
	Objects *[]htypes.Map `json:"objects" param:"require"`
}

func (this *Service) CreateM(req *CreateMRequest, res *core.SlotResponse) {
	o := this.objectsByKey[*req.Key]
	if o == nil {
		this.Response(res, nil, herrors.ErrSysInternal.New("key [%s] not found", *req.Key).D("failed to create data"))
		return
	}

	if o.deniedOperations[opCreate] {
		this.Response(res, nil, herrors.ErrUserUnauthorizedAct.New("object [%s] cannot be created", *req.Key).D("failed to create data"))
		return
	}

	var vss []htypes.Map
	instancesByTabName := make(map[string][]htypes.Any)
	for _, vals := range *req.Objects {
		tabName, herr := this.CheckTableName(o, vals, true)
		if herr != nil {
			this.Response(res, nil, herr)
			return
		}

		vs, err := this.shapeObjectFieldValues(opCreate, o.name, vals)
		if err != nil {
			this.Response(res, nil, err)
			return
		}
		vss = append(vss, vs)

		ins := hruntime.CloneObject(o.instance)
		if err := hruntime.SetObjectValues(ins, vs); err != nil {
			this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
			return
		}
		instancesByTabName[tabName] = append(instancesByTabName[tabName], ins)
	}

	if hook := this.beforeCreateHookNames[*req.Key]; hook != "" {
		reply := core.CallerResponse{}
		stop := false
		for _, vs := range vss {
			this.callBeforeCreateHook(hook, &CreateRequest{
				Key:    req.Key,
				Object: &vs,
			}, &reply, &stop)
			if reply.Error != nil {
				this.Response(res, nil, reply.Error)
				return
			}
			if stop {
				this.Response(res, reply.Data, reply.Error)
				return
			}
		}
	}

	//批量创建记录
	var ids []htypes.Any
	for tab, instances := range instancesByTabName {
		if err := this.getDB(o.database).Table(tab).Create(instances).Error; err != nil {
			this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
			return
		}
		for _, ins := range instances {
			ids = append(ids, this.getObjectFieldValue(ins, o.primaryField.name))
		}
	}

	//调用钩子函数
	if hook := this.afterCreateHookNames[*req.Key]; hook != "" {
		reply := core.CallerResponse{}
		for _, vs := range vss {
			this.callAfterCreateHook(hook, &CreateRequest{
				Key:    req.Key,
				Object: &vs,
			}, &reply)
			if reply.Error != nil {
				this.Response(res, nil, reply.Error)
				return
			}
		}
	}

	this.Response(res, ids, nil)
}

type UpdateRequest struct {
	core.SlotRequestBase

	Key    *string     `json:"key" param:"require"`
	Filter *rawFilter  `json:"filter" param:"require"`
	Value  *htypes.Map `json:"value" param:"require"`

	Bucket htypes.Map
}

func (this *Service) Update(req *UpdateRequest, res *core.SlotResponse) {
	o := this.objectsByKey[*req.Key]
	if o == nil {
		this.Response(res, nil, herrors.ErrCallerInvalidRequest.New("key [%s] not found", *req.Key))
		return
	}
	if o.deniedOperations[opUpdate] {
		this.Response(res, nil, herrors.ErrCallerUnauthorizedAccess.New("object [%s] cannot be updated", *req.Key))
		return
	}

	//检查字段权限
	for k := range *req.Value {
		f := o.fieldMap[k]
		if f != nil {
			if f.opDenies[opUpdate] || o.primaryField.key == f.key {
				this.Response(res, nil, herrors.ErrCallerUnauthorizedAccess.New("field [%s] of object [%s] cannot be updated", k, *req.Key))
				return
			}
		}
	}

	//提取Update的值
	vs, err := this.shapeObjectFieldValues(opUpdate, o.name, *req.Value)
	if err != nil {
		this.Response(res, nil, err)
		return
	}
	if len(vs) == 0 {
		this.Response(res, nil, herrors.ErrCallerInvalidRequest.New("valid updated field values not found"))
		return
	} else {
		req.Value = &vs
	}

	//处理filters
	filters, herr := this.parseRawFilter(o.iFieldMap, req.Filter)
	if herr != nil {
		this.Response(res, nil, herr)
		return
	} else if filters == nil {
		this.Response(res, nil, herrors.ErrUserInvalidAct.New("filter required"))
		return
	}
	filtersSetValues := this.getFieldValuesSetByFilters(filters)
	if filtersSetValues == nil || filtersSetValues[o.key] == nil || filtersSetValues[o.key][o.primaryField.key] == nil {
		this.Response(res, nil, herrors.ErrSysInternal.New("primary field [%s] required", o.primaryField.key))
		return
	}

	tableName, herr := this.CheckTableName(o, filtersSetValues[o.key], false)
	if herr != nil {
		this.Response(res, nil, herr)
		return
	}

	//处理where子句
	where, vals, herr := this.buildWhereClause(filters)
	if herr != nil {
		this.Response(res, nil, herr)
		return
	}

	//调用钩子函数
	if hook := this.beforeUpdateHookNames[*req.Key]; hook != "" {
		reply := core.CallerResponse{}
		stop := false
		this.callBeforeUpdateHook(hook, req, &reply, &stop)
		if reply.Error != nil && stop {
			this.Response(res, nil, reply.Error)
			return
		}
	}

	//执行update
	values := make(map[string]interface{})
	for n, v := range vs {
		values[o.fieldMapByName[n].col] = v
	}
	if err := this.getDB(o.database).Table(fmt.Sprintf("`%s` AS `%s`", tableName, o.key)).Where(where, vals...).Updates(values).Error; err != nil {
		this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
		return
	}

	//调用钩子函数
	if hook := this.afterUpdateHookNames[*req.Key]; hook != "" {
		reply := core.CallerResponse{}
		this.callAfterUpdateHook(hook, req, &reply)
		this.Response(res, reply.Data, reply.Error)
	} else {
		this.Response(res, nil, nil)
	}
}

type DeleteRequest struct {
	core.SlotRequestBase

	Key     *string       `json:"key" param:"require"`
	Objects *[]htypes.Map `json:"objects" param:"require;type:ObjectArray"`
}

func (this *Service) Delete(req *DeleteRequest, res *core.SlotResponse) {
	o := this.objectsByKey[*req.Key]
	if o == nil {
		this.Response(res, nil, herrors.ErrSysInternal.New("key [%s] not found:", *req.Key))
		return
	}

	if o.deniedOperations[opDelete] {
		this.Response(res, nil, herrors.ErrCallerUnauthorizedAccess.New("object [%s] cannot be deleted", *req.Key))
		return
	}

	if hook := this.beforeDelHookNames[*req.Key]; hook != "" {
		reply := core.CallerResponse{}
		stop := false
		this.callBeforeDelHook(hook, req, &reply, &stop)
		if reply.Error != nil && stop {
			this.Response(res, nil, reply.Error)
			return
		}
	}

	instancesByTabName := make(map[string][]htypes.Map)
	for _, vals := range *req.Objects {
		tabName, herr := this.CheckTableName(o, vals, false)
		if herr != nil {
			this.Response(res, nil, herr)
			return
		}
		instancesByTabName[tabName] = append(instancesByTabName[tabName], vals)
	}

	db := this.getDB(o.database)
	ins := hruntime.CloneObject(o.instance)
	for tab, valsSlice := range instancesByTabName {
		var where []string
		var values []interface{}
		for _, vals := range valsSlice {
			for k, v := range vals {
				where = append(where, fmt.Sprintf("`%s` = ?", o.fieldMap[k].col))
				values = append(values, v)
			}
		}
		if err := db.Table(tab).Where(strings.Join(where, "AND"), values...).Delete(ins).Error; err != nil {
			this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
		}
	}

	if hook := this.afterDelHookNames[*req.Key]; hook != "" {
		reply := core.CallerResponse{}
		this.callAfterDelHook(hook, req, &reply)
		this.Response(res, reply.Data, reply.Error)
	} else {
		this.Response(res, nil, nil)
	}
}

type QueryRequest struct {
	core.SlotRequestBase

	Key      *string    `json:"key" param:"require"`
	Filter   *rawFilter `json:"filter" param:"require"`
	Dims     *[]string  `json:"dims" param:"require;type:StringArray"`
	Ordering *[]string  `json:"ordering" param:"type:StringArray"`
	Paging   *[]int     `json:"paging" param:"type:NumberRange"`

	Records []htypes.Any `param:"-"`
}

func (this *Service) Query(req *QueryRequest, res *core.SlotResponse) {
	o := this.objectsByKey[*req.Key]
	if o == nil {
		this.Response(res, nil, herrors.ErrSysInternal.New("key [%s] not found", *req.Key))
		return
	}
	if o.deniedOperations[opQuery] {
		this.Response(res, nil, herrors.ErrCallerUnauthorizedAccess.New("object [%s] cannot be queried", *req.Key))
		return
	}

	//处理where 子句
	filters, herr := this.parseRawFilter(o.iFieldMap, req.Filter)
	if herr != nil {
		this.Response(res, nil, herr)
		return
	}
	filtersSetValues := this.getFieldValuesSetByFilters(filters)

	tableName, herr := this.CheckTableName(o, filtersSetValues[o.key], false)
	if herr != nil {
		this.Response(res, nil, herr)
		return
	}

	//处理where子句
	var where string
	var vals []interface{}
	if filters != nil {
		where, vals, herr = this.buildWhereClause(filters)
		if herr != nil {
			this.Response(res, nil, herr)
			return
		}
	}

	//处理select 子句
	var dims []string
	var selectFieldNames []string

	for _, d := range *req.Dims {
		f := o.fieldMap[d]
		if f == nil {
			continue
		}
		if f.opDenies[opQuery] {
			this.Response(res, nil, herrors.ErrCallerUnauthorizedAccess.New("field [%s] of object [%s] cannot be queried", *req.Key, f.key))
			return
		}

		if f.kind == reflect.Map || f.kind == reflect.Ptr {
			continue
		}

		if f.kind == reflect.Slice || f.kind == reflect.Struct {
			continue
		} else {
			dims = append(dims, f.Key())
			selectFieldNames = append(selectFieldNames, f.SQL())
		}
	}

	if len(dims) == 0 {
		this.Response(res, nil, herrors.ErrCallerInvalidRequest.New("parameter [dims] not found"))
		return
	}

	//处理order子句
	var orderBys []string
	if req.Ordering != nil {
		for _, s := range *(req.Ordering) {
			order, herr := this.parseOrderBy(o.iFieldMap, s)
			if herr != nil {
				this.Response(res, nil, herr)
				return
			}
			orderBys = append(orderBys, fmt.Sprintf("%s %s", order.column, order.direction))
		}
	}

	//处理limit 子句
	lmt := limit{}
	computeTotal := false
	if req.Paging != nil {
		computeTotal = true
		lmt.offset = ((*req.Paging)[0] - 1) * (*req.Paging)[1]
		lmt.count = (*req.Paging)[1]
	}

	if hook := this.beforeQueryHookNames[*req.Key]; hook != "" {
		reply := core.CallerResponse{}
		stop := false
		this.callBeforeQueryHook(hook, req, &reply, &stop)
		if reply.Error != nil && stop {
			this.Response(res, nil, reply.Error)
			return
		}
	}

	//查询数据
	var data []htypes.Any
	var scope *gorm.DB

	scope = this.getDB(o.database).Table(fmt.Sprintf("`%s` AS `%s`", tableName, o.key))
	scope = scope.Select(selectFieldNames)

	if strings.Trim(strings.Trim(where, ")"), "(") != "" {
		scope = scope.Where(where, vals...)
	}

	if len(orderBys) > 0 {
		scope = scope.Order(strings.Join(orderBys, ","))
	}

	if lmt.count > 0 {
		scope = scope.Limit(lmt.count).Offset(lmt.offset)
	}
	if rows, err := scope.Rows(); err != nil {
		this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
		return
	} else {
		defer rows.Close()
		for rows.Next() {
			vals := this.buildDimValues(o.instance, o.iFieldMap, dims)
			err = rows.Scan(vals...)
			if err != nil {
				this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
				return
			}
			data = append(data, this.bindDimValues(o.iFieldMap, dims, vals))
		}
	}

	var records interface{}
	records = data
	if hook := this.afterQueryHookNames[*req.Key]; hook != "" {
		reply := core.CallerResponse{}
		req.Records = data
		this.callAfterQueryHook(hook, req, &reply)
		records = reply.Data
	}

	if computeTotal {
		var total int64
		var err error
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
				resData["records"] = []interface{}{}
			} else {
				resData["records"] = records
			}
			this.Response(res, resData, nil)
		}
	} else {
		this.Response(res, records, nil)
	}
}

func (this *Service) View(req *QueryRequest, res *core.SlotResponse) {
	vw := this.viewsWithKey[*req.Key]
	if vw == nil {
		this.Response(res, nil, herrors.ErrSysInternal.New("key [%s] not found", *req.Key))
		return
	}
	db := this.dbs[vw.from.database]

	//解析filters
	filters, herr := this.parseRawFilter(vw.iFieldMap, req.Filter)
	if herr != nil {
		this.Response(res, nil, herr)
		return
	}
	filtersSetValues := this.getFieldValuesSetByFilters(filters)
	for _, f := range vw.fieldMap {
		if herr = this.checkTabNamingFieldsValue(f.owner.object, filtersSetValues[f.owner.object.key]); herr != nil {
			this.Response(res, nil, herr)
			return
		}
	}

	var (
		where string
		vals  []interface{}
		he    *herrors.Error
	)
	if filters != nil {
		where, vals, he = this.buildWhereClause(filters)
		if he != nil {
			this.Response(res, nil, he)
			return
		}
	}

	//处理select 子句
	var dims []string
	var selectFieldNames []string
	for _, d := range *req.Dims {
		f := vw.fieldMap[d]
		if f == nil {
			continue
		}

		if f.kind == reflect.Map || f.kind == reflect.Ptr || f.kind == reflect.Slice || f.kind == reflect.Struct {
			this.Response(res, nil, herrors.ErrSysInternal.New("invalid view [%s] dim [%s]", vw.name, f.name))
			return
		}

		dims = append(dims, f.key)
		selectFieldNames = append(selectFieldNames, f.SQL())
	}

	if len(dims) == 0 {
		this.Response(res, nil, herrors.ErrSysInternal.New("non-empty parameter [dims] required"))
		return
	}

	//处理order子句
	var orderBys []string
	if req.Ordering != nil {
		for _, s := range *(req.Ordering) {
			order, herr := this.parseOrderBy(vw.iFieldMap, s)
			if herr != nil {
				this.Response(res, nil, herr)
				return
			}
			orderBys = append(orderBys, fmt.Sprintf("%s %s", order.column, order.direction))
		}
	}

	//处理limit 子句
	lmt := limit{}
	computeTotal := false
	if req.Paging != nil {
		computeTotal = true
		lmt.offset = ((*req.Paging)[0] - 1) * (*req.Paging)[1]
		lmt.count = (*req.Paging)[1]
	}

	//var group groupBy
	//if ps["group"] != nil {
	//	gs := ps["group"].([]interface{})
	//	f := o.fieldMap[gs[0].(string)]
	//	if f != nil {
	//		group.column = f.key
	//		if len(gs) > 1 {
	//			ss, err := this.parseCondition(gs[1].(string))
	//			if err != nil {
	//				this.Response(res, nil, err)
	//				return
	//			}
	//			group.having = this.convCond2Filter(o.iFieldMap, ss)
	//		}
	//	}
	//}

	//查询数据
	var data []interface{}
	var tab string
	var scope *gorm.DB
	var joins []string
	var tableName string

	if tab, herr = this.CheckTableName(vw.from, filtersSetValues[vw.from.key], true); herr != nil {
		this.Response(res, nil, herr)
		return
	} else {
		tableName = fmt.Sprintf("`%s` AS `%s`", tab, vw.from.key)
		scope = db.Table(tableName)
	}

	scope = scope.Select(selectFieldNames)
	for _, join := range vw.joins {
		if tab, herr = this.CheckTableName(join.object, filtersSetValues[join.object.key], true); herr != nil {
			this.Response(res, nil, herr)
			return
		} else {
			j := fmt.Sprintf("LEFT JOIN `%s` AS `%s` ON `%s`.`%s` = `%s`.`%s`",
				tab, join.object.key,
				join.on.leftObj.key, join.on.leftField.col,
				join.on.rightObj.key, join.on.rightField.col)
			joins = append(joins, j)
			scope = scope.Joins(j)
		}
	}

	if where != "" {
		scope = scope.Where(where, vals...)
	}

	if len(orderBys) > 0 {
		scope = scope.Order(strings.Join(orderBys, ","))
	}

	if lmt.count > 0 {
		scope = scope.Limit(lmt.count).Offset(lmt.offset)
	}
	rows, err := scope.Rows()
	if err != nil {
		this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
		return
	}

	defer rows.Close()
	for rows.Next() {
		vals := this.buildDimValues(vw.instance, vw.iFieldMap, dims)
		err = rows.Scan(vals...)
		if err != nil {
			this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
			return
		}
		data = append(data, this.bindDimValues(vw.iFieldMap, dims, vals))
	}

	if computeTotal {
		var total int64
		scope = db.Table(tableName)
		for _, join := range joins {
			scope = scope.Joins(join)
		}
		if where != "" {
			err = scope.Where(where, vals...).Count(&total).Error
		} else {
			err = scope.Count(&total).Error
		}
		if err != nil {
			this.Response(res, nil, herrors.ErrSysInternal.New(err.Error()))
			return
		} else {
			resData := htypes.Map{
				"total": total,
			}
			if data == nil || reflect.ValueOf(data).IsNil() {
				resData["records"] = []interface{}{}
			} else {
				resData["records"] = data
			}
			this.Response(res, resData, nil)
		}
	} else {
		this.Response(res, data, nil)
	}

}
