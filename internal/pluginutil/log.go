package pluginutil

import (
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
)

// FormatLogMessage 统一格式化日志为 "msg k=v ..." 形式，并按 key 排序保证输出稳定。
func FormatLogMessage(msg string, fields map[string]any) string {
	var b strings.Builder
	b.WriteString(msg)
	if fields == nil {
		return b.String()
	}

	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, " %s=%v", k, fields[k])
	}
	return b.String()
}

// ShouldSample 返回当前计数是否命中采样窗口，用于热点路径轻量限流日志。
func ShouldSample(counter *uint64, every uint64) bool {
	if every <= 1 {
		return true
	}
	return atomic.AddUint64(counter, 1)%every == 0
}
