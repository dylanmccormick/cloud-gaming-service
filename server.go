package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/dylanmccormick/webrtc-streamer/internal/logging"
)

func runServer(c Config, ctx context.Context) {

	logger := logging.FromContext(ctx)
	logger.Info("Starting server")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /path/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "got path\n")
		logger.Info("GET PATH")
	})

	mux.HandleFunc("/connect/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Hello connected")
		logger.Info("CONNECT")

	})

	logger.Info("Listening and serving")
	log.Fatal(http.ListenAndServe("localhost:"+strconv.Itoa(c.Port), mux))
}
