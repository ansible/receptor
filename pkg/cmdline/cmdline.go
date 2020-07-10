package cmdline

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// Section defines a section of the help output, for grouping commands together
type Section struct {
	Description string
	Order       int
}

type param struct {
	Name        string
	Description string
	Type        reflect.Type
	Required    bool
	Exclusive   bool
	Hidden      bool
	Section     *Section
}

var configTypes []param

// AddConfigType registers a new config type with the system
func AddConfigType(name string, description string, configType interface{}, required bool, exclusive bool, hidden bool, section *Section) {
	configTypes = append(configTypes, param{
		Name:        name,
		Description: description,
		Type:        reflect.TypeOf(configType),
		Required:    required,
		Exclusive:   exclusive,
		Hidden:      hidden,
		Section:     section,
	})
}

func printCmdHelp(ct param) {
	if ct.Hidden {
		return
	}
	fmt.Printf("   --%s: %s", strings.ToLower(ct.Name), ct.Description)
	if ct.Required {
		fmt.Printf(" (required)")
	}
	fmt.Printf("\n")
	for i := 0; i < ct.Type.NumField(); i++ {
		fmt.Printf("      %s=<%s>: %s", strings.ToLower(ct.Type.Field(i).Name),
			ct.Type.Field(i).Type.Name(),
			ct.Type.Field(i).Tag.Get("description"))
		extras := make([]string, 0)
		req, err := betterParseBool(ct.Type.Field(i).Tag.Get("required"))
		if err == nil && req {
			extras = append(extras, "required")
		}
		def := ct.Type.Field(i).Tag.Get("default")
		if def != "" {
			extras = append(extras, fmt.Sprintf("default: %s", def))
		}
		if len(extras) > 0 {
			fmt.Printf(" (%s)", strings.Join(extras, ", "))
		}
		fmt.Printf("\n")
	}
	fmt.Printf("\n")
}

// ShowHelp prints command line help.  It does NOT exit.
func ShowHelp() {

	// Construct list of sections
	sections := make([]*Section, 1)
	sections[0] = &Section{
		Description: "",
		Order:       0,
	}
	for i := range configTypes {
		ct := configTypes[i]
		if ct.Section == nil || ct.Hidden {
			continue
		}
		found := false
		for j := range sections {
			if ct.Section == sections[j] {
				found = true
				break
			}
		}
		if found {
			continue
		}
		sections = append(sections, ct.Section)
	}
	sort.SliceStable(sections, func(i int, j int) bool {
		return sections[i].Order < sections[j].Order
	})

	progname := path.Base(os.Args[0])
	fmt.Printf("Usage: %s [--<action> [<param>=<value> ...] ...]\n\n", progname)
	fmt.Printf("   --help: Show this help\n\n")
	fmt.Printf("   --config <filename>: Load additional config options from a file\n\n")
	if runtime.GOOS != "windows" {
		fmt.Printf("   --bash-completion: Generate a completion script for the bash shell\n")
		fmt.Printf("         Run \". <(%s --bash-completion)\" to activate now\n\n", progname)
	}
	for s := range sections {
		sect := sections[s]
		if sect.Description != "" {
			fmt.Printf("%s\n\n", sect.Description)
		}
		for i := 0; i <= 1; i++ {
			for j := range configTypes {
				ct := configTypes[j]
				if (s == 0 && ct.Section != nil) || (s != 0 && ct.Section != sect) || ct.Hidden {
					continue
				}
				if (i == 0 && ct.Required) || (i == 1 && !ct.Required) {
					printCmdHelp(ct)
				}
			}
		}
	}
}

