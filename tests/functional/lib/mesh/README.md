# Building meshes

This package contains the code to build the various different mesh types we
have, LibMesh, CLIMesh, and ContainerMesh. This document will capture the
different implementation details and assumptions made when building the mesh.


This code is fairly tricky and could use a rework to consolidate the duplicated
code across the different mesh types. There is also significant complexity
added due to how the config file is designed and golangs lack of features to be
able to load a config file with that design. This leads to having to load the
config file as a list of interface objects, which then need to be type asserted
to be able to work with them in a meaningful way. I haven't figured out the
right solution to fix this, however it would be good if there was a way to
simplify this to reduce the code complexity and provide better error messages.

## Node dirs

A node directory is not a receptor concept, but in our tests we organize each
node into its own directory with its config and other useful data such as logs.
This is generated automatically as
`/tmp/receptor-testing/$TESTNAME/mesh-$RANDOMNUM1/$NODEID-$RANDOMNUM2/`.

## Node IDs

The mesh config need not specify a node id for each node in the config, instead
the key for the node will be used as its node id automatically. It is possible
to override this, but I would advise against it as it will be confusing to
manage and may break things in unnexpected ways

## DataDirs

The mesh config need not specify a datadir for each node in the config, this
will be automatically generated as `datadir` in the node dir

## Log-level

The mesh builder will automatically set each nodes log-level to "debug" and
when the node is started, it's stdout and stderr will be redirected to "stdout"
and "stderr" files in the node dir.

## Control service socket

If a control service is not specified in the mesh definition, the mesh builder
will create a control socket for it in `utils.ControlSocketBaseDir`. Note that
this does not go along side the node config, this is because socket names have
a limited length, and the node config may be nested deeply in a directory with
a long name.
