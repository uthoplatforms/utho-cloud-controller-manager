![Unit Tests](https://github.com/utho/utho-cloud-controller-manager/workflows/Unit%20Tests/badge.svg)
# Kubernetes Cloud Controller Manager for Utho

The Utho Cloud Controller Manager (ccm) provides a fully supported experience of Utho features in your Kubernetes cluster.

- Node resources are assigned their respective Utho instance hostnames, Region, PlanID and public/private IPs.
- Node resources get put into their proper state if they are shutdown or removed. This allows for Kubernetes to properly reschedule pods
- Utho LoadBalancers are automatically deployed when a LoadBalancer service is deployed.

This plugin is in active development and you can track progress in the [Milestones](https://github.com/utho/utho-cloud-controller-manager/milestone/1).

## Getting Started

More information about running Utho cloud controller manager can be found [here](docs)

Examples can also be found [here](docs/examples)

### **Note: do not modify utho load-balancers manually**
When a load-balancer is created through the CCM (Loadbalancer service type), you should not modify the load-balancer. Your changes will eventually get reverted back due to the CCM validating state.

Any changes to the load-balancer should be done through the service object.

## Development 

Go minimum version `1.19.0`

The `utho-cloud-controller-manager` uses go modules for its dependencies.

### Building the Binary

Since the `utho-cloud-controller-manager` is meant to run inside a kubernetes cluster you will need to build the binary to be Linux specific.

`GOOS=linux GOARCH=amd64 go build -o dist/utho-cloud-controller-manager .`

or by using our `Makefile`

`make build-linux`

This will build the binary and output it to a `dist` folder.

**Note** However if you wish to build the binary with the OS you are using you can run `make build`

### Building the Docker Image

To build a docker image of the `utho-cloud-controller-manager` you can use the `docker-build` entry in the make file. Take note that it requires 2 variables 

- Version 
- REGISTRY (dockerhub registry name)

an example could be 

`VERSION=v0.1.0 REGISTRY=utho make docker-build`

or if you chose to run it manually

`docker build . -t utho-cloud-controller-manager`

Running the image

`docker run -ti utho/utho-cloud-controller-manager`

### Deploying to a kubernetes cluster

You will need to make sure that your kubernetes cluster is configured to interact with a `external cloud provider`

More can be read about this in the [Running Cloud Controller](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/)

To deploy the versioned CCM that Utho providers you will need to apply two yaml files to your cluster which can be found [here](https://github.com/utho/utho-cloud-controller-manager/tree/master/docs/releases).

- Secret.yml will take in the region ID in which your cluster is deployed in and your API key.

- v0.X.X.yml is a preconfigured set of kubernetes resources which will help get the CCM installed.


