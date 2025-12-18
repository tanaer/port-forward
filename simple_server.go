//go:build ignore

package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Hello World")
	})

	addr := "0.0.0.0:88"
	fmt.Printf("HTTP server listening on %s\n", addr)
	http.ListenAndServe(addr, nil)
}
