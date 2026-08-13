package httprouteapps

import (
	"math"

	"github.com/toboshii/hajimari/internal/annotations"
	"github.com/toboshii/hajimari/internal/config"
	"github.com/toboshii/hajimari/internal/kube/lists/httproutes"
	"github.com/toboshii/hajimari/internal/kube/util"
	"github.com/toboshii/hajimari/internal/kube/wrappers"
	"github.com/toboshii/hajimari/internal/log"
	"github.com/toboshii/hajimari/internal/models"
	utilStrings "github.com/toboshii/hajimari/internal/util/strings"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var (
	logger           = log.New()
	gatewayResources = []schema.GroupVersionResource{
		{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"},
		{Group: "gateway.networking.k8s.io", Version: "v1beta1", Resource: "gateways"},
	}
)

// List is used for listing Hajimari applications from Gateway API HTTPRoutes.
type List struct {
	appConfig  config.Config
	err        error
	dynClient  dynamic.Interface
	kubeClient kubernetes.Interface
	items      []models.AppGroup
}

// NewList creates an HTTPRoute application lister.
func NewList(dynClient dynamic.Interface, kubeClient kubernetes.Interface, appConfig config.Config) *List {
	return &List{appConfig: appConfig, dynClient: dynClient, kubeClient: kubeClient}
}

// Populate discovers HTTPRoutes in the supplied namespaces.
func (al *List) Populate(namespaces ...string) *List {
	routes, err := httproutes.NewList(al.dynClient, al.appConfig).
		Populate(namespaces...).
		Filter(byHajimariEnableAnnotation).
		Get()

if al.appConfig.InstanceName != "" {
		filteredRoutes, filterErr := httproutes.NewList(al.dynClient, al.appConfig, routes...).
			Filter(byHajimariInstanceAnnotation).
			Get()
		routes = filteredRoutes
		if err == nil {
			err = filterErr
		}
	}

	if err != nil {
		al.err = err
	}
	al.items = convertHTTPRoutesToHajimariApps(routes, al.dynClient, util.NewReplicaStatusGetter(al.kubeClient))
	return al
}

// Get returns the application groups currently present in List.
func (al *List) Get() ([]models.AppGroup, error) {
	return al.items, al.err
}

func convertHTTPRoutesToHajimariApps(routes []unstructured.Unstructured, dynClient dynamic.Interface, rsg *util.ReplicaStatusGetter) (appGroups []models.AppGroup) {
	groupIndexes := make(map[string]int)
	for _, route := range routes {
		wrapper := wrappers.NewHTTPRouteWrapper(&route)
		logger.Debugf("Found HTTPRoute with Name '%v' in Namespace '%v'", route.GetName(), route.GetNamespace())

		groupIndex, exists := groupIndexes[wrapper.GetGroup()]
		if !exists {
			groupIndex = len(appGroups)
			groupIndexes[wrapper.GetGroup()] = groupIndex
			appGroups = append(appGroups, models.AppGroup{Group: wrapper.GetGroup()})
		}

		scheme, gatewayHostname := resolveGatewayListener(route, dynClient)
		app := models.App{
			Name:        wrapper.GetName(),
			Icon:        wrapper.GetAnnotationValue(annotations.HajimariIconAnnotation),
			URL:         wrapper.GetURLWithGateway(scheme, gatewayHostname),
			Info:        wrapper.GetInfo(),
			TargetBlank: wrapper.GetTargetBlank(),
		}
		if wrapper.GetStatusCheckEnabled() {
			replicaStatus := rsg.GetEndpointStatusesForServices(wrapper.GetName(), wrapper.GetBackendServiceReferences())
			if replicaStatus.GetReplicas() != 0 {
				app.Replicas = models.ReplicaInfo{
					Total:     replicaStatus.GetReplicas(),
					Available: replicaStatus.GetAvailableReplicas(),
					PctReady:  math.Round(replicaStatus.GetRatio() * 100),
				}
			}
		}

		appGroups[groupIndex].Apps = append(appGroups[groupIndex].Apps, app)
	}
	return appGroups
}

func byHajimariEnableAnnotation(route unstructured.Unstructured, appConfig config.Config) bool {
	if appConfig.DefaultEnable {
		return route.GetAnnotations()[annotations.HajimariEnableAnnotation] != "false"
	}
	return route.GetAnnotations()[annotations.HajimariEnableAnnotation] == "true"
}

func byHajimariInstanceAnnotation(route unstructured.Unstructured, appConfig config.Config) bool {
	return utilStrings.ContainsBetweenDelimiter(route.GetAnnotations()[annotations.HajimariInstanceAnnotation], appConfig.InstanceName, ",")
}
