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
	"encoding/json"

	"github.com/go-logr/logr"

	quayv1 "github.com/quay/quay-bridge-operator/api/v1"
	"github.com/redhat-cop/operator-utils/pkg/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// QuayIntegrationReconciler reconciles a QuayIntegration object
type QuayIntegrationReconciler struct {
	util.ReconcilerBase
	Log          logr.Logger
	LastSeenSpec map[types.NamespacedName]string
}

//+kubebuilder:rbac:groups=quay.redhat.com,resources=quayintegrations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=quay.redhat.com,resources=quayintegrations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=quay.redhat.com,resources=quayintegrations/finalizers,verbs=update

func (r *QuayIntegrationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("quayintegration", req.NamespacedName)

	instance := &quayv1.QuayIntegration{}
	err := r.GetClient().Get(ctx, req.NamespacedName, instance)

	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	specBytes, _ := json.Marshal(instance.Spec)
	if r.LastSeenSpec[req.NamespacedName] == string(specBytes) {
		logger.Info("No changes to QuayIntegration spec, skipping reconciliation")
		return reconcile.Result{Requeue: false}, nil
	}

	instance, err = instance.SetStatus(&quayv1.QuayIntegrationStatus{})
	if err != nil {
		return reconcile.Result{Requeue: true}, err
	}
	instance, err = instance.SetStatus(&quayv1.QuayIntegrationStatus{})
	if err != nil {
		return reconcile.Result{Requeue: true}, err
	}
	err = r.GetClient().Status().Update(ctx, instance)
	if err != nil {
		logger.Error(err, "Failed to update QuayIntegration status")
		return reconcile.Result{Requeue: true}, err
	}
	logger.Info("Updated QuayIntegration status")

	specBytes, _ = json.Marshal(instance.Spec)
	r.LastSeenSpec[req.NamespacedName] = string(specBytes)

	return reconcile.Result{Requeue: false}, nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *QuayIntegrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&quayv1.QuayIntegration{}).
		Complete(r)
}
