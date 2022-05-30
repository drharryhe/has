package hdatabaseplugin

/// 关系数据库访问plugin

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

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
	defaultDBInitAfterSecond = 30
	defaultReadTimeout       = 20
	defaultWriteTimeout      = 20

	dbTypeMysql      = "mysql"
	dbTypeClickhouse = "clickhouse"
	dbTypePostgres   = "postgres"
	dbTypeSqlLite    = "sqlite"
	defaultDBKey     = ""
)

var plugin = &Plugin{}

func New() *Plugin {
	return plugin

}

type Plugin struct {
	core.BasePlugin
	dbs     []*gorm.DB
	dbMap   map[string]*gorm.DB
	objects htypes.Map
	conf    DatabasePlugin
}

func (this *Plugin) Open(s core.IServer, ins core.IPlugin) *herrors.Error {
	if err := this.BasePlugin.Open(s, ins); err != nil {
		return err
	}

	this.dbMap = make(map[string]*gorm.DB)
	for i := 0; i < len(this.conf.Connections); i++ {
		db, herr := this.openDatabase(&this.conf.Connections[i])
		if herr != nil {
			return herr
		}
		this.dbMap[this.conf.Connections[i].Key] = db
		this.dbs = append(this.dbs, db)
	}
	return nil
}

func (this *Plugin) Capability() htypes.Any {
	return this.dbMap
}

func (this *Plugin) DefaultDB() *gorm.DB {
	return this.dbs[0]
}

func (this *Plugin) AddObjectsToDefaultDB(objs []interface{}) (*gorm.DB, *herrors.Error) {
	return this.AddObjects(defaultDBKey, objs)
}

func (this *Plugin) AddObjects(key string, objs []interface{}) (*gorm.DB, *herrors.Error) {
	db, herr := this.AutoMigrate(key, objs)
	if herr != nil {
		return nil, herr
	}

	for _, o := range objs {
		n := hruntime.GetObjectName(o)
		if this.objects[n] != nil {
			return nil, herrors.ErrSysInternal.New("object name [%s] duplicated", n)
		} else {
			this.objects[n] = o
		}
	}

	return db, nil
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
			Owner:             this,
			UpdateConfigItems: this.updateConfigItems,
		})
}

func (this *Plugin) Close() {
	for _, db := range this.dbs {
		v, _ := db.DB()
		if v != nil {
			_ = v.Close()
		}
	}
}

func (this *Plugin) AutoMigrate(key string, objs []interface{}) (*gorm.DB, *herrors.Error) {
	db, conn, herr := this.getConnAndDB(key)
	if herr != nil {
		return nil, herr
	}

	switch conn.Type {
	case dbTypeClickhouse:
		//TODO
	default:
		if err := db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(objs...); err != nil {
			return nil, herrors.ErrSysInternal.New(err.Error())
		}
	}

	return db, nil
}

func (this *Plugin) openDatabase(conn *connection) (*gorm.DB, *herrors.Error) {
	shouldCreateDB := false
	if conn.Reset {
		if err := this.dropDatabase(conn); err != nil {
			return nil, err
		}

		if err := this.createDatabase(conn); err != nil {
			return nil, err
		}

		conn.Reset = false
		_, err := this.EntityStub().Manage(core.ManageUpdateConfigItems, htypes.Map{fmt.Sprintf("Connections.%s.Reset", conn.Key): false})
		if err != nil {
			return nil, err
		}
		hconf.Save()
	}

LOOP:
	var err error
	var db *gorm.DB
	switch conn.Type {
	case dbTypeMysql:
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
			conn.User,
			conn.Pwd,
			conn.Server,
			conn.Port,
			conn.Name)
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err != nil {
			if strings.Index(err.Error(), "1049") < 0 || shouldCreateDB {
				return nil, herrors.ErrSysInternal.New(err.Error()).D("failed to open Plugin")
			}
			shouldCreateDB = true
		} else {
			shouldCreateDB = false
		}
		break
	case dbTypeSqlLite:
		break
	case dbTypePostgres:
	case dbTypeClickhouse:
		if conn.ReadTimeout == 0 {
			conn.ReadTimeout = defaultReadTimeout
		}
		if conn.WriteTimeout == 0 {
			conn.WriteTimeout = defaultWriteTimeout
		}

		dsn := fmt.Sprintf("tcp://%s:%ddatabase=%s&username=%s&password=%s&read_timeout=%d&write_timeout=%d",
			conn.Server, conn.Port, conn.Name, conn.User, conn.Pwd, conn.ReadTimeout, conn.WriteTimeout)
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err != nil {
			if strings.Index(err.Error(), "1049") < 0 || shouldCreateDB {
				return nil, herrors.ErrSysInternal.New(err.Error()).D("failed to open Plugin")
			}
			shouldCreateDB = true
		} else {
			shouldCreateDB = false
		}
	default:
		return nil, herrors.ErrSysInternal.New("data type [%s] not supported", conn.Type).D("failed to open Plugin")
	}

	if shouldCreateDB {
		if err := this.createDatabase(conn); err != nil {
			return nil, err
		}
		goto LOOP
	}

	this.objects = make(htypes.Map)
	if conn.InitDataAfterSecond == 0 {
		conn.InitDataAfterSecond = defaultDBInitAfterSecond
	}
	time.AfterFunc(time.Duration(conn.InitDataAfterSecond)*time.Second, func() {
		if err := this.initDatabaseDataIfNeeded(db, conn); err != nil {
			panic(herrors.ErrSysInternal.New(err.Error()))
		}
	})

	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(conn.MaxOpenConns)
	sqlDB.SetMaxIdleConns(conn.MaxIdleConns)

	return db, nil
}

