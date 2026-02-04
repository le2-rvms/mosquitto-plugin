package main

import "strings"

// parseList 拆分逗号分隔的列表。
func parseList(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

// parseSet 将列表转为集合，便于快速查找。
func parseSet(v string) map[string]struct{} {
	items := parseList(v)
	if len(items) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		set[item] = struct{}{}
	}
	return set
}

// parseFailMode 解析失败处理策略。
func parseFailMode(v string) (failMode, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "drop":
		return failModeDrop, true
	case "block":
		return failModeBlock, true
	case "disconnect":
		return failModeDisconnect, true
	default:
		return failModeDrop, false
	}
}

// failModeString 将失败策略转回配置字符串。
func failModeString(mode failMode) string {
	switch mode {
	case failModeDrop:
		return "drop"
	case failModeBlock:
		return "block"
	case failModeDisconnect:
		return "disconnect"
	default:
		return "drop"
	}
}
