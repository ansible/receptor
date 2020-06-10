package cmdline

import (
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
	Section     *Section
}

var configTypes []param

// AddConfigType registers a new config type with the system
func AddConfigType(name string, description string, configType interface{}, required bool, section *Section) {
	configTypes = append(configTypes, param{
		Name:        name,
		Description: description,
		Type:        reflect.TypeOf(configType),
		Required:    required,
		Section:     section,
	})
}

func printCmdHelp(ct param) {
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
		if ct.Section == nil {
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
		for i := range configTypes {
			ct := configTypes[i]
			if (s == 0 && ct.Section != nil) || (s != 0 && ct.Section != sect) {
				continue
			}
			if ct.Required {
				printCmdHelp(ct)
			}
		}
		for i := range configTypes {
			ct := configTypes[i]
			if (s == 0 && ct.Section != nil) || (s != 0 && ct.Section != sect) {
				continue
			}
			if !ct.Required {
				printCmdHelp(ct)
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
	ftn := field.Type().Name()
	if ftn == "string" {
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("type error: field must be a string")
		}
		field.SetString(str)
		return nil
	}
	if ftn == "int" || ftn == "int16" || ftn == "int32" || ftn == "int64" {
		iv, ok := value.(int64)
		if !ok {
			var i int
			i, ok = value.(int)
			if ok {
				iv = int64(i)
			}
		}
		if !ok {
			var i int32
			i, ok = value.(int32)
			if ok {
				iv = int64(i)
			}
		}
		if !ok {
			var i int16
			i, ok = value.(int16)
			if ok {
				iv = int64(i)
			}
		}
		if !ok {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("type error: field must be an integer or int-parseable string")
			}
			var err error
			iv, err = strconv.ParseInt(str, 0, 64)
			if err != nil {
				return err
			}
		}
		field.SetInt(iv)
		return nil
	}
	if ftn == "float32" || ftn == "float64" {
		fv, ok := value.(float64)
		if !ok {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("type error: field must be a float or float-parseable string")
			}
			var err error
			fv, err = strconv.ParseFloat(str, 64)
			if err != nil {
				return err
			}
		}
		field.SetFloat(fv)
		return nil
	}
	if ftn == "bool" {
		bv, ok := value.(bool)
		if !ok {
			str, ok := value.(string)
			if !ok {
				return fmt.Errorf("type error: field must be a bool or bool-parseable string")
			}
			var err error
			bv, err = betterParseBool(str)
			if err != nil {
				return err
			}
		}
		field.SetBool(bv)
		return nil
	}
	return fmt.Errorf("unknown type in config object: %s", field.Type().Name())
}

func joinMapKeys(m map[string]bool, sep string) string {
	sl := make([]string, 0, len(m))
	for p := range m {
		sl = append(sl, p)
	}
	return strings.Join(sl, sep)
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
		fmt.Printf("Required parameter%s missing for %s: %s\n", plural(len(requiredParams), "", "s"),
			commandName, strings.ToLower(joinMapKeys(requiredParams, ", ")))
		os.Exit(1)
	}
}

func loadConfigFromFile(filename string) ([]reflect.Value, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	config := make([]interface{}, 0)
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	cfgObjs := make([]reflect.Value, 0)
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
		cfgObj := reflect.New(ct.Type).Elem()
		for k, v := range params {
			f, err := getFieldByName(&cfgObj, k)
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
		}
		cfgObjs = append(cfgObjs, cfgObj)
	}
	return cfgObjs, nil
}

// ParseAndRun parses the command line configuration and runs the selected actions.
func ParseAndRun(args []string) {
	var accumulator reflect.Value
	var accumArg string
	var commandType reflect.Type
	var requiredParams map[string]bool
	requiredObjs := make(map[string]bool)
	activeObjs := make([]reflect.Value, 0)
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
			if commandType != nil {
				checkRequiredParams(requiredParams, accumArg)
				activeObjs = append(activeObjs, accumulator)
				accumulator = reflect.Value{}
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
				accumulator = reflect.New(commandType).Elem()
				accumArg = arg
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
					obj := newObjs[i]
					delete(requiredObjs, obj.Type().Name())
					activeObjs = append(activeObjs, obj)
				}
				continue
			}
			if commandType == nil {
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
				f := accumulator.FieldByName(bp)
				if !f.CanSet() {
					fmt.Printf("Internal error: field %s is not settable\n", bp)
					os.Exit(1)
				}
				err = setValue(&f, sarg[0])
				if err != nil {
					fmt.Printf("Error setting config value: %s\n", err)
					os.Exit(1)
				}
				delete(requiredParams, strings.ToLower(bp))
			} else if len(sarg) == 2 {
				// This is a key/value pair, so look for a parameter matching the key
				lcname := strings.ToLower(sarg[0])
				f, err := getFieldByName(&accumulator, lcname)
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
					fmt.Printf("Error setting config value: %s\n", err)
					os.Exit(1)
				}
				delete(requiredParams, lcname)
			}
		}
	}
	if commandType != nil {
		// If we were accumulating an object, store it now since we're done
		checkRequiredParams(requiredObjs, accumArg)
		activeObjs = append(activeObjs, accumulator)
	}

	if len(requiredObjs) > 0 {
		fmt.Printf("%s required for: %s\n", plural(len(requiredObjs), "A value is", "Values are"),
			joinMapKeys(requiredObjs, ", "))
		if len(args) == 0 {
			fmt.Printf("Run %s --help for command line instructions.\n", os.Args[0])
		}
		os.Exit(1)
	}

	// Set default values where required
	for i := range activeObjs {
		cfgObj := activeObjs[i]
		cfgType := reflect.TypeOf(cfgObj.Interface())
		for j := 0; j < cfgType.NumField(); j++ {
			f := cfgType.Field(j)
			defaultValue := f.Tag.Get("default")
			if defaultValue == "" {
				continue
			}
			s := cfgObj.FieldByName(f.Name)
			if s.IsZero() && s.CanSet() {
				err := setValue(&s, defaultValue)
				if err != nil {
					fmt.Printf("Error setting default value: %s\n", err)
					os.Exit(1)
				}
			}
		}
	}

	// Run a given named method on all the registered objects
	runMethod := func(methodName string) {
		for i := range activeObjs {
			cfgObj := activeObjs[i]
			m := cfgObj.MethodByName(methodName)
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
