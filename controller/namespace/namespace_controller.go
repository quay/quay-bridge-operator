package namespace

import (
	"context"
	"crypto/tls"
	"net/url"

	"fmt"
	"net/http"

	imagev1 "github.com/openshift/api/image/v1"
	"github.com/redhat-cop/operator-utils/pkg/util"
	redhatcopv1alpha1 "github.com/redhat-cop/quay-openshift-registry-operator/api/redhatcop/v1alpha1"
	qclient "github.com/redhat-cop/quay-openshift-registry-operator/pkg/client/quay"
	"github.com/redhat-cop/quay-openshift-registry-operator/pkg/constants"
	"github.com/redhat-cop/quay-openshift-registry-operator/pkg/core"
	"github.com/redhat-cop/quay-openshift-registry-operator/pkg/credentials"
	"github.com/redhat-cop/quay-openshift-registry-operator/pkg/k8sutils"
	"github.com/redhat-cop/quay-openshift-registry-operator/pkg/logging"
	qotypes "github.com/redhat-cop/quay-openshift-registry-operator/pkg/types"
	"github.com/redhat-cop/quay-openshift-registry-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	// QuayServiceAccountPermissionMatrix contains a mapping between OpenShift Service Accounts and Quay Roles
	QuayServiceAccountPermissionMatrix = map[qotypes.OpenShiftServiceAccount]qclient.QuayRole{
		qotypes.BuilderOpenShiftServiceAccount:  qclient.QuayRoleWrite,
		qotypes.DefaultOpenShiftServiceAccount:  qclient.QuayRoleRead,
		qotypes.DeployerOpenShiftServiceAccount: qclient.QuayRoleRead,
	}
)

