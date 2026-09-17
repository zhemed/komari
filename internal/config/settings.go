package config

import "time"

type Settings struct {
	ID                     uint   `json:"id,omitempty"`                                        // 1
	Sitename               string `json:"sitename" default:"Komari"`                           // 站点名称，默认 "Komari"
	Description            string `json:"description" default:"A simple server monitor tool."` // 站点描述
	CorsOriginCheckEnabled bool   `json:"cors_origin_check_enabled" default:"true"`            // 是否启用 API CORS 跨域请求校验，默认 true
	CorsAllowedOrigins     string `json:"cors_allowed_origins" default:""`                     // API 跨域允许列表
	WsOriginCheckEnabled   bool   `json:"ws_origin_check_enabled" default:"true"`              // 是否校验 WebSocket Origin
	WsAllowedOrigins       string `json:"ws_allowed_origins" default:""`                       // WebSocket Origin 允许列表
	Theme                  string `json:"theme" default:"default"`                             // 主题名称，默认 'default'
	PrivateSite            bool   `json:"private_site" default:"false"`                        // 是否为私有站点，默认 false
	ApiKey                 string `json:"api_key" default:""`                                  // API 密钥，默认空字符串
	AutoDiscoveryKey       string `json:"auto_discovery_key" default:""`                       // 自动发现密钥
	ScriptDomain           string `json:"script_domain" default:""`                            // 自定义脚本域名
	SendIpAddrToGuest      bool   `json:"send_ip_addr_to_guest" default:"false"`               // 是否向访客页面发送 IP 地址，默认 false
	VisitorAuditEnabled    bool   `json:"visitor_audit_enabled" default:"false"`               // 是否允许公开访客事件写入审计日志，默认 false
	EulaAccepted           bool   `json:"eula_accepted" default:"false"`
	BaseScriptsURLKey      string `json:"base_scripts_url" default:""`
	// GeoIP 配置
	GeoIpEnabled  bool   `json:"geo_ip_enabled" default:"true"`
	GeoIpProvider string `json:"geo_ip_provider" default:"ipinfo"` // empty, mmdb, ip-api, geojs
	// OAuth 配置
	OAuthEnabled         bool   `json:"o_auth_enabled" default:"false"`
	OAuthProvider        string `json:"o_auth_provider" default:"github"`
	DisablePasswordLogin bool   `json:"disable_password_login" default:"false"`
	// 自定义美化
	CustomHead string `json:"custom_head" default:""`
	CustomBody string `json:"custom_body" default:""`

	// 通知
	UpdatedAt time.Time
}

const (
	SitenameKey               = "sitename"
	DescriptionKey            = "description"
	CorsOriginCheckEnabledKey = "cors_origin_check_enabled"
	CorsAllowedOriginsKey     = "cors_allowed_origins"
	WsOriginCheckEnabledKey   = "ws_origin_check_enabled"
	WsAllowedOriginsKey       = "ws_allowed_origins"
	ThemeKey                  = "theme"
	PrivateSiteKey            = "private_site"
	ApiKeyKey                 = "api_key"
	AutoDiscoveryKeyKey       = "auto_discovery_key"
	ScriptDomainKey           = "script_domain"
	SendIpAddrToGuestKey      = "send_ip_addr_to_guest"
	VisitorAuditEnabledKey    = "visitor_audit_enabled"
	EulaAcceptedKey           = "eula_accepted"
	BaseScriptsURLKey         = "base_scripts_url"
	GeoIpEnabledKey           = "geo_ip_enabled"
	GeoIpProviderKey          = "geo_ip_provider"
	OAuthEnabledKey           = "o_auth_enabled"
	OAuthProviderKey          = "o_auth_provider"
	DisablePasswordLoginKey   = "disable_password_login"
	CustomHeadKey             = "custom_head"
	CustomBodyKey             = "custom_body"

	UpdatedAtKey          = "updated_at"
	XtermjsSettingsKey    = "xtermjs_settings"
	ThemeMarketSourcesKey = "theme_market_sources"

	// 面板一键升级（见 internal/upgrade）：开关与目标仓库用扁平的独立键，
	// 与设置页其它项一致（通用设置接口直接写这两个键，写入前有格式校验）。
	ServerUpgradeEnabledKey = "server_upgrade_enabled"
	ServerUpdateRepoKey     = "server_update_repo"
	// ServerUpgradeDockerSocketKey：容器一键升级用的 docker socket 路径（默认 /var/run/docker.sock）。
	ServerUpgradeDockerSocketKey = "server_upgrade_docker_socket"
)