func (this *Plugin) createDatabase(conn *connection) *herrors.Error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8", conn.User, conn.Pwd, conn.Server, conn.Port, conn.Type)
	db, err := sql.Open(conn.Type, dsn)
	if err != nil {
		return herrors.ErrSysInternal.New(err.Error()).D("failed to create database")
	}
	defer db.Close()

	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE `%s` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;", conn.Name))
	if err != nil {
		return herrors.ErrSysInternal.New(err.Error()).D("failed to create database")
	}
	return nil
}

func (this *Plugin) dropDatabase(conn *connection) *herrors.Error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8", conn.User, conn.Pwd, conn.Server, conn.Port, conn.Type)
	db, err := sql.Open(conn.Type, dsn)
	if err != nil {
		return herrors.ErrSysInternal.New(err.Error()).D("failed to drop database")
	}
	defer db.Close()

	_, err = db.Exec(fmt.Sprintf("DROP DATABASE `%s`;", conn.Name))
	if err != nil {
		if strings.Index(err.Error(), "1008") > 0 {
			return nil
		}
		return herrors.ErrSysInternal.New(err.Error()).D("failed to drop database")
	}
	return nil
}

func (this *Plugin) initDatabaseDataIfNeeded(db *gorm.DB, conn *connection) *herrors.Error {
	if !conn.InitData {
		return nil
	}

	if conn.InitDataDir == "" {
		return herrors.ErrSysInternal.New("DataDir of init database not configured")
	}

	if !hio.IsDirExist(conn.InitDataDir) {
		return herrors.ErrSysInternal.New("DataDir [%s] not found", conn.InitDataDir)
	}

	hlogger.Debug("start to init database data...")
	fs := hio.IteratorFiles(conn.InitDataDir, "sql")
	for _, f := range fs {
		hlogger.Debug("importing %s", f)
		file, err := hio.ReadFile(fmt.Sprintf("%s/%s", conn.InitDataDir, f))

		if err != nil {
			return herrors.ErrSysInternal.New(err.Error()).D("failed to read data file %s", f)
		}

		requests := strings.Split(string(file), ";")
		for _, request := range requests {
			if htext.IsEmptyLine(request) {
				continue
			}
			err = db.Exec(request).Error
			if err != nil {
				return herrors.ErrSysInternal.New(err.Error()).D("failed to exe sql %s", requests)
			}
		}
	}

	conn.InitData = false
	hconf.Save()
	hlogger.Debug("init database data done.")
	return nil
}

func (this *Plugin) getConnAndDB(key string) (*gorm.DB, *connection, *herrors.Error) {
	var db *gorm.DB
	var conn *connection

	if key == defaultDBKey {
		db = this.dbs[0]
		conn = &this.conf.Connections[0]
		return db, conn, nil
	} else {
		db = this.dbMap[key]
		if db == nil {
			return nil, nil, herrors.ErrCallerInvalidRequest.New("database [%s] not found", key)
		}

		for _, c := range this.conf.Connections {
			if c.Key == key {
				conn = &c
				break
			}
		}
		if conn == nil {
			return nil, nil, herrors.ErrSysInternal.New("database [%s] not found", key)
		}

		return db, conn, nil
	}
}

func (this *Plugin) updateConfigItems(params htypes.Map) *herrors.Error {
	ps := make(htypes.Map)
	conns := make(htypes.Map)
	for k, v := range params {
		if strings.HasPrefix(k, "Connections.") {
			ss := strings.Split(k, ".")
			if len(ss) != 3 {
				return herrors.ErrCallerInvalidRequest.New("invalid config name [%s]", k)
			}
			if conns[ss[1]] == nil {
				conns[ss[1]] = make(htypes.Map)
			}
			conns[ss[1]].(htypes.Map)[ss[2]] = v
		} else {
			ps[k] = v
		}
	}

	if len(ps) > 0 {
		if err := hruntime.SetObjectValues(&this.conf, ps); err != nil {
			return herrors.ErrCallerInvalidRequest.New(err.Error())
		}
	}

	for key, vals := range conns {
		found := false
		for i := range this.conf.Connections {
			if this.conf.Connections[i].Key == key {
				found = true
				if err := hruntime.SetObjectValues(&this.conf.Connections[i], vals.(htypes.Map)); err != nil {
					return herrors.ErrCallerInvalidRequest.New(err.Error())

				}
				break
			}
		}
		if !found {
			return herrors.ErrCallerInvalidRequest.New("connection key [%s] not found", key)
		}
	}

	return nil
}
