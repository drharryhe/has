package core

import (
	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/herrors"
	"github.com/drharryhe/has/common/htypes"
	"github.com/drharryhe/has/utils/hrandom"
	"github.com/drharryhe/has/utils/hruntime"
)

type BasePacker struct {
	class    string
	instance IAPIDataPacker
	server   IServer
}

func (this *BasePacker) Open(gw IAPIGateway, ins IAPIDataPacker) *herrors.Error {
	this.instance = ins
	this.class = hruntime.GetObjectName(ins.(IEntity).Config())
	this.server = gw.Server()
	hconf.Load(ins.(IEntity).Config())

	return nil
}

func (this *BasePacker) Close() {
}

func (this *BasePacker) Server() IServer {
	return this.server
}

func (this *BasePacker) Class() string {
	return this.class
}

func (this *BasePacker) Marshal(data htypes.Any) ([]byte, *herrors.Error) {
	return nil, herrors.ErrSysUnhandled.New("Marshal not implemented")
}

func (this *BasePacker) Unmarshal(bytes []byte) (htypes.Any, *herrors.Error) {
	return nil, herrors.ErrSysUnhandled.New("Unmarshal not implemented")
}

func (this *BasePacker) EntityMeta() *EntityMeta {
	if this.instance.(IEntity).Config().GetEID() == "" {
		this.instance.(IEntity).Config().SetEID(hrandom.UuidWithoutDash())
		hconf.Save()
	}

	return &EntityMeta{
		ServerEID: this.server.(IEntity).Config().GetEID(),
		EID:       this.instance.(IEntity).Config().GetEID(),
		Type:      EntityTypeDataPacker,
		Class:     this.class,
	}
}

func (this *BasePacker) EntityStub() *EntityStub {
	return NewEntityStub(
		&EntityStubOptions{
			Owner:       this.instance.(IEntity),
			Ping:        nil,
			GetLoad:     nil,
			ResetConfig: nil,
		})
}

//// 要被具体的Connector 调用
//func (this *BasePacker) GetConfigItem(ps htypes.Map) (htypes.Any, *herrors.Error) {
//	name, val, err := this.instance.(IEntity).Config().(*EntityConfBase).GetItem(ps)
//	if err == nil {
//		return val, nil
//	} else if err.Code != herrors.ECodeSysUnhandled {
//		return nil, err
//	}
//
//	switch name {
//	case "Lang":
//		return this.instance.(IEntity).Config().(*ConnectorConf).Lang, nil
//	case "Packer":
//		return this.instance.(IEntity).Config().(*ConnectorConf).Packer, nil
//	}
//
//	return nil, herrors.ErrSysUnhandled
//}
//
//// 需要被具体的Connector 调用
//func (this *BasePacker) UpdateConfigItems(ps htypes.Map) *herrors.Error {
//
//	return nil
//}
