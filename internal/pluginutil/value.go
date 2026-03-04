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

// ParseTimeoutMS 从配置中解析毫秒超时。
func ParseTimeoutMS(v string) (time.Duration, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n <= 0 {
		return 0, false
	}
	return time.Duration(n) * time.Millisecond, true
}
