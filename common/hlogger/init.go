package hlogger

import "fmt"

const (
	defaultLogFile = "has.log"
)

func Init(outputs []string, args ...interface{}) {
	for _, o := range outputs {
		switch o {
		case AdapterFile:
			var filePath string
			if len(args) == 0 {
				filePath = defaultLogFile
			} else {
				filePath = args[0].(string)
			}
			if err := SetLogger(AdapterMultiFiles, fmt.Sprintf("{\"filename\":\"%s\"}", filePath)); err != nil {
				fmt.Println(err)
				panic("init hlogger failed.")
			}
		case AdapterMultiFiles:
			if err := SetLogger(AdapterMultiFiles); err != nil {
				panic("init hlogger failed.")
			}
		default:
			if err := SetLogger(AdapterConsole); err != nil {
				fmt.Println(err)
				panic("init hlogger failed.")
			}

		}
	}
	if len(outputs) == 0 {
		if err := SetLogger(AdapterConsole); err != nil {
			panic("init hlogger failed.")
		}
	}

	EnableFuncCallDepth(true)
	SetLogFuncCallDepth(3)
}
