/**********************************
	Service 关系数据管理服务
	为了保证Service可以对所有注册到DatabasePlugin的对象进行管理，Service需要在其他服务注册之后，最后进行注册
 **********************************/

package hdatasvs

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"

	"gorm.io/gorm"

	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
	"github.com/drharryhe/has/plugins/hdatabaseplugin"
	"github.com/drharryhe/has/utils/hconverter"
	"github.com/drharryhe/has/utils/hdatetime"
	"github.com/drharryhe/has/utils/hruntime"
)

const (
	opCreate = "create"
	opUpdate = "update"
	opDelete = "delete"
	opQuery  = "query"
)

type FieldFunc func(param interface{}) (interface{}, *herrors.Error)

type FieldFuncMap map[string]FieldFunc

type Service struct {
	core.Service

	conf                    DataService
	dbPlugin                *hdatabaseplugin.Plugin
	defaultDB               *gorm.DB
	dbs                     map[string]*gorm.DB
	instancesWithName       map[string]interface{}
	objectsByKey            map[string]*object
	objectsByName           map[string]*object
	viewsWithKey            map[string]*view
	viewsWithName           map[string]*view
	tableNamesOfObj         map[string]string
	fieldFuncMap            map[string]FieldFunc
	tablesOfDatabases       map[string]map[string]bool
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
	afterViewHookCallers    map[string]*core.MethodCaller
	afterViewHookNames      map[string]string
	beforeViewHookCallers   map[string]*core.MethodCaller
	beforeViewHookNames     map[string]string
	afterDelHookCallers     map[string]*core.MethodCaller
	afterDelHookNames       map[string]string
	beforeDelHookCallers    map[string]*core.MethodCaller
	beforeDelHookNames      map[string]string
}

func (this *Service) Open(s core.IServer, instance core.IService, options htypes.Any) *herrors.Error {
	err := this.Service.Open(s, instance, options)
	if err != nil {
		return err
	}

	plugin := this.UsePlugin("DatabasePlugin").(*hdatabaseplugin.Plugin)
	this.defaultDB = plugin.DefaultDB()
	this.dbs = plugin.Capability().(map[string]*gorm.DB)

	this.instancesWithName = make(map[string]interface{})
	this.tableNamesOfObj = make(map[string]string)
	this.objectsByKey = make(map[string]*object)
	this.objectsByName = make(map[string]*object)
	this.viewsWithKey = make(map[string]*view)
	this.viewsWithName = make(map[string]*view)
	this.tablesOfDatabases = make(map[string]map[string]bool)

	this.beforeCreateHookNames = make(map[string]string)
	this.afterCreateHookNames = make(map[string]string)
	this.beforeUpdateHookNames = make(map[string]string)
	this.afterUpdateHookNames = make(map[string]string)
	this.beforeQueryHookNames = make(map[string]string)
	this.afterQueryHookNames = make(map[string]string)
	this.beforeViewHookNames = make(map[string]string)
	this.afterViewHookNames = make(map[string]string)
	this.beforeDelHookNames = make(map[string]string)
	this.afterDelHookNames = make(map[string]string)

	if options != nil {
		ops := options.(*Options)
		if ops.Hooks != nil {
			this.mountHookCallers(ops.Hooks)
		}

		if ops.FieldFuncMap != nil {
			this.fieldFuncMap = ops.FieldFuncMap
		}

		if ops.Objects != nil {
			for i, o := range ops.Objects {
				n := hruntime.GetObjectName(o)
				this.instancesWithName[n] = ops.Objects[i]

				obj, herr := this.parseObject(n, o)
				if herr != nil {
					return herr
				}
				if obj == nil {
					continue
				}

				this.objectsByKey[obj.key] = obj
				this.objectsByName[n] = obj

				if this.conf.AutoMigrate {
					if err := this.getDB(obj.database).AutoMigrate(o); err != nil {
						if hconf.IsDebug() {
							_ = herrors.ErrSysInternal.New(err.Error())
						} else {
							hlogger.Error(herrors.ErrSysInternal.New(err.Error()))
						}
					}
				}
			}
			hlogger.Info("AutoMigrate Done")
			this.conf.AutoMigrate = false
			hconf.Save()
		}

		if ops.Views != nil {
			for _, v := range ops.Views {
				view, he := this.parseView(v)
				if he != nil {
					return he
				}
				this.viewsWithKey[view.key] = view
				this.viewsWithName[view.name] = view
			}
		}
	}
	return nil
}

func (this *Service) EntityStub() *core.EntityStub {
	return core.NewEntityStub(
		&core.EntityStubOptions{
			Owner: this,
		})
}

func (this *Service) Config() core.IEntityConf {
	return &this.conf
}

