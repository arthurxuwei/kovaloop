package main

import (
	"os"

	"github.com/arthurxuwei/kovaloop/internal/kovaloopcli"
)

func main() {
	os.Exit(kovaloopcli.Run(os.Args[1:], os.Stdout, os.Stderr, kovaloopcli.ProcessEnv()))
}
