package config

import (
	"reflect"
	"testing"

	"github.com/spf13/viper"
)

func TestGetConfigReadsGatewayListenerPorts(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("gatewayListenerPorts", map[string]interface{}{
		"web":       80,
		"websecure": 443,
	})

	config, err := GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() returned an error: %v", err)
	}
	if want := map[string]int64{"web": 80, "websecure": 443}; !reflect.DeepEqual(config.GatewayListenerPorts, want) {
		t.Fatalf("GatewayListenerPorts = %#v, want %#v", config.GatewayListenerPorts, want)
	}
}
