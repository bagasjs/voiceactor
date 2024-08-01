package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func StartHttpServer(config Config) {
    protocol := "http"
    if config.Secure {
        protocol = "https"
    }

    host := fmt.Sprint(config.Hostname, ":", config.HttpPort)
    informativeHostAddr := fmt.Sprint(protocol, "://", config.RemoteAddr, ":", config.HttpPort)
	log.Println("HTTP Server is started at", host, "(", informativeHostAddr, ")")

	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/capture", http.StatusPermanentRedirect)
	})
	r.HandleFunc("/capture", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "views/capture.html")
	})
	r.PathPrefix(config.StaticUrlPath).Handler(http.StripPrefix(config.StaticUrlPath,
		http.FileServer(http.Dir(config.StaticDirPath))))


    if config.Secure {
        if err := http.ListenAndServeTLS(host, config.CertFile, config.KeyFile, r); err != nil {
            log.Fatal(err)
        }
    } else {
        if err := http.ListenAndServe(host, r); err != nil {
            log.Fatal(err)
        }
    }
}
