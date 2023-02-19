package copy

import (
	"fmt"
	"reflect"
	"strconv"
	"time"
)

var (
	DateTimeLayout = time.DateTime
	TimeZone       = "Asia/Shanghai"
)

type Service interface {
	CopyValue(reflect.Value, reflect.Value) bool
}

type DefaultService struct{}

var CopyService Service = DefaultService{}

func (s DefaultService) CopyValue(fromValue reflect.Value, toValue reflect.Value) bool {
	fromValue = indirectValue(fromValue)
	toValue = indirectValue(toValue)

	if !fromValue.IsValid() {
		return false
	}

	if !toValue.IsValid() {
		return false
	}

	fromType := indirectType(fromValue.Type())
	toType := indirectType(toValue.Type())

	if fromType.AssignableTo(toType) {
		toValue.Set(fromValue)

		return true
	}

	if toType.Kind() == reflect.String {
		switch fromType.Kind() {
		case reflect.Bool:
			toValue.Set(reflect.ValueOf(strconv.FormatBool(fromValue.Bool())).Convert(toType))

			return true
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			toValue.Set(reflect.ValueOf(strconv.FormatInt(fromValue.Int(), 10)).Convert(toType))

			return true
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			toValue.Set(reflect.ValueOf(strconv.FormatUint(fromValue.Uint(), 10)).Convert(toType))

			return true
		case reflect.Float32:
			toValue.Set(reflect.ValueOf(strconv.FormatFloat(fromValue.Float(), 'f', -1, 32)).Convert(toType))

			return true
		case reflect.Float64:
			toValue.Set(reflect.ValueOf(strconv.FormatFloat(fromValue.Float(), 'f', -1, 64)).Convert(toType))

			return true
		case reflect.Struct:
			if fromValue.CanInterface() {
				if v, ok := fromValue.Interface().(time.Time); ok {
					toValue.Set(reflect.ValueOf(v.In(getTimeZone()).Format(DateTimeLayout)).Convert(toType))

					return true
				}

				if v, ok := fromValue.Interface().(fmt.Stringer); ok {
					toValue.Set(reflect.ValueOf(v.String()).Convert(toType))

					return true
				}
			}
		}

		return false
	}

	if fromType.Kind() == reflect.String {
		switch toType.Kind() {
		case reflect.Bool:
			if v, err := strconv.ParseBool(fromValue.String()); err == nil {
				toValue.Set(reflect.ValueOf(v).Convert(toType))

				return true
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if v, err := strconv.ParseInt(fromValue.String(), 10, 0); err == nil {
				toValue.Set(reflect.ValueOf(v).Convert(toType))

				return true
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if v, err := strconv.ParseUint(fromValue.String(), 10, 0); err == nil {
				toValue.Set(reflect.ValueOf(v).Convert(toType))

				return true
			}
		case reflect.Float32, reflect.Float64:
			if v, err := strconv.ParseFloat(fromValue.String(), 64); err == nil {
				toValue.Set(reflect.ValueOf(v).Convert(toType))

				return true
			}
		case reflect.Struct:
			v := reflect.New(toType).Elem()

			if v.CanInterface() {
				if _, ok := v.Interface().(time.Time); ok {
					if t, err := time.ParseInLocation(DateTimeLayout, fromValue.String(), getTimeZone()); err == nil {
						toValue.Set(reflect.ValueOf(t).Convert(toType))
					}

					return true
				}
			}
		}

		return false
	}

	if fromValue.CanConvert(toType) {
		toValue.Set(fromValue.Convert(toType))

		return true
	}

	return false
}

func Copy(from any, to any) {
	fromValue := reflect.ValueOf(from)
	toValue := reflect.ValueOf(to)

	if !fromValue.IsValid() {
		return
	}

	if toValue.Type().Kind() != reflect.Pointer {
		return
	}

	fromValue = indirectValue(fromValue)
	toValue = indirectValue(toValue)

	fromType := indirectType(fromValue.Type())
	toType := indirectType(toValue.Type())

	if fromType.Kind() == reflect.Slice && toType.Kind() == reflect.Slice {
		// slice to slice

		for i := 0; i < fromValue.Len(); i++ {
			if !fromValue.Index(i).IsValid() {
				continue
			}

			v := reflect.New(toType.Elem()).Elem()

			if CopyService.CopyValue(fromValue.Index(i), v) {
				toValue.Set(reflect.Append(toValue, v))
			}
		}
	} else if fromType.Kind() == reflect.Struct && toType.Kind() == reflect.Struct {
		// struct to struct

		for i := 0; i < fromType.NumField(); i++ {
			fromField := fromType.Field(i)

			if toField, ok := toType.FieldByName(fromField.Name); ok {
				fromFieldValue := fromValue.FieldByName(fromField.Name)
				toFieldValue := toValue.FieldByName(fromField.Name)

				if !toFieldValue.CanSet() {
					continue
				}

				if toField.Type.Kind() == reflect.Pointer && toFieldValue.IsNil() {
					toFieldValue.Set(reflect.New(indirectType(toField.Type)))
				}

				CopyService.CopyValue(fromFieldValue, toFieldValue)
			}
		}
	} else if fromType.Kind() == reflect.Map && toType.Kind() == reflect.Map {
		// map to map

		if toValue.IsNil() {
			toValue.Set(reflect.MakeMap(toType))
		}

		kv := fromValue.MapRange()

		for kv.Next() {
			k := reflect.New(toType.Key()).Elem()

			if !CopyService.CopyValue(kv.Key(), k) {
				continue
			}

			v := reflect.New(toType.Elem()).Elem()

			if !CopyService.CopyValue(kv.Value(), v) {
				continue
			}

			toValue.SetMapIndex(k, v)
		}
	} else if fromType.Kind() == reflect.Map && toType.Kind() == reflect.Struct {
		// map to struct

		kv := fromValue.MapRange()

		for kv.Next() {
			if toField, ok := toType.FieldByName(kv.Key().String()); ok {
				toFieldValue := toValue.FieldByName(kv.Key().String())

				if !toFieldValue.CanSet() {
					continue
				}

				if toField.Type.Kind() == reflect.Pointer && toFieldValue.IsNil() {
					toFieldValue.Set(reflect.New(indirectType(toField.Type)))
				}

				CopyService.CopyValue(kv.Value(), toFieldValue)
			}
		}
	} else if fromType.Kind() == reflect.Struct && toType.Kind() == reflect.Map {
		// struct to map

		if toValue.IsNil() {
			toValue.Set(reflect.MakeMap(toType))
		}

		for i := 0; i < fromType.NumField(); i++ {
			fromField := fromType.Field(i)

			k := reflect.New(toType.Key()).Elem()

			if !CopyService.CopyValue(reflect.ValueOf(fromField.Name), k) {
				continue
			}

			v := reflect.New(toType.Elem()).Elem()

			if !CopyService.CopyValue(fromValue.FieldByName(fromField.Name), v) {
				continue
			}

			toValue.SetMapIndex(k, v)
		}
	} else {
		// value to value

		CopyService.CopyValue(fromValue, toValue)
	}
}

func getTimeZone() *time.Location {
	if v, err := time.LoadLocation(TimeZone); err == nil {
		return v
	}

	return time.UTC
}

func indirectValue(reflectValue reflect.Value) reflect.Value {
	for reflectValue.Kind() == reflect.Pointer {
		reflectValue = reflectValue.Elem()
	}

	return reflectValue
}

func indirectType(reflectType reflect.Type) reflect.Type {
	for reflectType.Kind() == reflect.Pointer {
		reflectType = reflectType.Elem()
	}

	return reflectType
}