// Add creates a new QuayIntegration Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {

	k8sclient, err := k8sutils.GetK8sClient(mgr.GetConfig())

	if err != nil {
		return err
	}

	return add(mgr, newReconciler(mgr, k8sclient))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, k8sclient kubernetes.Interface) reconcile.Reconciler {

	reconcilerBase := util.NewReconcilerBase(mgr.GetClient(), mgr.GetScheme(), mgr.GetConfig(), mgr.GetRecorder("namespace-controller"))

	coreComponents := core.NewCoreComponents(reconcilerBase)

	return &ReconcileNamespace{k8sclient: k8sclient, coreComponents: coreComponents}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("namespace-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Namespace
	err = c.Watch(&source.Kind{Type: &corev1.Namespace{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Retriggers a reconcilation of a namespace upon a change to an ImageStream within a namespace. Currently only supports adding repositories to Quay
	imageStreamToNamespace := handler.ToRequestsFunc(
		func(a handler.MapObject) []reconcile.Request {
			res := []reconcile.Request{}

			is := a.Object.(*imagev1.ImageStream)
			res = append(res, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: is.GetNamespace(),
				},
			})

			return res

		})

	// Watch for changes to ImageStreams and trigger namespace reconcile
	err = c.Watch(&source.Kind{Type: &imagev1.ImageStream{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: imageStreamToNamespace,
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileQuayIntegration implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileNamespace{}

// ReconcileNamespace reconciles a QuayIntegration object
type ReconcileNamespace struct {
	k8sclient      kubernetes.Interface
	coreComponents core.CoreComponents
}

// Reconcile reads that state of the cluster for a QuayIntegration object and makes changes based on the state read
// and what is in the QuayIntegration.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileNamespace) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	logging.Log.Info("Reconciling Namespace", "Name", request.Name)

	// Fetch the Namespace instance
	instance := &corev1.Namespace{}
	err := r.coreComponents.ReconcilerBase.GetClient().Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Find the Current Registered QuayIntegration objects
	quayIntegrations := redhatcopv1alpha1.QuayIntegrationList{}

	err = r.coreComponents.ReconcilerBase.GetClient().List(context.TODO(), &client.ListOptions{}, &quayIntegrations)

	if err != nil {
		return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:  instance,
			Error:   err,
			Message: "Error Retrieving QuayIntegration",
		})
	}

	if len(quayIntegrations.Items) != 1 {
		return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:  instance,
			Message: "No QuayIntegrations defined or more than 1 integration present",
			Reason:  "ConfigrurationError",
		})
	}

	quayIntegration := *&quayIntegrations.Items[0]

	// Check is this is a valid namespace (TODO: Use a predicate to filter out?)
	validNamespace := quayIntegration.IsAllowedNamespace(instance.Name)

	if !validNamespace {

		// Not a synchronized namespace
		return reconcile.Result{}, nil
	}

	if len(quayIntegration.Spec.CredentialsSecretName) == 0 {

		return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:  instance,
			Message: "Required parameter 'CredentialsSecretName' not found",
			Reason:  "ConfigrurationError",
		})

	}

	secretNamespace, secretName, secretError := cache.SplitMetaNamespaceKey(quayIntegration.Spec.CredentialsSecretName)

	if secretError != nil {
		return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:  instance,
			Message: "Error Parsing Quay Integration Secret Name",
			Reason:  "ConfigrurationError",
			Error:   secretError,
		})

	}

	secretCredential := &corev1.Secret{}

	err = r.coreComponents.ReconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Namespace: secretNamespace, Name: secretName}, secretCredential)

	if err != nil {
		return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       instance,
			Message:      "Error Locating Quay Integration Secret",
			Reason:       "ConfigrurationError",
			KeyAndValues: []interface{}{"Namespace", secretNamespace, "Secret", secretName},
		})
	}

	if _, ok := secretCredential.Data[constants.QuaySecretCredentialTokenKey]; !ok {
		return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       instance,
			Message:      "Credential Secret does not contain key 'token'",
			Reason:       "ConfigrurationError",
			KeyAndValues: []interface{}{"Namespace", secretNamespace, "Secret", secretName},
		})
	}

	authToken := string(secretCredential.Data[constants.QuaySecretCredentialTokenKey])

	// Setup Quay Client
	quayClient := qclient.NewClient(&http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}, quayIntegration.Spec.QuayHostname, authToken)

	// Create Organization
	quayOrganizationName := quayIntegration.GenerateQuayOrganizationNameFromNamespace(request.Name)

	if util.IsBeingDeleted(instance) {
		if !util.HasFinalizer(instance, constants.NamespaceFinalizer) {
			return reconcile.Result{}, nil
		}

		// Remove Resources
		result, err := r.cleanupResources(request, instance, quayClient, quayOrganizationName)

		if err != nil {
			return result, err
		}

		util.RemoveFinalizer(instance, constants.NamespaceFinalizer)
		err = r.coreComponents.ReconcilerBase.GetClient().Update(context.TODO(), instance)
		if err != nil {
			return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
				Object:       instance,
				Message:      "Unable to update namespace",
				KeyAndValues: []interface{}{"Namespace", instance.Name},
				Error:        err,
			})
		}
		return reconcile.Result{}, nil

	}

	// Finalizer Management
	if !util.HasFinalizer(instance, constants.NamespaceFinalizer) {

		// Check if OpenShift Project
		if utils.IsOpenShiftAnnotatedNamespace(instance) {

			if _, sccMcsFound := instance.Annotations[constants.OpenShiftSccMcsAnnotation]; !sccMcsFound {
				return reconcile.Result{}, nil
			}

		}

		util.AddFinalizer(instance, constants.NamespaceFinalizer)
		err := r.coreComponents.ReconcilerBase.GetClient().Update(context.TODO(), instance)
		if err != nil {
			return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
				Object:       instance,
				Message:      "Unable to update namespace",
				KeyAndValues: []interface{}{"Namespace", instance.Name},
				Error:        err,
			})
		}
		return reconcile.Result{}, nil
	}

	// Setup Resources
	result, err := r.setupResources(request, instance, quayClient, quayOrganizationName, quayIntegration.Spec.ClusterID, quayIntegration.Spec.QuayHostname)

	if err != nil {
		return result, err
	}

	return reconcile.Result{}, nil

}

