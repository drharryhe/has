package hrandom

import (
	"strings"

	uuid "github.com/satori/go.uuid"
)

func Uuid() string {
	return uuid.Must(uuid.NewV4(), nil).String()
}

func UuidWithoutDash() string {
	return strings.ReplaceAll(uuid.Must(uuid.NewV4(), nil).String(), "-", "")
}
