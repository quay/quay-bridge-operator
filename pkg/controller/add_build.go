package controller

import (
	"github.com/quay/quay-bridge-operator/pkg/controller/build"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, build.Add)
}
