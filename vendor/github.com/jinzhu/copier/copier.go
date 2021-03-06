package copier

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// These flags define options for tag handling
const (
	// Denotes that a destination field must be copied to. If copying fails then a panic will ensue.
	tagMust uint8 = 1 << iota

	// Denotes that the program should not panic when the must flag is on and
	// value is not copied. The program will return an error instead.
	tagNoPanic

	// Ignore a destation field from being copied to.
	tagIgnore

	// Denotes that the value as been copied
	hasCopied
)

// Copy copy things
func Copy(toValue interface{}, fromValue interface{}) (err error) {
	return copy(toValue, fromValue, false, false)
}

// Option sets copy options
type Option struct {
	IgnoreEmpty bool
	DeepCopy    bool
}

// CopyWithOption copy with option
func CopyWithOption(toValue interface{}, fromValue interface{}, option Option) (err error) {
	return copy(toValue, fromValue, option.IgnoreEmpty, option.DeepCopy)
}

func copy(toValue interface{}, fromValue interface{}, ignoreEmpty, deepCopy bool) (err error) {
	var (
		isSlice bool
		amount  = 1
		from    = indirect(reflect.ValueOf(fromValue))
		to      = indirect(reflect.ValueOf(toValue))
		options = Option{
			IgnoreEmpty: ignoreEmpty,
			DeepCopy:    deepCopy,
		}
	)

	if !to.CanAddr() {
		return errors.New("copy to value is unaddressable")
	}

	// Return is from value is invalid
	if !from.IsValid() {
		return
	}

	fromType := indirectType(from.Type())
	toType := indirectType(to.Type())

	if fromType.Kind() == reflect.Interface {
		fromType = reflect.TypeOf(from.Interface())
	}

	if toType.Kind() == reflect.Interface {
		toType = reflect.TypeOf(to.Interface())
	}

	// Just set it if possible to assign
	// And need to do copy anyway if the type is struct
	if fromType.Kind() != reflect.Struct && (!deepCopy || fromType.Kind() != reflect.Map) && from.Type().AssignableTo(to.Type()) {
		to.Set(from)
		return
	}

	if fromType.Kind() == reflect.Map && toType.Kind() == reflect.Map {
		if !fromType.Key().ConvertibleTo(toType.Key()) {
			return errors.New("map's key type doesn't match")
		}
		if to.IsNil() {
			to.Set(reflect.MakeMapWithSize(toType, from.Len()))
		}
		for _, k := range from.MapKeys() {
			toKey := indirect(reflect.New(toType.Key()))
			if !set(toKey, k, deepCopy) {
				continue
			}

			elemType := toType.Elem()
			for elemType.Kind() == reflect.Ptr {
				elemType = elemType.Elem()
			}
			toValue := indirect(reflect.New(elemType))
			if !set(toValue, from.MapIndex(k), deepCopy) {
				err = CopyWithOption(toValue.Addr().Interface(), from.MapIndex(k).Interface(), options)
				if err != nil {
					continue
				}
			}

			for {
				if elemType == toType.Elem() {
					to.SetMapIndex(toKey, toValue)
					break
				}
				elemType = reflect.PtrTo(elemType)
				toValue = toValue.Addr()
			}
		}
	}

	if fromType.Kind() != reflect.Struct || toType.Kind() != reflect.Struct {
		return
	}

	if to.Kind() == reflect.Slice {
		isSlice = true
		if from.Kind() == reflect.Slice {
			amount = from.Len()
		}
	}

	for i := 0; i < amount; i++ {
		var dest, source reflect.Value

		if isSlice {
			// source
			if from.Kind() == reflect.Slice {
				source = indirect(from.Index(i))
			} else {
				source = indirect(from)
			}
			// dest
			dest = indirect(reflect.New(toType).Elem())
		} else {
			source = indirect(from)
			dest = indirect(to)
		}

		destKind := dest.Kind()
		initDest := false
		if destKind == reflect.Interface {
			initDest = true
			dest = indirect(reflect.New(toType))
		}

		// Get tag options
		tagBitFlags := map[string]uint8{}
		if dest.IsValid() {
			tagBitFlags = getBitFlags(toType)
		}

		// check source
		if source.IsValid() {
			fromTypeFields := deepFields(fromType)
			// Copy from field to field or method
			for _, field := range fromTypeFields {
				name := field.Name

				// Get bit flags for field
				fieldFlags, _ := tagBitFlags[name]

				// Check if we should ignore copying
				if (fieldFlags & tagIgnore) != 0 {
					continue
				}

				if fromField := source.FieldByName(name); fromField.IsValid() && !shouldIgnore(fromField, ignoreEmpty) {
					// has field
					if toField := dest.FieldByName(name); toField.IsValid() {
						if toField.CanSet() {
							if !set(toField, fromField, deepCopy) {
								if err := CopyWithOption(toField.Addr().Interface(), fromField.Interface(), options); err != nil {
									return err
								}
							} else {
								if fieldFlags != 0 {
									// Note that a copy was made
									tagBitFlags[name] = fieldFlags | hasCopied
								}
							}
						}
					} else {
						// try to set to method
						var toMethod reflect.Value
						if dest.CanAddr() {
							toMethod = dest.Addr().MethodByName(name)
						} else {
							toMethod = dest.MethodByName(name)
						}

						if toMethod.IsValid() && toMethod.Type().NumIn() == 1 && fromField.Type().AssignableTo(toMethod.Type().In(0)) {
							toMethod.Call([]reflect.Value{fromField})
						}
					}
				}
			}

			// Copy from method to field
			for _, field := range deepFields(toType) {
				name := field.Name

				var fromMethod reflect.Value
				if source.CanAddr() {
					fromMethod = source.Addr().MethodByName(name)
				} else {
					fromMethod = source.MethodByName(name)
				}

				if fromMethod.IsValid() && fromMethod.Type().NumIn() == 0 && fromMethod.Type().NumOut() == 1 && !shouldIgnore(fromMethod, ignoreEmpty) {
					if toField := dest.FieldByName(name); toField.IsValid() && toField.CanSet() {
						values := fromMethod.Call([]reflect.Value{})
						if len(values) >= 1 {
							set(toField, values[0], deepCopy)
						}
					}
				}
			}
		}
		if isSlice {
			if dest.Addr().Type().AssignableTo(to.Type().Elem()) {
				to.Set(reflect.Append(to, dest.Addr()))
			} else if dest.Type().AssignableTo(to.Type().Elem()) {
				to.Set(reflect.Append(to, dest))
			}
		}
		if initDest {
			to.Set(dest)
		}
		err = checkBitFlags(tagBitFlags)
	}
	return
}