func (r *ReconcileNamespace) setupResources(request reconcile.Request, namespace *corev1.Namespace, quayClient *qclient.QuayClient, quayOrganizationName string, quayName string, quayHostname string) (reconcile.Result, error) {
	_, organizationResponse, organizationError := quayClient.GetOrganizationByname(quayOrganizationName)

	if organizationError.Error != nil {
		return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       namespace,
			Message:      "Error occurred retrieving Quay Organization",
			KeyAndValues: []interface{}{"Organization", quayOrganizationName},
			Error:        organizationError.Error,
		})
	}

	// Check to see if Organization Exists (Response Code)
	if organizationResponse.StatusCode == 404 {

		// Create Organization
		logging.Log.Info("Organization Does Not Exist", "Name", quayOrganizationName)

		_, createOrganizationResponse, createOrganizationError := quayClient.CreateOrganization(quayOrganizationName)

		if createOrganizationError.Error != nil || createOrganizationResponse.StatusCode != 201 {
			return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
				Object:       namespace,
				Message:      "Error occurred creating Quay Organization",
				KeyAndValues: []interface{}{"Status Code", createOrganizationResponse.StatusCode},
				Error:        organizationError.Error,
			})
		}

	} else if organizationResponse.StatusCode != 200 {

		return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       namespace,
			Message:      "Error occurred retrieving Quay Organization",
			KeyAndValues: []interface{}{"Organization", quayOrganizationName},
		})
	}

	// Create Default Permissions
	for quayServiceAccountPermissionMatrixKey, quayServiceAccountPermissionMatrixValue := range QuayServiceAccountPermissionMatrix {

		robotAccountResult, robotAccountErr := r.createRobotAccountAssociateToSA(request, namespace, quayClient, quayOrganizationName, quayServiceAccountPermissionMatrixKey, quayServiceAccountPermissionMatrixValue, quayName, quayHostname)

		if robotAccountErr != nil {
			return robotAccountResult, robotAccountErr
		}

	}

	// Synchronize Namespaces
	imageStreams := imagev1.ImageStreamList{}

	err := r.coreComponents.ReconcilerBase.GetClient().List(context.TODO(), &client.ListOptions{Namespace: namespace.Name}, &imageStreams)

	if err != nil {
		return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       namespace,
			Message:      "Error Retrieving ImageStreams for Namespace",
			KeyAndValues: []interface{}{"Namespace", namespace.Name},
			Error:        err,
		})

	}

	for _, imageStream := range imageStreams.Items {

		imageStreamName := imageStream.Name
		// Check if Repository Exists
		_, repositoryHttpResponse, repositoryErr := quayClient.GetRepository(quayOrganizationName, imageStreamName)

		if repositoryErr.Error != nil {
			return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
				Object:       namespace,
				Message:      "Error Retrieving Repository",
				KeyAndValues: []interface{}{"Namespace", namespace.Name, "Name", imageStreamName, "Status Code", repositoryHttpResponse.StatusCode},
				Error:        repositoryErr.Error,
			})

		}

		// If an Repository reports back that it cannot be found or permission dened
		if repositoryHttpResponse.StatusCode == 403 || repositoryHttpResponse.StatusCode == 404 {
			logging.Log.Info("Creating Repository", "Organization", quayOrganizationName, "Name", imageStreamName)

			_, createRepositoryResponse, createRepositoryErr := quayClient.CreateRepository(quayOrganizationName, imageStreamName)

			if createRepositoryErr.Error != nil || createRepositoryResponse.StatusCode != 201 {
				return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
					Object:       namespace,
					Message:      "Error occurred creating Quay Repository",
					KeyAndValues: []interface{}{"Quay Repository", fmt.Sprintf("%s/%s", quayOrganizationName, imageStreamName), "Status Code", createRepositoryResponse.StatusCode},
					Error:        createRepositoryErr.Error,
				})

			}

		} else if repositoryHttpResponse.StatusCode != 200 {
			return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
				Object:       namespace,
				Message:      "Error Retrieving Repository for Namespace",
				KeyAndValues: []interface{}{"Quay Repository", fmt.Sprintf("%s/%s", quayOrganizationName, imageStreamName), "Status Code", repositoryHttpResponse.StatusCode},
			})
		}

	}

	return reconcile.Result{}, nil

}

