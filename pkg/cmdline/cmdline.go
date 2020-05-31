package cmdline

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type param struct {
	Name        string
	Description string
	Type        reflect.Type
	Required    bool
}

var configTypes []param

// AddConfigType registers a new config type with the system
func AddConfigType(name string, description string, configType interface{}, required bool) {
	configTypes = append(configTypes, param{
		Name:        name,
		Description: description,
		Type:        reflect.TypeOf(configType),
		Required:    required,
	})
}

// ShowHelp prints command line help.  It does NOT exit.
func ShowHelp() {
	fmt.Printf("Would show help\n")
}

func setValue(field *reflect.Value, value string) {
	ftn := field.Type().Name()
	if ftn == "string" {
		field.SetString(value)
	} else if ftn == "int" || ftn == "int16" || ftn == "int32" || ftn == "int64" {
		iv, err := strconv.ParseInt(value, 0, 64); if err != nil {
			fmt.Printf("Field %s must be an integer\n", ftn)
			os.Exit(1)
		}
		field.SetInt(iv)
	} else if ftn == "float32" || ftn == "float64" {
		fv, err := strconv.ParseFloat(value, 64); if err != nil {
			fmt.Printf("Field %s must be a floating point number\n", ftn)
			os.Exit(1)
		}
		field.SetFloat(fv)
	} else if ftn == "bool" {
		bv, err := betterParseBool(value); if err != nil {
			fmt.Printf("Field %s must be a Boolean (true/false) value\n", ftn)
			os.Exit(1)
		}
		field.SetBool(bv)
	} else {
		fmt.Printf("Unknown type in config object: %s\n", field.Type().Name())
	}
}

func joinMapKeys(m map[string]bool, sep string) string {
	sl := make([]string, 0, len(m))
	for p := range m {
		sl = append(sl, p)
	}
	return strings.Join(sl, sep)
}

func plural(count int) string {
	if count > 1 {
		return "s"
	}
	return ""
}

func betterParseBool(s string) (bool, error) {
	switch s {
	case "1", "t", "T", "Y", "true", "TRUE", "True", "yes", "Yes", "YES":
		return true, nil
	case "0", "f", "F", "N", "false", "FALSE", "False", "no", "No", "NO":
		return false, nil
	}
	return false, fmt.Errorf("could not parse %s as boolean", s)
}

func convTagToBool(tag string, def bool) bool {
	if tag == "" {
		return def
	}
	b, err := betterParseBool(tag); if err != nil {
		fmt.Printf("Could not parse %s as boolean\n", tag)
		os.Exit(1)
	}
	return b
}

