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

// GetLoadBalancer retrieves the LoadBalancer status, existence, and any errors for a given service.
func (l *loadbalancers) GetLoadBalancer(ctx context.Context, _ string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	lb, err := l.getUthoLB(ctx, service)
	if err != nil {
		if err == errLbNotFound {
			return nil, false, nil
		}
		return nil, false, err
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
	if label, ok := service.Annotations[annoUthoLoadBalancerLabel]; ok {
		return label
	}
	return getDefaultLBName(service)
}

func (l *loadbalancers) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	_, exists, err := l.GetLoadBalancer(ctx, clusterName, service)
	if err != nil {
		return nil, err
	}

	// if Load balancer doesn't exist
	if !exists {
		klog.Infof("Load balancer for cluster %q doesn't exist, creating", clusterName)

		lbName := l.GetLoadBalancerName(context.Background(), "", service)

		// get cluster id
		clusterId, err := GetLabelValue("cluster_id")
		if err != nil {
			return nil, fmt.Errorf("failed to get cluster ID: %w", err)
		}

		// get vpc id
		vpcId, err := GetLabelValue("cluster_vpc")
		if err != nil {
			return nil, fmt.Errorf("failed to get vpc ID: %w", err)
		}

		// get nodepool id
		nodePoolId, err := GetNodePoolsID()
		if err != nil {
			return nil, fmt.Errorf("failed to get nodepool ID: %w", err)
		}

		lb, err := l.CreateUthoLoadBalancer(lbName, vpcId, service, nodePoolId, clusterId)

		if err != nil {
			return nil, fmt.Errorf("failed to create load-balancer: %s", err)
		}
		klog.Infof("Created load balancer %q", lb.ID)

		// Set the Utho VLB ID annotation
		if _, ok := service.Annotations[annoUthoLoadBalancerID]; !ok {
			if err = l.GetKubeClient(); err != nil {
				return nil, fmt.Errorf("failed to get kubeclient to update service: %s", err)
			}

			// get k8s services
			service, err = l.kubeClient.CoreV1().Services(service.Namespace).Get(ctx, service.Name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to get service with loadbalancer ID: %s", err)
			}

			if service.Annotations == nil {
				service.Annotations = make(map[string]string)
			}
			service.Annotations[annoUthoLoadBalancerID] = lb.ID

			_, err = l.kubeClient.CoreV1().Services(service.Namespace).Update(ctx, service, metav1.UpdateOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to update service with loadbalancer ID: %s", err)
			}
		}

		getLb, err := l.client.Loadbalancers().Read(lb.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get utho lb with loadbalancer ID: %s", err)
		}

		return &v1.LoadBalancerStatus{
			Ingress: []v1.LoadBalancerIngress{
				{
					IP: getLb.IP,
				},
			},
		}, nil
	}

	klog.Infof("Load balancer exists for cluster %q", clusterName)

	lb, err := l.getUthoLB(ctx, service)
	if err != nil {
		if err == errLbNotFound {
			return nil, errLbNotFound
		}

		return nil, err
	}

	klog.Infof("Found load balancer: %q", lb.Name)

	// Set the Utho VLB ID annotation
	if _, ok := service.Annotations[annoUthoLoadBalancerID]; !ok {
		if service.Annotations == nil {
			service.Annotations = make(map[string]string)
		}
		service.Annotations[annoUthoLoadBalancerID] = lb.ID
		if err = l.GetKubeClient(); err != nil {
			return nil, fmt.Errorf("failed to get kubeclient to update service: %s", err)
		}
		_, err = l.kubeClient.CoreV1().Services(service.Namespace).Update(ctx, service, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to update service with loadbalancer ID: %s", err)
		}
	}

	if err2 := l.UpdateLoadBalancer(ctx, clusterName, service, nodes); err2 != nil { //////////////////////!!!!!
		return nil, err2
	}

	lbStatus, _, err := l.GetLoadBalancer(ctx, clusterName, service)
	if err != nil {
		return nil, err
	}

	return lbStatus, nil
}

