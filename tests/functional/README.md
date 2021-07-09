# Receptor Functional Test Docs

This document will serve as a reference for how the upstream receptor tests
work. This will cover both high level and low level details of how the tests
are setup and run including the assumptions being made and the best way to
add tests going forward.

## CLI Tests

The cli tests are found in `functional/cli/cli_test.go`. The intention for
these tests is to validate that the cli handles flags correctly. For example
the flags that take bools correctly interpret their input as bools and those
that take maps are able to take a string of json and convert it to a map.
These tests are fairly simple because of this and wont be a large focus of this
document.

## Mesh Tests

The mesh tests are found in `functional/mesh/`. These tests are the most
complicated and provide the most value. They test that the receptor mesh works
as expected, nodes can connect to eachother, route packets, start work, etc. In
many cases this is where tests should be added.

Each test launches its own mesh by using yaml or the in language specification
and calling a helper function to build a mesh based on that mesh definition.
These meshes can be built in 3 ways:

* As part of the test process (LibMesh and LibNode)
* As new processes using the cli (CliMesh and CliNode)
* As containers using docker or podman (ContainerMesh and ContainerNode)

In most cases the CliMesh is appropriate and almost all existing tests use it.
The LibMesh was made to allow for testing receptor as an imported library into
other golang projects and was used to find some bugs, however at this point it
is unclear when/if receptor will be used in this way. The LibNodes also dont
support all receptor features as the mesh building code must manually import
the right functions and call them to setup the receptor node. For these 2
reasons the LibMesh should probably be avoided unless you know what you want to
test is using receptor as a importable library. Finally the ContainerMeshes are
essentially CliMeshes wrapped in a container. The initial intention was to use
these to simulate various network conditions using tc (which is supported in
the mesh definition and building for ContainerMeshes), and I have used them
for that purpose in somewhat manual ways, but no existing automation uses them.

One thing to note is all nodes in the mesh for our test suite are always of the
same type, you cannot mix CLINodes with LibNodes, I decided to keep that
separation for simplicity however in practice there's no reason this cant be
done.

Another thing to consider is when running tests in parallel we make some
assumptions to make this easier to reason about so nodes dont try use
overlapping ports.
LibNodes select their own ports by asking the OS to give them an unused port.
ContainerNodes and CLINodes use a shared port pool and reserve ports to use.
However it is possible from the time it is reserved to the time the process
starts that another process could grab that port, we are unable to prevent
this. The most likely scenario that could cause this is running LibNodes and
CLI/ContainerNodes at the same time. To reduce the chance of this we only
run 1 test binary at once, however the various tests in that binary can run
in parallel.

Mesh logs are saved to a unique directory in `/tmp/receptor-tests/` based on
the name of the test. All the config files required to start the nodes are
also stored in this directory, this makes it easy to navigate to and debug
nodes when a test fails.

## Automation

The receptor tests run in github actions and must pass before merging a PR.
There is a make `ci` target that does linting, building, and runs the receptor
tests as well as the receptorctl tests, this is what github actions runs.
The github actions run in a docker container that is built nightly and uploaded
to the github image registry. The tests also start a minikube k8s cluster for
testing the k8s integration. If the tests fail, we collect all the node logs
for each test mesh and archive it however we do not have a way to generate
a junit test report and archive that.
