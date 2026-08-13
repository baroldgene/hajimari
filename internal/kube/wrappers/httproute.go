package wrappers

import (
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/toboshii/hajimari/internal/annotations"
	"github.com/toboshii/hajimari/internal/kube/util"
	utilStrings "github.com/toboshii/hajimari/internal/util/strings"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// HTTPRouteWrapper wraps a Gateway API HTTPRoute object.
type HTTPRouteWrapper struct {
	route *unstructured.Unstructured
}

// NewHTTPRouteWrapper creates an HTTPRoute wrapper.
func NewHTTPRouteWrapper(route *unstructured.Unstructured) *HTTPRouteWrapper {
	return &HTTPRouteWrapper{route: route}
}

// GetAnnotationValue extracts an annotation value from the wrapped route.
func (hw *HTTPRouteWrapper) GetAnnotationValue(annotationKey string) string {
	return hw.route.GetAnnotations()[annotationKey]
}

// GetName extracts the application name from the route.
func (hw *HTTPRouteWrapper) GetName() string {
	if name := hw.GetAnnotationValue(annotations.HajimariAppNameAnnotation); name != "" {
		return name
	}
	return hw.route.GetName()
}

// GetNamespace extracts the route namespace.
func (hw *HTTPRouteWrapper) GetNamespace() string {
	return hw.route.GetNamespace()
}

// GetGroup extracts the application group from the route.
func (hw *HTTPRouteWrapper) GetGroup() string {
	if group := hw.GetAnnotationValue(annotations.HajimariGroupAnnotation); group != "" {
		return group
	}
	return hw.GetNamespace()
}

// GetInfo extracts the optional app info line.
func (hw *HTTPRouteWrapper) GetInfo() string {
	return hw.GetAnnotationValue(annotations.HajimariInfoAnnotation)
}

// GetStatusCheckEnabled reports whether endpoint replica status should be shown.
func (hw *HTTPRouteWrapper) GetStatusCheckEnabled() bool {
	if value := hw.GetAnnotationValue(annotations.HajimariStatusCheckEnabledAnnotation); value != "" {
		return utilStrings.ParseBool(value)
	}
	return true
}

// GetTargetBlank reports whether the app link should open in a new tab.
func (hw *HTTPRouteWrapper) GetTargetBlank() bool {
	if value := hw.GetAnnotationValue(annotations.HajimariTargetBlankAnnotation); value != "" {
		return utilStrings.ParseBool(value)
	}
	return false
}

// GetURL returns the route URL using HTTP when no explicit URL is configured.
func (hw *HTTPRouteWrapper) GetURL() string {
	return hw.GetURLWithGateway("http", "", 0)
}

// GetURLWithGateway returns the explicit Hajimari URL annotation when present.
// Otherwise it builds a URL from the route hostname, its first path match, and
// the selected Gateway listener details.
func (hw *HTTPRouteWrapper) GetURLWithGateway(scheme, gatewayHostname string, gatewayPort int64) string {
	if annotatedURL := hw.GetAnnotationValue(annotations.HajimariURLAnnotation); annotatedURL != "" {
		parsedURL, err := url.ParseRequestURI(annotatedURL)
		if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
			logger.Warn("Invalid hajimari URL annotation for HTTPRoute ", hw.route.GetName())
			return ""
		}
		return parsedURL.String()
	}

	hostname := hw.getHostname()
	if hostname == "" {
		hostname = gatewayHostname
	}
	if hostname == "" || strings.HasPrefix(hostname, "*.") {
		logger.Warn("No concrete hostname found for HTTPRoute: ", hw.route.GetName())
		return ""
	}

	if scheme != "https" {
		scheme = "http"
	}
	if shouldIncludePort(scheme, gatewayPort) {
		hostname = net.JoinHostPort(hostname, strconv.FormatInt(gatewayPort, 10))
	}
	return scheme + "://" + hostname + hw.getPath()
}

// GetBackendServiceReferences returns Service backend references used by the route.
func (hw *HTTPRouteWrapper) GetBackendServiceReferences() []util.ServiceReference {
	rules, found, err := unstructured.NestedSlice(hw.route.Object, "spec", "rules")
	if err != nil || !found {
		return nil
	}

	services := make([]util.ServiceReference, 0)
	seen := make(map[util.ServiceReference]struct{})
	routeNamespace := hw.GetNamespace()
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]interface{})
		if !ok {
			continue
		}
		backendRefs, found, err := unstructured.NestedSlice(ruleMap, "backendRefs")
		if err != nil || !found {
			continue
		}
		for _, backendRef := range backendRefs {
			backendMap, ok := backendRef.(map[string]interface{})
			if !ok || !isServiceBackend(backendMap) {
				continue
			}
			name, _, _ := unstructured.NestedString(backendMap, "name")
			if name == "" {
				continue
			}
			namespace, _, _ := unstructured.NestedString(backendMap, "namespace")
			if namespace == "" {
				namespace = routeNamespace
			}
			if namespace != routeNamespace {
				continue
			}
			service := util.ServiceReference{Namespace: namespace, Name: name}
			if _, exists := seen[service]; !exists {
				services = append(services, service)
				seen[service] = struct{}{}
			}
		}
	}

	return services
}

func (hw *HTTPRouteWrapper) getHostname() string {
	hostnames, found, err := unstructured.NestedStringSlice(hw.route.Object, "spec", "hostnames")
	if err != nil || !found || len(hostnames) == 0 {
		return ""
	}
	return hostnames[0]
}

func (hw *HTTPRouteWrapper) getPath() string {
	rules, found, err := unstructured.NestedSlice(hw.route.Object, "spec", "rules")
	if err != nil || !found {
		return ""
	}
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]interface{})
		if !ok {
			continue
		}
		matches, found, err := unstructured.NestedSlice(ruleMap, "matches")
		if err != nil || !found {
			continue
		}
		for _, match := range matches {
			matchMap, ok := match.(map[string]interface{})
			if !ok {
				continue
			}
			path, found, err := unstructured.NestedString(matchMap, "path", "value")
			if err == nil && found {
				return path
			}
		}
	}
	return ""
}

func shouldIncludePort(scheme string, port int64) bool {
	return port != 0 && !(scheme == "http" && port == 80) && !(scheme == "https" && port == 443)
}

func isServiceBackend(backendRef map[string]interface{}) bool {
	group, found, _ := unstructured.NestedString(backendRef, "group")
	if found && group != "" {
		return false
	}
	kind, found, _ := unstructured.NestedString(backendRef, "kind")
	return !found || kind == "" || kind == "Service"
}
