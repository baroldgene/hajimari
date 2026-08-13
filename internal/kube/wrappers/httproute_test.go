package wrappers

import (
	"reflect"
	"testing"

	"github.com/toboshii/hajimari/internal/kube/util"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestHTTPRouteWrapperExtractsURLAndBackendServices(t *testing.T) {
	route := unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{
			"hostnames": []interface{}{"app.example.test"},
			"rules": []interface{}{
				map[string]interface{}{
					"matches": []interface{}{
						map[string]interface{}{"path": map[string]interface{}{"value": "/dashboard/"}},
					},
					"backendRefs": []interface{}{
						map[string]interface{}{"name": "app"},
						map[string]interface{}{"name": "shared", "namespace": "platform"},
						map[string]interface{}{"name": "app"},
						map[string]interface{}{"name": "non-service", "kind": "ConfigMap"},
					},
				},
			},
		},
	}}
	route.SetName("app-route")
	route.SetNamespace("apps")

	wrapper := NewHTTPRouteWrapper(&route)
	if got, want := wrapper.GetURLWithGateway("https", ""), "https://app.example.test/dashboard"; got != want {
		t.Fatalf("GetURLWithGateway() = %q, want %q", got, want)
	}

	if got, want := wrapper.GetBackendServiceReferences(), []util.ServiceReference{
		{Namespace: "apps", Name: "app"},
		{Namespace: "platform", Name: "shared"},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("GetBackendServiceReferences() = %#v, want %#v", got, want)
	}
}

func TestHTTPRouteWrapperUsesExplicitURL(t *testing.T) {
	route := unstructured.Unstructured{}
	route.SetName("app-route")
	route.SetAnnotations(map[string]string{"hajimari.io/url": "https://external.example.test/app"})

	if got, want := NewHTTPRouteWrapper(&route).GetURLWithGateway("http", "gateway.example.test"), "https://external.example.test/app"; got != want {
		t.Fatalf("GetURLWithGateway() = %q, want %q", got, want)
	}
}