func bashCompletion() {
	cmdName := filepath.Base(os.Args[0])
	fmt.Printf("_%s()\n", cmdName)
	fmt.Printf("{\n")
	fmt.Printf("  local cur prevdashed count DASHCMDS\n")
	fmt.Printf("  cur=${COMP_WORDS[COMP_CWORD]}\n")
	fmt.Printf("  count=$((COMP_CWORD-1))\n")
	fmt.Printf("  while [[ $count > 1 && ! ${COMP_WORDS[$count]} == --* ]]; do\n")
	fmt.Printf("    count=$((count-1))\n")
	fmt.Printf("  done\n")
	fmt.Printf("  prevdashed=${COMP_WORDS[$count]}\n")
	actions := make([]string, 0)
	actions = append(actions, "--help")
	actions = append(actions, "--bash-completion")
	actions = append(actions, "--config")
	actions = append(actions, "-c")
	for i := range configTypes {
		ct := configTypes[i]
		actions = append(actions, fmt.Sprintf("--%s", strings.ToLower(ct.Name)))
	}
	fmt.Printf("  DASHCMDS=\"%s\"\n", strings.Join(actions, " "))
	fmt.Printf("  if [[ $cur == -* ]]; then\n")
	fmt.Printf("    COMPREPLY=($(compgen -W \"$DASHCMDS\" -- ${cur}))\n")
	fmt.Printf("  else")
	fmt.Printf("    case ${prevdashed} in\n")
	fmt.Printf("      -c|--config)\n")
	fmt.Printf("        COMPREPLY=($(compgen -f -- ${cur}))\n")
	fmt.Printf("        ;;\n")
	for i := range configTypes {
		ct := configTypes[i]
		if ct.Hidden {
			continue
		}
		fmt.Printf("      --%s)\n", strings.ToLower(ct.Name))
		params := make([]string, 0)
		for j := 0; j < ct.Type.NumField(); j++ {
			params = append(params, fmt.Sprintf("%s=", strings.ToLower(ct.Type.Field(j).Name)))
		}
		fmt.Printf("        COMPREPLY=($(compgen -W \"%s\" -- ${cur}))\n", strings.Join(params, " "))
		fmt.Printf("        ;;\n")
	}
	fmt.Printf("      *)\n")
	fmt.Printf("        COMPREPLY=($(compgen -W \"$DASHCMDS\" -- ${cur}))\n")
	fmt.Printf("        ;;\n")
	fmt.Printf("    esac\n")
	fmt.Printf("  fi\n")
	fmt.Printf("  [[ $COMPREPLY == *= ]] && compopt -o nospace\n")
	fmt.Printf("}\n")
	fmt.Printf("complete -F _%s %s\n", cmdName, cmdName)
}

func setValue(field *reflect.Value, value interface{}) error {
	fieldType := field.Type()
	fieldKind := fieldType.Kind()
	valueType := reflect.TypeOf(value)

	// If the value is directly convertible to the field, just set it
	if valueType.ConvertibleTo(fieldType) {
		field.Set(reflect.ValueOf(value))
		return nil
	}

	// Get string version of value
	valueStr, isString := value.(string)

	// If the field is a map, check if we were given a JSON-encoded string
	if fieldKind == reflect.Map && valueType.Kind() == reflect.String && strings.HasPrefix(valueStr, "{") {
		dest := reflect.MakeMap(reflect.MapOf(fieldType.Key(), fieldType.Elem()))
		value = dest.Interface()
		err := json.Unmarshal([]byte(valueStr), &value)
		if err != nil {
			return err
		}
		valueType = reflect.TypeOf(value)
		// We do not return here because we still need the map copy below
	}

	// If the field and value are a map type, attempt to copy the keys/values
	if fieldKind == reflect.Map && valueType.Kind() == fieldKind {
		fieldMap := reflect.MakeMap(reflect.MapOf(fieldType.Key(), fieldType.Elem()))
		iter := reflect.ValueOf(value).MapRange()
		for iter.Next() {
			fieldMap.SetMapIndex(reflect.ValueOf(iter.Key().Interface()), reflect.ValueOf(iter.Value().Interface()))
		}
		field.Set(fieldMap)
		return nil
	}

	// If the field and value are a slice type, attempt to copy the values
	if fieldKind == reflect.Slice && valueType.Kind() == fieldKind {
		valueSlice, ok := value.([]interface{})
		if !ok {
			return fmt.Errorf("invalid value for slice type")
		}
		fieldSlice := reflect.MakeSlice(fieldType, 0, 0)
		for i := range valueSlice {
			reflect.Append(fieldSlice, reflect.ValueOf(valueSlice[i]))
		}
		field.Set(fieldSlice)
		return nil
	}

	// No direct field conversions were possible, so let's try a string conversion
	if isString {
		switch fieldKind {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			v, err := strconv.ParseInt(valueStr, 0, 64)
			if err != nil {
				return err
			}
			field.SetInt(v)
			return nil
		case reflect.Float32, reflect.Float64:
			v, err := strconv.ParseFloat(valueStr, 64)
			if err != nil {
				return err
			}
			field.SetFloat(v)
			return nil
		case reflect.Bool:
			v, err := betterParseBool(valueStr)
			if err != nil {
				return err
			}
			field.SetBool(v)
			return nil
		}
	}
	return fmt.Errorf("type error")
}

