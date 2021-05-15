package logging

import (
	logf "sigs.k8s.io/controller-runtime"
)

var Log = logf.Log.WithName("quay-bridge-operator")
