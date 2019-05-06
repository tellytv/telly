package main

import (
	"fmt"
	"net"

	"github.com/spf13/viper"
)

func getTCPAddr(key string) *net.TCPAddr {
	addr, addrErr := net.ResolveTCPAddr("tcp", viper.GetString(key))
	if addrErr != nil {
		panic(fmt.Errorf("error parsing address %s: %s", viper.GetString(key), addrErr))
	}
	return addr
}

func getDiscoveryData() DiscoveryData {
	return DiscoveryData{
		FriendlyName:    viper.GetString("discovery.device-friendly-name"),
		Manufacturer:    viper.GetString("discovery.device-manufacturer"),
		ModelNumber:     viper.GetString("discovery.device-model-number"),
		FirmwareName:    viper.GetString("discovery.device-firmware-name"),
		TunerCount:      viper.GetInt("iptv.streams"),
		FirmwareVersion: viper.GetString("discovery.device-firmware-version"),
		DeviceID:        viper.GetString("discovery.device-id"),
		DeviceAuth:      viper.GetString("discovery.device-auth"),
		BaseURL:         fmt.Sprintf("http://%s", viper.GetString("web.base-address")),
		LineupURL:       fmt.Sprintf("http://%s/lineup.json", viper.GetString("web.base-address")),
	}
}
