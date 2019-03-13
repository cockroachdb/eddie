// Copyright 2019 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/cockroachdb/eddie/pkg/gen"
	"github.com/cockroachdb/eddie/pkg/util"
	"github.com/spf13/cobra"
)

func main() {
	exec, err := os.Executable()
	if err == nil {
		exec = filepath.Base(exec)
	} else {
		exec = "eddie"
	}

	var stdlib bool
	e := gen.Eddie{}
	root := &cobra.Command{
		Use:          exec + " [packages]",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			e.Logger = log.New(cmd.OutOrStdout(), "", 0 /* no flags */)
			e.Packages = args
			if stdlib {
				e.Packages = append(e.Packages,
					util.Base+"contract/...",
					"./vendor/"+util.Base+"contract/...",
				)
			}
			return e.Execute()
		},
	}
	root.Flags().StringSliceVar(&e.BuildFlags, "build_flags",
		nil, "Additional build flags to pass to the compiler.")
	root.Flags().StringVarP(&e.Dir, "dir", "d", ".", "The directory to operate in")
	root.Flags().StringVarP(&e.Name, "name", "n", "enforcer", "The name of the enforcer to generate")
	root.Flags().StringVarP(&e.Outfile, "out", "o", "", "Override the output filename (defaults to --name.go)")
	root.Flags().BoolVarP(&stdlib, "include_builtins", "b", false, "Include contracts from eddie source")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
