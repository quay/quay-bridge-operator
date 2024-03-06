package utils

import (
	"fmt"
	"reflect"

	"github.com/quay/quay-bridge-operator/pkg/constants"
	"github.com/quay/quay-bridge-operator/pkg/logging"
	corev1 "k8s.io/api/core/v1"
)

func IsZeroOfUnderlyingType(x interface{}) bool {
	return reflect.DeepEqual(x, reflect.Zero(reflect.TypeOf(x)).Interface())
}

func RemoveItemsFromSlice(s []string, r []string) []string {

	for i, v := range s {
		for _, u := range r {
			if v == u {
				return append(s[:i], s[i+1:]...)
			}
		}
	}

	return s
}

func HasNamespaceFinalizer(namespace *corev1.Namespace, finalizer string) bool {

	for _, fin := range namespace.Spec.Finalizers {
		if fin == corev1.FinalizerName(finalizer) {
			return true
		}
	}

	return false
}

func AddNamespaceFinalizer(namespace *corev1.Namespace, finalizer string) {

	if !HasNamespaceFinalizer(namespace, finalizer) {
		namespace.Spec.Finalizers = append(namespace.Spec.Finalizers, corev1.FinalizerName(finalizer))
	}

	logging.Log.Info("Printing out add namespace finalizer", "Namespace", namespace.Spec.Finalizers)
}

func RemoveNamespaceFinalizer(namespace *corev1.Namespace, finalizer string) {

	for i, fin := range namespace.Spec.Finalizers {
		if fin == corev1.FinalizerName(finalizer) {
			finalizers := namespace.Spec.Finalizers
			finalizers[i] = finalizers[len(finalizers)-1]
			namespace.Spec.Finalizers = finalizers[:len(finalizers)-1]
			return
		}
	}
}

func FormatOrganizationRobotAccountName(organizationName string, robotAccountShortname string) string {
	return fmt.Sprintf("%s+%s", organizationName, robotAccountShortname)
}

func GenerateDockerJsonSecretNameForServiceAccount(serviceAccount string, quayName string) string {
	return fmt.Sprintf("%s-quay-%s", serviceAccount, quayName)
}

func LocalObjectReferenceNameExists(localObjectReferenceNames []corev1.LocalObjectReference, name string) bool {

	for _, l := range localObjectReferenceNames {
		if l.Name == name {
			return true
		}
	}

	return false
}

func ObjectReferenceNameExists(objectReferenceNames []corev1.ObjectReference, name string) bool {

	for _, o := range objectReferenceNames {
		if o.Name == name {
			return true
		}
	}

	return false
}

func IsOpenShiftAnnotatedNamespace(namespace *corev1.Namespace) bool {

	_, displayNameFound := namespace.Annotations[constants.OpenShiftDisplayNameAnnotation]
	_, descriptionFound := namespace.Annotations[constants.OpenShiftDescriptionAnnotation]

	return displayNameFound && descriptionFound
}
