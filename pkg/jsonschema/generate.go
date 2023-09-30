//go:generate go run generator.go

package jsonschema

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/invopop/jsonschema"
	"github.com/jesseduffield/lazycore/pkg/utils"
	"github.com/jesseduffield/lazygit/pkg/config"
)

func GetSchemaDir() string {
	return utils.GetLazyRootDirectory() + "/schema"
}

func GenerateSchema() {
	schema := customReflect(&config.UserConfig{})
	obj, _ := json.MarshalIndent(schema, "", "  ")

	if err := os.WriteFile(GetSchemaDir()+"/schema.json", obj, 0o644); err != nil {
		fmt.Println("Error writing to file:", err)
		return
	}
}

func customReflect(v *config.UserConfig) *jsonschema.Schema {
	defaultConfig := config.GetDefaultConfig()
	r := &jsonschema.Reflector{KeyNamer: func(name string) string { return name }, FieldNameTag: "yaml"}
	if err := r.AddGoComments("github.com/jesseduffield/lazygit", "./"); err != nil {
		panic(err)
	}
	schema := r.Reflect(v)

	setDefaultVals(defaultConfig, schema)

	return schema
}

// setDefaultVals sets the default values of the schema based on the values of the defaults struct
func setDefaultVals(defaults interface{}, schema *jsonschema.Schema) {
	v := reflect.ValueOf(defaults)

	if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	t := v.Type()
	parentKey := t.Name()
	for i := 0; i < t.NumField(); i++ {
		value := v.Field(i)
		if isStruct(value.Interface()) {
			setDefaultVals(value.Interface(), schema)
			continue
		}
		key := t.Field(i).Name

		isNil := true
		switch value.Kind() {
		case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.Interface, reflect.Slice:
			isNil = value.IsNil()
		default:
		}

		if isNil {
			continue
		}

		definition := schema.Definitions[parentKey]
		if definition == nil {
			continue
		}

		propertyName, ok := definition.OriginalPropertiesMapping[key]
		if !ok {
			continue
		}

		property, ok := definition.Properties.Get(propertyName)
		if !ok {
			continue
		}

		if property.Default != nil {
			continue
		}

		property.Default = value.Interface()
	}
}

func isStruct(v interface{}) bool {
	return reflect.TypeOf(v).Kind() == reflect.Struct
}
