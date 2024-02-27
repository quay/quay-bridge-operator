/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-logr/logr"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/redhat-cop/operator-utils/pkg/util"

	qclient "github.com/quay/quay-bridge-operator/pkg/client/quay"
	qotypes "github.com/quay/quay-bridge-operator/pkg/types"

	quayv1 "github.com/quay/quay-bridge-operator/api/v1"

	"github.com/quay/quay-bridge-operator/pkg/constants"
	"github.com/quay/quay-bridge-operator/pkg/core"
	"github.com/quay/quay-bridge-operator/pkg/credentials"
	"github.com/quay/quay-bridge-operator/pkg/logging"
	"github.com/quay/quay-bridge-operator/pkg/utils"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
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

// NamespaceIntegrationReconciler reconciles a QuayIntegration object
type NamespaceIntegrationReconciler struct {
	CoreComponents core.CoreComponents
	Log            logr.Logger
}

//+kubebuilder:rbac:groups=quay.redhat.com,resources=quayintegrations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=quay.redhat.com,resources=quayintegrations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=quay.redhat.com,resources=quayintegrations/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;update
//+kubebuilder:rbac:groups="image.openshift.io",resources=imagestreams;imagestreamimports,verbs=get;list;watch;create;update;patch

func (r *NamespaceIntegrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.Info("Reconciling Namespace", "Name", req.Name)

	// Fetch the Namespace instance
	instance := &corev1.Namespace{}
	err := r.CoreComponents.ReconcilerBase.GetClient().Get(ctx, req.NamespacedName, instance)
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
	quayIntegrations := quayv1.QuayIntegrationList{}

	err = r.CoreComponents.ReconcilerBase.GetClient().List(ctx, &quayIntegrations, &client.ListOptions{})
	if err != nil {
		return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:  instance,
			Error:   err,
			Message: "Error Retrieving QuayIntegration",
		})
	}

	if len(quayIntegrations.Items) != 1 {
		return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:  instance,
			Message: "No QuayIntegrations defined or more than 1 integration present",
			Reason:  "ConfigurationError",
		})
	}

	quayIntegration := *&quayIntegrations.Items[0]

	// Check is this is a valid namespace (TODO: Use a predicate to filter out?)
	validNamespace := quayIntegration.IsAllowedNamespace(instance.Name)
	if !validNamespace {
		// Not a synchronized namespace
		return reconcile.Result{}, nil
	}

	if quayIntegration.Spec.CredentialsSecret == nil {
		return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:  instance,
			Message: "Required parameter 'CredentialsSecret' not found",
			Reason:  "ConfigurationError",
		})
	}

	secretCredential := &corev1.Secret{}

	err = r.CoreComponents.ReconcilerBase.GetClient().Get(ctx, types.NamespacedName{Namespace: quayIntegration.Spec.CredentialsSecret.Namespace, Name: quayIntegration.Spec.CredentialsSecret.Name}, secretCredential)
	if err != nil {
		return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       instance,
			Message:      "Error Locating Quay Integration Secret",
			Reason:       "ConfigurationError",
			KeyAndValues: []interface{}{"Namespace", quayIntegration.Spec.CredentialsSecret.Namespace, "Secret", quayIntegration.Spec.CredentialsSecret.Name},
		})
	}

	quaySecretCredentialTokenKey := constants.QuaySecretCredentialTokenKey

	if quayIntegration.Spec.CredentialsSecret.Key != "" {
		quaySecretCredentialTokenKey = quayIntegration.Spec.CredentialsSecret.Key
	}

	if _, ok := secretCredential.Data[quaySecretCredentialTokenKey]; !ok {
		return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       instance,
			Message:      fmt.Sprintf("Credential Secret does not contain key '%s'", quaySecretCredentialTokenKey),
			Reason:       "ConfigurationError",
			KeyAndValues: []interface{}{"Namespace", quayIntegration.Spec.CredentialsSecret.Namespace, "Secret", quayIntegration.Spec.CredentialsSecret.Name},
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
	quayOrganizationName := quayIntegration.GenerateQuayOrganizationNameFromNamespace(req.Name)

	if util.IsBeingDeleted(instance) {
		if !util.HasFinalizer(instance, constants.NamespaceFinalizer) {
			return reconcile.Result{}, nil
		}

		// Remove Resources
		result, err := r.cleanupResources(req, instance, quayClient, quayOrganizationName)
		if err != nil {
			return result, err
		}

		util.RemoveFinalizer(instance, constants.NamespaceFinalizer)
		err = r.CoreComponents.ReconcilerBase.GetClient().Update(ctx, instance)
		if err != nil {
			return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
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
		err := r.CoreComponents.ReconcilerBase.GetClient().Update(ctx, instance)
		if err != nil {
			return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
				Object:       instance,
				Message:      "Unable to update namespace",
				KeyAndValues: []interface{}{"Namespace", instance.Name},
				Error:        err,
			})
		}
		return reconcile.Result{}, nil
	}

	// Setup Resources
	result, err := r.setupResources(ctx, req, instance, quayClient, quayOrganizationName, quayIntegration.Spec.ClusterID, quayIntegration.Spec.QuayHostname)
	if err != nil {
		return result, err
	}

	return reconcile.Result{}, nil
}

