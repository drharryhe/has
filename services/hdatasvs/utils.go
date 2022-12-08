package hdatasvs

import (
	"fmt"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"gorm.io/gorm"
	"reflect"
)

func (this *Service) ObjectConditionValues(key string, conds []string) (htypes.Map, *herrors.Error) {
	obj := this.objectsByKey[key]
	ret := make(htypes.Map)
	for _, cond := range conds {
		c, herr := this.parseCondition(obj.iFieldMap, cond)
		if herr != nil {
			return nil, herr
		}
		ret[c.field.Key()] = c.value
	}

	return ret, nil
}

func (this *Service) ViewConditionValues(key string, conds []string) (htypes.Map, *herrors.Error) {
	vw := this.viewsWithKey[key]
	ret := make(htypes.Map)
	for _, cond := range conds {
		c, herr := this.parseCondition(vw.iFieldMap, cond)
		if herr != nil {
			return nil, herr
		}
		ret[c.field.Key()] = c.value
	}

	return ret, nil
}

func (this *Service) DbOfObject(key string) *gorm.DB {
	obj := this.objectsByKey[key]
	if obj == nil {
		return nil
	}
	if key == "" {
		return this.defaultDB
	} else {
		return this.dbs[obj.database]
	}
}

func (this *Service) Object(key string) *object {
	return this.objectsByKey[key]
}

func (this *Service) CheckTableName(o *object, data htypes.Map, createIfNotExist bool) (string, *herrors.Error) {
	tab := o.tableName
	if len(o.tabNamingFieldKeys) == 0 {
		return tab, nil
	}

	for _, f := range o.tabNamingFieldKeys {
		if data[f] == nil {
			return "", herrors.ErrSysInternal.New("table naming field [%s] value required", f)
		} else {
			var result string
			SWLevel1:
			switch o.iFieldMap[f].Kind() {
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				switch reflect.TypeOf(data[f]).Kind() {
				case reflect.Int64:
					result = fmt.Sprintf("%d", data[f].(int64))
					break SWLevel1
				default:
					result = fmt.Sprintf("%.0f", data[f].(float64))
				}
			case reflect.String:
				result = data[f].(string)
			default:
				result = fmt.Sprintf("%v", data[f])
			}
			tab = fmt.Sprintf("%s_%s%v", tab, f, result)
		}
	}

	if this.tablesOfDatabases[o.database] == nil {
		this.tablesOfDatabases[o.database] = make(map[string]bool)
	}

	if this.tablesOfDatabases[o.database][tab] != true {
		if !this.hasTable(tab, o) {
			if createIfNotExist {
				if herr := this.createTable(tab, o); herr != nil {
					return "", herr
				}
			}
		}
		this.tablesOfDatabases[o.database][tab] = true
	}

	return tab, nil
}
