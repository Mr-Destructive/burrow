package plugins

import (
	"reflect"

	"github.com/mr-destructive/burrow/models"
)

type Plugin interface {
	Name() string
	Execute(ssg *models.SSG)
}

var pluginRegistry = make(map[string]reflect.Type)

func RegisterPlugin(name string, pluginType reflect.Type) {
	pluginRegistry[name] = pluginType
}

func GetPluginType(name string) (reflect.Type, bool) {
	pluginType, exists := pluginRegistry[name]
	return pluginType, exists
}
