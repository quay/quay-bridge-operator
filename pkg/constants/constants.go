package constants

import "time"

const (
	AnnotationBase                                   = "quay-registry-operator.redhatcop.redhat.io"
	OrganizationPrefix                               = "openshift"
	QuaySecretCredentialTokenKey                     = "token"
	NamespaceFinalizer                               = "redhatcop.redhat.io/quayintegrations"
	OpenShiftDisplayNameAnnotation                   = "openshift.io/display-name"
	OpenShiftDescriptionAnnotation                   = "openshift.io/description"
	OpenShiftSccMcsAnnotation                        = "openshift.io/sa.scc.mcs"
	WebHookOnlyModeEnabledEnvVar                     = "WEBHOOK_ONLY_MODE"
	DisableWebhookEnvVar                             = "DISABLE_WEBHOOK"
	WebHookCertFileLocation                          = "/etc/webhook/certs/tls.crt"
	WebHookkeyFileLocation                           = "/etc/webhook/certs/tls.key"
	BuildOperatorManagedAnnotation                   = AnnotationBase + "/quay-registry-operator-managed"
	BuildDestinationImageStreamAnnotation            = AnnotationBase + "/destination-imagestream"
	BuildDestinationImageStreamTagImportedAnnotation = AnnotationBase + "/destination-imagestreamtag-imported"
	RequeuePeriod                                    = time.Second * 5
)