func (r *NamespaceIntegrationReconciler) setupResources(ctx context.Context, request reconcile.Request, namespace *corev1.Namespace, quayClient *qclient.QuayClient, quayOrganizationName string, quayName string, quayHostname string) (reconcile.Result, error) {
	_, organizationResponse, organizationError := quayClient.GetOrganizationByname(quayOrganizationName)

	if organizationError.Error != nil {
		return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
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
			return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
				Object:       namespace,
				Message:      "Error occurred creating Quay Organization",
				KeyAndValues: []interface{}{"Status Code", createOrganizationResponse.StatusCode},
				Error:        organizationError.Error,
			})
		}
	} else if organizationResponse.StatusCode != 200 {
		return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       namespace,
			Message:      "Error occurred retrieving Quay Organization",
			KeyAndValues: []interface{}{"Organization", quayOrganizationName},
		})
	}

	var g errgroup.Group

	// Create Default Permissions
	for quayServiceAccountPermissionMatrixKey, quayServiceAccountPermissionMatrixValue := range QuayServiceAccountPermissionMatrix {
		func(quayServiceAccountPermissionMatrixKey qotypes.OpenShiftServiceAccount, quayServiceAccountPermissionMatrixValue qclient.QuayRole) {
			g.Go(func() error {
				if _, robotAccountErr := r.createRobotAccountAssociateToSA(ctx, request, namespace, quayClient, quayOrganizationName, quayServiceAccountPermissionMatrixKey, quayServiceAccountPermissionMatrixValue, quayName, quayHostname); robotAccountErr != nil {
					return robotAccountErr
				}
				return nil
			})
		}(quayServiceAccountPermissionMatrixKey, quayServiceAccountPermissionMatrixValue)
	}

	if err := g.Wait(); err != nil {
		return reconcile.Result{}, err
	}

	// Synchronize Namespaces
	imageStreams := imagev1.ImageStreamList{}

	err := r.CoreComponents.ReconcilerBase.GetClient().List(ctx, &imageStreams, &client.ListOptions{Namespace: namespace.Name})
	if err != nil {
		return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
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
		if repositoryHttpResponse == nil {
			return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
				Object:       namespace,
				Message:      "Error creating request to retrieve repository",
				KeyAndValues: []interface{}{"Namespace", namespace.Name, "Name", imageStreamName},
				Error:        repositoryErr.Error,
			})
		}

		if repositoryErr.Error != nil {
			return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
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
				return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
					Object:       namespace,
					Message:      "Error occurred creating Quay Repository",
					KeyAndValues: []interface{}{"Quay Repository", fmt.Sprintf("%s/%s", quayOrganizationName, imageStreamName), "Status Code", createRepositoryResponse.StatusCode},
					Error:        createRepositoryErr.Error,
				})
			}
		} else if repositoryHttpResponse.StatusCode != 200 {
			return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
				Object:       namespace,
				Message:      "Error Retrieving Repository for Namespace",
				KeyAndValues: []interface{}{"Quay Repository", fmt.Sprintf("%s/%s", quayOrganizationName, imageStreamName), "Status Code", repositoryHttpResponse.StatusCode},
			})
		}
	}

	return reconcile.Result{}, nil
}

