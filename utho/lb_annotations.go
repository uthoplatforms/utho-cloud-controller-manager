package utho

const (
	// annoUthoLoadBalancerName is used to set custom labels for load balancers
	annoUthoLoadBalancerName = "service.beta.kubernetes.io/utho-loadbalancer-name"

	// annoUthoLoadBalancerID is used to identify individual Utho load balancers, this is managed by the CCM
	annoUthoLoadBalancerID = "service.beta.kubernetes.io/utho-loadbalancer-id"
)
