package types

type QuayInstance struct {
	URL       string
	AuthToken string
}

type OpenShiftServiceAccount string

const (
	BuilderOpenShiftServiceAccount  OpenShiftServiceAccount = "builder"
	DefaultOpenShiftServiceAccount  OpenShiftServiceAccount = "default"
	DeployerOpenShiftServiceAccount OpenShiftServiceAccount = "deployer"
)
