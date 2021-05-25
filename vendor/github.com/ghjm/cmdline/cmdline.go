package cmdline

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v2"
	"io"
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

// ConfigSection defines a section of the help output, for grouping commands together
type ConfigSection struct {
	Description string
	Order       int
}

// ConfigType describes a kind of parameter that can be used on the command line
type ConfigType struct {
	name        string
	description string
	objType     reflect.Type
	required    bool
	singleton   bool
	exclusive   bool
	hidden      bool
	section     *ConfigSection
}

// Required means this config type must be provided
func Required(param *ConfigType) {
	param.required = true
}

// Singleton means there can only be one of this config type
func Singleton(param *ConfigType) {
	param.singleton = true
}

// Exclusive means if this config type exists, none other is allowed
func Exclusive(param *ConfigType) {
	param.exclusive = true
}

// Hidden means this config type is not listed in help or bash completion
func Hidden(param *ConfigType) {
	param.hidden = true
}

// Section means this config type should be listed within a section in the help
func Section(section *ConfigSection) func(param *ConfigType) {
	return func(param *ConfigType) {
		param.section = section
	}
}

// Cmdline is a command-line processor object
type Cmdline struct {
	configTypes []*ConfigType
	out         io.Writer
	whatRan     string
}

// NewCmdline constructs a new cmdline object
func NewCmdline() *Cmdline {
	cl := &Cmdline{
		out: os.Stdout,
	}
	return cl
}

// SetOutput configures where the output of a Cmdline instance will go
func (cl *Cmdline) SetOutput(out io.Writer) {
	cl.out = out
}

// WhatRan returns the name of an exclusive command, if any, that ran on the last invocation of ParseAndRun
func (cl *Cmdline) WhatRan() string {
	return cl.whatRan
}

var globalCmdline *Cmdline

// GlobalInstance returns a singleton global instance of a Cmdline
func GlobalInstance() *Cmdline {
	if globalCmdline == nil {
		globalCmdline = NewCmdline()
	}
	return globalCmdline
}

// AddConfigType registers a new config type with the system
func (cl *Cmdline) AddConfigType(name string, description string, configType interface{}, options ...func(*ConfigType)) {
	newCT := &ConfigType{
		name:        name,
		description: description,
		objType:     reflect.TypeOf(configType),
	}
	for _, opt := range options {
		opt(newCT)
	}
	cl.configTypes = append(cl.configTypes, newCT)
}

func printableTypeName(typ reflect.Type) string {
	if typ.String() == "interface {}" {
		return fmt.Sprintf("JSON data")
	} else if typ.String() == "map[string]interface {}" {
		return fmt.Sprintf("JSON dict with string keys")
	} else if typ.Kind() == reflect.Map {
		return fmt.Sprintf("JSON dict of %s to %s", printableTypeName(typ.Key()), printableTypeName(typ.Elem()))
	} else if typ.Kind() == reflect.Slice {
		if typ.Elem() == reflect.TypeOf("") {
			return fmt.Sprintf("%s (may be repeated)", typ.String())
		}
		return fmt.Sprintf("JSON list of %s", printableTypeName(typ.Elem()))
	} else if typ.String() == "interface {}" {
		return "anything"
	}
	return typ.String()
}

// recursiveEnumerateFields is the recursive companion of enumerateFields and should only be called from there.
func recursiveEnumerateFields(typ reflect.Type, results chan<- reflect.StructField) {
	for i := 0; i < typ.NumField(); i++ {
		ctf := typ.Field(i)
		if ctf.Type.Kind() == reflect.Struct {
			recursiveEnumerateFields(ctf.Type, results)
		} else {
			results <- ctf
		}
	}
}

// enumerateFields enumerates primitive fields in a struct, including composed structs.
// If a composed struct has the same name as a struct member, that name will be returned twice.
func enumerateFields(typ reflect.Type) <-chan reflect.StructField {
	results := make(chan reflect.StructField)
	go func() {
		recursiveEnumerateFields(typ, results)
		close(results)
	}()
	return results
}

