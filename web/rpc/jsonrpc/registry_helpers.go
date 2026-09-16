package jsonrpc

import "github.com/komari-monitor/komari/pkg/rpc"

// reg 是 admin 命名空间方法的注册便捷封装。
//
// 注：该助手原本定义在 admin.notification.go 里（上游的文件组织），通知系统整体移除后
// 迁移到本文件——它被所有 admin RPC 文件共用。
func reg(name string, h rpc.Handler, summary string) {
	RegisterWithGroupAndMeta(name, rpc.RoleAdmin, h, &rpc.MethodMeta{Name: "admin:" + name, Summary: summary})
}
