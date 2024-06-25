# Utho Application Operator
## Table of Contents
- [Prerequisites](#prerequiites)
- [Build Process](#build)
- [How it Works](#work)
- [Versioning](#versioning)
- [Utho Application CRD Reference](#crd)
  - [Utho Application Spec](#mainspec)
  - [Load Balancer Spec](#lb)
  - [Target Group Spec](#tg)
  - [Frontend Spec](#fe)
  - [ACL Spec](#acl)


Utho Application Operator is a [Kubernetes Operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) that is used to manage various resources required by your Kubernetes Based Application like Load Balancer, etc.

With this Operator, you can do the following:
- Create a CRD called **UthoApplication** and provide networking parameters.
- Manage your resources from the CRD
- Hassle Free Netwokr Resource Provisioning.

<article id="prerequiites"></article>

### Prerequisites

In order to build the operator you'll have to have Go installed on your machine.
In order to do so, follow the instructions on its [website](https://go.dev/).

<article id="build"></article>

### Build process

Building the operator should be as simple as running:

```console
make build
```

This `Makefile` target will take care of everything from generating client side code,
generating Kubernetes manifests, downloading the dependencies and the tools used
in the build process and finally, it will build the binary.

After this step has finished successfully you should see the operator's binary `bin/manager`.

You can also run it directly via `make run` which will run the operator on your
machine against the cluster that you have configured via your `KUBECONFIG`.

<article id="work"></article>

### How it works
Utho Application Controller provides a `UthoApplication` CRD to specify the deployment of network resources like Utho LB, Utho TG, etc. Here is an example of `UthoApplication`.

#### ALB Example
```yaml
apiVersion: apps.utho.com/v1alpha1
kind: UthoApplication
metadata:
  name: my-application
  namespace: <namespace>
spec:
  loadBalancer:
    aclRule:
      - name: test-rule
        conditionType: url_path
        value:
          type: url_path
          data:
            - "/"
            - "/path"
    frontend:
      name: test-fe-3
      algorithm: roundrobin
      protocol: http
      port: 81
    type: application
    dcslug: innoida
    name: test-lb
  targetGroups:
    - health_check_timeout: 5
      health_check_interval: 30
      health_check_path: /
      health_check_protocol: TCP
      healthy_threshold: 2
      name: test-tg-blaa
      protocol: TCP
      unhealthy_threshold: 3
      port: 30002
    - health_check_timeout: 5
      health_check_interval: 30
      health_check_path: /
      health_check_protocol: TCP
      healthy_threshold: 2
      name: test-tg-2
      protocol: TCP
      unhealthy_threshold: 4
      port: 30002
```

#### NLB example
```yaml
apiVersion: apps.utho.com/v1alpha1
kind: UthoApplication
metadata:
  name: test-app-nlb
spec:
  loadBalancer:
    backendPort: 30080
    frontend:
      name: test-fe
      algorithm: roundrobin
      protocol: tcp
      port: 80
    type: network
    dcslug: innoida
    name: test-lb
```
You can choose to apply the CRD in any namespace that you want. However, we recommend to create a separate namespace so that you can track all of your CRs easily.

<article id="versioning"></article>

### Versioning
As of now, this operator is still in developmental state. This is the v1alpha1 version. In case, you face a problem/bug, please raise an issue in Github.

<article id="crd"></article>

### UthoApplication CRD Reference

<article id="mainspec"></article>

#### UthoApplicationSpec
Defines the desired state of the UthoApplication.

| FIELD         | DESCRIPTION                                             |
|---------------|---------------------------------------------------------|
| `apiVersion`  | `apps.utho.com/v1alpha1`                                |
| `kind`        | `UthoApplication`                                       |
| `metadata`    | Refer to Kubernetes API documentation for fields of metadata. |
| `spec`        | `UthoApplicationSpec`                                   |

#### UthoApplicationSpec
Specifies the UthoApplication configuration.

| FIELD           | DESCRIPTION                                             |
|-----------------|---------------------------------------------------------|
| `loadBalancer`  | `LoadBalancer`                                          |
| `targetGroups`  | `[]TargetGroup`                                         |

<article id="lb"></article>

#### LoadBalancer
Specifies the load balancer configuration.

| FIELD           | DESCRIPTION                                             | EXAMPLE VALUES |
|-----------------|---------------------------------------------------------|----------------|
| `frontend`      | `Frontend`, optional                                     |                |
| `type`          | `string`, default: `application`                        | `application`  |
| `dcslug`        | `string`                                                | `innoida`      |
| `name`          | `string`                                                | `my-lb`        | 
| `aclRule`       | `[]ACLRule`, optional                                    |                |

<article id="fe"></article>

#### Frontend
Specifies the frontend configuration.

| FIELD             | DESCRIPTION                           | EXAMPLE VALUES          |
|-------------------|---------------------------------------|-------------------------|
| `name`            | `string`                              | `test-fe`               |
| `algorithm`       | `string`                              | `roundrobin` or `leastconn` |
| `protocol`        | `string`                              | `http` or `https`           |
| `port`            | `int64`                               | `80` or `443`               |
| `certificateName` | `string`, optional                    | test-name               |
| `redirectHttps`   | `bool`, optional                      | `0` for no, `1` for yes     |
| `cookie`          | `bool`, optional                      | `0` for no, `1` for yes     |

<article id="acl"></article>

#### ACLRule
Specifies an ACL rule.

| FIELD           | DESCRIPTION                             | EXAMPLE VALUES |
|-----------------|-----------------------------------------|----------------|
| `name`          | `string`                                | `test-rule` |         
| `conditionType` | `string`                                | `http_user_agent`, `http_referer`, `url_path`, `http_method`, `query_string`, `http_header` |
| `value`         | `ACLData`                               | |

#### ACLData
Specifies the ACL data.

| FIELD         | DESCRIPTION                               | EXAMPLE VALUES |
|---------------|-------------------------------------------|----------------|
| `type`        | `string`                                  | `http_user_agent`, `http_referer`, `url_path`, `http_method`, `query_string`, `http_header` |
| `data`        | `[]string`                                | `/` |

<article id="tg"></article>

#### TargetGroup
Specifies a target group configuration.

| FIELD                   | DESCRIPTION                       | EXAMPLE VALUES                |
|-------------------------|-----------------------------------|-------------------------------|
| `name`                  | `string`                          | `test-tg`                     |
| `protocol`              | `string`                          | `HTTP`, `TCP`, `HTTPS`, `UDP` |
| `health_check_path`     | `string`                          | `/healthz`                    |
| `health_check_protocol` | `string`                          | `HTTP`, `TCP`, `HTTPS`, `UDP` |
| `health_check_interval` | `int64`                           | `30`                          |
| `health_check_timeout`  | `int64`                           | `5` |
| `healthy_threshold`     | `int64`                           | `3` |
| `unhealthy_threshold`   | `int64`                           | `2` |
| `port`                  | `int64`                           | `80` |

[//]: # (## API Calls Used in Order)

[//]: # (- Create Load Balancer - API Route - https://api.utho.com/v2/loadbalancer)

[//]: # (- Create Target Group &#40;API To Utho&#41;)

[//]: # (- Get Control Plane IP from the cluster &#40;Kubernetes API&#41; - GET Node --label selctor = "node-role.kubernetes.io/control-plane". Status Field Internal IP)

[//]: # (- List Kubernetes for the Account &#40;API to Utho&#41;)

[//]: # (- Get Kubernetes ID from the result)

[//]: # (- Attach Load Balancer to the Cluster)

[//]: # (- Attach Target Group to Cluster)

[//]: # ()
[//]: # (Important Issue: https://github.com/kubernetes-sigs/kubebuilder/issues/618)

[//]: # ()
[//]: # (	if app.Status.ObservedGeneration != app.ObjectMeta.Generation {)

[//]: # (		app.Status.ObservedGeneration = app.ObjectMeta.Generation)

[//]: # (		if err := r.Status&#40;&#41;.Update&#40;ctx, app&#41;; err != nil {)

[//]: # (			l.Error&#40;err, "Couldn't Set Observed Generation"&#41;)

[//]: # (			return ctrl.Result{}, errors.Wrap&#40;err, "Couldn't Set Observed Generation"&#41;)

[//]: # (		})