// createRobotAccountAndSecret creates a robot account, creates a secret and adds the secret to the service account
func (r *NamespaceIntegrationReconciler) createRobotAccountAssociateToSA(ctx context.Context, request reconcile.Request, namespace *corev1.Namespace, quayClient *qclient.QuayClient, quayOrganizationName string, serviceAccount qotypes.OpenShiftServiceAccount, role qclient.QuayRole, quayName string, quayHostname string) (reconcile.Result, error) {
	// Setup Robot Account
	robotAccount, robotAccountResponse, robotAccountError := quayClient.GetOrganizationRobotAccount(quayOrganizationName, string(serviceAccount))
	if robotAccountResponse == nil {
		return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       namespace,
			Message:      "Error occured creating HTTP request to fetch Quay Organization Robot Acccount",
			KeyAndValues: []interface{}{"Quay Repository", quayOrganizationName, "Robot Account", serviceAccount},
			Error:        robotAccountError.Error,
		})
	}

	if robotAccountError.Error != nil {
		return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
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
			return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
				Object:       namespace,
				Message:      "Error occurred retrieving robot account for Quay Organization",
				KeyAndValues: []interface{}{"Quay Repository", quayOrganizationName, "Robot Account", serviceAccount, "Status Code", robotAccountResponse.StatusCode},
			})
		}
	}

	organizationPrototypes, organizationPrototypesResponse, organizationPrototypesError := quayClient.GetPrototypesByOrganization(quayOrganizationName)
	if organizationPrototypesError.Error != nil {
		return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       namespace,
			Message:      "Error occurred retrieving Prototypes for Quay Organization",
			KeyAndValues: []interface{}{"Quay Repository", quayOrganizationName, "Status Code", robotAccountResponse.StatusCode},
			Error:        organizationPrototypesError.Error,
		})
	}

	if organizationPrototypesResponse.StatusCode != 200 {
		return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       namespace,
			Message:      "Error occurred retrieving Prototypes for Quay Organization",
			KeyAndValues: []interface{}{"Quay Repository", quayOrganizationName, "Status Code", robotAccountResponse.StatusCode},
		})
	}

	if found := qclient.IsRobotAccountInPrototypeByRole(organizationPrototypes.Prototypes, robotAccount.Name, string(role)); !found {
		// Create Prototype
		_, robotPrototypeResponse, robotPrototypeError := quayClient.CreateRobotPermissionForOrganization(quayOrganizationName, robotAccount.Name, string(role))
		if robotPrototypeError.Error != nil || robotPrototypeResponse.StatusCode != 200 {
			return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
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
		return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       namespace,
			Message:      "Failed to parse Quay hostname",
			KeyAndValues: []interface{}{"Hostname", quayHostname},
			Error:        quayURLErr,
		})
	}

	// Setup Secret for Quay Robot Account
	robotSecret, robotSecretErr := credentials.GenerateDockerJsonSecret(utils.GenerateDockerJsonSecretNameForServiceAccount(string(serviceAccount), quayName), quayURL.Host, robotAccount.Name, robotAccount.Token, "")
	if robotSecretErr != nil {
		return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       namespace,
			Message:      "Failed to generate Docker JSON Secret for Service Account",
			KeyAndValues: []interface{}{"Namespace", namespace.Name, "Robot Account", robotAccount.Name, "Service Account", serviceAccount},
			Error:        robotSecretErr,
		})
	}

	robotSecret.ObjectMeta.Namespace = namespace.Name

	robotCreateSecretErr := r.CoreComponents.ReconcilerBase.CreateOrUpdateResource(ctx, nil, namespace.Name, robotSecret)
	if robotCreateSecretErr != nil {
		return reconcile.Result{Requeue: true}, robotSecretErr
	}

	existingServiceAccount := &corev1.ServiceAccount{}
	serviceAccountErr := r.CoreComponents.ReconcilerBase.GetClient().Get(ctx, types.NamespacedName{Namespace: namespace.Name, Name: string(serviceAccount)}, existingServiceAccount)
	if serviceAccountErr != nil {
		return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       namespace,
			Message:      "Failed to get existing platform service account",
			KeyAndValues: []interface{}{"Namespace", namespace.Name, "Service Account", serviceAccount},
			Error:        serviceAccountErr,
		})

	}

	_, updated := r.updateSecretWithMountablePullSecret(existingServiceAccount, robotSecret.Name)
	if updated {
		updatedServiceAccountErr := r.CoreComponents.ReconcilerBase.CreateOrUpdateResource(ctx, nil, namespace.Name, existingServiceAccount)
		if updatedServiceAccountErr != nil {
			return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
				Object:       namespace,
				Message:      "Failed to to updated existing platform service account",
				KeyAndValues: []interface{}{"Namespace", namespace.Name, "Service Account", serviceAccount},
				Error:        updatedServiceAccountErr,
			})
		}
	}

	return reconcile.Result{}, nil
}