// UpdateLoadBalancer updates the configuration of the specified Kubernetes LoadBalancer.
func (l *loadbalancers) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	klog.V(3).Info("Called UpdateLoadBalancers")

	// Check if the load balancer already exists
	if _, _, err := l.GetLoadBalancer(ctx, clusterName, service); err != nil {
		return err
	}

	// Retrieve the Utho load balancer
	lb, err := l.getUthoLB(ctx, service)
	if err != nil {
		return err
	}

	// Ensure the Utho load balancer ID annotation is set
	if service.Annotations == nil {
		service.Annotations = make(map[string]string)
	}
	if _, ok := service.Annotations[annoUthoLoadBalancerID]; !ok {
		service.Annotations[annoUthoLoadBalancerID] = lb.ID
		if err := l.GetKubeClient(); err != nil {
			return fmt.Errorf("failed to get kubeclient to update service: %w", err)
		}
		_, err = l.kubeClient.CoreV1().Services(service.Namespace).Update(ctx, service, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update service with load balancer ID: %w", err)
		}
	}

	// Get cluster ID
	clusterId, err := GetLabelValue("cluster_id")
	if err != nil {
		return fmt.Errorf("failed to get cluster ID: %w", err)
	}

	// Get node pool IDs
	nodePoolId, err := GetNodePoolsID()
	if err != nil {
		return fmt.Errorf("failed to get node pool IDs: %w", err)
	}

	// Map of desired ports
	desiredPorts := map[string]*v1.ServicePort{}
	for _, port := range service.Spec.Ports {
		if port.Protocol == v1.ProtocolTCP {
			portStr := strconv.Itoa(int(port.Port))
			desiredPorts[portStr] = &port
		} else {
			klog.Warningf("Skipping unsupported protocol for port %d: %s", port.Port, port.Protocol)
		}
	}

	// Fetch existing frontends
	currentFrontends := make(map[string]utho.Frontends)
	for _, fe := range lb.Frontends {
		currentFrontends[fe.Port] = fe
	}

	// Create or update frontends/backends for desired ports
	for portStr, port := range desiredPorts {
		if _, exists := currentFrontends[portStr]; exists {
			klog.Infof("Frontend already exists for port %s, skipping creation.", portStr)
			continue
		}

		// Create new frontend
		feRequest := utho.CreateLoadbalancerFrontendParams{
			LoadbalancerId: lb.ID,
			Name:           GenerateRandomString(10),
			Proto:          "tcp",
			Port:           portStr,
			Algorithm:      "roundrobin",
			Cookie:         "0",
		}
		klog.Infof("Creating new load balancer frontend: %+v", feRequest)
		lbFe, err := l.client.Loadbalancers().CreateFrontend(feRequest)
		if err != nil {
			return fmt.Errorf("error creating load balancer frontend: %w", err)
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
			klog.Infof("Creating new load balancer backend: %+v", feBackend)
			_, err = l.client.Loadbalancers().CreateBackend(feBackend)
			if err != nil {
				return fmt.Errorf("error creating load balancer backend: %w", err)
			}
		}
	}

	// Remove frontends for ports no longer desired
	for portStr, fe := range currentFrontends {
		if _, exists := desiredPorts[portStr]; !exists {
			klog.Infof("Deleting unused frontend for port %s", portStr)
			_, err := l.client.Loadbalancers().DeleteFrontend(lb.ID, fe.ID)
			if err != nil {
				return fmt.Errorf("error deleting load balancer frontend: %w", err)
			}
		}
	}
	klog.Infof("Finish updateing Load balancer for cluster %q, LB ID %q", clusterName, lb.ID)

	return nil
}

func (l *loadbalancers) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	_, exists, err := l.GetLoadBalancer(ctx, clusterName, service)
	if err != nil {
		return err
	}
	// This is the same as if we were to check if err == errLbNotFound {
	if !exists {
		return nil
	}

	lb, err := l.getUthoLB(ctx, service)
	if err != nil {
		return err
	}

	_, err = l.client.Loadbalancers().Delete(lb.ID)
	if err != nil {
		return err
	}
	klog.Infof("Finish deleting Load balancer for cluster %q, LB ID %q", clusterName, lb.ID)

	return nil
}

