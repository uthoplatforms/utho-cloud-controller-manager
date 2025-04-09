package utho

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/uthoplatforms/utho-go/utho"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

var errLbNotFound = fmt.Errorf("loadbalancer not found")
var _ cloudprovider.LoadBalancer = &loadbalancers{}

type loadbalancers struct {
	client utho.Client
	zone   string

	kubeClient kubernetes.Interface
}

func newLoadbalancers(client utho.Client, zone string) cloudprovider.LoadBalancer {
	return &loadbalancers{client: client, zone: zone}
}

func (l *loadbalancers) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	_, exists, err := l.GetLoadBalancer(ctx, clusterName, service)
	if err != nil {
		return nil, fmt.Errorf("EnsureLoadBalancer: %w", err)
	}

	// If LoadBalancer doesn't exist
	if !exists {
		klog.Infof("EnsureLoadBalancer: Load balancer for cluster %q doesn't exist, creating", clusterName)

		// Get cluster ID
		clusterId, err := GetLabelValue(l.kubeClient, "cluster_id")
		if err != nil {
			return nil, fmt.Errorf("EnsureLoadBalancer: failed to get cluster ID: %w", err)
		}

		// Get VPC ID
		vpcId, err := GetLabelValue(l.kubeClient, "cluster_vpc")
		if err != nil {
			return nil, fmt.Errorf("EnsureLoadBalancer: failed to get VPC ID: %w", err)
		}

		// Get nodepool ID
		nodePoolId, err := GetNodePoolsID()
		if err != nil {
			return nil, fmt.Errorf("EnsureLoadBalancer: failed to get nodepool ID: %w", err)
		}

		lbName := l.GetLoadBalancerName(context.Background(), "", service)

		lb, err := l.CreateUthoLoadBalancer(lbName, vpcId, service, nodePoolId, clusterId)
		if err != nil {
			return nil, fmt.Errorf("EnsureLoadBalancer: failed to create load-balancer: %w", err)
		}
		klog.Infof("EnsureLoadBalancer: Created load balancer %q", lb.ID)

		// Set the Utho VLB ID annotation
		if _, ok := service.Annotations[annoUthoLoadBalancerID]; !ok {
			if err = l.GetKubeClient(); err != nil {
				return nil, fmt.Errorf("EnsureLoadBalancer: failed to get kubeclient to update service: %w", err)
			}

			// Get Kubernetes services
			service, err = l.kubeClient.CoreV1().Services(service.Namespace).Get(ctx, service.Name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("EnsureLoadBalancer: failed to get service with LoadBalancer ID: %w", err)
			}

			if service.Annotations == nil {
				service.Annotations = make(map[string]string)
			}
			service.Annotations[annoUthoLoadBalancerID] = lb.ID

			_, err = l.kubeClient.CoreV1().Services(service.Namespace).Update(ctx, service, metav1.UpdateOptions{})
			if err != nil {
				return nil, fmt.Errorf("EnsureLoadBalancer: failed to update service with LoadBalancer ID: %w", err)
			}
		}

		getLb, err := l.client.Loadbalancers().Read(lb.ID)
		if err != nil {
			return nil, fmt.Errorf("EnsureLoadBalancer: failed to get Utho LoadBalancer with ID: %w", err)
		}

		return &v1.LoadBalancerStatus{
			Ingress: []v1.LoadBalancerIngress{
				{
					IP: getLb.IP,
				},
			},
		}, nil
	}

	klog.Infof("EnsureLoadBalancer: Load balancer exists for cluster %q", clusterName)

	lb, err := l.getUthoLB(ctx, service)
	if err != nil {
		if err == errLbNotFound {
			return nil, fmt.Errorf("EnsureLoadBalancer: %w", errLbNotFound)
		}

		return nil, fmt.Errorf("EnsureLoadBalancer: %w", err)
	}

	klog.Infof("EnsureLoadBalancer: Found load balancer: %q", lb.Name)

	// Set the Utho VLB ID annotation
	if _, ok := service.Annotations[annoUthoLoadBalancerID]; !ok {
		if service.Annotations == nil {
			service.Annotations = make(map[string]string)
		}
		service.Annotations[annoUthoLoadBalancerID] = lb.ID
		if err = l.GetKubeClient(); err != nil {
			return nil, fmt.Errorf("EnsureLoadBalancer: failed to get kubeclient to update service: %w", err)
		}
		_, err = l.kubeClient.CoreV1().Services(service.Namespace).Update(ctx, service, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("EnsureLoadBalancer: failed to update service with LoadBalancer ID: %w", err)
		}
	}

	if err2 := l.UpdateLoadBalancer(ctx, clusterName, service, nodes); err2 != nil {
		return nil, fmt.Errorf("EnsureLoadBalancer: %w", err2)
	}

	lbStatus, _, err := l.GetLoadBalancer(ctx, clusterName, service)
	if err != nil {
		return nil, fmt.Errorf("EnsureLoadBalancer: %w", err)
	}

	return lbStatus, nil
}

