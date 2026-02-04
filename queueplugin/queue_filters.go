package main

import "strings"

// topicMatch 实现 MQTT 通配符（+ 与 #）匹配。
func topicMatch(pattern, topic string) bool {
	if pattern == "#" {
		return true
	}
	pLevels := strings.Split(pattern, "/")
	tLevels := strings.Split(topic, "/")
	for i, p := range pLevels {
		if p == "#" {
			return i == len(pLevels)-1
		}
		if i >= len(tLevels) {
			return false
		}
		if p == "+" {
			continue
		}
		if p != tLevels[i] {
			return false
		}
	}
	return len(pLevels) == len(tLevels)
}

// matchAny 只要任一模式匹配就返回 true。
func matchAny(patterns []string, topic string) bool {
	for _, pattern := range patterns {
		if topicMatch(pattern, topic) {
			return true
		}
	}
	return false
}

// setContains 判断集合中是否包含指定值。
func setContains(set map[string]struct{}, value string) bool {
	if len(set) == 0 {
		return false
	}
	_, ok := set[value]
	return ok
}

// allowMessage 根据主题/用户/客户端/保留消息的过滤规则判断是否放行。
func allowMessage(topic, username, clientID string, retain bool) (bool, string) {
	if !cfg.includeRetained && retain {
		return false, "retained"
	}
	if len(cfg.excludeTopics) > 0 && matchAny(cfg.excludeTopics, topic) {
		return false, "exclude_topic"
	}
	if len(cfg.includeTopics) > 0 && !matchAny(cfg.includeTopics, topic) {
		return false, "not_included_topic"
	}
	if len(cfg.excludeUsers) > 0 && setContains(cfg.excludeUsers, username) {
		return false, "exclude_user"
	}
	if len(cfg.includeUsers) > 0 && !setContains(cfg.includeUsers, username) {
		return false, "not_included_user"
	}
	if len(cfg.excludeClients) > 0 && setContains(cfg.excludeClients, clientID) {
		return false, "exclude_client"
	}
	if len(cfg.includeClients) > 0 && !setContains(cfg.includeClients, clientID) {
		return false, "not_included_client"
	}
	return true, ""
}
