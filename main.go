package main

import (
	"fmt"

	"github.com/SgtDaJim/gmtr/cli"
)

func main() {
	err := cli.RootCmd.Execute()
	if err != nil {
		fmt.Println(err)
	}
}
