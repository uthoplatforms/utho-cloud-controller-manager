package utho

const (
	// annoUthoLoadBalancerName is used to set custom labels for load balancers.
	// This allows users to define a specific name for the Utho load balancer.
	annoUthoLoadBalancerName = "service.beta.kubernetes.io/utho-loadbalancer-name"

	// annoUthoLoadBalancerID is used to identify individual Utho load balancers.
	// This annotation is managed automatically by the CCM and should not be manually modified.
	annoUthoLoadBalancerID = "service.beta.kubernetes.io/utho-loadbalancer-id"

	// annoUthoAlgorithm defines the load balancing algorithm for the load balancer.
	// Accepted values: "roundrobin" or "leastconn".
	annoUthoAlgorithm = "service.beta.kubernetes.io/utho-loadbalancer-algorithm"

	// annoUthoStickySessionEnabled determines whether sticky sessions are enabled for the load balancer.
	// Accepted values: "true" or "false".
	annoUthoStickySessionEnabled = "service.beta.kubernetes.io/utho-loadbalancer-sticky-session-enabled"

	// annoUthoRedirectHTTPToHTTPS determines whether HTTP traffic should be redirected to HTTPS.
	// Accepted values: "true" or "false".
	annoUthoRedirectHTTPToHTTPS = "service.beta.kubernetes.io/utho-loadbalancer-redirect-http-to-https"

	// annoUthoLBSSLID is used to specify the SSL certificate ID for the load balancer.
	// This is required when enabling HTTPS on a load balancer.
	annoUthoLBSSLID = "service.beta.kubernetes.io/utho-loadbalancer-ssl-id"

	// annoUthoNetworkType defines the network type for the load balancer.
	// Accepted values: "private" or "public" (defaults to "public" if not specified).
	annoUthoNetworkType = "service.beta.kubernetes.io/utho-loadbalancer-network-type"
)
