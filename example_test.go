package tenet_test

import (
	"net/http"

	"github.com/dogmatiq/tenet"
)

func ExampleConnect() {
	c, err := tenet.Connect("wss://example.com/tunnel")
	if err != nil {
		panic(err)
	}
	defer c.Close()

	l, err := c.Listen("tcp", "api.internal:443")
	if err != nil {
		panic(err)
	}

	// Serve the Listener in the same way as any other net.Listener.
	if err := http.Serve(l, http.DefaultServeMux); err != nil {
		panic(err)
	}
}

func ExampleAccept() {
	http.HandleFunc("/tunnel", func(w http.ResponseWriter, r *http.Request) {
		d, err := tenet.Accept(w, r)
		if err != nil {
			return
		}

		conn, err := d.DialContext(r.Context(), "tcp", "api.internal:443")
		if err != nil {
			return
		}
		defer conn.Close()
	})
}
