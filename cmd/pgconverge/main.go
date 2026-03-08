// pgconverge is a CLI tool for PostgreSQL multi-master replication.
package main

import (
	"fmt"
	"os"

	"github.com/sobowalebukola/pgconverge/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
