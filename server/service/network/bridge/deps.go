package bridge

import "os"

var (
	HookEnsureOVSNetworkReady     func() error
	HookEnsureAllVPCSwitchRuntime func() error
	HookWriteFileIfChanged        func(path string, content []byte, perm os.FileMode) (bool, error)
)
