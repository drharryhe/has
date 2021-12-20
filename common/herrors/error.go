package herrors

import (
	"fmt"
	"github.com/drharryhe/has/utils/hencoder"
	"github.com/drharryhe/has/utils/hruntime"
	"github.com/drharryhe/has/utils/htext"
	"strings"
)

type Error struct {
	Code  int    `json:"code"`
	Desc  string `json:"desc"`
	ID    string `json:"id,omitempty"`
	Cause string `json:"cause,omitempty"`
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
	s := fmt.Sprintf("ERROR: \t%s", this.Desc)
	s = fmt.Sprintf("%s\r\n\t |\tCODE: %d", s, this.Code)
	if this.Cause != "" {
		s = fmt.Sprintf("%s\r\n\t |\tCAUSE: %s", s, this.Cause)
	}
	return s
}

func (this *Error) String() string {
	s := fmt.Sprintf("ERROR: \t%s", this.Desc)
	s = fmt.Sprintf("%s\r\n\t |\tCODE: %d", s, this.Code)
	if this.Cause != "" {
		s = fmt.Sprintf("%s\r\n\t |\tCAUSE: %s", s, this.Cause)
	}
	if len(this.stack) > 0 {
		var tmp string
		for _, s := range this.stack {
			tmp = fmt.Sprintf("%s \t\t %s\r\n", tmp, s)
		}
		s = fmt.Sprintf("%s\r\n\t |\tSTACK: \r\n%s", s, tmp)
	}
	return s
}

func (this *Error) C(format string, v ...interface{}) *Error {
	this.Cause = fmt.Sprintf(format, v...)
	this.Desc = ""
	this.stack = []string{}
	return this
}

func (this *Error) D(format string, v ...interface{}) *Error {
	this.Desc = fmt.Sprintf(format, v...)
	return this
}

func (this *Error) WithStack() *Error {
	this.stack = hruntime.SprintCallers(32, 4)
	return this
}

func (this *Error) WithIDs() *Error {
	var ids []string
	for _, s := range this.stack {
		index := strings.LastIndex(s, "-->")
		t := strings.TrimSpace(s[index+3:])
		ss := strings.Split(t, ":")
		f := hencoder.Md5ToString([]byte(ss[0]))
		n, _ := htext.ParseDecimal(ss[1])
		ids = append(ids, fmt.Sprintf("%s%04x", f, n))
	}
	this.ID = strings.Join(ids, ",")
	return this
}

func (this *Error) WithID() *Error {
	if len(this.stack) > 0 {
		index := strings.LastIndex(this.stack[0], "-->")
		t := strings.TrimSpace(this.stack[0][index+3:])
		ss := strings.Split(t, ":")
		f := hencoder.Md5ToString([]byte(ss[0]))
		n, _ := htext.ParseDecimal(ss[1])
		this.ID = fmt.Sprintf("%s%04x", f, n)
	}
	return this
}
