package core

import (
	"context"
	"fmt"
	"strconv"
	"time"

	quayv1 "github.com/quay/quay-bridge-operator/api/v1"

	"github.com/redhat-cop/operator-utils/pkg/util"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/quay/quay-bridge-operator/pkg/constants"
	"github.com/quay/quay-bridge-operator/pkg/logging"
)

const (
	defaultReason = "Warning"
)

type CoreComponents struct {
	ReconcilerBase util.ReconcilerBase
}

type QuayIntegrationCoreError struct {
	Error         error
	Message       string
	KeyAndValues  []interface{}
	RequeuePeriod time.Duration
	Object        runtime.Object
	SkipRequeue   bool
	Reason        string
}

func NewCoreComponents(reconcilerBase util.ReconcilerBase) CoreComponents {
	return CoreComponents{
		ReconcilerBase: reconcilerBase,
	}
}

func (c *CoreComponents) ManageError(quayIntegrationCoreError *QuayIntegrationCoreError) (reconcile.Result, error) {

	// Setup Defaults

	if len(quayIntegrationCoreError.Reason) == 0 {
		quayIntegrationCoreError.Reason = defaultReason
	}

	if quayIntegrationCoreError.RequeuePeriod == 0 {
		quayIntegrationCoreError.RequeuePeriod = constants.RequeuePeriod
	}

	eventMessage := quayIntegrationCoreError.Message
	eventMessage = fmt.Sprintf("%s - %s", eventMessage, buildKeyAndValueMessage(quayIntegrationCoreError.KeyAndValues))

	logging.Log.Error(quayIntegrationCoreError.Error, quayIntegrationCoreError.Message, quayIntegrationCoreError.KeyAndValues...)
	c.ReconcilerBase.GetRecorder().Event(quayIntegrationCoreError.Object, "Warning", quayIntegrationCoreError.Reason, eventMessage)

	return reconcile.Result{
		RequeueAfter: constants.RequeuePeriod,
		Requeue:      !quayIntegrationCoreError.SkipRequeue,
	}, quayIntegrationCoreError.Error

}

func (c *CoreComponents) GetQuayIntegration(object runtime.Object) (quayv1.QuayIntegration, reconcile.Result, error) {

	// Find the Current Registered QuayIntegration objects
	quayIntegrations := quayv1.QuayIntegrationList{}

	err := c.ReconcilerBase.GetClient().List(context.TODO(), &quayIntegrations, &client.ListOptions{})

	if err != nil {
		return quayv1.QuayIntegration{}, reconcile.Result{}, err
	}

	if len(quayIntegrations.Items) != 1 {

		result, err := c.ManageError(&QuayIntegrationCoreError{
			Object:       object,
			Message:      "No QuayIntegrations defined or more than 1 integration present",
			KeyAndValues: []interface{}{"Expected", "1", "Actual", len(quayIntegrations.Items)},
			Reason:       "ConfigrurationError",
			Error:        fmt.Errorf("No QuayIntegrations defined or more than 1 integration present"),
		})

		return quayv1.QuayIntegration{}, result, err
	}

	return *&quayIntegrations.Items[0], reconcile.Result{}, err
}

func buildKeyAndValueMessage(keyAndValues []interface{}) string {

	output := ""

	for idx, val := range keyAndValues {

		if idx > 0 {
			output = output + ", "
		}

		strVal := ""

		switch v := val.(type) {
		case int:
			strVal = strconv.FormatInt(int64(v), 10)
		case bool:
			strVal = strconv.FormatBool(v)
		case string:
			strVal = v
		}

		output = output + strVal

	}

	return output

}
