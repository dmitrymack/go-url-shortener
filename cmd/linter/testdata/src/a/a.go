// Package a is not package main, so os.Exit and log.Fatal calls here must
// not be reported — only the panic rule applies regardless of package.
package a

import (
	"log"
	"os"
)

func DoSomething(ok bool) {
	if !ok {
		panic("something went wrong") // want "use of panic is forbidden, return an error instead"
	}
}

func handleFatalError(err error) {
	if err != nil {
		log.Fatal(err) // not reported: this package is not main
	}
}

func exitEarly(code int) {
	os.Exit(code) // not reported: this package is not main
}
