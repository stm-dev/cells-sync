package config

import (
	"os/runtime"
	"os/user"

	"github.com/kardianos/service"
)

var ServiceConfig = &service.Config{
	Name:        "com.pydio.CellsSync",
	DisplayName: "Cells Sync",
	Description: "Synchronization tool for Pydio Cells",
	Arguments:   []string{"start", "--headless"},
	Option: map[string]interface{}{
		"RunAtLoad": true,
	},
}

func init() {
	u, _ := user.Current()
	if runtime.GOOS == "linux" {
		ServiceConfig.UserName = u.Username
	}
}
