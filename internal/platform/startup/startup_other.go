//go:build !windows && !darwin

package startup

import "fmt"

func Set(enabled bool, executable string) error {
	if enabled {
		return fmt.Errorf("当前平台暂不支持开机启动")
	}
	return nil
}
