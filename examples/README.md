# Load Balancers

Utho cloud controller manager runs service controller, which is
responsible for watching Custom services called `UthoApplication` and creating Loadbalancer inside Utho platform
loadbalancers to satify its requirements. 

Here are some examples of how it's used.

## Examples

### Network Load Balancer

Here's an example of an application running Nginx with newtowrk load balancer in namespace `dev` 

Feel free to change around before applying

```bash
kubectl apply -f network/
```

Get deployment status:

Note: update namespace accordingly
```bash
kubectl get all,uthoapplications -n dev
```

### Application Load Balancer

Here's an example of an application running Nginx with application load balancer in namespace `dev` 

Feel free to change around before applying
```bash
kubectl apply -f application/
```

Get deployment status:

Note: update namespace accordingly
```bash
kubectl get all,uthoapplications -n dev
```