package main

import (
	"flag"
	"fmt"
	"os"

	_ "github.com/golang/glog"
	"github.com/jeefy/kaas/cmd/kaasctl/get"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var root = &cobra.Command{
	Use:  "kaasctl",
	Long: "Command line tool for KaaS",
}

func init() {
	err := flag.Set("logtostderr", "true")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't set default error stream: %v\n", err)
		os.Exit(1)
	}

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	// Register the subcommands:
	root.AddCommand(get.Cmd)
}

func main() {
	err := flag.CommandLine.Parse([]string{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't parse empty command line to satisfy 'glog': %v\n", err)
		os.Exit(1)
	}

	// Execute the root command:
	root.SetArgs(os.Args[1:])
	if err = root.Execute(); err != nil {
		os.Exit(1)
	}
}
