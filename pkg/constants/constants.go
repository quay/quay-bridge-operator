package constants

import "time"

const (
	AnnotationBase                                   = "quay-registry-operator.quay.redhat.com"
	OrganizationPrefix                               = "openshift"
	QuaySecretCredentialTokenKey                     = "token"
	NamespaceFinalizer                               = "quay.redhat.com/quayintegrations"
	OpenShiftDisplayNameAnnotation                   = "openshift.io/display-name"
	OpenShiftDescriptionAnnotation                   = "openshift.io/description"
	OpenShiftSccMcsAnnotation                        = "openshift.io/sa.scc.mcs"
	DisableWebhookEnvVar                             = "DISABLE_WEBHOOK"
	WebHookCertDirEnv                                = "WEBHOOK_CERT_DIR"
	DefaultWebhookCertDir                            = "/apiserver.local.config/certificates"
	WebhookCertName                                  = "apiserver.crt"
	WebhookKeyName                                   = "apiserver.key"
	BuildOperatorManagedAnnotation                   = AnnotationBase + "/quay-registry-operator-managed"
	BuildDestinationImageStreamAnnotation            = AnnotationBase + "/destination-imagestream"
	BuildDestinationImageStreamTagImportedAnnotation = AnnotationBase + "/destination-imagestreamtag-imported"
	RequeuePeriod                                    = time.Second * 5
)
