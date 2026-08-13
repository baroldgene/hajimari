package util

import (
	"context"
	"fmt"
	"math"

	// v1 "k8s.io/api/apps/v1"
	"github.com/toboshii/hajimari/internal/log"
	networkingV1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
)

const (
	appNameLabelKey     = "app.kubernetes.io/name"
	serviceNameLabelKey = "kubernetes.io/service-name"
)

var logger = log.New()

// struct for the ReplicaStatusGetter object
type ReplicaStatusGetter struct {
	err               error
	replicas          int
	availableReplicas int
	kubeClient        kubernetes.Interface
}

// ServiceReference identifies a Service whose endpoints should be included in
// an application's replica status.
type ServiceReference struct {
	Namespace string
	Name      string
}

// Initializes a ReplicaStatusGetter
func NewReplicaStatusGetter(kubeClient kubernetes.Interface) *ReplicaStatusGetter {
	return &ReplicaStatusGetter{
		kubeClient: kubeClient,
	}
}

// Gets replicaStatuses using the DiscoveryV1 api
func (rsg *ReplicaStatusGetter) GetEndpointStatuses(ingress networkingV1.Ingress) *ReplicaStatusGetter {
	return rsg.GetEndpointStatusesForServices(ingress.Name, ingressServiceReferences(ingress))
}

// GetEndpointStatusesForServices gets replica status for the supplied Service
// references. HTTPRoutes can reference Services in a namespace other than the
// route's namespace, so EndpointSlices are queried once per namespace.
func (rsg *ReplicaStatusGetter) GetEndpointStatusesForServices(resourceName string, services []ServiceReference) *ReplicaStatusGetter {
	rsg.err = nil
	rsg.replicas = 0
	rsg.availableReplicas = 0

	serviceNamesByNamespace := make(map[string][]string)
	for _, service := range services {
		if service.Name == "" {
			continue
		}
		serviceNamesByNamespace[service.Namespace] = append(serviceNamesByNamespace[service.Namespace], service.Name)
	}

	if len(serviceNamesByNamespace) == 0 {
		rsg.err = fmt.Errorf("No backend services found for %s", resourceName)
		return rsg
	}

	for namespace, serviceNames := range serviceNamesByNamespace {
		labelRequirements, err := labels.NewRequirement(serviceNameLabelKey, selection.In, serviceNames)
		if err != nil {
			logger.Error("Error setting labelSelector Requirements", err)
			rsg.err = err
			return rsg
		}

		labelOptions := metav1.ListOptions{
			LabelSelector: labels.NewSelector().Add(*labelRequirements).String(),
		}
		epslices, err := rsg.kubeClient.DiscoveryV1().EndpointSlices(namespace).List(context.Background(), labelOptions)
		if err != nil {
			logger.Error("Error Getting EndpointSlices: ", err)
			rsg.err = err
			return rsg
		}

		if len(epslices.Items) > 1 {
			logger.Debug(resourceName, " Multiple EndpointSlices found. Will try using all of them.")
		}
		if len(epslices.Items) == 0 {
			logger.Debug(resourceName, " No EndpointSlice Found")
		}

		for _, epslice := range epslices.Items {
			logger.Debug("Checking EndpointSlice: ", epslice.Name)
			rsg.replicas += len(epslice.Endpoints)
			for _, endpoint := range epslice.Endpoints {
				if endpoint.Conditions.Ready == nil || *endpoint.Conditions.Ready {
					rsg.availableReplicas++
				}
			}
		}
	}

	if rsg.replicas == 0 {
		rsg.err = fmt.Errorf("No endpoints found for %s", resourceName)
	}

	return rsg
}

// Gets the current value of replicas
func (rsg *ReplicaStatusGetter) GetReplicas() int {
	if rsg.err != nil {
		logger.Warn(rsg.err)
		return 0
	}
	return rsg.replicas
}

// Gets the current value of replicas
func (rsg *ReplicaStatusGetter) GetAvailableReplicas() int {
	if rsg.err != nil {
		logger.Warn(rsg.err)
		return 0
	}
	return rsg.availableReplicas
}

// Gets the current ratio of availableReplicas to replicas
// math.Round only works with float64
func (rsg *ReplicaStatusGetter) GetRatio() float64 {
	if rsg.err != nil {
		logger.Warn(rsg.err)
		return 0
	}
	return math.Round(float64(rsg.availableReplicas) / float64(rsg.replicas))
}

// Gets Service Names that the Ingress is actually meant for
func ingressServiceReferences(ingress networkingV1.Ingress) []ServiceReference {
	services := []ServiceReference{}
	namespace := ingress.GetNamespace()

	if ingress.Spec.DefaultBackend != nil {
		services = append(services, ServiceReference{Namespace: namespace, Name: ingress.Spec.DefaultBackend.Service.Name})
	}
	if len(ingress.Spec.Rules) > 0 {
		for _, rule := range ingress.Spec.Rules {
			for _, path := range rule.HTTP.Paths {
				services = append(services, ServiceReference{Namespace: namespace, Name: path.Backend.Service.Name})
			}
		}
	}

	return services
}
