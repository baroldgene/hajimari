package httprouteapps

import (
	"testing"

	"github.com/toboshii/hajimari/internal/kube/wrappers"
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

	scheme, hostname, port, found := gatewayListenerURL(gateway, parentRef, nil)
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

	scheme, hostname, port, found := gatewayListenerURL(gateway, map[string]interface{}{"name": "public"}, nil)
	if !found || scheme != "https" || hostname != "" || port != 443 {
		t.Fatalf("gatewayListenerURL() = (%q, %q, %d, %t), want (%q, %q, %d, %t)", scheme, hostname, port, found, "https", "", 443, true)
	}
}

func TestGatewayListenerURLUsesDeclaredPortWithoutPublicPortOverride(t *testing.T) {
	gateway := unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{
			"listeners": []interface{}{
				map[string]interface{}{"name": "websecure", "protocol": "HTTPS", "port": int64(8443), "hostname": "app.example.test"},
			},
		},
	}}

	scheme, hostname, port, found := gatewayListenerURL(gateway, map[string]interface{}{"name": "public"}, nil)
	if !found || scheme != "https" || hostname != "app.example.test" || port != 8443 {
		t.Fatalf("gatewayListenerURL() = (%q, %q, %d, %t), want (%q, %q, %d, %t)", scheme, hostname, port, found, "https", "app.example.test", 8443, true)
	}
}

func TestGatewayListenerURLUsesConfiguredPublicPort(t *testing.T) {
	gateway := unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{
			"listeners": []interface{}{
				map[string]interface{}{"name": "websecure", "protocol": "HTTPS", "port": int64(8443), "hostname": "app.example.test"},
			},
		},
	}}

	scheme, hostname, port, found := gatewayListenerURL(gateway, map[string]interface{}{"name": "public"}, map[string]int64{"websecure": 443})
	if !found || scheme != "https" || hostname != "app.example.test" || port != 443 {
		t.Fatalf("gatewayListenerURL() = (%q, %q, %d, %t), want (%q, %q, %d, %t)", scheme, hostname, port, found, "https", "app.example.test", 443, true)
	}

	route := unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{
			"hostnames": []interface{}{"app.example.test"},
			"rules": []interface{}{
				map[string]interface{}{
					"matches": []interface{}{
						map[string]interface{}{"path": map[string]interface{}{"value": "/dashboard"}},
					},
				},
			},
		},
	}}
	if got, want := wrappers.NewHTTPRouteWrapper(&route).GetURLWithGateway(scheme, hostname, port), "https://app.example.test/dashboard"; got != want {
		t.Fatalf("tile URL = %q, want %q", got, want)
	}
}
