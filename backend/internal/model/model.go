package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Host 是测绘结果的存储单元：一个 ip:port/proto 对应一条文档。
type Host struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	IP         string             `bson:"ip" json:"ip"`
	IPNum      uint32             `bson:"ipnum" json:"-"`
	Port       int                `bson:"port" json:"port"`
	Protocol   string             `bson:"protocol" json:"protocol"`
	Service    Service            `bson:"service" json:"service"`
	Banner     string             `bson:"banner" json:"banner"`
	Title      string             `bson:"title" json:"title"`
	Headers    map[string]string  `bson:"headers" json:"headers"`
	HeaderText string             `bson:"header_text" json:"-"`
	StatusCode int                `bson:"status_code" json:"statusCode"`
	Favicon    *Favicon           `bson:"favicon,omitempty" json:"favicon,omitempty"`
	FaviconImg []byte             `bson:"favicon_img,omitempty" json:"-"`
	TLS        *TLSInfo           `bson:"tls,omitempty" json:"tls,omitempty"`
	Apps       []string           `bson:"apps" json:"apps"`
	CPE        []string           `bson:"cpe,omitempty" json:"cpe,omitempty"`
	Device     string             `bson:"device,omitempty" json:"device,omitempty"`
	OS         string             `bson:"os,omitempty" json:"os,omitempty"`
	Hostnames  []string           `bson:"hostnames,omitempty" json:"hostnames,omitempty"`
	Domains    []string           `bson:"domains,omitempty" json:"domains,omitempty"`
	Geo        Geo                `bson:"geo" json:"geo"`
	ASN        *ASN               `bson:"asn,omitempty" json:"asn,omitempty"`
	Scripts    map[string]string  `bson:"scripts,omitempty" json:"scripts,omitempty"`
	Tags       []string           `bson:"tags" json:"tags"`
	Source     string             `bson:"source" json:"source"`
	FirstSeen  time.Time          `bson:"first_seen" json:"firstSeen"`
	LastSeen   time.Time          `bson:"last_seen" json:"lastSeen"`
}

type Service struct {
	Name       string `bson:"name" json:"name"`
	Product    string `bson:"product,omitempty" json:"product,omitempty"`
	Version    string `bson:"version,omitempty" json:"version,omitempty"`
	ExtraInfo  string `bson:"extra_info,omitempty" json:"extraInfo,omitempty"`
	OSType     string `bson:"os_type,omitempty" json:"osType,omitempty"`
	DeviceType string `bson:"device_type,omitempty" json:"deviceType,omitempty"`
	Method     string `bson:"method,omitempty" json:"method,omitempty"`
	Confidence int    `bson:"confidence,omitempty" json:"confidence,omitempty"`
	Banner     string `bson:"banner,omitempty" json:"banner,omitempty"`
}

type Favicon struct {
	MD5  string `bson:"md5" json:"md5"`
	MMH3 int32  `bson:"mmh3" json:"mmh3"`
	URL  string `bson:"url,omitempty" json:"url,omitempty"`
	Size int    `bson:"size,omitempty" json:"size,omitempty"`
}

type TLSInfo struct {
	Subject    string    `bson:"subject,omitempty" json:"subject,omitempty"`
	Issuer     string    `bson:"issuer,omitempty" json:"issuer,omitempty"`
	CommonName string    `bson:"common_name,omitempty" json:"commonName,omitempty"`
	SANs       []string  `bson:"sans,omitempty" json:"sans,omitempty"`
	Serial     string    `bson:"serial,omitempty" json:"serial,omitempty"`
	Version    string    `bson:"version,omitempty" json:"version,omitempty"`
	Cipher     string    `bson:"cipher,omitempty" json:"cipher,omitempty"`
	NotBefore  time.Time `bson:"not_before,omitempty" json:"notBefore,omitempty"`
	NotAfter   time.Time `bson:"not_after,omitempty" json:"notAfter,omitempty"`
}

