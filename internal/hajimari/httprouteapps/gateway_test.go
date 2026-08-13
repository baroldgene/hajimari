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

	scheme, hostname, port, found := gatewayListenerURL(gateway, parentRef)
	if !found {
		t.Fatal("gatewayListenerURL() did not find the referenced listener")
	}
	if scheme != "https" || hostname != "app.example.test" || port != 443 {
		t.Fatalf("gatewayListenerURL() = (%q, %q, %d), want (%q, %q, %d)", scheme, hostname, port, "https", "app.example.test", 443)
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

	scheme, hostname, port, found := gatewayListenerURL(gateway, map[string]interface{}{"name": "public"})
	if !found || scheme != "https" || hostname != "" || port != 443 {
		t.Fatalf("gatewayListenerURL() = (%q, %q, %d, %t), want (%q, %q, %d, %t)", scheme, hostname, port, found, "https", "", 443, true)
	}
}

func TestGatewayListenerURLReturnsNonDefaultHTTPPort(t *testing.T) {
	gateway := unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{
			"listeners": []interface{}{
				map[string]interface{}{"name": "http", "protocol": "HTTP", "port": int64(8080), "hostname": "app.example.test"},
			},
		},
	}}

	scheme, hostname, port, found := gatewayListenerURL(gateway, map[string]interface{}{"name": "public"})
	if !found || scheme != "http" || hostname != "app.example.test" || port != 8080 {
		t.Fatalf("gatewayListenerURL() = (%q, %q, %d, %t), want (%q, %q, %d, %t)", scheme, hostname, port, found, "http", "app.example.test", 8080, true)
	}
}