// createRobotAccountAndSecret creates a robot account, creates a secret and adds the secret to the service account
func (r *ReconcileNamespace) createRobotAccountAssociateToSA(request reconcile.Request, namespace *corev1.Namespace, quayClient *qclient.QuayClient, quayOrganizationName string, serviceAccount qotypes.OpenShiftServiceAccount, role qclient.QuayRole, quayName string, quayHostname string) (reconcile.Result, error) {
	// Setup Robot Account
	robotAccount, robotAccountResponse, robotAccountError := quayClient.GetOrganizationRobotAccount(quayOrganizationName, string(serviceAccount))

	if robotAccountError.Error != nil {
		return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       namespace,
			Message:      "Error occurred retrieving robot account for Quay Organization",
			KeyAndValues: []interface{}{"Quay Repository", quayOrganizationName, "Robot Account", serviceAccount, "Status Code", robotAccountResponse.StatusCode},
			Error:        robotAccountError.Error,
		})
	}

	// Check to see if Robot Exists
	if robotAccountResponse.StatusCode == 400 {

		// Create Robot Account
		robotAccount, robotAccountResponse, robotAccountError = quayClient.CreateOrganizationRobotAccount(quayOrganizationName, string(serviceAccount))

		if robotAccountError.Error != nil || robotAccountResponse.StatusCode != 201 {
			return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
				Object:       namespace,
				Message:      "Error occurred retrieving robot account for Quay Organization",
				KeyAndValues: []interface{}{"Quay Repository", quayOrganizationName, "Robot Account", serviceAccount, "Status Code", robotAccountResponse.StatusCode},
			})

		}

	}

	organizationPrototypes, organizationPrototypesResponse, organizationPrototypesError := quayClient.GetPrototypesByOrganization(quayOrganizationName)

	if organizationPrototypesError.Error != nil {
		return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       namespace,
			Message:      "Error occurred retrieving Prototypes for Quay Organization",
			KeyAndValues: []interface{}{"Quay Repository", quayOrganizationName, "Status Code", robotAccountResponse.StatusCode},
			Error:        organizationPrototypesError.Error,
		})

	}

	if organizationPrototypesResponse.StatusCode != 200 {
		return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       namespace,
			Message:      "Error occurred retrieving Prototypes for Quay Organization",
			KeyAndValues: []interface{}{"Quay Repository", quayOrganizationName, "Status Code", robotAccountResponse.StatusCode},
		})

	}

	if found := qclient.IsRobotAccountInPrototypeByRole(organizationPrototypes.Prototypes, robotAccount.Name, string(role)); !found {
		// Create Prototype
		_, robotPrototypeResponse, robotPrototypeError := quayClient.CreateRobotPermissionForOrganization(quayOrganizationName, robotAccount.Name, string(role))

		if robotPrototypeError.Error != nil || robotPrototypeResponse.StatusCode != 200 {
			return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
				Object:       namespace,
				Message:      "Error occurred creating Robot account permissions for Prototype",
				KeyAndValues: []interface{}{"Quay Repository", quayOrganizationName, "Robot Account", robotAccount.Name, "Prototype", role, "Status Code", robotPrototypeResponse.StatusCode},
				Error:        robotPrototypeError.Error,
			})
		}

	}

	// Parse out hostname from Quay Hostname
	quayURL, quayURLErr := url.Parse(quayHostname)

	if quayURLErr != nil {
		return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       namespace,
			Message:      "Failed to parse Quay hostname",
			KeyAndValues: []interface{}{"Hostname", quayHostname},
			Error:        quayURLErr,
		})

	}

	// Setup Secret for Quay Robot Account
	robotSecret, robotSecretErr := credentials.GenerateDockerJsonSecret(utils.GenerateDockerJsonSecretNameForServiceAccount(string(serviceAccount), quayName), quayURL.Host, robotAccount.Name, robotAccount.Token, "")
	robotSecret.ObjectMeta.Namespace = namespace.Name

	if robotSecretErr != nil {
		return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       namespace,
			Message:      "Failed to generate Docker JSON Secret for Service Account",
			KeyAndValues: []interface{}{"Namespace", namespace.Name, "Robot Account", robotAccount.Name, "Service Account", serviceAccount},
			Error:        robotSecretErr,
		})
	}

	robotCreateSecretErr := r.coreComponents.ReconcilerBase.CreateOrUpdateResource(nil, namespace.Name, robotSecret)

	if robotCreateSecretErr != nil {
		return reconcile.Result{Requeue: true}, robotSecretErr
	}

	existingServiceAccount := &corev1.ServiceAccount{}
	serviceAccountErr := r.coreComponents.ReconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Namespace: namespace.Name, Name: string(serviceAccount)}, existingServiceAccount)

	if serviceAccountErr != nil {
		return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       namespace,
			Message:      "Failed to get existing platform service account",
			KeyAndValues: []interface{}{"Namespace", namespace.Name, "Service Account", serviceAccount},
			Error:        serviceAccountErr,
		})

	}

	_, updated := r.updateSecretWithMountablePullSecret(existingServiceAccount, robotSecret.Name)

	if updated {

		updatedServiceAccountErr := r.coreComponents.ReconcilerBase.CreateOrUpdateResource(nil, namespace.Name, existingServiceAccount)

		if updatedServiceAccountErr != nil {
			return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
				Object:       namespace,
				Message:      "Failed to to updated existing platform service account",
				KeyAndValues: []interface{}{"Namespace", namespace.Name, "Service Account", serviceAccount},
				Error:        updatedServiceAccountErr,
			})

		}
	}

	return reconcile.Result{}, nil

}