func plural(count int, singular string, plural string) string {
	if count > 1 {
		return plural
	}
	return singular
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
	b, err := betterParseBool(tag)
	if err != nil {
		fmt.Printf("Could not parse %s as boolean\n", tag)
		os.Exit(1)
	}
	return b
}

func getCfgObjType(objType string) (*param, error) {
	for i := range configTypes {
		ct := configTypes[i]
		if objType == strings.ToLower(ct.Name) {
			return &ct, nil
		}
	}
	return nil, fmt.Errorf("unknown config type %s", objType)
}

func getBareParam(commandType reflect.Type) (string, error) {
	for i := 0; i < commandType.NumField(); i++ {
		ctf := commandType.Field(i)
		if convTagToBool(ctf.Tag.Get("barevalue"), false) {
			return ctf.Name, nil
		}
	}
	return "", fmt.Errorf("command does not allow bare values")
}

func getFieldByName(obj *reflect.Value, fieldName string) (*reflect.Value, error) {
	commandType := obj.Type()
	for i := 0; i < commandType.NumField(); i++ {
		ctf := commandType.Field(i)
		if strings.ToLower(ctf.Name) == strings.ToLower(fieldName) {
			f := obj.FieldByName(ctf.Name)
			return &f, nil
		}
	}
	return nil, fmt.Errorf("unknown field name %s", fieldName)
}

func buildRequiredParams(commandType reflect.Type) map[string]bool {
	requiredParams := make(map[string]bool)
	for j := 0; j < commandType.NumField(); j++ {
		ctf := commandType.Field(j)
		if convTagToBool(ctf.Tag.Get("required"), false) {
			requiredParams[strings.ToLower(ctf.Name)] = true
		}
	}
	return requiredParams
}

func checkRequiredParams(requiredParams map[string]bool, commandName string) {
	if len(requiredParams) > 0 {
		sl := make([]string, 0, len(requiredParams))
		for p := range requiredParams {
			sl = append(sl, p)
		}
		fmt.Printf("Required parameter%s missing for %s: %s\n", plural(len(requiredParams), "", "s"),
			commandName, strings.Join(sl, ", "))
		os.Exit(1)
	}
}

type cfgObjInfo struct {
	obj       reflect.Value
	arg       string
	fieldsSet []string
}

func newCOI() *cfgObjInfo {
	return &cfgObjInfo{
		obj:       reflect.Value{},
		arg:       "",
		fieldsSet: make([]string, 0),
	}
}

