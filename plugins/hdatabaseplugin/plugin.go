package hdatabaseplugin

/// 关系数据库访问plugin

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/clickhouse"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"

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
	Conf    DatabasePlugin
}

func (this *Plugin) Open(s core.IServer, ins core.IPlugin) *herrors.Error {
	if err := this.BasePlugin.Open(s, ins); err != nil {
		return err
	}

	this.dbMap = make(map[string]*gorm.DB)
	for i := 0; i < len(this.Conf.Connections); i++ {
		db, herr := this.openDatabase(&this.Conf.Connections[i])
		if herr != nil {
			return herr
		}
		this.dbMap[this.Conf.Connections[i].Key] = db
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

func (this *Plugin) DB(key string) *gorm.DB {
	return this.dbMap[key]
}

func (this *Plugin) AddObjectsToDefaultDB(objs []interface{}) (*gorm.DB, *herrors.Error) {
	return this.AddObjects(defaultDBKey, objs)
}

func (this *Plugin) AddObjects(key string, objs []interface{}) (*gorm.DB, *herrors.Error) {
	var db *gorm.DB
	//if this.Conf.AutoMigrate {
	//	var err *herrors.Error
	//	db, err = this.AutoMigrate(key, objs)
	//	if err != nil {
	//		return nil, err
	//	}
	//}


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
	return &this.Conf
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
	case dbTypeClickhouse, dbTypePostgres:
		if err := db.AutoMigrate(objs...); err != nil {
			return nil, herrors.ErrSysInternal.New(err.Error())
		}
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
	var dbCfg gorm.Config

	if conn.SingularTable {
		dbCfg.NamingStrategy = schema.NamingStrategy{
			SingularTable: true,
		}
	}

	switch conn.Type {
	case dbTypeMysql:
		db, err = gorm.Open(mysql.Open(this.Dsn(conn, false)), &dbCfg)
		if err != nil {
			if strings.Index(err.Error(), "1049") < 0 || shouldCreateDB {
				return nil, herrors.ErrSysInternal.New(err.Error()).D("failed to open database")
			}
			shouldCreateDB = true
		} else {
			shouldCreateDB = false
		}
		break
	case dbTypePostgres:
		db, err = gorm.Open(postgres.Open(this.Dsn(conn, false)), &dbCfg)
		if err != nil {
			if strings.Index(err.Error(), "1049") < 0 || shouldCreateDB {
				return nil, herrors.ErrSysInternal.New(err.Error()).D("failed to open database")
			}
			shouldCreateDB = true
		} else {
			shouldCreateDB = false
		}
		break
	case dbTypeClickhouse:
		if conn.ReadTimeout == 0 {
			conn.ReadTimeout = defaultReadTimeout
		}
		if conn.WriteTimeout == 0 {
			conn.WriteTimeout = defaultWriteTimeout
		}

		db, err = gorm.Open(clickhouse.Open(this.Dsn(conn, false)), &dbCfg)
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
	db, err := sql.Open(conn.Type, this.Dsn(conn, true))
	if err != nil {
		return herrors.ErrSysInternal.New(err.Error()).D("failed to create database")
	}
	defer db.Close()

	switch conn.Type {
	case dbTypeMysql:
		_, err = db.Exec(fmt.Sprintf("CREATE DATABASE `%s` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;", conn.Name))
	case dbTypeClickhouse:
		_, err = db.Exec(fmt.Sprintf("CREATE DATABASE `%s`", conn.Name))
	default:
		return herrors.ErrSysInternal.New("unsupported database type")
	}

	if err != nil {
		return herrors.ErrSysInternal.New(err.Error()).D("failed to create database")
	}
	return nil
}

func (this *Plugin) dropDatabase(conn *connection) *herrors.Error {
	db, err := sql.Open(conn.Type, this.Dsn(conn, true))
	if err != nil {
		return herrors.ErrSysInternal.New(err.Error()).D("failed to drop database")
	}
	defer db.Close()

	_, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", conn.Name))
	if err != nil {
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
		conn = &this.Conf.Connections[0]
		return db, conn, nil
	} else {
		db = this.dbMap[key]
		if db == nil {
			return nil, nil, herrors.ErrCallerInvalidRequest.New("database [%s] not found", key)
		}

		for _, c := range this.Conf.Connections {
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
		if err := hruntime.SetObjectValues(&this.Conf, ps); err != nil {
			return herrors.ErrCallerInvalidRequest.New(err.Error())
		}
	}

	for key, vals := range conns {
		found := false
		for i := range this.Conf.Connections {
			if this.Conf.Connections[i].Key == key {
				found = true
				if err := hruntime.SetObjectValues(&this.Conf.Connections[i], vals.(htypes.Map)); err != nil {
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

func (this *Plugin) Dsn(conn *connection, new bool) string {
	switch conn.Type {
	case dbTypeClickhouse:
		if new {
			return fmt.Sprintf("tcp://%s:%d?database=default&username=%s&password=%s&read_timeout=%d&write_timeout=%d",
				conn.Server, conn.Port, conn.User, conn.Pwd, conn.ReadTimeout, conn.WriteTimeout)

		} else {
			return fmt.Sprintf("tcp://%s:%d?database=%s&username=%s&password=%s&read_timeout=%d&write_timeout=%d",
				conn.Server, conn.Port, conn.Name, conn.User, conn.Pwd, conn.ReadTimeout, conn.WriteTimeout)
		}
	case dbTypePostgres:
		if new {
			return fmt.Sprintf("host=%s user=%s password=%s dbname=default port=%d sslmode=disable TimeZone=Asia/Shanghai",
				conn.Server, conn.User, conn.Pwd, conn.Port)
		} else {
			return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=Asia/Shanghai",
				conn.Server, conn.User, conn.Pwd, conn.Name, conn.Port)
		}
	default:
		if new {
			return fmt.Sprintf("%s:%s@tcp(%s:%d)/",
				conn.User, conn.Pwd, conn.Server, conn.Port)
		} else {
			return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Asia%%2FShanghai",
				conn.User, conn.Pwd, conn.Server, conn.Port, conn.Name)

		}
	}
}
