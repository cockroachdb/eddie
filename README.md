# Big Eddie, the Golang contract enforcer

> It would be nice if we could write linters that applied only to specific
> elements within a program.

Current linter approaches apply rules across an entire program.  These
produce a wealth of useful and actionable data about how to improve the
quality of a program. However, it's often the case where developers may
want to apply some custom, configurable logic that isn't appropriate for
a general-purpose linter.

Eddie solves the following two problems:
* Make it easy to define new contracts
* Bind those contracts to specific elements within a program

## Building an enforcer

Eddie aggregates any number of types that implement the
`contract.Contract` interface and compiles them into a stand-alone
enforcer binary. For an example, see the
[concrete-return contract](https://godoc.org/github.com/cockroachdb/eddie/pkg/contract/retcon).

Here's a trivial example:

```go
package noop

import "github.com/cockroachdb/eddie/pkg/contract"

type NoOpContract struct {}
// A specific, top-level type assertion is required. 
var _ contract.Contract = &NoOpContract{}
func (n *NoOpContract) Enforce(contract.Context) error { return nil }
```

The enforcer is created by running the `eddie` command:

```
$ go run github.com/cockroachdb/eddie/cmd/eddie \
  -n myenforcer \
  path/to/noop
  
wrote output to /Users/bob/eddie/myenforcer

$ ./myenforcer contracts
contract:NoOpContract
	https://godoc.org/github.com/cockroachdb/eddie/noop/#NoOpContract
	
$ ./myenforcer enforce --help
Enforce contracts defined in the given packages

Usage:
  myenforcer enforce [packages] [flags]

Flags:
      --asserted_only     only consider explicit type assertions
  -d, --dir string        override the current working directory (default ".")
  -h, --help              help for enforce
      --set_exit_status   return a non-zero exit code if errors are reported
  -t, --tests             include test sources in the analysis
  -v, --verbose           enable additional diagnostic messages
```

The generated `myenforcer` binary is a regular go binary and can be
deployed in the usual manner.

Contracts that are part of the Eddie project can be included by using
the `--include_builtins` flag.

## Applying contracts

Refer to the [contract package](https://godoc.org/github.com/cockroachdb/eddie/pkg/contract)
for how to bind contracts to source elements.
