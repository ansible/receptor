package cmdline

import (
	"fmt"
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
	fmt.Printf("   --help: Show this help\n")
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
	for i := range configTypes {
		ct := configTypes[i]
		actions = append(actions, fmt.Sprintf("--%s", strings.ToLower(ct.Name)))
	}
	fmt.Printf("  DASHCMDS=\"%s\"\n", strings.Join(actions, " "))
	fmt.Printf("  if [[ $cur == -* ]]; then\n")
	fmt.Printf("    COMPREPLY=($(compgen -W \"$DASHCMDS\" -- ${cur}))\n")
	fmt.Printf("  else")
	fmt.Printf("    case ${prevdashed} in\n")
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

func setValue(field *reflect.Value, value string) {
	ftn := field.Type().Name()
	if ftn == "string" {
		field.SetString(value)
	} else if ftn == "int" || ftn == "int16" || ftn == "int32" || ftn == "int64" {
		iv, err := strconv.ParseInt(value, 0, 64)
		if err != nil {
			fmt.Printf("Field %s must be an integer\n", ftn)
			os.Exit(1)
		}
		field.SetInt(iv)
	} else if ftn == "float32" || ftn == "float64" {
		fv, err := strconv.ParseFloat(value, 64)
		if err != nil {
			fmt.Printf("Field %s must be a floating point number\n", ftn)
			os.Exit(1)
		}
		field.SetFloat(fv)
	} else if ftn == "bool" {
		bv, err := betterParseBool(value)
		if err != nil {
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
			fmt.Printf("Required parameter%s missing for %s: %s\n", plural(len(requiredParams), "", "s"),
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
						if !f.CanSet() {
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
						if !f.CanSet() {
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
		fmt.Printf("%s required for: %s\n", plural(len(requiredObjs), "A value is", "Values are"),
			joinMapKeys(requiredObjs, ", "))
		if len(args) == 0 {
			fmt.Printf("Run %s --help for command line instructions.\n", os.Args[0])
		}
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
