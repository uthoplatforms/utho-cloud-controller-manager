package lb_controller

// Declare global Error statuses to be used

const (
	CertificateIDNotFound    string = "Certificate ID Not Found"
	TGAlreadyAttached        string = "Requested Target group already part of this Cluster."
	LBAlreadyAttached        string = "Requested Load balancer already part of this Cluster."
	FrontendIDNotFound       string = "no frontend id found in the status field"
	LBIDNotFound             string = "no lb id found in the status field"
	TGAlreadyExists          string = "Target Group with same name already in your account, Please provide different name."
	ACLAlreadyExists         string = "Duplicate Entry"
	LBAlreadyDeleted         string = "Sorry we unable to find this load balancer or you dont have access!"
	TGAlreadyDeleted         string = "Permission Denied, Possible reason not resource not exists."
	ACLIDNotFound            string = "ACL ID Not Found"
	RoutingRuleAlreadyExists string = "A routing rule with the same lbid, acl_id, and backend_id already exists."
)
