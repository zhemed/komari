package clients

import (
	"fmt"
	"strings"
)

// 节点级流量周期配置的解析与校验。
//
// 校验必须硬：越界/非法值一律返回错误、**不落库**，不能静默退化成"恒 0"
// （agent 侧 GetLastResetDate 越界返回 currentDate 导致区间恒空，是反面教材）。
// 键名与 Client 的 json tag 一致（admin editClient 直接收 JSON 对象）。

const (
	// TrafficResetDayKey / TrafficResetTimezoneKey 是节点编辑 RPC 的参数名。
	TrafficResetDayKey      = "traffic_reset_day"
	TrafficResetTimezoneKey = "traffic_reset_timezone"
)

// periodConfigState 描述节点级周期配置的**运行期**状态。
//
// 三态必须分开处理（design.md §4）：把"库里配置非法"混同于"明确禁用"，会在外部改坏配置时
// 顺手清空该节点已有的周期数据——记账失败不该销毁数据。
type periodConfigState int

const (
	// periodConfigDisabled：未配置周期（TrafficResetDay = 0），或节点已不存在。
	// 语义：不记账，并把周期列清空（"禁用 ⇒ 周期列为空"的不变量）。
	periodConfigDisabled periodConfigState = iota
	// periodConfigEnabled：配置合法，正常记账。
	periodConfigEnabled
	// periodConfigInvalid：库里是越界重置日或非法时区（正常保存路径会拦住，只可能来自外部改库）。
	// 语义：本次只跳过周期记账，**保留**库中现值，并留 WARN。
	periodConfigInvalid
)

func (s periodConfigState) String() string {
	switch s {
	case periodConfigEnabled:
		return "enabled"
	case periodConfigInvalid:
		return "invalid"
	default:
		return "disabled"
	}
}

// TrafficPeriodConfig 是一个节点的周期配置（归一化后的形态）。
type TrafficPeriodConfig struct {
	// ResetDay 0 = 禁用；1..31 = 每月重置日。
	ResetDay int
	// Timezone 归一化后的 IANA 名称；禁用时为空。
	Timezone string
	// Disable 为真表示 ResetDay == 0。
	Disable bool
}

func (c TrafficPeriodConfig) String() string {
	if c.Disable {
		return "disabled"
	}
	return fmt.Sprintf("day=%d tz=%s", c.ResetDay, periodTimezoneLabel(c.Timezone))
}

// NormalizeTrafficPeriodFields 校验并归一化 update 里的 traffic_reset_day /
// traffic_reset_timezone，把归一化结果写回 update。
//
// 返回 provided 表示请求里是否带了这两个字段（用于决定是否置再基线）。
// 校验失败返回明确错误，调用方据此拒绝保存。
//
// 只提供了其中一个字段时，另一个按"保持现值"处理：旧的 resetDay 由 oldResetDay 传入，
// 旧的时区由 currentTimezone 传入（见 ResolveTrafficPeriodConfig）。
func NormalizeTrafficPeriodFields(update map[string]interface{}, oldResetDay int, currentTimezone string) (provided bool, err error) {
	dayValue, dayProvided := update[TrafficResetDayKey]
	tzValue, tzProvided := update[TrafficResetTimezoneKey]
	if !dayProvided && !tzProvided {
		return false, nil
	}

	resetDay := oldResetDay
	if dayProvided && dayValue != nil {
		parsed, err := parseTrafficResetDay(dayValue)
		if err != nil {
			return true, err
		}
		resetDay = parsed
	}

	tzName := currentTimezone
	if tzProvided && tzValue != nil {
		text, ok := tzValue.(string)
		if !ok {
			return true, fmt.Errorf("%s must be a string IANA timezone name, got %T", TrafficResetTimezoneKey, tzValue)
		}
		tzName = text
	}

	if resetDay == 0 {
		// 禁用态：时区没有意义。允许带空时区（前端把整个配置清掉），但拒绝"只调时区、
		// 重置日仍是禁用"这种自相矛盾的请求——否则落库的是一个永远不会生效的时区。
		if tzProvided && strings.TrimSpace(tzName) != "" {
			return true, fmt.Errorf("%s requires %s to be 1..31 (got %d): a timezone has no effect while the period is disabled",
				TrafficResetTimezoneKey, TrafficResetDayKey, resetDay)
		}
		if dayProvided {
			update[TrafficResetDayKey] = 0
		}
		if tzProvided {
			update[TrafficResetTimezoneKey] = ""
		}
		return true, nil
	}

	normalizedTZ, err := ValidatePeriodConfig(resetDay, tzName)
	if err != nil {
		return true, err
	}
	if dayProvided {
		update[TrafficResetDayKey] = resetDay
	}
	if tzProvided {
		// 归一化：空串落库为 "UTC"（显式默认，不用服务器本地时区）。
		update[TrafficResetTimezoneKey] = normalizedTZ
	}
	return true, nil
}

// ResolveTrafficPeriodConfig 由旧配置与（可能只含部分字段的）update 求最终配置。
//
// 会先做校验与归一化（等价于 NormalizeTrafficPeriodFields），因此调用方拿到的一定是
// 可以直接落库/写审计的形态；非法请求在这里返回错误，调用方据此拒绝保存。
func ResolveTrafficPeriodConfig(uuid string, update map[string]interface{}) (TrafficPeriodConfig, error) {
	old, err := LoadTrafficPeriodConfig(uuid)
	if err != nil {
		return TrafficPeriodConfig{}, err
	}
	if _, err := NormalizeTrafficPeriodFields(update, old.ResetDay, old.Timezone); err != nil {
		return TrafficPeriodConfig{}, err
	}
	resolved := old
	if value, ok := update[TrafficResetDayKey]; ok {
		if day, ok := value.(int); ok {
			resolved.ResetDay = day
		}
	}
	if value, ok := update[TrafficResetTimezoneKey]; ok {
		if tz, ok := value.(string); ok {
			resolved.Timezone = tz
		}
	}
	resolved.Disable = resolved.ResetDay == 0
	if resolved.Disable {
		// 禁用 ⇒ 时区无意义，与落库归一化保持一致。
		resolved.Timezone = ""
	}
	return resolved, nil
}

// LoadTrafficPeriodConfig 从库里读该节点的周期配置；节点不存在时返回错误。
func LoadTrafficPeriodConfig(uuid string) (TrafficPeriodConfig, error) {
	client, err := GetClientByUUID(uuid)
	if err != nil {
		return TrafficPeriodConfig{}, err
	}
	return TrafficPeriodConfig{
		ResetDay: client.TrafficResetDay,
		Timezone: client.TrafficResetTimezone,
		Disable:  client.TrafficResetDay == 0,
	}, nil
}

// parseTrafficResetDay 解析 JSON 反序列化出来的数字（float64 / 各类整数）。
// 非整数（如 1.5）与非法类型一律报错，不做取整、不做静默降级。
func parseTrafficResetDay(value interface{}) (int, error) {
	switch typed := value.(type) {
	case float64:
		if typed != float64(int(typed)) {
			return 0, fmt.Errorf("%s must be an integer, got %v", TrafficResetDayKey, typed)
		}
		return int(typed), nil
	case float32:
		if typed != float32(int(typed)) {
			return 0, fmt.Errorf("%s must be an integer, got %v", TrafficResetDayKey, typed)
		}
		return int(typed), nil
	case int:
		return typed, nil
	case int64:
		return int(typed), nil
	case int32:
		return int(typed), nil
	default:
		return 0, fmt.Errorf("%s must be an integer between 0 (disabled) and 31, got %T", TrafficResetDayKey, value)
	}
}
