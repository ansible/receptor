Cmdline
=======

Cmdline is a Go CLI parsing and execution framework based on the idea
that you create structs containing the fields you want the user to
provide, and then the framework handles all the details of accepting
those from the command-line or from YAML configuration files. Methods
of the structs are called in user-defined phases to execute their
functions.

## Motivation

Go already has good command line packages, so why write another? The
main reason I wrote this was that I could not find a package where it
was easy to provide multiple "things" that should all be executed at
once.  My use case was less like `git clone|add|commit|etc...` and
more like `receptor --listen 1.2.3.4:80 --listen 4.5.6.7:8080`.  I
don't want a single command with subcommands, I want many different
configuration sections, each modifying the execution of a main program.

Subsequent work on this has made it more generally useful: you
_could_ implement a CLI with subcommands like git.  And the paradigm
of writing structs, receiver functions and execution phases has
turned out to be fairly useful.  So I don't claim this is _better_
than existing libraries, just that it is _different_, and perhaps
it will be handy for your particular use case.

## Example

A simple example is provided in [cmd/example.go](cmd/example.go). This
program pretends to draw shapes.  (It doesn't actually draw anything.)
On the command line, the user can specify one or more of different shape
types, each with their own properties.  For example, the rectangle is
defined with the following struct:

```
type rectangle struct {
	Width  float64 `description:"Width of the rectangle." required:"yes"`
	Height float64 `description:"Height of the rectangle." required:"yes"`
	Color  string  `description:"Color to use when drawing the rectangle" default:"white"`
}
```

This tells the library that a rectangle must be given a width and height,
and can optionally also be given a color.  The rectangle is registered
to the cmdline system with the following call:

```
cl.AddConfigType("rectangle", "Rectangle Shape", rectangle{})
```

This tells the library that this item should be instantiated when the
user gives a `--rectangle` command, that the help description should
describe this as a `Rectangle Shape`, and that the `rectangle` struct
contains the configuration details.  Once registered, users can give
CLI parameters like `--rectangle width=3 height=4`.

Once all our CLI parameter objects are registered, the main function
of the library is invoked:

```
err := cl.ParseAndRun(os.Args[1:], []string{"Check", "Draw"}, cmdline.ShowHelpIfNoArgs)
```

This parses the command-line arguments, and then runs two execution
phases.  ShowHelpIfNoArgs means that if the user provides no command
line arguments, we should show the help and exit.

An execution phase simply calls receiver methods on the
configuration objects that have the same name as the phase.  These
methods must take no parameters and return an error value.  For
example, here is the rectangle's Check method:

```
func (r rectangle) Check() error {
	if r.Height < 0 || r.Width < 0 {
		return fmt.Errorf("rectangle height and width cannot be negative")
	}
	return nil
}
```

Phases are run in the order given, so the Check method will be run on
all instantiated objects (that have one), and if they all succeed, then
the Draw method will be run.  Any error stops the whole process.

## YAML config files

Users can supply a YAML config file instead of specifying all the arguments
on the command line, by using `--config <filename>` or `-c <filename>`.
The YAML is expected to be formatted as a map of options, with the keys
corresponding to config types and the values corresponding to the field
values of the config types.  For example, a YAML config file for rectangle
would look like this:

```
---
- rectangle:
    height: 3.0
    width: 4.0
```

This will be processed the same as if the user entered
`--rectangle height=3.0 width=4.0` on the command line.

## Options for config types

When registering a config type with `AddConfigType`, the following options
can be specified:

* `Required`: At least one of this item must appear on the command line.
* `Singleton`: Only one of this item can be instantiated.
* `Exclusive`: If this item is on the command line, then it must be the only one.
* `Hidden`: Do not include this item in help or command line completion. 
* `Section`: Group this item within a given section in the help output.

## Options for configuration items

Fields within a config type struct use struct tags to control their function.
The following tags can be applied to a struct field:

* `description`: Description to show to the user in help output.
* `required`: If true, the field must be provided.
* `default`: Default value to use if the user does not specify one.
* `barevalue`: If true, this field can be used as a bare value.    
  (A bare value is like `--item 37` rather than `--item id=37`.)
* `ignore`: If true, do not use this field for command line processing at all.    
  (This can be used for private data to be passed between phases, etc.)

## Data types

The cmdline library attempts to handle a large variety of data types by
using the type conversion capabilities of the reflect package.  In most
cases you can just declare the struct field as the type you want, and
the string from the CLI will be converted to it.

If the destination field is a slice or map, then the user can either
supply a JSON string on the command line itself, or use a YAML configuration
file to define the list or dict.  Complex data types are possible, but
become cumbersome when not using YAML configuration files.

In the special case where a struct field is a `[]string`, the library
will combine multiple single-string values.  For example, if you have:

```
type listen struct {
	IP []string `description:"IP address to listen on"`
}
```

then the user can do `--listen ip=1.2.3.4 ip=2.3.4.5`, and the struct
field will be filled in with a list of the provided values. 
