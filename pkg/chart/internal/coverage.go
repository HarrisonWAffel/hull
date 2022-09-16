package internal

import (
	"reflect"
	"strings"

	"github.com/iancoleman/strcase"
)

func CalculateCoverage(values map[string]interface{}, valuesStructType reflect.Type) float64 {
	setKeys := getSetKeysFromMapInterface(values)
	allKeys := getAllKeysFromStructType(valuesStructType)

	numSetKeys := 0
	for k := range allKeys {
		if setKeys[k] {
			numSetKeys += 1
		}
	}

	return float64(numSetKeys) / float64(len(allKeys))
}

func getSetKeysFromMapInterface(values map[string]interface{}) map[string]bool {
	setKeys := make(map[string]bool)
	if values == nil {
		return setKeys
	}
	var collectSetKeys func(string, interface{})
	collectSetKeys = func(prefix string, valuesInterface interface{}) {
		switch val := valuesInterface.(type) {
		case map[string]interface{}:
			for k, v := range val {
				// if key represents struct key
				collectSetKeys(prefix+"."+k, v)
				if len(prefix) > 0 {
					// if key represents map key; cannot be at root
					collectSetKeys(prefix+"[]", v)
				}
			}
		case []interface{}:
			for _, v := range val {
				collectSetKeys(prefix+"[]", v)
			}
		default:
			for strings.HasSuffix(prefix, "[]") {
				prefix = strings.TrimSuffix(prefix, "[]")
			}
			if valuesInterface != nil && len(prefix) != 0 {
				setKeys[prefix] = true
			}
		}
	}
	collectSetKeys("", values)
	return setKeys
}

func getAllKeysFromStructType(valuesStructType reflect.Type) map[string]bool {
	allKeys := make(map[string]bool)
	var collectAllKeys func(string, reflect.Type)
	collectAllKeys = func(prefix string, valuesType reflect.Type) {
		if valuesType.Kind() == reflect.Ptr {
			valuesType = valuesType.Elem()
		}
		switch valuesType.Kind() {
		case reflect.Struct:
			for i := 0; i < valuesType.NumField(); i++ {
				field := valuesType.Field(i)
				if string(field.Name[0]) == strings.ToLower(string(field.Name[0])) {
					// ignore unexported fields
					continue
				}
				fieldType := field.Type
				jsonFieldName, ok := field.Tag.Lookup("json")
				if !ok {
					jsonFieldName = strcase.ToLowerCamel(field.Name)
				}
				collectAllKeys(prefix+"."+jsonFieldName, fieldType)
			}
		case reflect.Slice, reflect.Map:
			collectAllKeys(prefix+"[]", valuesType.Elem())
		default:
			for strings.HasSuffix(prefix, "[]") {
				prefix = strings.TrimSuffix(prefix, "[]")
			}
			if len(prefix) > 0 {
				allKeys[prefix] = true
			}
		}
	}
	collectAllKeys("", valuesStructType)
	return allKeys
}