func (this *Service) buildWhereClause(fs *filter) (string, []interface{}, *herrors.Error) {
	var (
		where string
		vals  []interface{}
		w     string
		v     interface{}
	)

	logic := "AND"
	for i, f := range fs.conditions {
		if f.field.Kind() == reflect.Slice || f.field.Kind() == reflect.Map || f.field.Kind() == reflect.Struct || f.field.Kind() == reflect.Ptr {
			continue
		}
		com := strings.ToUpper(f.compare)
		switch com {
		case "BETWEEN", "!BETWEEN":
			if com == "BETWEEN" {
				w = fmt.Sprintf("%s BETWEEN ? AND ?", f.field.Column())
			} else {
				w = fmt.Sprintf("%s NOT BETWEEN ? AND ?", f.field.Column())
			}
			if f.field.Kind() == reflect.String {
				for _, s := range f.value.([]string) {
					vals = append(vals, s)
				}
			} else {
				for _, t := range f.value.([]float64) {
					vals = append(vals, t)
				}
			}
		case "IN", "!IN":
			if com == "IN" {
				w = fmt.Sprintf("%s IN (", f.field.Column())
			} else {
				w = fmt.Sprintf("%s NOT IN (", f.field.Column())
			}

			if f.field.Kind() == reflect.String {
				for i, t := range f.value.([]string) {
					vals = append(vals, t)
					if i == len(f.value.([]string))-1 {
						w += "?"
					} else {
						w += "?,"
					}
				}
			} else {
				for i, t := range f.value.([]float64) {
					vals = append(vals, t)
					if i == len(f.value.([]float64))-1 {
						w += "?"
					} else {
						w += "?,"
					}
				}
			}
			w += ")"
		case "LIKE", "!LIKE":
			if com == "LIKE" {
				w = fmt.Sprintf("%s LIKE ?", f.field.Column())
			} else {
				w = fmt.Sprintf("%s NOT LIKE ?", f.field.Column())
			}
			v = fmt.Sprintf("%%%v%%", f.value)
			vals = append(vals, v)
		case "HAS_PREFIX", "!HAS_PREFIX":
			if com == "HAS_PREFIX" {
				w = fmt.Sprintf("%s LIKE ?", f.field.Column())
			} else {
				w = fmt.Sprintf("%s NOT LIKE ?", f.field.Column())
			}
			v = fmt.Sprintf("%v%%", f.value)
			vals = append(vals, v)
		case "HAS_SUFFIX", "!HAS_SUFFIX":
			if com == "HAS_SUFFIX" {
				w = fmt.Sprintf("%s LIKE ?", f.field.Column())
			} else {
				w = fmt.Sprintf("%s NOT LIKE ?", f.field.Column())
			}
			v = fmt.Sprintf("%%%v", f.value)
			vals = append(vals, v)
		default:
			w = fmt.Sprintf("%s %s ?", f.field.Column(), f.compare)
			vals = append(vals, f.value)
		}
		if i == 0 {
			where = w
		} else {
			where = fmt.Sprintf("%s %s %s", where, logic, w)
		}

	}

	if len(fs.conditions) > 0 {
		where = fmt.Sprintf("(%s)", where)
	}

	if fs.or {
		logic = "OR"
	}
	for _, c := range fs.filters {
		if c == nil {
			continue
		}
		w, vs, err := this.buildWhereClause(c)
		if err != nil {
			return "", nil, err
		}
		if where == "" {
			where = w
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
	case reflect.Struct:
		return &sql.NullTime{}
	default:
		return &v
	}
}

func (this *Service) convertSql2GoValue(v interface{}, kind reflect.Kind) interface{} {
	if reflect.TypeOf(v).Kind() == reflect.Ptr && reflect.ValueOf(v).Elem().Type().Name() == "NullTime" {
		return v.(*sql.NullTime).Time.Unix()
	}

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

func (this *Service) buildDimValues(ins interface{}, fMap map[string]iField, dims []string) []interface{} {
	var ret []interface{}
	v := reflect.ValueOf(hruntime.CloneObject(ins))
	if dims == nil || len(dims) == 0 {
		for i := 0; i < v.Elem().NumField(); i++ {
			//k := v.Elem().Field(i).Kind()
			//if k == reflect.Map || k == reflect.Slice || k == reflect.Ptr {
			//	continue
			//}
			//if k == reflect.Struct && v.Elem().Field(i).Type().Name() != "Time" {
			//	continue
			//}
			f := this.convertGo2SqlType(v.Elem().Field(i).Interface())
			ret = append(ret, f)
		}
	} else {
		for _, s := range dims {
			//k := v.Elem().FieldByName(fMap[s].Name()).Kind()
			//if k == reflect.Map || k == reflect.Slice || k == reflect.Ptr {
			//	continue
			//}
			//if k == reflect.Struct && v.Elem().FieldByName(fMap[s].Name()).Type().Name() != "Time" {
			//	continue
			//}
			f := this.convertGo2SqlType(v.Elem().FieldByName(fMap[s].Name()).Interface())
			ret = append(ret, f)
		}
	}

	return ret
}

func (this *Service) bindDimValues(fMap map[string]iField, dims []string, values []interface{}) map[string]interface{} {
	ret := make(map[string]interface{})
	for i, d := range dims {
		//if fMap[d].Kind() == reflect.Struct || fMap[d].Kind() == reflect.Slice {
		//	continue
		//}
		ret[d] = this.convertSql2GoValue(values[i], fMap[d].Kind())
	}
	return ret
}

func (this *Service) parseCondition(fMap map[string]iField, s string) (*condition, *herrors.Error) {
	ss := strings.ToUpper(s)

	i := strings.Index(ss, ">=")
	if i >= 0 {
		f := fMap[strings.TrimSpace(s[:i])]
		if f == nil {
			return nil, herrors.ErrUserInvalidAct.New("invalid condition [%s], field not found", s)
		}

		switch f.Kind() {
		case reflect.String:
			return &condition{
				field:   f,
				compare: ">=",
				value:   strings.TrimSpace(s[i+2:]),
			}, nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float64, reflect.Float32:
			n, ok := hconverter.String2NumberDecimal(strings.TrimSpace(s[i+2:]))
			if !ok {
				return nil, herrors.ErrCallerInvalidRequest.New("invalid condition [%s]", s[i+2:])
			}
			return &condition{
				field:   f,
				compare: ">=",
				value:   n,
			}, nil
		default:
			return nil, herrors.ErrUserInvalidAct.New("field [%s] cannot use [>=]", f.Key())
		}
	}

	if i = strings.Index(ss, "<="); i >= 0 {
		f := fMap[strings.TrimSpace(s[:i])]
		if f == nil {
			return nil, herrors.ErrUserInvalidAct.New("invalid condition [%s], field not found", s)
		}

		switch f.Kind() {
		case reflect.String:
			return &condition{
				field:   f,
				compare: "<=",
				value:   strings.TrimSpace(s[i+2:]),
			}, nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float64, reflect.Float32:
			n, ok := hconverter.String2NumberDecimal(strings.TrimSpace(s[i+2:]))
			if !ok {
				return nil, herrors.ErrCallerInvalidRequest.New("invalid condition [%s]", s[i+2:])
			}
			return &condition{
				field:   f,
				compare: "<=",
				value:   n,
			}, nil
		default:
			return nil, herrors.ErrUserInvalidAct.New("field [%s] cannot use [<=]", f.Key())
		}
	}

	if i = strings.Index(ss, "!="); i >= 0 {
		f := fMap[strings.TrimSpace(s[:i])]
		if f == nil {
			return nil, herrors.ErrUserInvalidAct.New("invalid condition [%s], field not found", s)
		}

		switch f.Kind() {
		case reflect.Bool:
			return &condition{
				field:   f,
				compare: "!=",
				value:   strings.TrimSpace(ss[i+2:]) == "TRUE",
			}, nil
		case reflect.String:
			return &condition{
				field:   f,
				compare: "!=",
				value:   strings.TrimSpace(s[i+2:]),
			}, nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float64, reflect.Float32:
			n, ok := hconverter.String2NumberDecimal(strings.TrimSpace(s[i+2:]))
			if !ok {
				return nil, herrors.ErrCallerInvalidRequest.New("invalid condition [%s]", s[i+2:])
			}
			return &condition{
				field:   f,
				compare: "!=",
				value:   n,
			}, nil
		default:
			return nil, herrors.ErrUserInvalidAct.New("field [%s] cannot use [!=]", f.Key())
		}
	}

	if i = strings.Index(ss, "<"); i >= 0 {
		f := fMap[strings.TrimSpace(s[:i])]
		if f == nil {
			return nil, herrors.ErrUserInvalidAct.New("invalid condition [%s], field not found", s)
		}

		switch f.Kind() {
		case reflect.String:
			return &condition{
				field:   f,
				compare: "<",
				value:   strings.TrimSpace(s[i+1:]),
			}, nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float64, reflect.Float32:
			n, ok := hconverter.String2NumberDecimal(strings.TrimSpace(s[i+1:]))
			if !ok {
				return nil, herrors.ErrCallerInvalidRequest.New("invalid condition [%s]", s[i+1:])
			}
			return &condition{
				field:   f,
				compare: "<",
				value:   n,
			}, nil
		default:
			return nil, herrors.ErrUserInvalidAct.New("field [%s] cannot use [<]", f.Key())
		}
	}

	if i = strings.Index(ss, ">"); i >= 0 {
		f := fMap[strings.TrimSpace(s[:i])]
		if f == nil {
			return nil, herrors.ErrUserInvalidAct.New("invalid condition [%s], field not found", s)
		}

		switch f.Kind() {
		case reflect.String:
			return &condition{
				field:   f,
				compare: ">",
				value:   strings.TrimSpace(s[i+1:]),
			}, nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float64, reflect.Float32:
			n, ok := hconverter.String2NumberDecimal(strings.TrimSpace(s[i+1:]))
			if !ok {
				return nil, herrors.ErrCallerInvalidRequest.New("invalid condition [%s]", s[i+1:])
			}
			return &condition{
				field:   f,
				compare: ">",
				value:   n,
			}, nil
		default:
			return nil, herrors.ErrUserInvalidAct.New("field [%s] cannot use [>]", f.Key())
		}
	}

	if i = strings.Index(ss, "=="); i >= 0 {
		f := fMap[strings.TrimSpace(s[:i])]
		if f == nil {
			return nil, herrors.ErrUserInvalidAct.New("invalid condition [%s], field not found", s)
		}

		switch f.Kind() {
		case reflect.Bool:
			return &condition{
				field:   f,
				compare: "=",
				value:   strings.TrimSpace(ss[i+2:]) == "TRUE",
			}, nil
		case reflect.String:
			return &condition{
				field:   f,
				compare: "=",
				value:   strings.TrimSpace(s[i+2:]),
			}, nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float64, reflect.Float32:
			n, ok := hconverter.String2NumberDecimal(strings.TrimSpace(s[i+2:]))
			if !ok {
				return nil, herrors.ErrCallerInvalidRequest.New("invalid condition [%s]", s)
			}
			return &condition{
				field:   f,
				compare: "=",
				value:   n,
			}, nil
		default:
			return nil, herrors.ErrUserInvalidAct.New("field [%s] cannot use [=]", f.Key())
		}
	}

	if i = strings.Index(ss, " !IN "); i >= 0 {
		f := fMap[strings.TrimSpace(s[:i])]
		if f == nil {
			return nil, herrors.ErrUserInvalidAct.New("invalid condition [%s], field not found", s)
		}

		switch f.Kind() {
		case reflect.String:
			n, ok := hconverter.String2StringArray(strings.TrimSpace(s[i+5:]))
			if !ok {
				return nil, herrors.ErrCallerInvalidRequest.New("invalid condition [%s]", s)
			}
			return &condition{
				field:   f,
				compare: "!IN",
				value:   n,
			}, nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float64, reflect.Float32:
			n, ok := hconverter.String2NumberArray(strings.TrimSpace(s[i+5:]))
			if !ok {
				return nil, herrors.ErrCallerInvalidRequest.New("invalid condition [%s]", s)
			}
			return &condition{
				field:   f,
				compare: "!IN",
				value:   n,
			}, nil
		default:
			return nil, herrors.ErrUserInvalidAct.New("field [%s] cannot use [!IN]", f.Key())
		}
	}

	if i = strings.Index(ss, " IN "); i >= 0 {
		f := fMap[strings.TrimSpace(s[:i])]
		if f == nil {
			return nil, herrors.ErrUserInvalidAct.New("invalid condition [%s], field not found", s)
		}

		switch f.Kind() {
		case reflect.String:
			n, ok := hconverter.String2StringArray(strings.TrimSpace(s[i+4:]))
			if !ok {
				return nil, herrors.ErrCallerInvalidRequest.New("invalid condition [%s]", s)
			}
			return &condition{
				field:   f,
				compare: "IN",
				value:   n,
			}, nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float64, reflect.Float32:
			n, ok := hconverter.String2NumberArray(strings.TrimSpace(s[i+4:]))
			if !ok {
				return nil, herrors.ErrCallerInvalidRequest.New("invalid condition [%s]", s)
			}
			return &condition{
				field:   f,
				compare: "IN",
				value:   n,
			}, nil
		default:
			return nil, herrors.ErrUserInvalidAct.New("field [%s] cannot use [IN]", f.Key())
		}
	}

	if i = strings.Index(ss, " !BETWEEN "); i >= 0 {
		f := fMap[strings.TrimSpace(s[:i])]
		if f == nil {
			return nil, herrors.ErrUserInvalidAct.New("invalid condition [%s], field not found", s)
		}

		switch f.Kind() {
		case reflect.String:
			from, to, ok := hconverter.String2StringRange(strings.TrimSpace(s[i+10:]))
			if !ok {
				return nil, herrors.ErrCallerInvalidRequest.New("invalid condition [%s]", s)
			}
			return &condition{
				field:   f,
				compare: "!BETWEEN",
				value:   []string{from, to},
			}, nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float64, reflect.Float32:
			from, to, ok := hconverter.String2NumberRange(strings.TrimSpace(s[i+10:]))
			if !ok {
				return nil, herrors.ErrCallerInvalidRequest.New("invalid condition [%s]", s)
			}
			return &condition{
				field:   f,
				compare: "!BETWEEN",
				value:   []float64{from, to},
			}, nil
		default:
			return nil, herrors.ErrUserInvalidAct.New("field [%s] cannot use [!BETWEEN]", f.Key())
		}
	} else if i = strings.Index(ss, " BETWEEN "); i >= 0 {
		f := fMap[strings.TrimSpace(s[:i])]
		if f == nil {
			return nil, herrors.ErrUserInvalidAct.New("invalid condition [%s], field not found", s)
		}

		switch f.Kind() {
		case reflect.String:
			from, to, ok := hconverter.String2StringRange(strings.TrimSpace(s[i+9:]))
			if !ok {
				return nil, herrors.ErrCallerInvalidRequest.New("invalid condition [%s]", s)
			}
			return &condition{
				field:   f,
				compare: "BETWEEN",
				value:   []string{from, to},
			}, nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float64, reflect.Float32:
			from, to, ok := hconverter.String2NumberRange(strings.TrimSpace(s[i+9:]))
			if !ok {
				return nil, herrors.ErrCallerInvalidRequest.New("invalid condition [%s]", s)
			}
			return &condition{
				field:   f,
				compare: "BETWEEN",
				value:   []float64{from, to},
			}, nil
		default:
			return nil, herrors.ErrUserInvalidAct.New("field [%s] cannot use [BETWEEN]", f.Key())
		}
	}

	if i = strings.Index(ss, " !LIKE "); i >= 0 {
		f := fMap[strings.TrimSpace(s[:i])]
		if f == nil {
			return nil, herrors.ErrUserInvalidAct.New("invalid condition [%s], field not found", s)
		}

		switch f.Kind() {
		case reflect.String:
			return &condition{
				field:   f,
				compare: "!LIKE",
				value:   strings.TrimSpace(s[i+7:]),
			}, nil
		default:
			return nil, herrors.ErrUserInvalidAct.New("field [%s] cannot use [!LIKE]", f.Key())
		}
	}

	if i = strings.Index(ss, " LIKE "); i >= 0 {
		f := fMap[strings.TrimSpace(s[:i])]
		if f == nil {
			return nil, herrors.ErrUserInvalidAct.New("invalid condition [%s], field not found", s)
		}

		switch f.Kind() {
		case reflect.String:
			return &condition{
				field:   f,
				compare: "LIKE",
				value:   strings.TrimSpace(s[i+6:]),
			}, nil
		default:
			return nil, herrors.ErrUserInvalidAct.New("field [%s] cannot use [LIKE]", f.Key())
		}
	}

	if i = strings.Index(ss, " !HAS_PREFIX "); i >= 0 {
		f := fMap[strings.TrimSpace(s[:i])]
		if f == nil {
			return nil, herrors.ErrUserInvalidAct.New("invalid condition [%s], field not found", s)
		}

		switch f.Kind() {
		case reflect.String:
			return &condition{
				field:   f,
				compare: "!HAS_PREFIX",
				value:   strings.TrimSpace(s[i+13:]),
			}, nil
		default:
			return nil, herrors.ErrUserInvalidAct.New("field [%s] cannot use [!HAS_PREFIX]", f.Key())
		}
	}

	if i = strings.Index(ss, " HAS_PREFIX "); i >= 0 {
		f := fMap[strings.TrimSpace(s[:i])]
		if f == nil {
			return nil, herrors.ErrUserInvalidAct.New("invalid condition [%s], field not found", s)
		}

		switch f.Kind() {
		case reflect.String:
			return &condition{
				field:   f,
				compare: "HAS_PREFIX",
				value:   strings.TrimSpace(s[i+12:]),
			}, nil
		default:
			return nil, herrors.ErrUserInvalidAct.New("field [%s] cannot use [HAS_PREFIX]", f.Key())
		}
	}

	if i = strings.Index(ss, " !HAS_SUFFIX "); i >= 0 {
		f := fMap[strings.TrimSpace(s[:i])]
		if f == nil {
			return nil, herrors.ErrUserInvalidAct.New("invalid condition [%s], field not found", s)
		}

		switch f.Kind() {
		case reflect.String:
			return &condition{
				field:   f,
				compare: "!HAS_SUFFIX",
				value:   strings.TrimSpace(s[i+13:]),
			}, nil
		default:
			return nil, herrors.ErrUserInvalidAct.New("field [%s] cannot use [!HAS_SUFFIX]", f.Key())
		}
	}

	if i = strings.Index(ss, " HAS_SUFFIX "); i >= 0 {
		f := fMap[strings.TrimSpace(s[:i])]
		if f == nil {
			return nil, herrors.ErrUserInvalidAct.New("invalid condition [%s], field not found", s)
		}

		switch f.Kind() {
		case reflect.String:
			return &condition{
				field:   f,
				compare: "HAS_SUFFIX",
				value:   strings.TrimSpace(s[i+12:]),
			}, nil
		default:
			return nil, herrors.ErrUserInvalidAct.New("field [%s] cannot use [HAS_SUFFIX]", f.Key())
		}
	}

	return nil, herrors.ErrSysInternal.New("invalid condition operator in condition parameter [%s]", s)
}

func (this *Service) mountHookCallers(h htypes.Any) {
	this.mountAfterDelHook(h)
	this.mountBeforeDelHook(h)
	this.mountAfterQueryHook(h)
	this.mountBeforeQueryHook(h)
	this.mountAfterViewHook(h)
	this.mountBeforeViewHook(h)
	this.mountAfterCreateHook(h)
	this.mountBeforeCreateHook(h)
	this.mountAfterUpdateHook(h)
	this.mountBeforeUpdateHook(h)
}

func (this *Service) mountBeforeCreateHook(anchor htypes.Any) {
	this.beforeCreateHookCallers = make(map[string]*core.MethodCaller)
	typ := reflect.TypeOf(anchor)
	val := reflect.ValueOf(anchor)
	n := val.NumMethod()
	for i := 0; i < n; i++ {
		method := typ.Method(i)
		if !method.IsExported() {
			continue
		}

		mtype := method.Type

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
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Name() != "CreateRequest" {
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

		this.beforeCreateHookCallers[method.Name] = &core.MethodCaller{Object: val, Handler: method.Func}
	}
}

func (this *Service) mountAfterCreateHook(anchor htypes.Any) {
	this.afterCreateHookCallers = make(map[string]*core.MethodCaller)
	typ := reflect.TypeOf(anchor)
	val := reflect.ValueOf(anchor)
	n := val.NumMethod()
	for i := 0; i < n; i++ {
		method := typ.Method(i)
		if !method.IsExported() {
			continue
		}

		mtype := method.Type

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
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Name() != "CreateRequest" {
			continue
		}

		ctxType = mtype.In(3)
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Name() != "CallerResponse" {
			continue
		}

		this.afterCreateHookCallers[method.Name] = &core.MethodCaller{Object: val, Handler: method.Func}
	}
}

func (this *Service) mountBeforeUpdateHook(anchor htypes.Any) {
	this.beforeUpdateHookCallers = make(map[string]*core.MethodCaller)
	typ := reflect.TypeOf(anchor)
	val := reflect.ValueOf(anchor)
	n := val.NumMethod()
	for i := 0; i < n; i++ {
		method := typ.Method(i)
		if !method.IsExported() {
			continue
		}

		mtype := method.Type

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
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Name() != "UpdateRequest" {
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

		this.beforeUpdateHookCallers[method.Name] = &core.MethodCaller{Object: val, Handler: method.Func}
	}
}

func (this *Service) mountAfterUpdateHook(anchor htypes.Any) {
	this.afterUpdateHookCallers = make(map[string]*core.MethodCaller)
	typ := reflect.TypeOf(anchor)
	val := reflect.ValueOf(anchor)
	n := val.NumMethod()
	for i := 0; i < n; i++ {
		method := typ.Method(i)
		if !method.IsExported() {
			continue
		}

		mtype := method.Type

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
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Name() != "UpdateRequest" {
			continue
		}

		ctxType = mtype.In(3)
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Name() != "CallerResponse" {
			continue
		}

		this.afterUpdateHookCallers[method.Name] = &core.MethodCaller{Object: val, Handler: method.Func}
	}
}

func (this *Service) mountBeforeDelHook(anchor htypes.Any) {
	this.beforeDelHookCallers = make(map[string]*core.MethodCaller)
	typ := reflect.TypeOf(anchor)
	val := reflect.ValueOf(anchor)
	n := val.NumMethod()
	for i := 0; i < n; i++ {
		method := typ.Method(i)
		if !method.IsExported() {
			continue
		}
		mtype := method.Type

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
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Name() != "DeleteRequest" {
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

		this.beforeDelHookCallers[method.Name] = &core.MethodCaller{Object: val, Handler: method.Func}
	}
}

func (this *Service) mountAfterDelHook(anchor htypes.Any) {
	this.afterDelHookCallers = make(map[string]*core.MethodCaller)
	typ := reflect.TypeOf(anchor)
	val := reflect.ValueOf(anchor)
	n := val.NumMethod()
	for i := 0; i < n; i++ {
		method := typ.Method(i)
		if !method.IsExported() {
			continue
		}

		mtype := method.Type

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
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Name() != "DeleteRequest" {
			continue
		}

		ctxType = mtype.In(3)
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Name() != "CallerResponse" {
			continue
		}

		this.afterDelHookCallers[method.Name] = &core.MethodCaller{Object: val, Handler: method.Func}
	}
}

func (this *Service) mountBeforeQueryHook(anchor htypes.Any) {
	this.beforeQueryHookCallers = make(map[string]*core.MethodCaller)
	typ := reflect.TypeOf(anchor)
	val := reflect.ValueOf(anchor)
	n := val.NumMethod()
	for i := 0; i < n; i++ {
		method := typ.Method(i)
		if !method.IsExported() {
			continue
		}

		mtype := method.Type

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
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Name() != "QueryRequest" {
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

		this.beforeQueryHookCallers[method.Name] = &core.MethodCaller{Object: val, Handler: method.Func}
	}
}

func (this *Service) mountBeforeViewHook(anchor htypes.Any) {
	this.beforeViewHookCallers = make(map[string]*core.MethodCaller)
	typ := reflect.TypeOf(anchor)
	val := reflect.ValueOf(anchor)
	n := val.NumMethod()
	for i := 0; i < n; i++ {
		method := typ.Method(i)
		if !method.IsExported() {
			continue
		}

		mtype := method.Type

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
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Name() != "ViewRequest" {
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

		this.beforeViewHookCallers[method.Name] = &core.MethodCaller{Object: val, Handler: method.Func}
	}
}

func (this *Service) mountAfterQueryHook(anchor htypes.Any) {
	this.afterQueryHookCallers = make(map[string]*core.MethodCaller)
	typ := reflect.TypeOf(anchor)
	val := reflect.ValueOf(anchor)
	n := val.NumMethod()
	for i := 0; i < n; i++ {
		method := typ.Method(i)
		if !method.IsExported() {
			continue
		}
		mtype := method.Type

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

		// parameters
		ctxType = mtype.In(2)
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Name() != "QueryRequest" {
			continue
		}

		ctxType = mtype.In(3)
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Name() != "CallerResponse" {
			continue
		}

		this.afterQueryHookCallers[method.Name] = &core.MethodCaller{Object: val, Handler: method.Func}
	}
}

func (this *Service) mountAfterViewHook(anchor htypes.Any) {
	this.afterViewHookCallers = make(map[string]*core.MethodCaller)
	typ := reflect.TypeOf(anchor)
	val := reflect.ValueOf(anchor)
	n := val.NumMethod()
	for i := 0; i < n; i++ {
		method := typ.Method(i)
		if !method.IsExported() {
			continue
		}
		mtype := method.Type

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

		// parameters
		ctxType = mtype.In(2)
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Name() != "ViewRequest" {
			continue
		}

		ctxType = mtype.In(3)
		if ctxType.Kind() != reflect.Ptr {
			continue
		}
		if ctxType.Elem().Name() != "CallerResponse" {
			continue
		}

		this.afterViewHookCallers[method.Name] = &core.MethodCaller{Object: val, Handler: method.Func}
	}
}

func (this *Service) callBeforeCreateHook(name string, req *CreateRequest, reply *core.CallerResponse, stop *bool) {
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

	caller.Handler.Call([]reflect.Value{caller.Object, reflect.ValueOf(this), reflect.ValueOf(req), reflect.ValueOf(reply), reflect.ValueOf(stop)})
}

func (this *Service) callBeforeUpdateHook(name string, req *UpdateRequest, reply *core.CallerResponse, stop *bool) {
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

	caller.Handler.Call([]reflect.Value{caller.Object, reflect.ValueOf(this), reflect.ValueOf(req), reflect.ValueOf(reply), reflect.ValueOf(stop)})
}

func (this *Service) callBeforeQueryHook(name string, req *QueryRequest, reply *core.CallerResponse, stop *bool) {
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
		reply.Error = herrors.ErrSysInternal.New("before query hook [%s] not found", name)
		return
	}

	caller.Handler.Call([]reflect.Value{caller.Object, reflect.ValueOf(this), reflect.ValueOf(req), reflect.ValueOf(reply), reflect.ValueOf(stop)})
}

func (this *Service) callBeforeViewHook(name string, req *ViewRequest, reply *core.CallerResponse, stop *bool) {
	if !hconf.IsDebug() {
		defer func() {
			e := recover()
			if e != nil {
				reply.Error = herrors.ErrSysInternal.New(e.(error).Error())
			}
		}()
	}

	caller := this.beforeViewHookCallers[name]
	if caller == nil {
		reply.Error = herrors.ErrSysInternal.New("before view hook [%s] not found", name)
		return
	}

	caller.Handler.Call([]reflect.Value{caller.Object, reflect.ValueOf(this), reflect.ValueOf(req), reflect.ValueOf(reply), reflect.ValueOf(stop)})
}

func (this *Service) callBeforeDelHook(name string, req *DeleteRequest, reply *core.CallerResponse, stop *bool) {
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

	caller.Handler.Call([]reflect.Value{caller.Object, reflect.ValueOf(this), reflect.ValueOf(req), reflect.ValueOf(reply), reflect.ValueOf(stop)})
}

func (this *Service) callAfterCreateHook(name string, req *CreateRequest, reply *core.CallerResponse) {
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

	caller.Handler.Call([]reflect.Value{caller.Object, reflect.ValueOf(this), reflect.ValueOf(req), reflect.ValueOf(reply)})
}

func (this *Service) callAfterUpdateHook(name string, req *UpdateRequest, reply *core.CallerResponse) {
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

	caller.Handler.Call([]reflect.Value{caller.Object, reflect.ValueOf(this), reflect.ValueOf(req), reflect.ValueOf(reply)})
}

func (this *Service) callAfterQueryHook(name string, req *QueryRequest, reply *core.CallerResponse) {
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

	caller.Handler.Call([]reflect.Value{caller.Object, reflect.ValueOf(this), reflect.ValueOf(req), reflect.ValueOf(reply)})
}

func (this *Service) callAfterViewHook(name string, req *ViewRequest, reply *core.CallerResponse) {
	if !hconf.IsDebug() {
		defer func() {
			e := recover()
			if e != nil {
				reply.Error = herrors.ErrSysInternal.New(e.(error).Error())
			}
		}()
	}

	caller := this.afterViewHookCallers[name]
	if caller == nil {
		reply.Error = herrors.ErrSysInternal.New("after view hook %s not found", name)
		return
	}

	caller.Handler.Call([]reflect.Value{caller.Object, reflect.ValueOf(this), reflect.ValueOf(req), reflect.ValueOf(reply)})
}

func (this *Service) callAfterDelHook(name string, req *DeleteRequest, reply *core.CallerResponse) {
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

	caller.Handler.Call([]reflect.Value{caller.Object, reflect.ValueOf(this), reflect.ValueOf(req), reflect.ValueOf(reply)})
}

func (this *Service) autoFill(funName string, _ htypes.Map) interface{} {
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

func (this *Service) parseObject(name string, o interface{}) (*object, *herrors.Error) {
	obj := &object{}
	obj.instance = o
	obj.name = name

	manageable := false
	t := reflect.TypeOf(o)
	if t.Kind() == reflect.Ptr {
		t = reflect.ValueOf(o).Elem().Type()
	}

	var tabNamingFields []string
	fieldKeyByNameMap := make(map[string]string)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("data")

		var hookNames struct {
			beforeCreate string
			afterCreate  string
			beforeUpdate string
			afterUpdate  string
			beforeDelete string
			afterDelete  string
			beforeQuery  string
			afterQuery   string
			beforeView   string
			afterView    string
		}

		if f.Type.Kind() == reflect.Struct && f.Type.Name() == "DataObject" {
			obj.deniedOperations = make(map[string]bool)
			manageable = true

			if tag == "" {
				obj.key = this.defaultDB.Config.NamingStrategy.TableName(hruntime.GetObjectName(o))
			} else {
				kv := hruntime.ParseTag(tag)
				key := ""
				for k, v := range kv {
					switch k {
					case "key", "":
						key = v
					case "db":
						obj.database = v
					case "deny":
						for _, s := range strings.Split(v, ",") {
							obj.deniedOperations[s] = true
						}
					case "afterUpdate":
						hookNames.afterUpdate = v
					case "beforeUpdate":
						hookNames.beforeUpdate = v
					case "beforeCreate":
						hookNames.beforeCreate = v
					case "afterCreate":
						hookNames.afterCreate = v
					case "beforeQuery":
						hookNames.beforeQuery = v
					case "afterQuery":
						hookNames.afterQuery = v
					case "beforeView":
						hookNames.beforeView = v
					case "afterView":
						hookNames.afterView = v
					case "beforeDelete":
						hookNames.beforeDelete = v
					case "afterDelete":
						hookNames.afterDelete = v
					}
				}

				if key == "" {
					obj.key = this.getDB(obj.database).NamingStrategy.TableName(obj.name)
				} else {
					obj.key = key
				}

				if hookNames.beforeCreate != "" {
					this.beforeCreateHookNames[obj.key] = hookNames.beforeCreate
				}
				if hookNames.afterCreate != "" {
					this.afterCreateHookNames[obj.key] = hookNames.afterCreate
				}
				if hookNames.beforeUpdate != "" {
					this.beforeUpdateHookNames[obj.key] = hookNames.beforeUpdate
				}
				if hookNames.afterUpdate != "" {
					this.afterUpdateHookNames[obj.key] = hookNames.afterUpdate
				}
				if hookNames.beforeDelete != "" {
					this.beforeDelHookNames[obj.key] = hookNames.beforeDelete
				}
				if hookNames.afterDelete != "" {
					this.afterDelHookNames[obj.key] = hookNames.afterDelete
				}
				if hookNames.beforeQuery != "" {
					this.beforeQueryHookNames[obj.key] = hookNames.beforeQuery
				}
				if hookNames.afterQuery != "" {
					this.afterQueryHookNames[obj.key] = hookNames.afterQuery
				}
				if hookNames.beforeView != "" {
					this.beforeViewHookNames[obj.key] = hookNames.beforeView
				}
				if hookNames.afterView != "" {
					this.afterViewHookNames[obj.key] = hookNames.afterView
				}
			}
			continue
		}

		if tag == "-" || !manageable {
			continue
		}

		of := newObjectField()
		of.kind = f.Type.Kind()
		if of.kind == reflect.Struct {
			if f.Type.Name() == "Time" {
				// 时间类型使用Int64时出现了很多数据没被查询到的情况
				of.kind = reflect.String
			} else {
				of.kind = reflect.Map
			}
		}
		of.name = f.Name

		if tag != "" {
			kv := hruntime.ParseTag(tag)
			for k, v := range kv {
				switch k {
				case "col":
					of.col = v
				case "tabNaming": //分表tag
					tabNamingFields = append(tabNamingFields, f.Name)
				case "primary":
					obj.primaryField = of
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
				case opDelete:
					for _, v := range strings.Split(v, ",") {
						switch strings.TrimSpace(v) {
						case "require":
							of.opRequired[opDelete] = true
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
			of.key = this.getDB(obj.database).NamingStrategy.ColumnName("", of.name)
		}
		if of.col == "" {
			of.col = this.getDB(obj.database).NamingStrategy.ColumnName("", of.name)
		}

		of.owner = &fieldOwner{
			object:   obj,
			fieldKey: of.key,
			fieldCol: of.col,
		}
		fieldKeyByNameMap[of.name] = of.key
		obj.AddField(of)
	}

	if manageable {
		db := this.getDB(obj.database)
		obj.tableName = db.NamingStrategy.TableName(obj.name)

		for _, f := range tabNamingFields {
			obj.tabNamingFieldKeys = append(obj.tabNamingFieldKeys, fieldKeyByNameMap[f])
		}

		if obj.primaryField == nil {
			f := obj.fieldMap["id"]
			if f == nil {
				return nil, herrors.ErrSysInternal.New("data object [%s] primary field not found", obj.key)
			} else {
				obj.primaryField = f
			}
		}
		return obj, nil
	}

	return nil, herrors.ErrSysInternal.New("invalid data object [%s], not derived from DataObject", name)
}

func (this *Service) shapeObjectFieldValues(op string, objName string, vals htypes.Map) (htypes.Map, *herrors.Error) {
	obj := this.objectsByName[objName]
	if obj == nil {
		return nil, herrors.ErrSysInternal.New("object [%s] is not declared as DataObject", objName)
	}

	vs := make(htypes.Map)
	for key, of := range obj.fieldMap {
		if of.autoInitFunc != "" {
			if op == opCreate {
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
			vv, err := ff(vals[key])
			if err != nil {
				return nil, err
			}
			vs[of.name] = vv
		}
		if vals[key] == nil {
			if of.opRequired[op] {
				return nil, herrors.ErrSysInternal.New("object field [%s] required", of.key)
			}
		} else {
			v := vals[key]
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
				return nil, herrors.ErrSysInternal.New("object field [%s.%s] invalid kind", obj.key, of.key)
			} else {
				vs[of.name] = v
			}
		}
	}

	return vs, nil
}

func (this *Service) parseJoinOn(view *view, strJoin string) (*join, *herrors.Error) {
	var jn join
	vv := strings.Split(strings.TrimSpace(strJoin), "@")
	if len(vv) != 2 {
		return nil, herrors.ErrSysInternal.New("invalid join tag of view [%s]:[%s]", view.name, strJoin)
	}

	if this.objectsByKey[strings.TrimSpace(vv[0])] == nil {
		return nil, herrors.ErrSysInternal.New("view [%s] join object [%s] not found", view.name, vv[0])
	} else {
		jn.object = this.objectsByKey[strings.TrimSpace(vv[0])]

		vv = strings.Split(vv[1], "=")
		if len(vv) != 2 {
			return nil, herrors.ErrSysInternal.New("invalid view [%s] join clause [%s]", view.name, strJoin)
		}

		lefts := strings.Split(strings.TrimSpace(vv[0]), ".")
		if len(lefts) != 2 {
			return nil, herrors.ErrSysInternal.New("invalid view [%s] join clause [%s]", view.name, strJoin)
		}
		if obj := this.objectsByKey[strings.TrimSpace(lefts[0])]; obj == nil {
			return nil, herrors.ErrSysInternal.New("object [%s] in view [%s] join clause [%s] not found", lefts[0], view.name, strJoin)
		} else {
			if field := obj.fieldMap[strings.TrimSpace(lefts[1])]; field == nil {
				return nil, herrors.ErrSysInternal.New("field [%s] in view %s join clause [%s] not found", lefts[1], view.name, strJoin)
			} else {
				jn.on.leftObj = obj
				jn.on.leftField = field
			}
		}

		rights := strings.Split(strings.TrimSpace(vv[1]), ".")
		if len(rights) != 2 {
			return nil, herrors.ErrSysInternal.New("invalid view [%s] join clause [%s]", view.name, strJoin)
		}
		if obj := this.objectsByKey[strings.TrimSpace(rights[0])]; obj == nil {
			return nil, herrors.ErrSysInternal.New("object [%s] in view [%s] join clause [%s] not found", rights[0], view.name, strJoin)
		} else {
			if field := obj.fieldMap[strings.TrimSpace(rights[1])]; field == nil {
				return nil, herrors.ErrSysInternal.New("field [%s] in view [%s] join clause [%s] not found", rights[1], view.name, strJoin)
			} else {
				jn.on.rightObj = obj
				jn.on.rightField = field
			}
		}
	}

	return &jn, nil
}

func (this *Service) parseView(o htypes.Any) (*view, *herrors.Error) {
	vw := &view{}
	vw.instance = o
	vw.name = hruntime.GetObjectName(o)

	manageable := false
	t := reflect.TypeOf(o)
	if t.Kind() == reflect.Ptr {
		t = reflect.ValueOf(o).Elem().Type()
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("data")
		if f.Type.Kind() == reflect.Struct && f.Type.Name() == "DataView" {
			manageable = true
			if tag == "" {
				return nil, herrors.ErrSysInternal.New("view [%s] field tag  not found", vw.name)
			} else {
				kv := hruntime.ParseTag(tag)
				for k, v := range kv {
					switch k {
					case "key":
						if v == "" {
							return nil, herrors.ErrSysInternal.New("view [%s] tag [key] not found", vw.name)
						}
						vw.key = v
					case "from":
						vw.from = this.objectsByKey[v]
						if vw.from == nil {
							return nil, herrors.ErrSysInternal.New("view [%s] join object not found:%s", vw.name, v)
						}
					case "join":
						ss := strings.Split(strings.TrimSpace(v), ",")
						for _, s := range ss {
							join, err := this.parseJoinOn(vw, s)
							if err != nil {
								return nil, err
							}
							vw.joins = append(vw.joins, join)
						}
					default:
						if v == "" {
							vw.key = k
						} else {
							return nil, herrors.ErrSysInternal.New("invalid view [%s] tag [%s]", vw.name, k)
						}
					}
				}
			}
			continue
		}

		if tag == "-" {
			continue
		}

		vf := newViewField()
		vf.kind = f.Type.Kind()
		if vf.kind == reflect.Ptr || vf.kind == reflect.Map || vf.kind == reflect.Slice { // 时间类型需要time.Time Struct
			continue
		}
		vf.name = f.Name

		if tag != "" {
			kv := hruntime.ParseTag(tag)
			for k, v := range kv {
				switch k {
				case "key":
					vf.key = v
				case "field":
					err := this.parseViewField(vw, vf, v)
					if err != nil {
						return nil, err
					}
				default:
					if v == "" {
						vf.key = k
					} else {
						return nil, herrors.ErrSysInternal.New("invalid view field [%s] tag [%s]", vf.name, k)
					}
				}
			}
		}

		if vf.key == "" {
			return nil, herrors.ErrSysInternal.New("view [%s] field [%s] tag [key]  not found", vw.name, f.Name)
		}

		vw.AddField(vf)
	}

	if manageable {
		for _, j := range vw.joins {
			if j.object.database != vw.from.database {
				return nil, herrors.ErrSysInternal.New(" view [%s] cross multi-databases: joined object [%s] not in the same database of from object database", vw.name, j.object.name)
			}
		}
		return vw, nil
	}

	return nil, herrors.ErrSysInternal.New("invalid view [%s], not derived from DataView")
}

func (this *Service) parseViewField(view *view, f *viewField, s string) *herrors.Error {
	ss := strings.Split(s, ".")
	if len(ss) != 2 {
		return herrors.ErrSysInternal.New("view [%s] has invalid field tag [%s]", view.name, s)
	}

	n := strings.TrimSpace(ss[0])
	var obj *object
	if obj = this.objectsByKey[n]; obj == nil {
		return herrors.ErrSysInternal.New("view [%s] field specify invalid object key [%s]", view.name, s)
	}

	n = strings.TrimSpace(ss[1])
	of := obj.fieldMap[n]
	if of == nil {
		return herrors.ErrSysInternal.New("view [%s] field specify invalid object field [%s]", view.name, s)
	}
	f.owner = &fieldOwner{
		object:   obj,
		fieldKey: of.key,
		fieldCol: of.col,
	}

	return nil
}

func (this *Service) getDB(key string) *gorm.DB {
	if key == "" {
		return this.defaultDB
	} else {
		return this.dbs[key]
	}
}

func (this *Service) getFieldValuesSetByFilters(fs *filter) map[string] /*objKey*/ htypes.Map {
	objFieldVals := make(map[string]htypes.Map)
	if fs == nil {
		return objFieldVals
	}

	for _, cond := range fs.conditions {
		if cond.compare == "=" {
			obj := cond.field.Owner().object
			if objFieldVals[obj.key] == nil {
				objFieldVals[obj.key] = make(htypes.Map)
			}
			objFieldVals[obj.key][cond.field.Owner().fieldKey] = cond.value
		}
	}

	for _, f := range fs.filters {
		vals := this.getFieldValuesSetByFilters(f)
		for o, vs := range vals {
			obj := objFieldVals[o]
			if obj == nil {
				objFieldVals[o] = make(htypes.Map)
			}
			for k, v := range vs {
				objFieldVals[o][k] = v
			}
		}
	}

	return objFieldVals
}

func (this *Service) parseRawFilter(fMap map[string]iField, fs *rawFilter) (*filter, *herrors.Error) {
	var fls *filter
	if len(fs.Conditions) > 0 {
		fls = &filter{
			or: fs.Or,
		}

		for _, str := range fs.Conditions {
			cond, herr := this.parseCondition(fMap, str)
			if herr != nil {
				return nil, herr
			}
			fls.conditions = append(fls.conditions, cond)
		}

		for _, rcf := range fs.Filters {
			v, herr := this.parseRawFilter(fMap, &rcf)
			if herr != nil {
				return nil, herr
			}
			fls.filters = append(fls.filters, v)
		}
	}
	return fls, nil
}

func (this *Service) createTable(tab string, obj *object) *herrors.Error {
	db := this.getDB(obj.database).Session(&gorm.Session{})
	db.Statement.Table = tab
	if err := db.Migrator().CreateTable(obj.instance); err != nil {
		return herrors.ErrSysInternal.New(err.Error())
	}

	return nil
}

func (this *Service) hasTable(tab string, obj *object) bool {
	db := this.getDB(obj.database).Session(&gorm.Session{})
	db.Statement.Table = tab
	return db.Migrator().HasTable(obj.instance)
}

func (this *Service) checkTabNamingFieldsValue(obj *object, vals htypes.Map) *herrors.Error {
	for _, f := range obj.tabNamingFieldKeys {
		if vals[f] == nil {
			return herrors.ErrUserInvalidAct.New("object [%s] table naming field [%s] required", obj.key, f)
		}
	}

	return nil
}

func (this *Service) parseOrderBy(fldMap map[string]iField, s string) (*ordering, *herrors.Error) {
	ss := strings.Split(s, ",")
	if len(ss) > 2 {
		return nil, herrors.ErrCallerInvalidRequest.New("invalid ordering parameter [%s]", s)
	}

	f := fldMap[strings.TrimSpace(ss[0])]
	if f == nil {
		return nil, herrors.ErrCallerInvalidRequest.New("invalid ordering parameter [%s], [%s] not found", s, ss[0])
	}
	order := &ordering{}
	order.object = f.Owner().object.key
	order.column = f.Owner().fieldCol
	if len(ss) == 1 || strings.ToUpper(strings.Trim(ss[1], " ")) == "ASC" {
		order.direction = "ASC"
	} else {
		order.direction = "DESC"
	}

	return order, nil
}