// CreateUthoLoadBalancer sets up a LoadBalancer, its frontend, and backend configurations.
func (l *loadbalancers) CreateUthoLoadBalancer(lbName, vpcId string, service *v1.Service, nodePoolId []string, clusterId string) (*utho.CreateLoadbalancerResponse, error) {
	// Create LoadBalancer request parameters
	lbRequest := utho.CreateLoadbalancerParams{
		Name:                lbName,
		Dcslug:              l.zone,
		Vpc:                 vpcId,
		Type:                "network",
		EnablePublicip:      "true",
		Cpumodel:            "amd",
		KubernetesClusterid: clusterId,
	}
	klog.Infof("CreateUthoLoadBalancer: LoadBalancer request: %+v", lbRequest)

	// Create the LoadBalancer
	lb, err := l.client.Loadbalancers().Create(lbRequest)
	if err != nil {
		return nil, fmt.Errorf("CreateUthoLoadBalancer: failed to create LoadBalancer: %w", err)
	}

	// Check the status of the LoadBalancer
	for i := 0; i < 5; i++ {
		readLb, err := l.client.Loadbalancers().Read(lb.ID)
		if err != nil {
			return nil, fmt.Errorf("CreateUthoLoadBalancer: failed to read LoadBalancer status: %w", err)
		}
		klog.Infof("CreateUthoLoadBalancer: LoadBalancer status app check: %+v", readLb)
		klog.Infof("CreateUthoLoadBalancer: LoadBalancer app status: %s", readLb.AppStatus)
		if strings.EqualFold(readLb.AppStatus, string(utho.Installed)) {
			break
		}
		time.Sleep(45 * time.Second)
	}

	// get services annotion
	algo := getAlgorithm(service)

	stickySessionEnabled := getStickySessionEnabled(service)

	sslRedirect := getSSLRedirect(service)

	var lBSSLID string
	if lBSSLIDVal, ok := service.Annotations[annoUthoLBSSLID]; ok {
		lBSSLID = lBSSLIDVal
	}

	// Iterate over each service port to configure frontend and backend
	for _, port := range service.Spec.Ports {
		// Ensure the protocol is TCP
		if port.Protocol != v1.ProtocolTCP {
			return nil, fmt.Errorf("CreateUthoLoadBalancer: only TCP protocol is supported, got: %q", port.Protocol)
		}

		isHTTPPort := int(port.Port) == 80 || int(port.Port) == 443

		// Create LoadBalancer frontend request parameters
		feRequest := utho.CreateLoadbalancerFrontendParams{
			LoadbalancerId: lb.ID,
			Name:           GenerateRandomString(10),
			Proto:          "tcp",
			Port:           strconv.Itoa(int(port.Port)),
			Algorithm:      algo,
			Cookie:         stickySessionEnabled,
		}

		// Add `Redirecthttps` and `CertificateID` only for HTTP ports
		if isHTTPPort {
			if sslRedirect {
				feRequest.Redirecthttps = "1"
			}
			if lBSSLID != "" {
				feRequest.CertificateID = lBSSLID
				feRequest.Proto = "https"
			}
		}

		klog.Infof("CreateUthoLoadBalancer: LoadBalancer Frontend request: %+v", feRequest)

		// Create the frontend
		lbFe, err := l.client.Loadbalancers().CreateFrontend(feRequest)
		if err != nil {
			return nil, fmt.Errorf("CreateUthoLoadBalancer: error creating LoadBalancer frontend: %w", err)
		}

		// Configure backends for each node pool
		for _, id := range nodePoolId {
			feBackend := utho.CreateLoadbalancerBackendParams{
				LoadbalancerId: lb.ID,
				FrontendID:     lbFe.ID,
				Type:           "kubernetes",
				BackendPort:    strconv.Itoa(int(port.NodePort)),
				Cloudid:        clusterId,
				PoolName:       id,
			}
			klog.Infof("CreateUthoLoadBalancer: LoadBalancer Backend request: %+v", feBackend)

			// Create the backend
			_, err := l.client.Loadbalancers().CreateBackend(feBackend)
			if err != nil {
				return nil, fmt.Errorf("CreateUthoLoadBalancer: error creating backend: %w", err)
			}
		}
	}

	// Return the created LoadBalancer
	return lb, nil
}

