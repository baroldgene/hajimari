package httprouteapps

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGatewayListenerURLSelectsReferencedHTTPSListener(t *testing.T) {
	gateway := unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{
			"listeners": []interface{}{
				map[string]interface{}{"name": "http", "protocol": "HTTP", "port": int64(80), "hostname": "app.example.test"},
				map[string]interface{}{"name": "https", "protocol": "HTTPS", "port": int64(443), "hostname": "app.example.test"},
			},
		},
	}}
	parentRef := map[string]interface{}{"name": "public", "sectionName": "https"}

	scheme, hostname, found := gatewayListenerURL(gateway, parentRef)
	if !found {
		t.Fatal("gatewayListenerURL() did not find the referenced listener")
	}
	if scheme != "https" || hostname != "app.example.test" {
		t.Fatalf("gatewayListenerURL() = (%q, %q), want (%q, %q)", scheme, hostname, "https", "app.example.test")
	}
}

func TestGatewayListenerURLRejectsWildcardListenerHostname(t *testing.T) {
	gateway := unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{
			"listeners": []interface{}{
				map[string]interface{}{"name": "https", "protocol": "HTTPS", "port": int64(443), "hostname": "*.example.test"},
			},
		},
	}}

	scheme, hostname, found := gatewayListenerURL(gateway, map[string]interface{}{"name": "public"})
	if !found || scheme != "https" || hostname != "" {
		t.Fatalf("gatewayListenerURL() = (%q, %q, %t), want (%q, %q, %t)", scheme, hostname, found, "https", "", true)
	}
}
