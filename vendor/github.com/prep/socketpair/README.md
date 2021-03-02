socketpair
[![TravisCI](https://travis-ci.org/prep/socketpair.svg?branch=master)](https://travis-ci.org/prep/socketpair.svg?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/prep/socketpair)](https://goreportcard.com/report/github.com/prep/socketpair)
[![GoDoc](https://godoc.org/github.com/prep/socketpair?status.svg)](https://godoc.org/github.com/prep/socketpair)
==========
This is a simple package for Go that provides an interface to socketpair(2).

Usage
-----
```go
import "github.com/prep/socketpair"
```

```go
func testSocketPair() error {
    sock1, sock2, err := socketpair.New("unix")
    if err != nil {
        return err
    }

    defer sock1.Close()
    defer sock2.Close()

    if _, err := sock1.Write([]byte("Hello World")); err != nil {
        return err
    }

    data := make([]byte, 11)
    if _, err := sock2.Read(data); err != nil {
        return err
    }

    return nil
}
```

License
-------
This software is distributed under the BSD-style license found in the LICENSE file.
