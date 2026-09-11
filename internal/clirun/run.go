// Package clirun runs a parsed lewkit x/cmd spec and prints usage.
package clirun

import (
	"context"
	"fmt"
	"os"
	"reflect"

	"github.com/lewtec/lewkit/x/cmd"
)

// Run walks v and calls Run(ctx) error on the selected command.
func Run(ctx context.Context, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer {
		rv = reflect.ValueOf(&v).Elem()
	}
	return runSelected(ctx, rv)
}

func runSelected(ctx context.Context, v reflect.Value) error {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	if child := selectedCommand(v); child.IsValid() {
		return runSelected(ctx, child)
	}
	return callRun(ctx, v)
}

func selectedCommand(v reflect.Value) reflect.Value {
	t := v.Type()
	for i := range t.NumField() {
		sf := t.Field(i)
		fv := v.Field(i)
		_, flatten := sf.Tag.Lookup("flatten")
		if (sf.Anonymous || flatten) && fv.Kind() == reflect.Struct {
			if child := selectedCommand(fv); child.IsValid() {
				return child
			}
			continue
		}
		if fv.Kind() != reflect.Pointer || fv.IsNil() || fv.Type().Elem().Kind() != reflect.Struct {
			continue
		}
		if isCommandField(sf, fv) {
			return fv
		}
	}
	return reflect.Value{}
}

func isCommandField(sf reflect.StructField, fv reflect.Value) bool {
	if fv.Kind() != reflect.Pointer || fv.Type().Elem().Kind() != reflect.Struct {
		return false
	}
	elem := reflect.New(fv.Type().Elem())
	if hasParse(elem) || hasCount(elem) {
		return false
	}
	return true
}

func hasParse(ptr reflect.Value) bool {
	return methodSig(ptr, "Parse", reflect.TypeFor[string]())
}

func hasCount(ptr reflect.Value) bool {
	return methodSig(ptr, "Count", reflect.TypeFor[int]())
}

func methodSig(ptr reflect.Value, name string, in reflect.Type) bool {
	m := ptr.MethodByName(name)
	if !m.IsValid() {
		return false
	}
	t := m.Type()
	return t.NumIn() == 1 && t.In(0) == in && t.NumOut() == 1 && t.Out(0) == reflect.TypeFor[error]()
}

func callRun(ctx context.Context, v reflect.Value) error {
	m := v.Addr().MethodByName("Run")
	if !m.IsValid() {
		return nil
	}
	mt := m.Type()
	if mt.NumIn() != 1 || mt.In(0) != reflect.TypeFor[context.Context]() {
		return nil
	}
	if mt.NumOut() != 1 || mt.Out(0) != reflect.TypeFor[error]() {
		return nil
	}
	out := m.Call([]reflect.Value{reflect.ValueOf(ctx)})[0].Interface()
	if err, ok := out.(error); ok {
		return err
	}
	return nil
}

// PrintUsage writes cmd.Usage[T] to stdout.
func PrintUsage[T any](name string) error {
	text, err := cmd.Usage[T](name)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(os.Stdout, text)
	return err
}
