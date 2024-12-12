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

const (
	// Supported Protocols
	protocolHTTP  = "http"
	protocolHTTPS = "https"
	protocolTCP   = "tcp"

	portProtocolTCP = "TCP"
	portProtocolUDP = "UDP"

	healthCheckInterval  = 15
	healthCheckResponse  = 5
	healthCheckUnhealthy = 5
	healthCheckHealthy   = 5

	lbStatusActive = "active"
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

func (l *loadbalancers) GetLoadBalancer(ctx context.Context, _ string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	lb, err := l.getUthoLB(ctx, service)
	if err != nil {
		if err == errLbNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}

	return &v1.LoadBalancerStatus{
		Ingress: ingress,
	}, true, nil
}

func (l *loadbalancers) GetLoadBalancerName(_ context.Context, _ string, service *v1.Service) string {
	if label, ok := service.Annotations[annoVultrLoadBalancerLabel]; ok {
		return label
	}
	return getDefaultLBName(service)
}

func (l *loadbalancers) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	_, exists, err := l.GetLoadBalancer(ctx, clusterName, service)
	if err != nil {
		return nil, err
	}

	// if exists is false and the err above was nil then this is errLbNotFound
	if !exists {
		klog.Infof("Load balancer for cluster %q doesn't exist, creating", clusterName)

		lbName := l.GetLoadBalancerName(context.Background(), "", service)

		x := utho.CreateLoadbalancerParams{
			Name:     lbName,
			Dcslug:   l.zone,
			Type:     "network",
			Vpc:      annoVultrVPC,
			Firewall: annoVultrFirewallRules,
		}
		lb, err := l.client.Loadbalancers().Create(x)
		if err != nil {
			return nil, fmt.Errorf("failed to create load-balancer: %s", err)
		}
		klog.Infof("Created load balancer %q", lb.ID)

		// Set the Vultr VLB ID annotation
		if _, ok := service.Annotations[annoVultrLoadBalancerID]; !ok {
			if err = l.GetKubeClient(); err != nil {
				return nil, fmt.Errorf("failed to get kubeclient to update service: %s", err)
			}

			service, err = l.kubeClient.CoreV1().Services(service.Namespace).Get(ctx, service.Name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to get service with loadbalancer ID: %s", err)
			}

			if service.Annotations == nil {
				service.Annotations = make(map[string]string)
			}
			service.Annotations[annoVultrLoadBalancerID] = lb.ID

			_, err = l.kubeClient.CoreV1().Services(service.Namespace).Update(ctx, service, metav1.UpdateOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to update service with loadbalancer ID: %s", err)
			}
		}

		// if lb.Status != lbStatusActive {
		// 	return nil, fmt.Errorf("load-balancer is not yet active - current status: %s", lb.Status)
		// }

		var ingress []v1.LoadBalancerIngress

		getLb, err := l.client.Loadbalancers().Read(lb.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get utho lb with loadbalancer ID: %s", err)
		}

		hostname := getLb.Name
		// Check if hostname annotation is blank and set if not
		if _, ok := service.Annotations[annoVultrHostname]; ok {
			if service.Annotations[annoVultrHostname] != "" {
				if govalidator.IsDNSName(service.Annotations[annoVultrHostname]) {
					hostname = service.Annotations[annoVultrHostname]
				} else {
					return nil, fmt.Errorf("hostname %s is not a valid DNS name", service.Annotations[annoVultrHostname])
				}
				klog.Infof("setting hostname for loadbalancer to: %s", hostname)
				ingress = append(ingress, v1.LoadBalancerIngress{Hostname: hostname})
			}
		} else {
			ingress = append(ingress, v1.LoadBalancerIngress{IP: getLb.IP})
		}

		return &v1.LoadBalancerStatus{
			Ingress: ingress,
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

	// Set the Vultr VLB ID annotation
	if _, ok := service.Annotations[annoVultrLoadBalancerID]; !ok {
		if service.Annotations == nil {
			service.Annotations = make(map[string]string)
		}
		service.Annotations[annoVultrLoadBalancerID] = lb.ID
		if err = l.GetKubeClient(); err != nil {
			return nil, fmt.Errorf("failed to get kubeclient to update service: %s", err)
		}
		_, err = l.kubeClient.CoreV1().Services(service.Namespace).Update(ctx, service, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to update service with loadbalancer ID: %s", err)
		}
	}

	// if lb.Status != lbStatusActive {
	// 	return nil, fmt.Errorf("load-balancer is not yet active - current status: %s", lb.Status)
	// }

	if err2 := l.UpdateLoadBalancer(ctx, clusterName, service, nodes); err2 != nil {
		return nil, err2
	}

	lbStatus, _, err := l.GetLoadBalancer(ctx, clusterName, service)
	if err != nil {
		return nil, err
	}

	return lbStatus, nil
}

func (l *loadbalancers) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	// klog.V(3).Info("Called UpdateLoadBalancers")
	// if _, _, err := l.GetLoadBalancer(ctx, clusterName, service); err != nil {
	// 	return err
	// }

	// lb, err := l.getUthoLB(ctx, service)
	// if err != nil {
	// 	return err
	// }

	// // Set the Vultr VLB ID annotation
	// if _, ok := service.Annotations[annoVultrLoadBalancerID]; !ok {
	// 	service.Annotations[annoVultrLoadBalancerID] = lb.ID
	// 	if err = l.GetKubeClient(); err != nil {
	// 		return fmt.Errorf("failed to get kubeclient to update service: %s", err)
	// 	}
	// 	_, err = l.kubeClient.CoreV1().Services(service.Namespace).Update(ctx, service, metav1.UpdateOptions{})
	// 	if err != nil {
	// 		return fmt.Errorf("failed to update service with loadbalancer ID: %s", err)
	// 	}
	// }

	// lbReq, err := l.buildLoadBalancerRequest(service, nodes)
	// if err != nil {
	// 	return fmt.Errorf("failed to create load balancer request: %s", err)
	// }

	// if err := l.client.LoadBalancer.Update(ctx, lb.ID, lbReq); err != nil {
	// 	return fmt.Errorf("failed to update LB: %s", err)
	// }

	return nil
}

func (l *loadbalancers) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	// _, exists, err := l.GetLoadBalancer(ctx, clusterName, service)
	// if err != nil {
	// 	return err
	// }
	// // This is the same as if we were to check if err == errLbNotFound {
	// if !exists {
	// 	return nil
	// }

	// lb, err := l.getUthoLB(ctx, service)
	// if err != nil {
	// 	return err
	// }

	// err = l.client.LoadBalancer.Delete(ctx, lb.ID)
	// if err != nil {
	// 	return err
	// }

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

func getSSLPassthrough(service *v1.Service) bool {
	passThrough, ok := service.Annotations[annoVultrLBSSLPassthrough]
	if !ok {
		return false
	}

	pass, err := strconv.ParseBool(passThrough)
	if err != nil {
		return false
	}
	return pass
}

func getProxyProtocol(service *v1.Service) bool {
	proxy, ok := service.Annotations[annoVultrProxyProtocol]
	if !ok {
		return false
	}

	pass, err := strconv.ParseBool(proxy)
	if err != nil {
		return false
	}

	return pass
}

func buildFirewallRules(service *v1.Service) ([]govultr.LBFirewallRule, error) {
	lbFWRules := []govultr.LBFirewallRule{}
	fwRules := getFirewallRules(service)
	if fwRules == "" {
		return lbFWRules, nil
	}

	for _, v := range strings.Split(fwRules, ";") {
		fwRule := govultr.LBFirewallRule{}

		rules := strings.Split(v, ",")
		if len(rules) != 2 {
			return nil, fmt.Errorf("loadbalancer fw rules : %s invalid configuration", rules)
		}

		source := rules[0]
		ipType := "v4"
		if source != "cloudflare" {
			ip, _, err := net.ParseCIDR(source)
			if err != nil {
				return nil, fmt.Errorf("loadbalancer fw rules : source %s is invalid", source)
			}

			if ip.To4() == nil {
				ipType = "v6"
			}
		}

		port, err := strconv.Atoi(rules[1])
		if err != nil {
			return nil, fmt.Errorf("loadbalancer fw rules : port %d is invalid", port)
		}

		fwRule.Source = source
		fwRule.IPType = ipType
		fwRule.Port = port
		lbFWRules = append(lbFWRules, fwRule)
	}
	return lbFWRules, nil
}

func getFirewallRules(service *v1.Service) string {
	fwRules, ok := service.Annotations[annoVultrFirewallRules]
	if !ok {
		return ""
	}

	return fwRules
}

func getVPC(service *v1.Service) (string, error) {
	var vpc string
	pn, pnOk := service.Annotations[annoVultrPrivateNetwork]
	v, vpcOk := service.Annotations[annoVultrVPC]

	if vpcOk && pnOk {
		return "", fmt.Errorf("can not use private_network and vpc annotations. Please use VPC as private network is deprecated")
	} else if pnOk {
		vpc = pn
	} else if vpcOk {
		vpc = v
	} else {
		return "", nil
	}

	if strings.EqualFold(vpc, "false") {
		return "", nil
	}

	meta := metadata.NewClient()
	m, err := meta.Metadata()
	if err != nil {
		return "", fmt.Errorf("error getting metadata for private_network: %v", err.Error())
	}

	pnID := ""
	for _, v := range m.Interfaces {
		if v.NetworkV2ID != "" {
			pnID = v.NetworkV2ID
			break
		}
	}

	return pnID, nil
}

func getBackendProtocol(service *v1.Service) string {
	proto, ok := service.Annotations[annoVultrLBBackendProtocol]
	if !ok {
		return ""
	}

	switch proto {
	case "http":
		return protocolHTTP
	case "https":
		return protocolHTTPS
	case "tcp":
		return protocolTCP
	default:
		return ""
	}
}
