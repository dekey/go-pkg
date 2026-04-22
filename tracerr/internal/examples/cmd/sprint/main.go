package main

import (
	"log"

	"github.com/dekey/go-pkg/tracerr"
	"github.com/dekey/go-pkg/tracerr/internal/examples/dir3"
)

func main() {
	errDS := dir3.DoSomething3()

	log.Print(
		tracerr.Sprint(errDS),
	)
}