func (l *loadbalancers) GetKubeClient() error {
	if l.kubeClient != nil {
		return nil
	}

	var (
		kubeConfig *rest.Config
		err        error
		config     string
	)

	// If no kubeconfig was passed in or set then we want to default to an empty string
	// This will have `clientcmd.BuildConfigFromFlags` default to `restclient.InClusterConfig()` which was existing behavior
	if Options.KubeconfigFlag == nil || Options.KubeconfigFlag.Value.String() == "" {
		config = ""
	} else {
		config = Options.KubeconfigFlag.Value.String()
	}

	kubeConfig, err = clientcmd.BuildConfigFromFlags("", config)
	if err != nil {
		return err
	}

	l.kubeClient, err = kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}

	return nil
}

func (l *loadbalancers) lbByName(lbName string) (*utho.Loadbalancer, error) {
	lbs, err := l.client.Loadbalancers().List()
	if err != nil {
		return nil, err
	}
	for _, lb := range lbs {
		if lb.Name == lbName {
			return &lb, nil
		}
	}

	return nil, errLbNotFound
}

func (l *loadbalancers) getUthoLB(ctx context.Context, service *v1.Service) (*utho.Loadbalancer, error) {
	if id, ok := service.Annotations[annoUthoLoadBalancerID]; ok {
		lb, err := l.client.Loadbalancers().Read(id)
		if err != nil {
			return nil, err
		}
		return lb, nil
	}

	defaultLBName := getDefaultLBName(service)
	if lb, err := l.lbByName(defaultLBName); err != nil {
		lbName := l.GetLoadBalancerName(ctx, "", service)
		lb, err = l.lbByName(lbName)
		if err != nil {
			return nil, err
		}
		return lb, nil
	} else {
		return lb, nil
	}
}

func getDefaultLBName(service *v1.Service) string {
	return cloudprovider.DefaultLoadBalancerName(service)
}

// CreateUthoLoadBalancer sets up a load balancer, its frontend, and backend configurations
func (l *loadbalancers) CreateUthoLoadBalancer(lbName, vpcId string, service *v1.Service, nodePoolId []string, clusterId string) (*utho.CreateLoadbalancerResponse, error) {
	// Create load balancer request parameters
	lbRequest := utho.CreateLoadbalancerParams{
		Name:           lbName,
		Dcslug:         l.zone,
		Vpc:            vpcId,
		Type:           "network",
		EnablePublicip: "true",
		Cpumodel:       "amd",
	}
	klog.Infof("Load balancer request: %+v", lbRequest)

	// Create the load balancer
	lb, err := l.client.Loadbalancers().Create(lbRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to create lb: %w", err)
	}

	ready := false
	// checks the status of a load balancer
	for i := 0; i < 10; i++ {
		readLb, err := l.client.Loadbalancers().Read(lb.ID)
		klog.Infof("Load balancer status request: %+v", readLb)
		klog.Infof("Load balancer status request: Status=%+v, AppStatus=%+v", readLb.Status, readLb.AppStatus)
		if err != nil {
			return nil, fmt.Errorf("failed to read lb: %w", err)
		}
		if strings.EqualFold(readLb.AppStatus, string(utho.Installed)) {
			ready = true
			break
		}

		time.Sleep(45 * time.Second)
	}
	if !ready {
		return nil, fmt.Errorf("the lb is not raedy in time please connect Utho support")
	}

	// Iterate over each service port to configure frontend and backend
	for _, port := range service.Spec.Ports {
		// Ensure the protocol is TCP
		if port.Protocol != v1.ProtocolTCP {
			return nil, fmt.Errorf("only TCP protocol is supported, got: %q", port.Protocol)
		}

		// Create load balancer frontend request parameters
		feRequest := utho.CreateLoadbalancerFrontendParams{
			LoadbalancerId: lb.ID,
			Name:           GenerateRandomString(10),
			Proto:          "tcp",
			Port:           strconv.Itoa(int(port.Port)),
			Algorithm:      "roundrobin",
			Cookie:         "0",
		}
		klog.Infof("Load balancer frontend request: %+v", feRequest)

		// Create the load balancer frontend
		lbFe, err := l.client.Loadbalancers().CreateFrontend(feRequest)
		if err != nil {
			return nil, fmt.Errorf("error creating load balancer frontend: %v", err)
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
			klog.Infof("Load balancer backend request: %+v", feBackend)

			// Create the load balancer backend
			_, err := l.client.Loadbalancers().CreateBackend(feBackend)
			if err != nil {
				return nil, fmt.Errorf("error creating load balancer backend: %v", err)
			}
		}
	}

	// Return the created load balancer
	return lb, nil
}
