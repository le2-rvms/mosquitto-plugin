package pluginutil

// ClientInfo 保存客户端事件中常用的标识字段。
type ClientInfo struct {
	ClientID string
	Username string
	Peer     string
	Protocol string
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
