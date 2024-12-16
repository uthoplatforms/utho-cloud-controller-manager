# Kubernetes Cloud Controller Manager for Utho

The Utho Cloud Controller Manager (ccm) provides a fully supported experience of Utho features in your Kubernetes cluster.

- Node resources are assigned their respective Utho instance hostnames, Region, PlanID and public/private IPs.
- Utho LoadBalancers are automatically deployed when a LoadBalancer service is deployed.

## Getting Started

More information about running Utho cloud controller manager can be found [here](docs)

Examples can also be found [here](docs/examples)

### **Note: do not modify utho load-balancers manually**
When a load-balancer is created through the CCM (Loadbalancer service type), you should not modify the load-balancer. Your changes will eventually get reverted back due to the CCM validating state.

Any changes to the load-balancer should be done through the service object.

## Development 

Go minimum version `1.23`

The `utho-cloud-controller-manager` uses go modules for its dependencies.

### Building the Docker Image

Since the `utho-cloud-controller-manager` is meant to run inside a kubernetes cluster you will need to build the binary to be Linux specific.

or by using our `Makefile`

`make build VERSION=0.1.0`

This will build the binary, docker image.

### Deploying to a kubernetes cluster

You will need to make sure that your kubernetes cluster is configured to interact with a `external cloud provider`

More can be read about this in the [Running Cloud Controller](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/)

To deploy the versioned CCM that Utho providers you will need to apply two yaml files to your cluster which can be found [here](docs/releases).

- Secret.yml will take in the region ID in which your cluster is deployed in and your API key.

- latest.yml is a preconfigured set of kubernetes resources which will help get the CCM installed.