// UpdateLoadBalancer updates the configuration of the specified Kubernetes LoadBalancer.
func (l *loadbalancers) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	klog.V(3).Info("UpdateLoadBalancer: Called UpdateLoadBalancers")

	// Check if the LoadBalancer already exists
	if _, _, err := l.GetLoadBalancer(ctx, clusterName, service); err != nil {
		return fmt.Errorf("UpdateLoadBalancer: %w", err)
	}

	// Retrieve the Utho LoadBalancer
	lb, err := l.getUthoLB(ctx, service)
	if err != nil {
		return fmt.Errorf("UpdateLoadBalancer: %w", err)
	}

	// Ensure the Utho LoadBalancer ID annotation is set
	if service.Annotations == nil {
		service.Annotations = make(map[string]string)
	}
	if err := l.GetKubeClient(); err != nil {
		return fmt.Errorf("UpdateLoadBalancer: failed to get kubeclient to update service: %w", err)
	}
	if _, ok := service.Annotations[annoUthoLoadBalancerID]; !ok {
		service.Annotations[annoUthoLoadBalancerID] = lb.ID
		_, err = l.kubeClient.CoreV1().Services(service.Namespace).Update(ctx, service, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("UpdateLoadBalancer: failed to update service with LoadBalancer ID: %w", err)
		}
	}

	// Get cluster ID
	clusterId, err := GetLabelValue(l.kubeClient, "cluster_id")
	if err != nil {
		return fmt.Errorf("UpdateLoadBalancer: failed to get cluster ID: %w", err)
	}

	// Get node pool IDs
	nodePoolId, err := GetNodePoolsID()
	if err != nil {
		return fmt.Errorf("UpdateLoadBalancer: failed to get node pool IDs: %w", err)
	}

	// Map of desired ports
	desiredPorts := map[string]*v1.ServicePort{}
	for _, port := range service.Spec.Ports {
		if port.Protocol == v1.ProtocolTCP {
			portStr := strconv.Itoa(int(port.Port))
			desiredPorts[portStr] = &port
		} else {
			klog.Warningf("UpdateLoadBalancer: Skipping unsupported protocol for port %d: %s", port.Port, port.Protocol)
		}
	}

	// Get services annotation
	algo := getAlgorithm(service)

	stickySessionEnabled := getStickySessionEnabled(service)

	sslRedirect := getSSLRedirect(service)

	var lBSSLID string
	if lBSSLIDVal, ok := service.Annotations[annoUthoLBSSLID]; ok {
		lBSSLID = lBSSLIDVal
	}

	// Fetch existing frontends
	currentFrontends := make(map[string]utho.Frontends)
	for _, fe := range lb.Frontends {
		currentFrontends[fe.Port] = fe
	}

	// Create or update frontends/backends for desired ports
	for portStr, port := range desiredPorts {
		if _, exists := currentFrontends[portStr]; exists {
			klog.Infof("UpdateLoadBalancer: Frontend already exists for port %s, skipping creation.", portStr)
			continue
		}

		isHTTPPort := int(port.Port) == 80 || int(port.Port) == 443

		// Create new frontend
		feRequest := utho.CreateLoadbalancerFrontendParams{
			LoadbalancerId: lb.ID,
			Name:           GenerateRandomString(10),
			Proto:          "tcp",
			Port:           portStr,
			Algorithm:      algo,
			Cookie:         stickySessionEnabled,
		}

		// Add `Redirecthttps` and `CertificateID` only for HTTP ports
		if isHTTPPort {
			if sslRedirect {
				feRequest.Redirecthttps = "1"
			}
			if lBSSLID != "" {
				feRequest.CertificateID = lBSSLID
				feRequest.Proto = "https"
			}
		}

		klog.Infof("UpdateLoadBalancer: Creating new load balancer frontend: %+v", feRequest)
		lbFe, err := l.client.Loadbalancers().CreateFrontend(feRequest)
		if err != nil {
			return fmt.Errorf("UpdateLoadBalancer: error creating load balancer frontend: %w", err)
		}

		// Create backends for the new frontend
		for _, id := range nodePoolId {
			feBackend := utho.CreateLoadbalancerBackendParams{
				LoadbalancerId: lb.ID,
				FrontendID:     lbFe.ID,
				Type:           "kubernetes",
				BackendPort:    strconv.Itoa(int(port.NodePort)),
				Cloudid:        clusterId,
				PoolName:       id,
			}
			klog.Infof("UpdateLoadBalancer: Creating new load balancer backend: %+v", feBackend)
			_, err = l.client.Loadbalancers().CreateBackend(feBackend)
			if err != nil {
				return fmt.Errorf("UpdateLoadBalancer: error creating load balancer backend: %w", err)
			}
		}
	}

	// Remove frontends for ports no longer desired
	for portStr, fe := range currentFrontends {
		if _, exists := desiredPorts[portStr]; !exists {
			klog.Infof("UpdateLoadBalancer: Deleting unused frontend for port %s", portStr)
			_, err := l.client.Loadbalancers().DeleteFrontend(lb.ID, fe.ID)
			if err != nil {
				return fmt.Errorf("UpdateLoadBalancer: error deleting load balancer frontend: %w", err)
			}
		}
	}

	klog.Infof("UpdateLoadBalancer: Finished updating LoadBalancer for cluster %q, LB ID %q", clusterName, lb.ID)

	return nil
}

