package utho

import (
	"context"
	"fmt"

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
		vpcId, err := GetLabelValue("vpc_id")
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

	// Fetch the load balancer details
	loadBalancer, err := l.client.Loadbalancers().Read(lb.ID)
	if err != nil {
		return fmt.Errorf("failed to get load balancer: %w", err)
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

	// Iterate over each service port to configure frontends and backends
	for _, port := range service.Spec.Ports {
		// Ensure the protocol is TCP
		if port.Protocol != v1.ProtocolTCP {
			return fmt.Errorf("only TCP protocol is supported, got: %q", port.Protocol)
		}

		// Check if the port is already configured in a frontend
		portStr := strconv.Itoa(int(port.Port))
		exists := false
		for _, fe := range loadBalancer.Frontends {
			if fe.Port == portStr {
				exists = true
				break
			}
		}

		// If frontend doesn't exist, create it
		if !exists {
			feRequest := utho.CreateLoadbalancerFrontendParams{
				LoadbalancerId: lb.ID,
				Name:           GenerateRandomString(10),
				Proto:          "tcp",
				Port:           portStr,
				Algorithm:      "roundrobin",
			}
			klog.Infof("Load balancer frontend request: %+v", feRequest)

			// Create the frontend
			lbFe, err := l.client.Loadbalancers().CreateFrontend(feRequest)
			if err != nil {
				return fmt.Errorf("error creating load balancer frontend: %w", err)
			}

			// Configure backends for the frontend
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

				// Create the backend
				_, err = l.client.Loadbalancers().CreateBackend(feBackend)
				if err != nil {
					return fmt.Errorf("error creating load balancer backend: %w", err)
				}
			}
		}
	}

	return nil
}

func (l *loadbalancers) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {

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

// buildInstanceList create list of nodes to be attached to a load balancer
func buildInstanceList(nodes []*v1.Node) ([]string, error) {
	var list []string

	for _, node := range nodes {
		instanceID, err := getInstanceIDFromProviderID(node)
		if err != nil {
			return nil, fmt.Errorf("error getting the provider ID %s : %s", node.Spec.ProviderID, err)
		}

		list = append(list, instanceID)
	}

	return list, nil
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