func (cl *Cmdline) printCmdHelp(p *ConfigType) error {
	if p.hidden {
		return nil
	}
	var err error
	_, err = fmt.Fprintf(cl.out, "   --%s", strings.ToLower(p.name))
	if err != nil {
		return err
	}
	if p.description != "" {
		_, err = fmt.Fprintf(cl.out, ": %s", p.description)
		if err != nil {
			return err
		}
	}
	if p.required {
		_, err = fmt.Fprintf(cl.out, " (required)")
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(cl.out, "\n")
	if err != nil {
		return err
	}
	for ctf := range enumerateFields(p.objType) {
		_, err = fmt.Fprintf(cl.out, "      %s=<%s>", strings.ToLower(ctf.Name),
			printableTypeName(ctf.Type))
		if err != nil {
			return err
		}
		descr := ctf.Tag.Get("description")
		if descr != "" {
			_, err = fmt.Fprintf(cl.out, ": %s", descr)
			if err != nil {
				return err
			}
		}
		extras := make([]string, 0)
		var req bool
		req, err = betterParseBool(ctf.Tag.Get("required"))
		if err == nil && req {
			extras = append(extras, "required")
		}
		def := ctf.Tag.Get("default")
		if def != "" {
			extras = append(extras, fmt.Sprintf("default: %s", def))
		}
		if len(extras) > 0 {
			_, err = fmt.Fprintf(cl.out, " (%s)", strings.Join(extras, ", "))
			if err != nil {
				return err
			}
		}
		_, err = fmt.Fprintf(cl.out, "\n")
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(cl.out, "\n")
	if err != nil {
		return err
	}
	return nil
}

type multiPrintfItem struct {
	format string
	values []interface{}
}

// mPI is a convenience function for constructing multiPrintfItems more laconically
func mPI(format string, values ...interface{}) *multiPrintfItem {
	return &multiPrintfItem{
		format: format,
		values: values,
	}
}

// multiPrintf calls fmt.Fprintf on multiple items, until there is an error
func multiPrintf(out io.Writer, items ...*multiPrintfItem) error {
	for _, item := range items {
		_, err := fmt.Fprintf(out, item.format, item.values...)
		if err != nil {
			return err
		}
	}
	return nil
}

// ShowHelp prints command line help.  It does NOT exit.  If out is nil, writes to stdout.
func (cl *Cmdline) ShowHelp() error {
	// Construct list of sections
	sections := make([]*ConfigSection, 1)
	sections[0] = &ConfigSection{
		Description: "",
		Order:       0,
	}
	for i := range cl.configTypes {
		ct := cl.configTypes[i]
		if ct.section == nil || ct.hidden {
			continue
		}
		found := false
		for j := range sections {
			if ct.section == sections[j] {
				found = true
				break
			}
		}
		if found {
			continue
		}
		sections = append(sections, ct.section)
	}
	sort.SliceStable(sections, func(i int, j int) bool {
		return sections[i].Order < sections[j].Order
	})

	progname := path.Base(os.Args[0])
	var err error
	err = multiPrintf(cl.out,
		mPI("Usage: %s [--<action> [<param>=<value> ...] ...]\n\n", progname),
		mPI("   --help: Show this help\n\n"),
		mPI("   --config <filename>: Load additional config options from a YAML file\n\n"))
	if err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		err = multiPrintf(cl.out,
			mPI("   --bash-completion: Generate a completion script for the bash shell\n"),
			mPI("         Run \". <(%s --bash-completion)\" to activate now\n\n", progname))
		if err != nil {
			return err
		}
	}
	for s := range sections {
		sect := sections[s]
		if sect.Description != "" {
			_, err = fmt.Fprintf(cl.out, "%s\n\n", sect.Description)
			if err != nil {
				return err
			}
		}
		for i := 0; i <= 1; i++ {
			for j := range cl.configTypes {
				ct := cl.configTypes[j]
				if (s == 0 && ct.section != nil) || (s != 0 && ct.section != sect) || ct.hidden {
					continue
				}
				if (i == 0 && ct.required) || (i == 1 && !ct.required) {
					err = cl.printCmdHelp(ct)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// BashCompletion outputs a Bash script for command-line completion of the current cmdline configuration.
func (cl *Cmdline) BashCompletion() error {
	var err error
	cmdName := filepath.Base(os.Args[0])
	err = multiPrintf(cl.out,
		mPI("_%s()\n", cmdName),
		mPI("{\n"),
		mPI("  local cur prevdashed count DASHCMDS\n"),
		mPI("  cur=${COMP_WORDS[COMP_CWORD]}\n"),
		mPI("  count=$((COMP_CWORD-1))\n"),
		mPI("  while [[ $count > 1 && ! ${COMP_WORDS[$count]} == --* ]]; do\n"),
		mPI("    count=$((count-1))\n"),
		mPI("  done\n"),
		mPI("  prevdashed=${COMP_WORDS[$count]}\n"))
	if err != nil {
		return err
	}
	actions := make([]string, 0)
	actions = append(actions, "--help")
	actions = append(actions, "--bash-completion")
	actions = append(actions, "--config")
	actions = append(actions, "-c")
	for i := range cl.configTypes {
		ct := cl.configTypes[i]
		actions = append(actions, fmt.Sprintf("--%s", strings.ToLower(ct.name)))
	}
	err = multiPrintf(cl.out,
		mPI("  DASHCMDS=\"%s\"\n", strings.Join(actions, " ")),
		mPI("  if [[ $cur == -* ]]; then\n"),
		mPI("    COMPREPLY=($(compgen -W \"$DASHCMDS\" -- ${cur}))\n"),
		mPI("  else"),
		mPI("    case ${prevdashed} in\n"),
		mPI("      -c|--config)\n"),
		mPI("        COMPREPLY=($(compgen -f -- ${cur}))\n"),
		mPI("        ;;\n"))
	if err != nil {
		return err
	}
	for i := range cl.configTypes {
		ct := cl.configTypes[i]
		if ct.hidden {
			continue
		}
		_, err = fmt.Fprintf(cl.out, "      --%s)\n", strings.ToLower(ct.name))
		if err != nil {
			return err
		}
		params := make([]string, 0)
		for ctf := range enumerateFields(ct.objType) {
			params = append(params, fmt.Sprintf("%s=", strings.ToLower(ctf.Name)))
		}
		err = multiPrintf(cl.out,
			mPI("        COMPREPLY=($(compgen -W \"%s\" -- ${cur}))\n", strings.Join(params, " ")),
			mPI("        ;;\n"))
		if err != nil {
			return err
		}
	}
	err = multiPrintf(cl.out,
		mPI("      *)\n"),
		mPI("        COMPREPLY=($(compgen -W \"$DASHCMDS\" -- ${cur}))\n"),
		mPI("        ;;\n"),
		mPI("    esac\n"),
		mPI("  fi\n"),
		mPI("  [[ $COMPREPLY == *= ]] && compopt -o nospace\n"),
		mPI("}\n"),
		mPI("complete -F _%s %s\n", cmdName, cmdName))
	if err != nil {
		return err
	}
	return nil
}

func setValue(field *reflect.Value, value interface{}) error {
	fieldType := field.Type()
	fieldKind := fieldType.Kind()
	valueType := reflect.TypeOf(value)
	valueKind := valueType.Kind()

	// If the destination is a string, try some string conversions
	if fieldKind == reflect.String {
		switch valueKind {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			field.SetString(fmt.Sprintf("%d", value))
			return nil
		case reflect.Float32, reflect.Float64:
			field.SetString(fmt.Sprintf("%f", value))
			return nil
		case reflect.Bool:
			field.SetString(fmt.Sprintf("%t", value))
			return nil
		}
	}

	// If the value is directly convertible to the field, just set it
	if valueType.ConvertibleTo(fieldType) {
		field.Set(reflect.ValueOf(value).Convert(fieldType))
		return nil
	}

	// Get string version of value
	valueStr, isString := value.(string)

	// If the field is a map, check if we were given a JSON-encoded string
	if fieldKind == reflect.Map && valueKind == reflect.String && isString && strings.HasPrefix(valueStr, "{") {
		valueType = reflect.MapOf(fieldType.Key(), fieldType.Elem())
		valueKind = valueType.Kind()
		dest := reflect.MakeMap(valueType)
		value = dest.Interface()
		err := json.Unmarshal([]byte(valueStr), &value)
		if err != nil {
			return err
		}
		// We do not return here because we still need the map copy below
	}

	// If the field and value are a map type, attempt to copy the keys/values
	if fieldKind == reflect.Map && valueKind == reflect.Map {
		fieldMap := reflect.MakeMap(reflect.MapOf(fieldType.Key(), fieldType.Elem()))
		iter := reflect.ValueOf(value).MapRange()
		for iter.Next() {
			itemKey := reflect.ValueOf(iter.Key().Interface())
			if !itemKey.Type().ConvertibleTo(fieldType.Key()) {
				return fmt.Errorf("invalid key %s: must be type %s", itemKey, fieldType.Key())
			}
			itemValue := reflect.ValueOf(iter.Value().Interface())
			if !itemValue.Type().ConvertibleTo(fieldType.Elem()) {
				return fmt.Errorf("invalid value %s: must be type %s", itemValue, fieldType.Elem())
			}
			fieldMap.SetMapIndex(itemKey.Convert(fieldType.Key()), itemValue.Convert(fieldType.Elem()))
		}
		field.Set(fieldMap)
		return nil
	}

	// If the field and value are a slice type, attempt to copy the values
	if fieldKind == reflect.Slice && valueKind == reflect.Slice {
		valueSlice, ok := value.([]interface{})
		if !ok {
			return fmt.Errorf("invalid value for slice type")
		}
		fieldSlice := reflect.MakeSlice(fieldType, 0, 0)
		for i := range valueSlice {
			item := reflect.ValueOf(valueSlice[i])
			if !item.Type().ConvertibleTo(fieldType.Elem()) {
				return fmt.Errorf("invalid value %s: must be type %s", item, fieldType.Elem())
			}
			fieldSlice = reflect.Append(fieldSlice, item.Convert(fieldType.Elem()))
		}
		field.Set(fieldSlice)
		return nil
	}

	// If the value is a string and no direct conversions were possible, try some string conversions
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

		// If param is a string and field is a string array, append it
		stringSlice, ok := field.Interface().([]string)
		if ok {
			stringSlice = append(stringSlice, valueStr)
			field.Set(reflect.ValueOf(stringSlice))
			return nil
		}
	}

	return fmt.Errorf("type error (expected %s)", fieldType)
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

func convTagToBool(tag string, def bool) (bool, error) {
	if tag == "" {
		return def, nil
	}
	b, err := betterParseBool(tag)
	if err != nil {
		return def, fmt.Errorf("could not parse %s as boolean: %s", tag, err)
	}
	return b, nil
}

func (cl *Cmdline) getCfgObjType(objType string) (*ConfigType, error) {
	for i := range cl.configTypes {
		ct := cl.configTypes[i]
		if objType == strings.ToLower(ct.name) {
			return ct, nil
		}
	}
	return nil, fmt.Errorf("unknown config type %s", objType)
}

func getBareParam(commandType reflect.Type) (string, error) {
	for ctf := range enumerateFields(commandType) {
		b, err := convTagToBool(ctf.Tag.Get("barevalue"), false)
		if err != nil {
			return "", err
		}
		if b {
			return ctf.Name, nil
		}
	}
	return "", fmt.Errorf("command does not allow bare values")
}

// getFieldByName searches for a field by case-insensitive name and returns it if found
func getFieldByName(obj *reflect.Value, fieldName string) (*reflect.Value, error) {
	for ctf := range enumerateFields(obj.Type()) {
		if strings.ToLower(ctf.Name) == strings.ToLower(fieldName) {
			fobj := obj.FieldByName(ctf.Name)
			return &fobj, nil
		}
	}
	return nil, fmt.Errorf("unknown field name %s", fieldName)
}

func buildRequiredParams(commandType reflect.Type) (map[string]bool, error) {
	requiredParams := make(map[string]bool)
	for ctf := range enumerateFields(commandType) {
		req, err := convTagToBool(ctf.Tag.Get("required"), false)
		if err != nil {
			return nil, err
		}
		if req {
			requiredParams[strings.ToLower(ctf.Name)] = true
		}
	}
	return requiredParams, nil
}

func checkRequiredParams(requiredParams map[string]bool, commandName string) error {
	if len(requiredParams) > 0 {
		sl := make([]string, 0, len(requiredParams))
		for p := range requiredParams {
			sl = append(sl, p)
		}
		return fmt.Errorf("required parameter%s missing for %s: %s",
			plural(len(requiredParams), "", "s"),
			commandName, strings.Join(sl, ", "))
	}
	return nil
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

func (cl *Cmdline) loadConfigFromFile(filename string) ([]*cfgObjInfo, error) {
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
		ct, err := cl.getCfgObjType(command)
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
				bareparam, err := getBareParam(ct.objType)
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
		if ct.singleton {
			for c := range cfgObjs {
				if cfgObjs[c].obj.Type() == ct.objType {
					return nil, fmt.Errorf("only one %s directive is allowed", command)
				}
			}
		}
		coi := newCOI()
		coi.obj = reflect.New(ct.objType).Elem()
		coi.arg = command
		requiredParams, err := buildRequiredParams(ct.objType)
		if err != nil {
			return nil, err
		}
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
			delete(requiredParams, strings.ToLower(k))
		}
		err = checkRequiredParams(requiredParams, command)
		if err != nil {
			return nil, err
		}
		cfgObjs = append(cfgObjs, coi)
	}
	return cfgObjs, nil
}

// parseAndRunOptions is the configuration struct for ParseAndRun
type parseAndRunOptions struct {
	helpIfNoArgs bool
}

// ShowHelpIfNoArgs means that if the user provides no arguments, print the help instead of doing anything
func ShowHelpIfNoArgs(pro *parseAndRunOptions) {
	pro.helpIfNoArgs = true
}

// ParseAndRun parses the command line configuration and runs the selected actions.  Phases is a list of function
// names that will be called on each config objects.  If some objects need to be configured before others, use
// multiple phases.  Each phase is run against all objects before moving to the next phase.  The return value is
// the name of the exclusive object that was run, if any, or an empty string if the normal, non-exclusive command ran.
func (cl *Cmdline) ParseAndRun(args []string, phases []string, options ...func(*parseAndRunOptions)) error {

	pro := parseAndRunOptions{}
	for _, proFunc := range options {
		proFunc(&pro)
	}

	if len(args) == 0 && pro.helpIfNoArgs {
		err := cl.ShowHelp()
		if err != nil {
			return err
		}
		cl.whatRan = "help"
		return nil
	}

	var accumulator *cfgObjInfo
	var commandType reflect.Type
	var requiredParams map[string]bool
	var err error

	requiredObjs := make(map[string]bool)
	activeObjs := make([]*cfgObjInfo, 0)
	configCmd := false

	for i := range cl.configTypes {
		ct := cl.configTypes[i]
		if ct.required {
			requiredObjs[ct.objType.Name()] = true
		}
	}

	for i := range args {
		arg := args[i]
		lcarg := strings.ToLower(arg)
		if lcarg == "-h" || lcarg == "--help" && cl.out != nil {
			err = cl.ShowHelp()
			if err != nil {
				return err
			}
			cl.whatRan = "help"
			return nil
		} else if lcarg == "--bash-completion" && cl.out != nil {
			err = cl.BashCompletion()
			if err != nil {
				return err
			}
			cl.whatRan = "bash-completion"
			return nil
		} else if lcarg[0] == '-' {
			// This is a param with dashes, which starts a new action
			for lcarg[0] == '-' {
				lcarg = lcarg[1:]
			}
			// If we were accumulating an action, store it (it is now complete)
			if commandType != nil && accumulator != nil {
				err = checkRequiredParams(requiredParams, accumulator.arg)
				if err != nil {
					return err
				}
				activeObjs = append(activeObjs, accumulator)
				accumulator = nil
			}
			if lcarg == "config" || lcarg == "c" {
				configCmd = true
			} else {
				// Search for the command in our known config types, and start a new accumulator
				var ct *ConfigType
				ct, err = cl.getCfgObjType(lcarg)
				if err != nil {
					return fmt.Errorf("command error: %s", err)
				}
				commandType = ct.objType
				if ct.singleton {
					for c := range activeObjs {
						if activeObjs[c].obj.Type() == ct.objType {
							return fmt.Errorf("directive \"%s\" is only allowed once", ct.name)
						}
					}
				}
				accumulator = newCOI()
				accumulator.obj = reflect.New(commandType).Elem()
				accumulator.arg = arg
				delete(requiredObjs, ct.objType.Name())
				requiredParams, err = buildRequiredParams(ct.objType)
				if err != nil {
					return err
				}
			}
		} else {
			// This arg did not start with a dash, so it is a parameter to the current accumulation
			if configCmd {
				configCmd = false
				var newObjs []*cfgObjInfo
				newObjs, err = cl.loadConfigFromFile(arg)
				if err != nil {
					return fmt.Errorf("error loading config file: %s", err)
				}
				for j := range newObjs {
					coi := newObjs[j]
					delete(requiredObjs, coi.obj.Type().Name())
					activeObjs = append(activeObjs, coi)
				}
				continue
			}
			if commandType == nil || accumulator == nil {
				return fmt.Errorf("parameter specified before command")
			}
			sarg := strings.SplitN(arg, "=", 2)
			if len(sarg) == 1 {
				// This is a barevalue (not in the form x=y), so look for a barevalue-accepting parameter
				var bp string
				bp, err = getBareParam(commandType)
				if err != nil {
					return fmt.Errorf("config error: %s", err)
				}
				f := accumulator.obj.FieldByName(bp)
				if !f.CanSet() {
					return fmt.Errorf("internal error: field %s is not settable", bp)
				}
				err = setValue(&f, sarg[0])
				if err != nil {
					return fmt.Errorf("error setting config value for field %s: %s", bp, err)
				}
				accumulator.fieldsSet = append(accumulator.fieldsSet, bp)
				delete(requiredParams, strings.ToLower(bp))
			} else if len(sarg) == 2 {
				// This is a key/value pair, so look for a parameter matching the key
				lcname := strings.ToLower(sarg[0])
				var f *reflect.Value
				f, err = getFieldByName(&accumulator.obj, lcname)
				if err != nil {
					return fmt.Errorf("config error: %s", err)
				}
				if !f.CanSet() {
					return fmt.Errorf("internal error: field %s is not settable", lcname)
				}
				err = setValue(f, sarg[1])
				if err != nil {
					return fmt.Errorf("error setting config value for field %s: %s", lcname, err)
				}
				accumulator.fieldsSet = append(accumulator.fieldsSet, lcname)
				delete(requiredParams, lcname)
			}
		}
	}
	if commandType != nil && accumulator != nil {
		// If we were accumulating an object, store it now since we're done
		err = checkRequiredParams(requiredParams, accumulator.arg)
		if err != nil {
			return err
		}
		activeObjs = append(activeObjs, accumulator)
	}

	// Enforce exclusive objects
	haveExclusive := false
	exclusiveName := ""
	for i := range activeObjs {
		ao := activeObjs[i]
		found := false
		for j := range cl.configTypes {
			ct := cl.configTypes[j]
			if ao.obj.Type() == ct.objType {
				if ct.exclusive {
					haveExclusive = true
					exclusiveName = ct.name
				}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("internal error: type %s not found", ao.obj.Type().String())
		}
		if haveExclusive {
			break
		}
	}
	if haveExclusive && len(activeObjs) > 1 {
		return fmt.Errorf("cannot specify any other options with %s", exclusiveName)
	}

	// Add missing required singletons
	if !haveExclusive {
		for i := range cl.configTypes {
			ct := cl.configTypes[i]
			if ct.singleton && ct.required {
				haveThis := false
				for j := range activeObjs {
					ao := activeObjs[j]
					if ao.obj.Type() == ct.objType {
						haveThis = true
						break
					}
				}
				if !haveThis {
					a := newCOI()
					a.obj = reflect.New(ct.objType).Elem()
					a.arg = fmt.Sprintf("implicit %s", ct.name)
					var reqs map[string]bool
					reqs, err = buildRequiredParams(ct.objType)
					if err != nil {
						return err
					}
					err = checkRequiredParams(reqs, a.arg)
					if err != nil {
						return err
					}
					activeObjs = append(activeObjs, a)
					delete(requiredObjs, ct.objType.Name())
				}
			}
		}
	}

	// Error out if we didn't get all required objects
	if len(requiredObjs) > 0 && !haveExclusive {
		sl := make([]string, 0, len(requiredObjs))
		for p := range requiredObjs {
			for i := range cl.configTypes {
				ct := cl.configTypes[i]
				if ct.objType.Name() == p {
					sl = append(sl, ct.name)
					break
				}
			}
		}
		return fmt.Errorf("%s required for: %s",
			plural(len(requiredObjs), "a value is", "values are"),
			strings.Join(sl, ", "))
	}

	// Set default values where required
	for i := range activeObjs {
		cfgObj := activeObjs[i]
		cfgType := reflect.TypeOf(cfgObj.obj.Interface())
		for ctf := range enumerateFields(cfgType) {
			defaultValue := ctf.Tag.Get("default")
			if defaultValue == "" {
				continue
			}
			lcname := strings.ToLower(ctf.Name)
			hasBeenSet := false
			for i := range cfgObj.fieldsSet {
				if strings.ToLower(cfgObj.fieldsSet[i]) == lcname {
					hasBeenSet = true
					break
				}
			}
			if !hasBeenSet {
				s := cfgObj.obj.FieldByName(ctf.Name)
				if s.CanSet() {
					err = setValue(&s, defaultValue)
					if err != nil {
						return fmt.Errorf("error setting default value for field %s: %s", ctf.Name, err)
					}
				}
			}
		}
	}

	// Run a given named method on all the registered objects
	runMethod := func(methodName string) error {
		for i := range activeObjs {
			cfgObj := activeObjs[i]
			m := cfgObj.obj.MethodByName(methodName)
			if m.IsValid() {
				result := m.Call(make([]reflect.Value, 0))
				errIf := result[0].Interface()
				if errIf != nil {
					return fmt.Errorf("%s", errIf)
				}
			}
		}
		return nil
	}

	// Run phases
	for _, phase := range phases {
		err = runMethod(phase)
		if err != nil {
			return err
		}
	}

	cl.whatRan = exclusiveName
	return nil
}
