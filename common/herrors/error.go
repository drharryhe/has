package herrors

import (
	"fmt"
	"strings"

	"github.com/drharryhe/has/common/hconf"
	"github.com/drharryhe/has/common/hlogger"
	"github.com/drharryhe/has/utils/hconverter"
	"github.com/drharryhe/has/utils/hencoder"
	"github.com/drharryhe/has/utils/hruntime"
)

type Error struct {
	Code        int    `json:"code"`
	Desc        string `json:"desc"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Cause       string `json:"cause"`

	stack []string
}

func New(code int) *Error {
	return &Error{
		Code: code,
	}
}

func (this *Error) Equal(err *Error) bool {
	if err == nil {
		return false
	}

	return this.Code == err.Code
}

func (this *Error) Error() string {
	return this.Cause

}

func (this *Error) New(format string, v ...interface{}) *Error {
	err := &Error{
		Code:  this.Code,
		Cause: fmt.Sprintf(format, v...),
	}

	if hconf.IsDebug() {
		err.log()
	}

	return err
}

func (this *Error) D(format string, v ...interface{}) *Error {
	this.Desc = fmt.Sprintf(format, v...)

	return this
}

func (this *Error) log() {
	s := fmt.Sprintf("ERROR:%s", this.Desc)
	s = fmt.Sprintf("%s\r\n\t|CODE: %d", s, this.Code)
	if this.Cause != "" {
		s = fmt.Sprintf("%s\r\n\t|CAUSE: %s", s, this.Cause)
	}

	_ = this.withStack().withFingerprint()
	if len(this.stack) > 0 {
		var tmp string
		for _, s := range this.stack {
			tmp = fmt.Sprintf("%s\t| > %s\r\n", tmp, s)
		}
		s = fmt.Sprintf("%s\r\n\t|STACK: \r\n%s", s, tmp)
	}
	s = "\r\n" + s

	hlogger.Error(s)
}

func (this *Error) withStack() *Error {
	this.stack = hruntime.SprintCallers(32, 5)
	return this
}

func (this *Error) withFingerprint() *Error {
	var pointFPs []string
	for _, s := range this.stack {
		file, fun, line := this.parseStackItem(s)
		pointFPs = append(pointFPs, addPointFingerprint(file, fun, line))
	}
	this.Fingerprint = hencoder.Md5ToString([]byte(strings.Join(pointFPs, "")))
	addStackFingerprint(this.Fingerprint, pointFPs)
	return this
}

func (this *Error) parseStackItem(caller string) (file string, fun string, line int) {
	index := strings.LastIndex(caller, "-->")
	t := strings.TrimSpace(caller[index+3:])
	ss := strings.Split(t, ":")
	n, _ := hconverter.String2Decimal(ss[1])
	return ss[0], caller[:index], int(n)
}
