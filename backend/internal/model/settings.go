package model

// ScanSettings 保存控制台可调整的扫描器参数。
type ScanSettings struct {
	ScanInterface    string   `bson:"scan_interface" json:"scanInterface"`
	MasscanInterface string   `bson:"masscan_interface" json:"masscanInterface"`
	MasscanRouterMAC string   `bson:"masscan_router_mac" json:"masscanRouterMac"`
	MasscanExtraArgs string   `bson:"masscan_extra_args" json:"masscanExtraArgs"`
	DefaultPorts     string   `bson:"default_ports" json:"defaultPorts"`
	DenyCIDRs        []string `bson:"deny_cidrs" json:"denyCidrs"`
}

// GeoSettings 保存地理库的开关与下载源，允许在控制台自定义镜像地址。
type GeoSettings struct {
	Enabled      bool   `bson:"enabled" json:"enabled"`
	AutoDownload bool   `bson:"auto_download" json:"autoDownload"`
	CityURL      string `bson:"city_url" json:"cityUrl"`
	ASNURL       string `bson:"asn_url" json:"asnUrl"`
}
