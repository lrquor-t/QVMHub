package service

import (
	"context"

	vmpkg "qvmhub/service/vm"
)

// MakeVMIndependentParams is an alias for vm.MakeVMIndependentParams.
type MakeVMIndependentParams = vmpkg.MakeVMIndependentParams

// MakeVMIndependent delegates to vm.MakeVMIndependent.
func MakeVMIndependent(ctx context.Context, params *MakeVMIndependentParams, progressFn func(int, string)) error {
	return vmpkg.MakeVMIndependent(ctx, params, progressFn)
}

// ParseMakeVMIndependentParams delegates to vm.ParseMakeVMIndependentParams.
func ParseMakeVMIndependentParams(jsonStr string) (*MakeVMIndependentParams, error) {
	return vmpkg.ParseMakeVMIndependentParams(jsonStr)
}
