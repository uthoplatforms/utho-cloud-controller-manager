# utho-cloud-controller-manager

## API Calls Used in Order
- Create Load Balancer - API Route - https://api.utho.com/v2/loadbalancer
- Create Target Group (API To Utho)
- Get Control Plane IP from the cluster (Kubernetes API) - GET Node --label selctor = "node-role.kubernetes.io/control-plane". Status Field Internal IP
- List Kubernetes for the Account (API to Utho)
- Get Kubernetes ID from the result
- Attach Load Balancer to the Cluster
- Attach Target Group to Cluster

Important Issue: https://github.com/kubernetes-sigs/kubebuilder/issues/618

	if app.Status.ObservedGeneration != app.ObjectMeta.Generation {
		app.Status.ObservedGeneration = app.ObjectMeta.Generation
		if err := r.Status().Update(ctx, app); err != nil {
			l.Error(err, "Couldn't Set Observed Generation")
			return ctrl.Result{}, errors.Wrap(err, "Couldn't Set Observed Generation")
		}
