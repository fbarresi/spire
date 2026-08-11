package windows_service

import "github.com/spiffe/spire/pkg/common/catalog"

const (
	pluginName = "windows_service"
)

func BuiltIn() catalog.BuiltIn {
	return builtin(New())
}
