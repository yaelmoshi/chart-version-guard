package main

import (
	"context"
	"os"

	"git.m0sh1.cc/m0sh1/chart-version-guard/internal/cli"
)

func main() {
	os.Exit(cli.Run(context.Background(), os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}