func (r *NamespaceIntegrationReconciler) cleanupResources(request reconcile.Request, namespace *corev1.Namespace, quayClient *qclient.QuayClient, quayOrganizationName string) (reconcile.Result, error) {
	logging.Log.Info("Deleting Organization", "Organization Name", quayOrganizationName)

	_, organizationResponse, organizationError := quayClient.GetOrganizationByname(quayOrganizationName)
	if organizationError.Error != nil {
		return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       namespace,
			Message:      "Error occurred retrieving Organization",
			KeyAndValues: []interface{}{"Quay Organization", quayOrganizationName, "Status Code", organizationResponse.StatusCode},
			Error:        organizationError.Error,
		})
	}

	// Check to see if Organization Exists (Response Code)
	if organizationResponse.StatusCode == 404 {
		return reconcile.Result{}, nil
		// Organization is not present
	} else if organizationResponse.StatusCode == 200 {
		organizationDeleteResponse, organizationDeleteError := quayClient.DeleteOrganization(quayOrganizationName)
		if organizationDeleteResponse == nil {
			return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
				Object:       namespace,
				Message:      "Error occurred building HTTP request to delete Organization",
				KeyAndValues: []interface{}{"Quay Organization", quayOrganizationName},
				Error:        organizationDeleteError.Error,
			})
		}

		if organizationDeleteError.Error != nil {
			return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
				Object:       namespace,
				Message:      "Error occurred deleting Organization",
				KeyAndValues: []interface{}{"Quay Organization", quayOrganizationName, "Status Code", organizationDeleteResponse.StatusCode},
				Error:        organizationDeleteError.Error,
			})
		}

		if organizationDeleteResponse.StatusCode != 204 {
			return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
				Object:       namespace,
				Message:      "Error occurred deleting Organization",
				KeyAndValues: []interface{}{"Quay Organization", quayOrganizationName, "Status Code", organizationDeleteResponse.StatusCode},
			})
		}
		return reconcile.Result{}, nil

	} else {
		return r.CoreComponents.ManageError(&core.QuayIntegrationCoreError{
			Object:       namespace,
			Message:      "Error occurred retrieving Organization",
			KeyAndValues: []interface{}{"Quay Organization", quayOrganizationName, "Status Code", organizationResponse.StatusCode},
		})
	}
}

func (r *NamespaceIntegrationReconciler) updateSecretWithMountablePullSecret(serviceAccount *corev1.ServiceAccount, name string) (*corev1.ServiceAccount, bool) {
	var updated bool

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

// SetupWithManager sets up the controller with the Manager.
func (r *NamespaceIntegrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	//Retriggers a reconcilation of a namespace upon a change to an ImageStream within a namespace. Currently only supports adding repositories to Quay
	imageStreamToNamespace := handler.MapFunc(
		func(a client.Object) []reconcile.Request {
			res := []reconcile.Request{}
			res = append(res, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: a.GetNamespace(),
				},
			})
			return res
		})

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Watches(&source.Kind{Type: &imagev1.ImageStream{}}, handler.EnqueueRequestsFromMapFunc(imageStreamToNamespace)).
		Complete(r)
}
