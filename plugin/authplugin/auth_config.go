package main

import "strings"

// parseBoolOption 解析 auth 插件配置中的布尔值。
func parseBoolOption(v string) (value bool, ok bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "t", "yes", "y", "on":
		return true, true
	case "0", "false", "f", "no", "n", "off":
		return false, true
	default:
		return false, false
	}
}
