package controller

const (
	CertificateIDNotFound string = "Certificate ID Not Found"
	TGAlreadyAttached     string = "Requested Target group already part of this Cluster."
	LBAlreadyAttached     string = "Requested Load balancer already part of this Cluster."
	FrontendIDNotFound    string = "no frontend id found in the status field"
	LBIDNotFound          string = "no lb id found in the status field"
	TGAlreadyExists       string = "Target Group with same name already in your account, Please provide different name."
)