func (r *ReconcileNamespace) cleanupResources(request reconcile.Request, namespace *corev1.Namespace, quayClient *qclient.QuayClient, quayOrganizationName string) (reconcile.Result, error) {

	logging.Log.Info("Deleting Organization", "Organization Name", quayOrganizationName)

	_, organizationResponse, orgniazationError := quayClient.GetOrganizationByname(quayOrganizationName)

	if orgniazationError.Error != nil {
		return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       namespace,
			Message:      "Error occurred retrieving Organization",
			KeyAndValues: []interface{}{"Quay Organization", quayOrganizationName, "Status Code", organizationResponse.StatusCode},
			Error:        orgniazationError.Error,
		})
	}

	// Check to see if Organization Exists (Response Code)
	if organizationResponse.StatusCode == 404 {
		return reconcile.Result{}, nil
		// Organization is not present
	} else if organizationResponse.StatusCode == 200 {
		organizationDeleteResponse, orgniazationDeleteError := quayClient.DeleteOrganization(quayOrganizationName)

		if orgniazationDeleteError.Error != nil {
			return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
				Object:       namespace,
				Message:      "Error occurred deleting Organization",
				KeyAndValues: []interface{}{"Quay Organization", quayOrganizationName, "Status Code", organizationDeleteResponse.StatusCode},
				Error:        orgniazationDeleteError.Error,
			})
		}

		if organizationDeleteResponse.StatusCode != 204 {
			return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
				Object:       namespace,
				Message:      "Error occurred deleting Organization",
				KeyAndValues: []interface{}{"Quay Organization", quayOrganizationName, "Status Code", organizationDeleteResponse.StatusCode},
			})
		}

		return reconcile.Result{}, nil

	} else {
		return r.coreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       namespace,
			Message:      "Error occurred retrieving Organization",
			KeyAndValues: []interface{}{"Quay Organization", quayOrganizationName, "Status Code", organizationResponse.StatusCode},
		})
	}

}

func (r *ReconcileNamespace) updateSecretWithMountablePullSecret(serviceAccount *corev1.ServiceAccount, name string) (*corev1.ServiceAccount, bool) {

	updated := false

	if found := utils.LocalObjectReferenceNameExists(serviceAccount.ImagePullSecrets, name); !found {

		serviceAccount.ImagePullSecrets = append(serviceAccount.ImagePullSecrets, corev1.LocalObjectReference{Name: name})

		updated = true
	}

	if found := utils.ObjectReferenceNameExists(serviceAccount.Secrets, name); !found {

		serviceAccount.Secrets = append(serviceAccount.Secrets, corev1.ObjectReference{Name: name})

		updated = true
	}

	return serviceAccount, updated
}
