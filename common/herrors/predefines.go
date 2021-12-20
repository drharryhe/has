package herrors

// 1000以下的错误编码，为HAS框架保留

const (
	ECodeOK      = 0
	ECodeUnknown = -1 //未知错误编码

	// 服务器错误代码
	ECodeSysInternal  = 101 //服务器内部错误
	ECodeSysBusy      = 102 //服务器忙
	ECodeSysUnhandled = 103 //未处理. 这种报错多用于父类向子类返回，以便子类继续处理，

	// 调用方错误代码
	ECodeCallerInvalidRequest     = 201 //无效请求
	ECodeCallerUnauthorizedAccess = 202 //非法请求

	// 用户端错误
	ECodeUserInvalidAct      = 301 // 无效用户行为
	ECodeUserUnauthorizedAct = 302 // 非法用户行为
)

var (
	ErrOK = New(ECodeOK)

	// Backend errors
	ErrSysInternal  = New(ECodeSysInternal)
	ErrSysBusy      = New(ECodeSysBusy)
	ErrSysUnhandled = New(ECodeSysUnhandled)

	// Caller errors
	ErrCallerInvalidRequest     = New(ECodeCallerInvalidRequest)
	ErrCallerUnauthorizedAccess = New(ECodeCallerUnauthorizedAccess)

	// User errors
	ErrUserInvalidAct      = New(ECodeUserInvalidAct)
	ErrUserUnauthorizedAct = New(ECodeUserUnauthorizedAct)
)