type Geo struct {
	CountryCode string  `bson:"country_code,omitempty" json:"countryCode,omitempty"`
	Country     string  `bson:"country,omitempty" json:"country,omitempty"`
	Province    string  `bson:"province,omitempty" json:"province,omitempty"`
	City        string  `bson:"city,omitempty" json:"city,omitempty"`
	ISP         string  `bson:"isp,omitempty" json:"isp,omitempty"`
	Latitude    float64 `bson:"latitude,omitempty" json:"latitude,omitempty"`
	Longitude   float64 `bson:"longitude,omitempty" json:"longitude,omitempty"`
	Private     bool    `bson:"private,omitempty" json:"private,omitempty"`
}

type ASN struct {
	Number int    `bson:"number,omitempty" json:"number,omitempty"`
	Org    string `bson:"org,omitempty" json:"org,omitempty"`
}

const (
	TaskPending     = "pending"
	TaskRunning     = "running"
	TaskDone        = "done"
	TaskFailed      = "failed"
	TaskCanceled    = "canceled"
	TaskInterrupted = "interrupted"
)

type Task struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name       string             `bson:"name" json:"name"`
	Targets    []string           `bson:"targets" json:"targets"`
	Ports      string             `bson:"ports" json:"ports"`
	Rate       int                `bson:"rate" json:"rate"`
	Tool       string             `bson:"tool" json:"tool"`
	Deep       bool               `bson:"deep" json:"deep"`
	Status     string             `bson:"status" json:"status"`
	Lang       string             `bson:"lang" json:"lang,omitempty"`
	Phase      string             `bson:"phase" json:"phase"`
	Progress   float64            `bson:"progress" json:"progress"`
	TotalHosts int                `bson:"total_hosts" json:"totalHosts"`
	Stats      TaskStats          `bson:"stats" json:"stats"`
	Error      string             `bson:"error,omitempty" json:"error,omitempty"`
	CreatedBy  string             `bson:"created_by" json:"createdBy"`
	CreatedAt  time.Time          `bson:"created_at" json:"createdAt"`
	StartedAt  *time.Time         `bson:"started_at,omitempty" json:"startedAt,omitempty"`
	FinishedAt *time.Time         `bson:"finished_at,omitempty" json:"finishedAt,omitempty"`
	UpdatedAt  time.Time          `bson:"updated_at" json:"updatedAt"`
	LogSeq     int64              `bson:"log_seq" json:"-"`
}

type TaskStats struct {
	HostsScanned int `bson:"hosts_scanned" json:"hostsScanned"`
	OpenPorts    int `bson:"open_ports" json:"openPorts"`
	NewAssets    int `bson:"new_assets" json:"newAssets"`
	Updated      int `bson:"updated" json:"updated"`
	Errors       int `bson:"errors" json:"errors"`
}

type TaskLog struct {
	ID     primitive.ObjectID `bson:"_id,omitempty" json:"-"`
	TaskID primitive.ObjectID `bson:"task_id" json:"-"`
	Seq    int64              `bson:"seq" json:"seq"`
	Level  string             `bson:"level" json:"level"`
	Msg    string             `bson:"msg" json:"msg"`
	At     time.Time          `bson:"at" json:"at"`
}

type User struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Username     string             `bson:"username" json:"username"`
	PasswordHash string             `bson:"password_hash" json:"-"`
	Role         string             `bson:"role" json:"role"`
	// SessionNonce 是当前会话标识，随令牌一起签发；改密码或删库重建账号都会换掉它，
	// 旧令牌在新值出现后立即失效。
	SessionNonce string     `bson:"session_nonce,omitempty" json:"-"`
	APIKey       string     `bson:"api_key,omitempty" json:"apiKey,omitempty"`
	CreatedAt    time.Time  `bson:"created_at" json:"createdAt"`
	LastLoginAt  *time.Time `bson:"last_login_at,omitempty" json:"lastLoginAt,omitempty"`
}