func loadConfigFromFile(filename string) ([]*cfgObjInfo, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	config := make([]interface{}, 0)
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	cfgObjs := make([]*cfgObjInfo, 0)
	for i := range config {
		cfg := config[i]
		str, ok := cfg.(string)
		var command string
		var rawParams interface{}
		if ok {
			command = str
			rawParams = nil
		} else {
			imap, ok := cfg.(map[interface{}]interface{})
			if ok {
				if len(imap) != 1 {
					return nil, fmt.Errorf("config format invalid: item has multiple names")
				}
				var k interface{}
				var v interface{}
				for k, v = range imap {
					break
				}
				str, ok = k.(string)
				if !ok {
					return nil, fmt.Errorf("section key is not a string")
				}
				command = str
				rawParams = v
			} else {
				return nil, fmt.Errorf("unknown config file format")
			}
		}
		ct, err := getCfgObjType(command)
		if err != nil {
			return nil, fmt.Errorf("could not get config type for command %s: %s", command, err)
		}
		params := make(map[string]interface{})
		if rawParams == nil {
			// this space intentionally left blank
		} else {
			str, ok := rawParams.(string)
			if ok {
				// param is a single string, so it is a barevalue
				bareparam, err := getBareParam(ct.Type)
				if err != nil {
					return nil, fmt.Errorf("could not get barevalue for command %s: %s", command, err)
				}
				params[bareparam] = str
			} else {
				// only other choice is for param to be a map
				pmap, ok := rawParams.(map[interface{}]interface{})
				if !ok {
					return nil, fmt.Errorf("invalid config format for %s", command)
				}
				for k, v := range pmap {
					ks, ok := k.(string)
					if !ok {
						return nil, fmt.Errorf("invalid config format for %s", command)
					}
					params[ks] = v
				}
			}
		}
		coi := newCOI()
		coi.obj = reflect.New(ct.Type).Elem()
		coi.arg = command
		for k, v := range params {
			f, err := getFieldByName(&coi.obj, k)
			if err != nil {
				return nil, fmt.Errorf("field %s not defined for command %s: %s", k, command, err)
			}
			if !f.CanSet() {
				return nil, fmt.Errorf("field %s is not settable", k)
			}
			err = setValue(f, v)
			if err != nil {
				return nil, fmt.Errorf("error setting field %s in command %s: %s", k, command, err)
			}
			coi.fieldsSet = append(coi.fieldsSet, k)
		}
		cfgObjs = append(cfgObjs, coi)
	}
	return cfgObjs, nil
}

