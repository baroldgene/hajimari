package httproutes

import (
	"context"

	"github.com/toboshii/hajimari/internal/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var resources = []schema.GroupVersionResource{
	{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"},
	{Group: "gateway.networking.k8s.io", Version: "v1beta1", Resource: "httproutes"},
}

// List is used to list Gateway API HTTPRoutes.
type List struct {
	appConfig config.Config
	err       error
	items     []unstructured.Unstructured
	dynClient dynamic.Interface
}

// FilterFunc defines a filter for HTTPRoutes.
type FilterFunc func(unstructured.Unstructured, config.Config) bool

// NewList creates a List object that can query HTTPRoutes.
func NewList(dynClient dynamic.Interface, appConfig config.Config, items ...unstructured.Unstructured) *List {
	return &List{
		dynClient: dynClient,
		appConfig: appConfig,
		items:     items,
	}
}

// Populate returns HTTPRoutes from the selected namespaces. Gateway API is an
// optional Kubernetes extension; its absence must not prevent Ingress or CRD
// application discovery from working.
func (hl *List) Populate(namespaces ...string) *List {
	for _, namespace := range namespaces {
		routes, err := hl.list(namespace)
		if err != nil {
			hl.err = err
			continue
		}
		hl.items = append(hl.items, routes.Items...)
	}

	return hl
}

func (hl *List) list(namespace string) (*unstructured.UnstructuredList, error) {
	for _, resource := range resources {
		routes, err := hl.dynClient.Resource(resource).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
		if !apierrors.IsNotFound(err) {
			return routes, err
		}
	}

	// Gateway API is not installed. It is optional, so callers should treat it
	// as an empty result rather than an application discovery error.
	return &unstructured.UnstructuredList{}, nil
}

// Filter applies a filter to the routes in the List.
func (hl *List) Filter(filterFunc FilterFunc) *List {
	filtered := make([]unstructured.Unstructured, 0, len(hl.items))
	for _, route := range hl.items {
		if filterFunc(route, hl.appConfig) {
			filtered = append(filtered, route)
		}
	}

	hl.items = filtered
	return hl
}

// Get returns the HTTPRoutes currently present in List.
func (hl *List) Get() ([]unstructured.Unstructured, error) {
	return hl.items, hl.err
}
