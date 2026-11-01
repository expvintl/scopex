package store

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/expvintl/scopex/internal/model"
)

const scanSettingsID = "scanner"

// GetScanSettings 读取扫描器参数，未保存过时返回零值。
func (s *Store) GetScanSettings(ctx context.Context) (model.ScanSettings, error) {
	var doc struct {
		ScanInterface    string   `bson:"scan_interface"`
		MasscanInterface string   `bson:"masscan_interface"`
		MasscanRouterMAC string   `bson:"masscan_router_mac"`
		MasscanExtraArgs string   `bson:"masscan_extra_args"`
		DefaultPorts     string   `bson:"default_ports"`
		DenyCIDRs        []string `bson:"deny_cidrs"`
	}
	err := s.Settings.FindOne(ctx, bson.M{"_id": scanSettingsID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return model.ScanSettings{}, nil
	}
	if err != nil {
		return model.ScanSettings{}, err
	}
	return model.ScanSettings{
		ScanInterface:    doc.ScanInterface,
		MasscanInterface: doc.MasscanInterface,
		MasscanRouterMAC: doc.MasscanRouterMAC,
		MasscanExtraArgs: doc.MasscanExtraArgs,
		DefaultPorts:     doc.DefaultPorts,
		DenyCIDRs:        doc.DenyCIDRs,
	}, nil
}

func (s *Store) SaveScanSettings(ctx context.Context, in model.ScanSettings) error {
	_, err := s.Settings.UpdateOne(ctx, bson.M{"_id": scanSettingsID}, bson.M{
		"$set": bson.M{
			"scan_interface":     in.ScanInterface,
			"masscan_interface":  in.MasscanInterface,
			"masscan_router_mac": in.MasscanRouterMAC,
			"masscan_extra_args": in.MasscanExtraArgs,
			"default_ports":      in.DefaultPorts,
			"deny_cidrs":         in.DenyCIDRs,
			"updated_at":         time.Now(),
		},
	}, options.Update().SetUpsert(true))
	return err
}

const geoSettingsID = "geo"

// GetGeoSettings 读取地理库配置，未保存过时返回 found=false。
func (s *Store) GetGeoSettings(ctx context.Context) (model.GeoSettings, bool, error) {
	var doc struct {
		Enabled      bool   `bson:"enabled"`
		AutoDownload bool   `bson:"auto_download"`
		CityURL      string `bson:"city_url"`
		ASNURL       string `bson:"asn_url"`
	}
	err := s.Settings.FindOne(ctx, bson.M{"_id": geoSettingsID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return model.GeoSettings{}, false, nil
	}
	if err != nil {
		return model.GeoSettings{}, false, err
	}
	return model.GeoSettings{
		Enabled:      doc.Enabled,
		AutoDownload: doc.AutoDownload,
		CityURL:      doc.CityURL,
		ASNURL:       doc.ASNURL,
	}, true, nil
}

func (s *Store) SaveGeoSettings(ctx context.Context, in model.GeoSettings) error {
	_, err := s.Settings.UpdateOne(ctx, bson.M{"_id": geoSettingsID}, bson.M{
		"$set": bson.M{
			"enabled":       in.Enabled,
			"auto_download": in.AutoDownload,
			"city_url":      in.CityURL,
			"asn_url":       in.ASNURL,
			"updated_at":    time.Now(),
		},
	}, options.Update().SetUpsert(true))
	return err
}
