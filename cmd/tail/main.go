package main

import (
	"fmt"
	"io"
	"os"

	"github.com/santhosh-tekuri/tail"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: tail <file>")
		os.Exit(1)
	}
	r, err := tail.Open(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer r.Close()
	io.Copy(os.Stdout, r)
}