// EnsureLoadBalancerDeleted ensures that a LoadBalancer associated with a specific service is deleted.
func (l *loadbalancers) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	_, exists, err := l.GetLoadBalancer(ctx, clusterName, service)
	if err != nil {
		return fmt.Errorf("EnsureLoadBalancerDeleted: %w", err)
	}
	// This is the same as if we were to check if err == errLbNotFound
	if !exists {
		return nil
	}

	lb, err := l.getUthoLB(ctx, service)
	if err != nil {
		return fmt.Errorf("EnsureLoadBalancerDeleted: %w", err)
	}

	_, err = l.client.Loadbalancers().Delete(lb.ID)
	if err != nil {
		return fmt.Errorf("EnsureLoadBalancerDeleted: failed to delete LoadBalancer: %w", err)
	}
	klog.Infof("EnsureLoadBalancerDeleted: Finished deleting LoadBalancer for cluster %q, LB ID %q", clusterName, lb.ID)

	return nil
}

// GetKubeClient initializes and retrieves a Kubernetes client if not already available.
func (l *loadbalancers) GetKubeClient() error {
	if l.kubeClient != nil {
		return nil
	}

	var (
		kubeConfig *rest.Config
		err        error
		config     string
	)

	// Default to an empty string if no kubeconfig is passed or set
	if Options.KubeconfigFlag == nil || Options.KubeconfigFlag.Value.String() == "" {
		config = ""
	} else {
		config = Options.KubeconfigFlag.Value.String()
	}

	kubeConfig, err = clientcmd.BuildConfigFromFlags("", config)
	if err != nil {
		return fmt.Errorf("GetKubeClient: error building Kubernetes config: %w", err)
	}

	l.kubeClient, err = kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("GetKubeClient: error creating Kubernetes client: %w", err)
	}

	return nil
}

