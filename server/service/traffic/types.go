package traffic

// TrafficUsageInfo 用户月流量使用信息
type TrafficUsageInfo struct {
	MaxTrafficDown    float64 `json:"max_traffic_down"`     // 下行月配额（GB）
	MaxTrafficUp      float64 `json:"max_traffic_up"`       // 上行月配额（GB）
	UsedTrafficDown   int64   `json:"used_traffic_down"`    // 本月已用下行（字节）
	UsedTrafficUp     int64   `json:"used_traffic_up"`      // 本月已用上行（字节）
	UsedTrafficDownGB string  `json:"used_traffic_down_gb"` // 本月已用下行（人类可读）
	UsedTrafficUpGB   string  `json:"used_traffic_up_gb"`   // 本月已用上行（人类可读）
	IsLimitedDown     bool    `json:"is_limited_down"`      // 下行是否已限速
	IsLimitedUp       bool    `json:"is_limited_up"`        // 上行是否已限速
}