// ParseAndRun parses the command line configuration and runs the selected actions.
func ParseAndRun(args []string) {
	var accumulator reflect.Value
	var accumArg string
	var commandType reflect.Type
	var requiredParams map[string]bool
	requiredObjs := make(map[string]bool)
	activeObjs := make([]reflect.Value, 0)

	for i := range configTypes {
		ct := configTypes[i]
		if ct.Required {
			requiredObjs[ct.Name] = true
		}
	}

	checkRequiredParams := func() {
		if len(requiredParams) > 0 {
			fmt.Printf("Required parameter%s missing for %s: %s\n", plural(len(requiredParams)),
				accumArg, strings.ToLower(joinMapKeys(requiredParams, ", ")))
			os.Exit(1)
		}
	}

	for i := range args {
		arg := args[i]
		lcarg := strings.ToLower(arg)
		if lcarg == "-h" || lcarg == "--help" {
			ShowHelp()
			os.Exit(0)
		} else if lcarg[0] == '-' {
			// This is a param with dashes, which starts a new action
			for lcarg[0] == '-' {
				lcarg = lcarg[1:]
			}
			// If we were accumulating an action, store it (it is now complete)
			if commandType != nil {
				checkRequiredParams()
				activeObjs = append(activeObjs, accumulator)
			}
			// Search for the command in our known config types, and start a new accumulator
			found := false
			for i := range configTypes {
				ct := configTypes[i]
				if lcarg == strings.ToLower(ct.Name) {
					commandType = ct.Type
					accumulator = reflect.New(commandType).Elem()
					accumArg = arg
					requiredParams = make(map[string]bool)
					delete(requiredObjs, ct.Name)
					for j := 0; j < ct.Type.NumField(); j++ {
						ctf := ct.Type.Field(j)
						if convTagToBool(ctf.Tag.Get("required"), false) {
							requiredParams[ctf.Name] = true
						}
					}
					found = true
					break
				}
			}
			if !found {
				fmt.Printf("Unknown command: %s\n", arg)
				os.Exit(1)
			}
		} else {
			// This arg did not start with a dash, so it is a parameter to the current accumulation
			if commandType == nil {
				fmt.Printf("Parameter specified before command\n")
				os.Exit(1)
			}
			sarg := strings.SplitN(arg, "=", 2)
			if len(sarg) == 1 {
				// This is a barevalue (not in the form x=y), so look for a barevalue-accepting parameter
				found := false
				for i := 0; i < commandType.NumField(); i++ {
					ctf := commandType.Field(i)
					if convTagToBool(ctf.Tag.Get("barevalue"), false) {
						f := accumulator.FieldByName(ctf.Name)
						if ! f.CanSet() {
							continue
						}
						setValue(&f, sarg[0])
						delete(requiredParams, ctf.Name)
						found = true
						break
					}
				}
				if !found {
					fmt.Printf("Unknown parameter %s\n", sarg[0])
					os.Exit(1)
				}
			} else if len(sarg) == 2 {
				// This is a key/value pair, so look for a parameter matching the key
				lcname := strings.ToLower(sarg[0])
				found := false
				for i := 0; i < commandType.NumField(); i++ {
					ctf := commandType.Field(i)
					if strings.ToLower(ctf.Name) == lcname {
						f := accumulator.FieldByName(ctf.Name)
						if ! f.CanSet() {
							continue
						}
						setValue(&f, sarg[1])
						delete(requiredParams, ctf.Name)
						found = true
						break
					}
				}
				if !found {
					fmt.Printf("Unknown parameter %s\n", sarg[0])
					os.Exit(1)
				}
			}
		}
	}
	if commandType != nil {
		// If we were accumulating an object, store it now since we're done
		checkRequiredParams()
		activeObjs = append(activeObjs, accumulator)
	}

	if len(requiredObjs) > 0 {
		fmt.Printf("Values%s are required for: %s\n", plural(len(requiredObjs)),
			joinMapKeys(requiredObjs, ", "))
		os.Exit(1)
	}

	// Set default values where required
	for i := range activeObjs {
		cfgobj := activeObjs[i]
		cfgType := reflect.TypeOf(cfgobj.Interface())
		for j := 0; j < cfgType.NumField(); j++ {
			f := cfgType.Field(j)
			defaultValue := f.Tag.Get("default")
			if defaultValue == "" {
				continue
			}
			s := cfgobj.FieldByName(f.Name)
			if s.IsZero() && s.CanSet() {
				setValue(&s, defaultValue)
			}
		}
	}

	// Set default values where required
	for i := range activeObjs {
		cfgobj := activeObjs[i]
		cfgType := reflect.TypeOf(cfgobj.Interface())
		for j := 0; j < cfgType.NumField(); j++ {
			f := cfgType.Field(j)
			defaultValue := f.Tag.Get("default")
			if defaultValue == "" {
				continue
			}
			s := cfgobj.FieldByName(f.Name)
			if s.IsZero() && s.CanSet() {
				s.SetString(defaultValue)
			}
		}
	}

	// Run a given named method on all the registered objects
	runMethod := func(methodName string) {
		for i := range activeObjs {
			cfgobj := activeObjs[i]
			m := cfgobj.MethodByName(methodName)
			if m.IsValid() {
				result := m.Call(make([]reflect.Value, 0))
				err := result[0].Interface()
				if err != nil {
					fmt.Printf("Error: %s\n", err)
					os.Exit(1)
				}
			}
		}
	}

	// Run phases

	// Prepare implementations must not refer to anything instantiated by any other object since
	// the other object's resources may not be initialized yet. Prepare should not return until
	// this object is ready to be accessed/used by other objects.
	runMethod("Prepare")

	// Run implementations can assume that everyone else's Prepare has already run.
	runMethod("Run")
}