func shouldIgnore(v reflect.Value, ignoreEmpty bool) bool {
	if !ignoreEmpty {
		return false
	}

	return v.IsZero()
}

func deepFields(reflectType reflect.Type) []reflect.StructField {
	var fields []reflect.StructField

	if reflectType = indirectType(reflectType); reflectType.Kind() == reflect.Struct {
		for i := 0; i < reflectType.NumField(); i++ {
			v := reflectType.Field(i)
			if v.Anonymous {
				fields = append(fields, deepFields(v.Type)...)
			} else {
				fields = append(fields, v)
			}
		}
	}

	return fields
}

func indirect(reflectValue reflect.Value) reflect.Value {
	for reflectValue.Kind() == reflect.Ptr {
		reflectValue = reflectValue.Elem()
	}
	return reflectValue
}

func indirectType(reflectType reflect.Type) reflect.Type {
	for reflectType.Kind() == reflect.Ptr || reflectType.Kind() == reflect.Slice {
		reflectType = reflectType.Elem()
	}
	return reflectType
}

func set(to, from reflect.Value, deepCopy bool) bool {
	if from.IsValid() {

		if to.Kind() == reflect.Ptr {
			// set `to` to nil if from is nil
			if from.Kind() == reflect.Ptr && from.IsNil() {
				to.Set(reflect.Zero(to.Type()))
				return true
			} else if to.IsNil() {
				to.Set(reflect.New(to.Type().Elem()))
			}
			to = to.Elem()
		}

		if deepCopy {
			toKind := to.Kind()
			if toKind == reflect.Interface && to.IsNil() {
				to.Set(reflect.New(reflect.TypeOf(from.Interface())).Elem())
				toKind = reflect.TypeOf(to.Interface()).Kind()
			}

			if toKind == reflect.Struct || toKind == reflect.Map {
				return false
			}
		}

		if from.Type().ConvertibleTo(to.Type()) {
			to.Set(from.Convert(to.Type()))
		} else if scanner, ok := to.Addr().Interface().(sql.Scanner); ok {
			err := scanner.Scan(from.Interface())
			if err != nil {
				return false
			}
		} else if from.Kind() == reflect.Ptr {
			return set(to, from.Elem(), deepCopy)
		} else {
			return false
		}
	}
	return true
}

// parseTags Parses struct tags and returns uint8 bit flags.
func parseTags(tag string) (flags uint8) {
	for _, t := range strings.Split(tag, ",") {
		switch t {
		case "-":
			flags = tagIgnore
			return
		case "must":
			flags = flags | tagMust
		case "nopanic":
			flags = flags | tagNoPanic
		}
	}
	return
}

// getBitFlags Parses struct tags for bit flags.
func getBitFlags(toType reflect.Type) map[string]uint8 {
	flags := map[string]uint8{}
	toTypeFields := deepFields(toType)

	// Get a list dest of tags
	for _, field := range toTypeFields {
		tags := field.Tag.Get("copier")
		if tags != "" {
			flags[field.Name] = parseTags(tags)
		}
	}
	return flags
}

// checkBitFlags Checks flags for error or panic conditions.
func checkBitFlags(flagsList map[string]uint8) (err error) {
	// Check flag conditions were met
	for name, flags := range flagsList {
		if flags&hasCopied == 0 {
			switch {
			case flags&tagMust != 0 && flags&tagNoPanic != 0:
				err = fmt.Errorf("Field %s has must tag but was not copied", name)
				return
			case flags&(tagMust) != 0:
				panic(fmt.Sprintf("Field %s has must tag but was not copied", name))
			}
		}
	}
	return
}
