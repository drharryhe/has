package hdatasvs

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
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
			tab = fmt.Sprintf("%s_%s%v", tab, f, data[f])
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
