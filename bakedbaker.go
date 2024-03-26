package main

import (
	"flag"

	"github.com/element-of-surprise/bakedbaker/internal/http"
	"github.com/element-of-surprise/bakedbaker/internal/versions"
)

var (
	addr = flag.String("addr", "localhost:8080", "address to listen on")
)

func main() {
	flag.Parse()

	// Create a new version map that maps versions to localhost addresses where
	// the agent baker service for that version is running.
	verMap, err := versions.New()
	if err != nil {
		panic(err)
	}

	// Create a new HTTP server that routes requests to the appropriate agent baker
	// service based on the version specified in the request.
	serv, err := http.New(verMap)
	if err != nil {
		panic(err)
	}

	panic(serv.ListenAndServe(*addr))
}
