package server

import (
	"context"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/utils"
	"github.com/komari-monitor/komari/web/public"
)

// Bootstrap initializes the data directory, primary database, and settings.
func (a *App) Bootstrap() error {
	if err := os.MkdirAll("./data/theme", os.ModePerm); err != nil {
		return fmt.Errorf("failed to create theme directory: %w", err)
	}

	dbcore.SetVersionID(utils.CurrentVersion + "-" + utils.VersionHash)
	if err := dbcore.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	a.dbReady = true
	a.addCleanup("database", func(context.Context) error { return dbcore.Close() })

	// 内置主题：缺失时从二进制恢复；全新实例（主题配置从未设置过）默认激活内置主题。
	// 已有安装/已选主题不覆盖（主题仍可走 data 目录覆盖更新）。
	if err := public.InstallBuiltinThemes(); err != nil {
		return fmt.Errorf("failed to install builtin themes: %w", err)
	}
	if err := public.ActivateBuiltinThemeIfFresh(); err != nil {
		return fmt.Errorf("failed to activate builtin theme: %w", err)
	}

	gin.SetMode(gin.ReleaseMode)
	settings, err := config.GetManyAs[config.Settings]()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}
	a.settings = settings
	return nil
}
