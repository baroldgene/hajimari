package httprouteapps

import (
	"context"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// resolveGatewayListener returns the URL scheme and hostname advertised by the
// first parent Gateway listener that can serve the route. A route hostname,
// when present, takes precedence over the listener hostname in the wrapper.
func resolveGatewayListener(route unstructured.Unstructured, dynClient dynamic.Interface, gatewayListenerPorts map[string]int64) (string, string, int64) {
	parentRefs, found, err := unstructured.NestedSlice(route.Object, "spec", "parentRefs")
	if err != nil || !found {
		return "http", "", 0
	}

	for _, parentRef := range parentRefs {
		parentRefMap, ok := parentRef.(map[string]interface{})
		if !ok || !isGatewayParentRef(parentRefMap) {
			continue
		}

		name, found, _ := unstructured.NestedString(parentRefMap, "name")
		if !found || name == "" {
			continue
		}
		namespace, _, _ := unstructured.NestedString(parentRefMap, "namespace")
		if namespace == "" {
			namespace = route.GetNamespace()
		}

		gateway, err := getGateway(dynClient, namespace, name)
		if err != nil {
			logger.Debugf("Could not read parent Gateway '%v' in Namespace '%v' for HTTPRoute '%v': %v", name, namespace, route.GetName(), err)
			continue
		}
		if scheme, hostname, port, found := gatewayListenerURL(*gateway, parentRefMap, gatewayListenerPorts); found {
			return scheme, hostname, port
		}
	}

	return "http", "", 0
}

func getGateway(dynClient dynamic.Interface, namespace, name string) (*unstructured.Unstructured, error) {
	for _, resource := range gatewayResources {
		gateway, err := dynClient.Resource(resource).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if !apierrors.IsNotFound(err) {
			return gateway, err
		}
	}

	return nil, apierrors.NewNotFound(gatewayResources[0].GroupResource(), name)
}

func isGatewayParentRef(parentRef map[string]interface{}) bool {
	group, found, _ := unstructured.NestedString(parentRef, "group")
	if found && group != "" && group != "gateway.networking.k8s.io" {
		return false
	}
	kind, found, _ := unstructured.NestedString(parentRef, "kind")
	return !found || kind == "" || kind == "Gateway"
}

func gatewayListenerURL(gateway unstructured.Unstructured, parentRef map[string]interface{}, gatewayListenerPorts map[string]int64) (string, string, int64, bool) {
	listeners, found, err := unstructured.NestedSlice(gateway.Object, "spec", "listeners")
	if err != nil || !found {
		return "", "", 0, false
	}
	sectionName, _, _ := unstructured.NestedString(parentRef, "sectionName")
	parentPort, hasParentPort, _ := unstructured.NestedInt64(parentRef, "port")

	var (
		httpHostname string
		httpPort     int64
		httpFound    bool
	)
	for _, listener := range listeners {
		listenerMap, ok := listener.(map[string]interface{})
		if !ok || !matchesParentRef(listenerMap, sectionName, parentPort, hasParentPort) {
			continue
		}
		protocol, _, _ := unstructured.NestedString(listenerMap, "protocol")
		hostname, _, _ := unstructured.NestedString(listenerMap, "hostname")
		port, _, _ := unstructured.NestedInt64(listenerMap, "port")
		port = publicListenerPort(listenerMap, port, gatewayListenerPorts)
		switch protocol {
		case "HTTPS":
			return "https", concreteHostname(hostname), port, true
		case "HTTP":
			if !httpFound {
				httpHostname = concreteHostname(hostname)
				httpPort = port
				httpFound = true
			}
		}
	}

	if httpFound {
		return "http", httpHostname, httpPort, true
	}
	return "", "", 0, false
}

// publicListenerPort returns a listener's configured public port when one is
// available. A non-positive configured port is ignored so the Gateway's
// declared listener port remains the safe fallback.
func publicListenerPort(listener map[string]interface{}, listenerPort int64, gatewayListenerPorts map[string]int64) int64 {
	listenerName, found, _ := unstructured.NestedString(listener, "name")
	if !found {
		return listenerPort
	}
	publicPort, found := gatewayListenerPorts[listenerName]
	if !found || publicPort <= 0 {
		return listenerPort
	}
	return publicPort
}

func matchesParentRef(listener map[string]interface{}, sectionName string, parentPort int64, hasParentPort bool) bool {
	if sectionName != "" {
		listenerName, _, _ := unstructured.NestedString(listener, "name")
		if listenerName != sectionName {
			return false
		}
	}
	if hasParentPort {
		listenerPort, found, _ := unstructured.NestedInt64(listener, "port")
		if !found || listenerPort != parentPort {
			return false
		}
	}
	return true
}

// gateway hostname values can contain a wildcard. They cannot be opened as a
// browser URL, so leave host selection to HTTPRoute hostnames or the explicit
// Hajimari URL annotation in that case.
func concreteHostname(hostname string) string {
	if strings.HasPrefix(hostname, "*.") {
		return ""
	}
	return hostname
}
