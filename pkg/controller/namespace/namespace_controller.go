package namespace

import (
	"context"
	"crypto/tls"
	"net/url"
	"time"

	"fmt"
	"net/http"

	imagev1 "github.com/openshift/api/image/v1"
	"github.com/redhat-cop/operator-utils/pkg/util"
	redhatcopv1alpha1 "github.com/redhat-cop/quay-openshift-registry-operator/pkg/apis/redhatcop/v1alpha1"
	qclient "github.com/redhat-cop/quay-openshift-registry-operator/pkg/client/quay"
	"github.com/redhat-cop/quay-openshift-registry-operator/pkg/constants"
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

	return &ReconcileNamespace{reconcilerBase: reconcilerBase, k8sclient: k8sclient}
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
	reconcilerBase util.ReconcilerBase
	k8sclient      kubernetes.Interface
}

// Reconcile reads that state of the cluster for a QuayIntegration object and makes changes based on the state read
// and what is in the QuayIntegration.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileNamespace) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := logging.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling QuayIntegration")
	// Fetch the Namespace instance
	instance := &corev1.Namespace{}
	err := r.reconcilerBase.GetClient().Get(context.TODO(), request.NamespacedName, instance)
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

	err = r.reconcilerBase.GetClient().List(context.TODO(), &client.ListOptions{}, &quayIntegrations)

	if err != nil {
		return reconcile.Result{}, err
	}

	if len(quayIntegrations.Items) != 1 {
		logging.Log.Info("No QuayIntegrations defined or more than 1 integration present")
		return reconcile.Result{}, nil
	}

	quayIntegration := *&quayIntegrations.Items[0]

	// Check is this is a valid namespace (TODO: Use a predicate to filter out?)
	validNamespace := quayIntegration.IsAllowedNamespace(instance.Name)

	if !validNamespace {
		return reconcile.Result{}, nil
	}

	if len(quayIntegration.Spec.CredentialsSecretName) == 0 {
		err := fmt.Errorf("Required parameter 'CredentialsSecretName' not found")
		logging.Log.Error(err, "Required parameter 'CredentialsSecretName' not found")

		return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, err
	}

	secretNamespace, secretName, secretError := cache.SplitMetaNamespaceKey(quayIntegration.Spec.CredentialsSecretName)

	if secretError != nil {
		logging.Log.Error(err, "Error Parsing Quay Integration Secret Name")
		return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, err
	}

	secretCredential := &corev1.Secret{}

	err = r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Namespace: secretNamespace, Name: secretName}, secretCredential)

	if err != nil {
		logging.Log.Error(err, "Error Locating Quay Integration Secret")
		return reconcile.Result{Requeue: true, RequeueAfter: time.Second * 5}, err
	}

	if _, ok := secretCredential.Data[constants.QuaySecretCredentialTokenKey]; !ok {
		logging.Log.Error(fmt.Errorf("Credential Secret does not contain key 'token'"), "Credential Secret does not contain key 'token'")
		return reconcile.Result{}, err
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
		result, err := r.cleanupResources(request, quayClient, quayOrganizationName)

		if err != nil {
			return result, err
		}

		util.RemoveFinalizer(instance, constants.NamespaceFinalizer)
		err = r.reconcilerBase.GetClient().Update(context.TODO(), instance)
		if err != nil {
			logging.Log.Error(err, "Unable to update namespace", "namespace", instance.Name)
			return reconcile.Result{}, err
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
		err := r.reconcilerBase.GetClient().Update(context.TODO(), instance)
		if err != nil {
			logging.Log.Error(err, "Unable to update namespace", "namespace", instance.Name)
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	// Setup Resources
	result, err := r.setupResources(request, instance.Name, quayClient, quayOrganizationName, quayIntegration.Spec.ClusterID, quayIntegration.Spec.QuayHostname)

	if err != nil {
		return result, err
	}

	return reconcile.Result{}, nil

}

func (r *ReconcileNamespace) setupResources(request reconcile.Request, namespaceName string, quayClient *qclient.QuayClient, quayOrganizationName string, quayName string, quayHostname string) (reconcile.Result, error) {
	_, organizationResponse, organizationError := quayClient.GetOrganizationByname(quayOrganizationName)

	if organizationError != nil {
		logging.Log.Error(fmt.Errorf("Error occurred retrieving Organization"), "Error occurred retrieving Organization")
		return reconcile.Result{}, organizationError
	}

	// Check to see if Organization Exists (Response Code)
	if organizationResponse.StatusCode == 404 {

		// Create Organization
		logging.Log.Info("Organization Does Not Exist", "Name", quayOrganizationName)

		_, createOrganizationResponse, createOrganizationError := quayClient.CreateOrganization(quayOrganizationName)

		if createOrganizationError != nil || createOrganizationResponse.StatusCode != 201 {
			logging.Log.Error(fmt.Errorf("Error occurred creating Organization"), "Error occurred creating Organization", "Status Code", createOrganizationResponse.StatusCode)
			return reconcile.Result{Requeue: true}, createOrganizationError
		}

	} else if organizationResponse.StatusCode != 200 {
		logging.Log.Error(organizationError, "Error occurred retrieving Organization")
		return reconcile.Result{Requeue: true}, organizationError
	}

	// Create Default Permissions
	for quayServiceAccountPermissionMatrixKey, quayServiceAccountPermissionMatrixValue := range QuayServiceAccountPermissionMatrix {

		robotAccountResult, robotAccountErr := r.createRobotAccountAssociateToSA(request, namespaceName, quayClient, quayOrganizationName, quayServiceAccountPermissionMatrixKey, quayServiceAccountPermissionMatrixValue, quayName, quayHostname)

		if robotAccountErr != nil {
			return robotAccountResult, robotAccountErr
		}

	}

	// Synchronize Namespaces
	imageStreams := imagev1.ImageStreamList{}

	err := r.reconcilerBase.GetClient().List(context.TODO(), &client.ListOptions{Namespace: namespaceName}, &imageStreams)

	if err != nil {
		logging.Log.Error(err, "Error Retrieving ImageStreams for Namespace", "Namespace", namespaceName)
		return reconcile.Result{Requeue: true}, err
	}

	for _, imageStream := range imageStreams.Items {
		logging.Log.Info("ImageStream Found in Namespace", "Namespace", namespaceName, "Name", imageStream.Name)

		imageStreamName := imageStream.Name
		// Check if Repository Exists
		_, repositoryHttpResponse, repositoryErr := quayClient.GetRepository(quayOrganizationName, imageStreamName)

		if repositoryErr != nil {

			logging.Log.Error(err, "Error Retrieving Repository", "Namespace", namespaceName, "Name", imageStreamName, "Status Code", repositoryHttpResponse.StatusCode)
			return reconcile.Result{}, err
		}

		// If an Repository reports back that it cannot be found or permission dened
		if repositoryHttpResponse.StatusCode == 403 || repositoryHttpResponse.StatusCode == 404 {
			logging.Log.Info("Creating Repository", "Organization", quayOrganizationName, "Name", imageStreamName)

			_, createRepositoryResponse, createRepositoryErr := quayClient.CreateRepository(quayOrganizationName, imageStreamName)

			if createRepositoryErr != nil || createRepositoryResponse.StatusCode != 201 {
				logging.Log.Error(fmt.Errorf("Error occurred creating repository"), "Error occurred creating repository", "Status Code", createRepositoryResponse.StatusCode)
				return reconcile.Result{Requeue: true}, createRepositoryErr
			}

		} else if repositoryHttpResponse.StatusCode != 200 {
			logging.Log.Error(err, "Error Retrieving Repository for Namespace", "Namespace", namespaceName, "Name", imageStreamName)
			return reconcile.Result{Requeue: true}, err

		}

	}

	return reconcile.Result{}, nil

}

// createRobotAccountAndSecret creates a robot account, creates a secret and adds the secret to the service account
func (r *ReconcileNamespace) createRobotAccountAssociateToSA(request reconcile.Request, namespaceName string, quayClient *qclient.QuayClient, quayOrganizationName string, serviceAccount qotypes.OpenShiftServiceAccount, role qclient.QuayRole, quayName string, quayHostname string) (reconcile.Result, error) {
	// Setup Robot Account
	robotAccount, robotAccountResponse, robotAccountError := quayClient.GetOrganizationRobotAccount(quayOrganizationName, string(serviceAccount))

	if robotAccountError != nil {
		logging.Log.Error(robotAccountError, "Error occurred retrieving robot")
		return reconcile.Result{}, robotAccountError
	}

	// Check to see if Robot Exists
	if robotAccountResponse.StatusCode == 400 {

		// Create Robot Account
		robotAccount, robotAccountResponse, robotAccountError = quayClient.CreateOrganizationRobotAccount(quayOrganizationName, string(serviceAccount))

		if robotAccountError != nil || robotAccountResponse.StatusCode != 201 {
			logging.Log.Error(robotAccountError, "Error creating robot account", "Robot Account", serviceAccount, "Status Code", robotAccountResponse.StatusCode)
			return reconcile.Result{Requeue: true}, robotAccountError
		}

	}

	organizationPrototypes, organizationPrototypesResponse, organizationPrototypesError := quayClient.GetPrototypesByOrganization(quayOrganizationName)

	if organizationPrototypesError != nil {
		logging.Log.Error(organizationPrototypesError, "Error occurred retrieving Prototypes")
		return reconcile.Result{}, organizationPrototypesError
	}

	if organizationPrototypesResponse.StatusCode != 200 {
		logging.Log.Error(organizationPrototypesError, "Error occurred retrieving Prototypes", "Organization", quayOrganizationName, "Status Code", organizationPrototypesResponse.StatusCode)
		return reconcile.Result{}, organizationPrototypesError
	}

	if found := qclient.IsRobotAccountInPrototypeByRole(organizationPrototypes.Prototypes, robotAccount.Name, string(role)); !found {
		// Create Prototype
		_, robotPrototypeResponse, robotPrototypeError := quayClient.CreateRobotPermissionForOrganization(quayOrganizationName, robotAccount.Name, string(role))

		if robotPrototypeError != nil || robotPrototypeResponse.StatusCode != 200 {
			logging.Log.Error(robotPrototypeError, "Error occurred creating robot account permissions", "Robot Account", robotAccount.Name, "Role", role, "Status Code", robotPrototypeResponse.StatusCode)
			return reconcile.Result{}, robotPrototypeError
		}

	}

	// Parse out hostname from Quay Hostname
	quayURL, quayURLErr := url.Parse(quayHostname)

	if quayURLErr != nil {
		logging.Log.Error(quayURLErr, "Failed to parse Quay hostname")
	}

	// Setup Secret for Quay Robot Account
	robotSecret, robotSecretErr := credentials.GenerateDockerJsonSecret(utils.GenerateDockerJsonSecretNameForServiceAccount(string(serviceAccount), quayName), quayURL.Host, robotAccount.Name, robotAccount.Token, "")
	robotSecret.ObjectMeta.Namespace = namespaceName

	if robotSecretErr != nil {
		return reconcile.Result{}, robotSecretErr
	}

	robotCreateSecretErr := r.reconcilerBase.CreateOrUpdateResource(nil, namespaceName, robotSecret)

	if robotCreateSecretErr != nil {
		return reconcile.Result{Requeue: true}, robotSecretErr
	}

	existingServiceAccount := &corev1.ServiceAccount{}
	serviceAccountErr := r.reconcilerBase.GetClient().Get(context.TODO(), types.NamespacedName{Namespace: namespaceName, Name: string(serviceAccount)}, existingServiceAccount)

	if serviceAccountErr != nil {
		logging.Log.Error(serviceAccountErr, "Failed to get existing platform service account", "Service Account", string(serviceAccount), "Namespace", namespaceName)
		return reconcile.Result{Requeue: true}, serviceAccountErr
	}

	_, updated := r.updateSecretWithMountablePullSecret(existingServiceAccount, robotSecret.Name)

	if updated {
		// updatedServiceAccountErr := r.reconcilerBase.GetClient().Update(context.TODO(), existingServiceAccount)
		updatedServiceAccountErr := r.reconcilerBase.CreateOrUpdateResource(nil, namespaceName, existingServiceAccount)

		if updatedServiceAccountErr != nil {
			logging.Log.Error(serviceAccountErr, "Failed to to updated existing platform service account", "Service Account", string(serviceAccount))
			return reconcile.Result{Requeue: true}, robotSecretErr
		}
	}

	return reconcile.Result{}, nil

}

func (r *ReconcileNamespace) cleanupResources(request reconcile.Request, quayClient *qclient.QuayClient, quayOrganizationName string) (reconcile.Result, error) {

	logging.Log.Info("Deleting Organization", "Organization Name", quayOrganizationName)

	_, organizationResponse, orgniazationError := quayClient.GetOrganizationByname(quayOrganizationName)

	if orgniazationError != nil {
		logging.Log.Error(fmt.Errorf("Error occurred retrieving Organization"), "Error occurred retrieving Organization")
		return reconcile.Result{}, orgniazationError
	}

	// Check to see if Organization Exists (Response Code)
	if organizationResponse.StatusCode == 404 {
		return reconcile.Result{}, nil
		// Organization is not present
	} else if organizationResponse.StatusCode == 200 {
		organizationDeleteResponse, orgniazationDeleteError := quayClient.DeleteOrganization(quayOrganizationName)

		if orgniazationDeleteError != nil {
			logging.Log.Error(orgniazationDeleteError, "Error occurred deleting Organization", "Status Code", organizationDeleteResponse.StatusCode)
			return reconcile.Result{Requeue: true}, orgniazationError
		}

		if organizationDeleteResponse.StatusCode != 204 {
			logging.Log.Error(fmt.Errorf("Error occurred deleting Organization"), "Status Code", organizationDeleteResponse.StatusCode)
			return reconcile.Result{Requeue: true}, fmt.Errorf("Error occurred deleting Organization")
		}

		return reconcile.Result{}, nil

	} else {
		logging.Log.Error(orgniazationError, "Error occurred retrieving Organization")
		return reconcile.Result{Requeue: true}, orgniazationError
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
