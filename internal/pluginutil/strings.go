package pluginutil

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

// SafeDSN 会屏蔽密码，避免日志泄露敏感信息。
func SafeDSN(dsn string) string {
	if dsn == "" {
		return ""
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return dsn
	}
	if u.User != nil {
		if _, has := u.User.Password(); has {
			u.User = url.UserPassword(u.User.Username(), "xxxxx")
		}
	}
	return u.String()
}

// OptionalString 将空字符串转为 nil，便于写入数据库可空字段。
func OptionalString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ParseBoolOption 解析常见的布尔配置值。
func ParseBoolOption(v string) (value bool, ok bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "t", "yes", "y", "on":
		return true, true
	case "0", "false", "f", "no", "n", "off":
		return false, true
	default:
		return false, false
	}
}

// ParseTimeoutMS 从配置中解析毫秒超时。
func ParseTimeoutMS(v string) (time.Duration, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n <= 0 {
		return 0, false
	}
	return time.Duration(n) * time.Millisecond, true
}

// ProtocolString 将协议版本号转为字符串。
func ProtocolString(version int) string {
	switch version {
	case 3:
		return "MQTT/3.1"
	case 4:
		return "MQTT/3.1.1"
	case 5:
		return "MQTT/5.0"
	default:
		return ""
	}
}
