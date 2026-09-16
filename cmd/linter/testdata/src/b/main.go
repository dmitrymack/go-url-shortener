// Package b simulates package main: log.Fatal and os.Exit are only allowed
// directly in func main (including closures defined inside it); calling
// them from any other function must be reported. panic is reported
// regardless of where it's called from.
package main

import (
	"log"
	"os"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err) // not reported: directly in main
	}

	cleanup := func() {
		os.Exit(1) // not reported: closure defined inside main
	}
	cleanup()

	os.Exit(0) // not reported: directly in main
}

func run() error {
	if somethingWrong() {
		panic("unexpected state") // want "use of panic is forbidden, return an error instead"
	}
	return nil
}

func somethingWrong() bool {
	return false
}

func fatalHelper() {
	log.Fatal("helper failing") // want "log.Fatal must only be called from func main"
}

func exitHelper(code int) {
	os.Exit(code) // want "os.Exit must only be called from func main"
}
