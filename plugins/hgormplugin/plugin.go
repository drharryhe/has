package hgormplugin

/// 关系数据库访问plugin

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"

	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/core"
	"github.com/drharryhe/has/utils/hio"
	"github.com/drharryhe/has/utils/hruntime"
	"github.com/drharryhe/has/utils/htext"
)

const (
	defaultDBInitAfter = 30 * time.Second
)

var plugin = &Plugin{}

func New() *Plugin {
	return plugin

}

type Plugin struct {
	core.BasePlugin
	db      *gorm.DB
	objects htypes.Map
	conf    GormPlugin
}

func (this *Plugin) Open(s core.IServer, ins core.IPlugin) *herrors.Error {
	if err := this.BasePlugin.Open(s, ins); err != nil {
		return err
	}

	shouldCreateDB := false
	if this.conf.DBReset {
		if err := this.dropDatabase(); err != nil {
			return err
		}

		if err := this.createDatabase(); err != nil {
			return err
		}

		if err := this.EntityStub().UpdateConfigItems(htypes.Map{"DBReset": false}); err != nil {
			return err
		}
		hconf.Save()
	}

LOOP:
	var err error
	switch this.conf.DBType {
	case "mysql":
		this.db, err = gorm.Open("mysql",
			fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
				this.conf.DBUser,
				this.conf.DBPwd,
				this.conf.DBServer,
				this.conf.DBPort,
				this.conf.DBName))
		if err != nil {
			if strings.Index(err.Error(), "1049") < 0 || shouldCreateDB {
				return herrors.ErrSysInternal.New(err.Error()).D("failed to open Plugin")
			}
			shouldCreateDB = true
		} else {
			shouldCreateDB = false
			this.db.SingularTable(true)
		}
		break
	case "pg":
		this.db, err = gorm.Open("postgres", "host=%s user=%s dbname=%s sslmode=disable password=%s",
			this.conf.DBServer,
			this.conf.DBUser,
			this.conf.DBName,
			this.conf.DBPwd)
		break
	default:
		return herrors.ErrSysInternal.New("data type %s not supoorted", this.conf.DBType).D("failed to open Plugin")
	}

	if shouldCreateDB {
		if err := this.createDatabase(); err != nil {
			return err
		}
		goto LOOP
	}

	if d := this.conf.DBMaxOpenConns; d > 0 {
		this.db.DB().SetMaxOpenConns(d)
	}
	if d := this.conf.DBMaxIdleConns; d > 0 {
		this.db.DB().SetMaxIdleConns(d)
	}

	this.objects = make(htypes.Map)

	if this.conf.DBInitAfter == 0 {
		this.conf.DBInitAfter = int(defaultDBInitAfter)
	}
	time.AfterFunc(time.Duration(this.conf.DBInitAfter), func() {
		if err := this.initDatabaseDataIfNeeded(); err != nil {
			panic(herrors.ErrSysInternal.New(err.Error()))
		}
	})

	return nil
}

func (this *Plugin) Close() {
	if this.db != nil {
		_ = this.db.Close()
	}
}

func (this *Plugin) Capability() htypes.Any {
	return this.db
}

func (this *Plugin) createDatabase() *herrors.Error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8", this.conf.DBUser, this.conf.DBPwd, this.conf.DBServer, this.conf.DBPort, this.conf.DBType)
	db, err := sql.Open(this.conf.DBType, dsn)
	if err != nil {
		return herrors.ErrSysInternal.New(err.Error()).D("failed to create database")
	}
	defer db.Close()

	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE `%s` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;", this.conf.DBName))
	if err != nil {
		return herrors.ErrSysInternal.New(err.Error()).D("failed to create database")
	}
	return nil
}

func (this *Plugin) dropDatabase() *herrors.Error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8", this.conf.DBUser, this.conf.DBPwd, this.conf.DBServer, this.conf.DBPort, this.conf.DBType)
	db, err := sql.Open(this.conf.DBType, dsn)
	if err != nil {
		return herrors.ErrSysInternal.New(err.Error()).D("failed to drop database")
	}
	defer db.Close()

	_, err = db.Exec(fmt.Sprintf("DROP DATABASE `%s`;", this.conf.DBName))
	if err != nil {
		if strings.Index(err.Error(), "1008") > 0 {
			return nil
		}
		return herrors.ErrSysInternal.New(err.Error()).D("failed to drop database")
	}
	return nil
}

func (this *Plugin) initDatabaseDataIfNeeded() *herrors.Error {
	if !this.conf.DBInitDB {
		return nil
	}

	if this.conf.DBDataDir == "" {
		return herrors.ErrSysInternal.New("DataDir of init database not configured")
	}

	if !hio.IsDirExist(this.conf.DBDataDir) {
		return herrors.ErrSysInternal.New("DataDir %s not found", this.conf.DBDataDir)
	}

	hlogger.Debug("start to init database data...")
	fs := hio.IteratorFiles(this.conf.DBDataDir, "sql")
	for _, f := range fs {
		hlogger.Debug("importing %s", f)
		file, err := hio.ReadFile(fmt.Sprintf("%s/%s", this.conf.DBDataDir, f))

		if err != nil {
			return herrors.ErrSysInternal.New(err.Error()).D("failed to read data file %s", f)
		}

		requests := strings.Split(string(file), ";")
		for _, request := range requests {
			if htext.IsEmptyLine(request) {
				continue
			}
			err = this.db.Exec(request).Error
			if err != nil {
				return herrors.ErrSysInternal.New(err.Error()).D("failed to exe sql %s", requests)
			}
		}
	}

	this.conf.DBInitDB = false
	hconf.Save()
	hlogger.Debug("init database data done.")
	return nil
}

func (this *Plugin) AddObjects(objs []interface{}) (*gorm.DB, *herrors.Error) {
	if err := this.db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(objs...).Error; err != nil {
		return nil, herrors.ErrSysInternal.New(err.Error())
	}

	for _, o := range objs {
		n := hruntime.GetObjectName(o)
		if this.objects[n] != nil {
			return nil, herrors.ErrSysInternal.New("object name [%s] duplicated", n)
		} else {
			this.objects[n] = o
		}
	}

	return this.db, nil
}

func (this *Plugin) Objects() htypes.Map {
	return this.objects
}

func (this *Plugin) Config() core.IEntityConf {
	return &this.conf
}

func (this *Plugin) EntityStub() *core.EntityStub {
	return core.NewEntityStub(
		&core.EntityStubOptions{
			Owner:       this,
			Ping:        nil,
			GetLoad:     nil,
			ResetConfig: nil,
		})
}
