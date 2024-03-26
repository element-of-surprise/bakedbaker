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

	verMap, err := versions.New()
	if err != nil {
		panic(err)
	}

	serv, err := http.New(verMap)
	if err != nil {
		panic(err)
	}

	serv.ListenAndServe(*addr)
}
