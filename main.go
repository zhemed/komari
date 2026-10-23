package main

import (
	"log/slog"
	// 内嵌 IANA 时区数据库：发布产物是静态链接（KOMARI_STATIC=1），裸机/systemd
	// 或精简容器可能没有 /usr/share/zoneinfo，此时 time.LoadLocation("Asia/Shanghai")
	// 会失败 ⇒ 节点级「流量重置时区」存不进去（见 database/clients/period.go）。
	// 官方容器镜像另外 apk add tzdata；这行是给没有系统时区库的部署形态兜底。
	_ "time/tzdata"

	"github.com/komari-monitor/komari/cmd"
	"github.com/komari-monitor/komari/utils"
	logger "github.com/komari-monitor/komari/utils/log"
)

func main() {
	if utils.VersionHash == "unknown" {
		logger.Setup(slog.LevelDebug)
	} else {
		logger.Setup(slog.LevelInfo)
	}

	logger.Infof("server", "Komari Monitor %s (hash: %s)", utils.CurrentVersion, utils.VersionHash)

	cmd.Execute()
}