// GetLoadBalancer retrieves the LoadBalancer status, existence, and any errors for a given service.
func (l *loadbalancers) GetLoadBalancer(ctx context.Context, _ string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	lb, err := l.getUthoLB(ctx, service)
	if err != nil {
		if err == errLbNotFound {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("GetLoadBalancer: %w", err)
	}

	return &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{
				IP: lb.IP,
			},
		},
	}, true, nil
}

// GetLoadBalancerName returns the LoadBalancer name from annotations or defaults to a generated name.
func (l *loadbalancers) GetLoadBalancerName(_ context.Context, _ string, service *v1.Service) string {
	if label, ok := service.Annotations[annoUthoLoadBalancerName]; ok {
		return label
	}
	return getDefaultLBName(service)
}

// lbByName retrieves a load balancer by name and matches it with the cluster ID.
func (l *loadbalancers) lbByName(lbName, clusterId string) (*utho.Loadbalancer, error) {
	lbs, err := l.client.Loadbalancers().List()
	if err != nil {
		return nil, err
	}
	for _, lb := range lbs {
		if lb.Name == lbName && strings.EqualFold(lb.KubernetesClusterid, clusterId) {
			return &lb, nil
		}
	}

	return nil, errLbNotFound
}

// getUthoLB retrieves a Utho LoadBalancer associated with a service, either by ID or by name.
func (l *loadbalancers) getUthoLB(ctx context.Context, service *v1.Service) (*utho.Loadbalancer, error) {
	// If the LoadBalancer ID is available in annotations, use it to fetch the LoadBalancer
	if id, ok := service.Annotations[annoUthoLoadBalancerID]; ok {
		lb, err := l.client.Loadbalancers().Read(id)
		if err != nil {
			return nil, err
		}
		return lb, nil
	}

	if err := l.GetKubeClient(); err != nil {
		return nil, fmt.Errorf("UpdateLoadBalancer: failed to get kubeclient: %w", err)
	}
	clusterId, err := GetLabelValue(l.kubeClient, "cluster_id")
	if err != nil {
		return nil, fmt.Errorf("UpdateLoadBalancer: failed to get cluster ID: %w", err)
	}

	// Otherwise, attempt to retrieve the LoadBalancer by its default or annotated name
	defaultLBName := getDefaultLBName(service)
	if lb, err := l.lbByName(defaultLBName, clusterId); err != nil {
		// If not found, attempt to retrieve by explicitly specified LoadBalancer name
		lbName := l.GetLoadBalancerName(ctx, "", service)
		lb, err = l.lbByName(lbName, clusterId)
		if err != nil {
			return nil, err
		}
		return lb, nil
	} else {
		return lb, nil
	}
}

// getDefaultLBName generates a default LoadBalancer name for a service.
func getDefaultLBName(service *v1.Service) string {
	return cloudprovider.DefaultLoadBalancerName(service)
}

// getSSLRedirect returns if traffic should be redirected to https
// default to false if not specified
func getSSLRedirect(service *v1.Service) bool {
	redirect, ok := service.Annotations[annoUthoRedirectHTTPToHTTPS]
	if !ok {
		return false
	}

	redirectBool, err := strconv.ParseBool(redirect)
	if err != nil {
		return false
	}

	return redirectBool
}

// getAlgorithm returns the algorithm to be used for load balancer service
// defaults to round_robin if no algorithm is provided.
func getAlgorithm(service *v1.Service) string {
	algo := service.Annotations[annoUthoAlgorithm]
	if algo == "leastconn" {
		return "leastconn"
	}

	return "roundrobin"
}

// getStickySessionEnabled returns whether or not sticky sessions should be enabled
// default is off
func getStickySessionEnabled(service *v1.Service) string {
	enabled, ok := service.Annotations[annoUthoStickySessionEnabled]
	if !ok {
		return "0"
	}

	if enabled == "0" {
		return "0"
	} else if enabled == "1" {
		return "1"
	}

	return "0"
}