// ParseAndRun parses the command line configuration and runs the selected actions.
func ParseAndRun(args []string) {
	var accumulator *cfgObjInfo
	var commandType reflect.Type
	var requiredParams map[string]bool
	requiredObjs := make(map[string]bool)
	activeObjs := make([]*cfgObjInfo, 0)
	configCmd := false

	for i := range configTypes {
		ct := configTypes[i]
		if ct.Required {
			requiredObjs[ct.Type.Name()] = true
		}
	}

	for i := range args {
		arg := args[i]
		lcarg := strings.ToLower(arg)
		if lcarg == "-h" || lcarg == "--help" {
			ShowHelp()
			os.Exit(0)
		} else if lcarg == "--bash-completion" {
			bashCompletion()
			os.Exit(0)
		} else if lcarg[0] == '-' {
			// This is a param with dashes, which starts a new action
			for lcarg[0] == '-' {
				lcarg = lcarg[1:]
			}
			// If we were accumulating an action, store it (it is now complete)
			if commandType != nil && accumulator != nil {
				checkRequiredParams(requiredParams, accumulator.arg)
				activeObjs = append(activeObjs, accumulator)
				accumulator = nil
			}
			if lcarg == "config" || lcarg == "c" {
				configCmd = true
			} else {
				// Search for the command in our known config types, and start a new accumulator
				ct, err := getCfgObjType(lcarg)
				if err != nil {
					fmt.Printf("Command error: %s\n", err)
					os.Exit(1)
				}
				commandType = ct.Type
				accumulator = newCOI()
				accumulator.obj = reflect.New(commandType).Elem()
				accumulator.arg = arg
				delete(requiredObjs, ct.Type.Name())
				requiredParams = buildRequiredParams(ct.Type)
			}
		} else {
			// This arg did not start with a dash, so it is a parameter to the current accumulation
			if configCmd {
				configCmd = false
				newObjs, err := loadConfigFromFile(arg)
				if err != nil {
					fmt.Printf("Error loading config file: %s\n", err)
					os.Exit(1)
				}
				for i := range newObjs {
					coi := newObjs[i]
					delete(requiredObjs, coi.obj.Type().Name())
					activeObjs = append(activeObjs, coi)
				}
				continue
			}
			if commandType == nil || accumulator == nil {
				fmt.Printf("Parameter specified before command\n")
				os.Exit(1)
			}
			sarg := strings.SplitN(arg, "=", 2)
			if len(sarg) == 1 {
				// This is a barevalue (not in the form x=y), so look for a barevalue-accepting parameter
				bp, err := getBareParam(commandType)
				if err != nil {
					fmt.Printf("Config error: %s\n", err)
					os.Exit(1)
				}
				f := accumulator.obj.FieldByName(bp)
				if !f.CanSet() {
					fmt.Printf("Internal error: field %s is not settable\n", bp)
					os.Exit(1)
				}
				err = setValue(&f, sarg[0])
				if err != nil {
					fmt.Printf("Error setting config value for field %s: %s\n", bp, err)
					os.Exit(1)
				}
				accumulator.fieldsSet = append(accumulator.fieldsSet, bp)
				delete(requiredParams, strings.ToLower(bp))
			} else if len(sarg) == 2 {
				// This is a key/value pair, so look for a parameter matching the key
				lcname := strings.ToLower(sarg[0])
				f, err := getFieldByName(&accumulator.obj, lcname)
				if err != nil {
					fmt.Printf("Config error: %s\n", err)
					os.Exit(1)
				}
				if !f.CanSet() {
					fmt.Printf("Internal error: field %s is not settable\n", lcname)
					os.Exit(1)
				}
				err = setValue(f, sarg[1])
				if err != nil {
					fmt.Printf("Error setting config value for field %s: %s\n", lcname, err)
					os.Exit(1)
				}
				accumulator.fieldsSet = append(accumulator.fieldsSet, lcname)
				delete(requiredParams, lcname)
			}
		}
	}
	if commandType != nil && accumulator != nil {
		// If we were accumulating an object, store it now since we're done
		checkRequiredParams(requiredParams, accumulator.arg)
		activeObjs = append(activeObjs, accumulator)
	}

	// Enforce exclusive objects
	haveExclusive := false
	exclusiveName := ""
	for i := range activeObjs {
		ao := activeObjs[i]
		found := false
		for j := range configTypes {
			ct := configTypes[j]
			if ao.obj.Type() == ct.Type {
				if ct.Exclusive {
					haveExclusive = true
					exclusiveName = ct.Name
				}
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("Internal error: type not found.\n")
			os.Exit(1)
		}
		if haveExclusive {
			break
		}
	}
	if haveExclusive && len(activeObjs) > 1 {
		fmt.Printf("Cannot specify any other options with %s.\n", exclusiveName)
		os.Exit(1)
	}

	// Error out if we didn't get all required objects
	if len(requiredObjs) > 0 && !haveExclusive {
		sl := make([]string, 0, len(requiredObjs))
		for p := range requiredObjs {
			for i := range configTypes {
				ct := configTypes[i]
				if ct.Type.Name() == p {
					sl = append(sl, ct.Name)
					break
				}
			}
		}
		fmt.Printf("%s required for: %s\n", plural(len(requiredObjs), "A value is", "Values are"),
			strings.Join(sl, ", "))
		if len(args) == 0 {
			fmt.Printf("Run %s --help for command line instructions.\n", os.Args[0])
		}
		os.Exit(1)
	}

	// Set default values where required
	for i := range activeObjs {
		cfgObj := activeObjs[i]
		cfgType := reflect.TypeOf(cfgObj.obj.Interface())
		for j := 0; j < cfgType.NumField(); j++ {
			f := cfgType.Field(j)
			defaultValue := f.Tag.Get("default")
			if defaultValue == "" {
				continue
			}
			lcname := strings.ToLower(f.Name)
			hasBeenSet := false
			for i := range cfgObj.fieldsSet {
				if cfgObj.fieldsSet[i] == lcname {
					hasBeenSet = true
					break
				}
			}
			if !hasBeenSet {
				s := cfgObj.obj.FieldByName(f.Name)
				if s.CanSet() {
					err := setValue(&s, defaultValue)
					if err != nil {
						fmt.Printf("Error setting default value for field %s: %s\n", f.Name, err)
						os.Exit(1)
					}
				}
			}
		}
	}

	// Run a given named method on all the registered objects
	runMethod := func(methodName string) {
		for i := range activeObjs {
			cfgObj := activeObjs[i]
			m := cfgObj.obj.MethodByName(methodName)
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
