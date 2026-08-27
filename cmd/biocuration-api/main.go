package main

import (
	"biocuration/internal/application"
	"biocuration/internal/httpapi"
	"biocuration/internal/repository"
	"flag"
	"log"
	"net/http"
	"os"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:19081", "监听地址")
	flag.Parse()
	if p := os.Getenv("PORT"); p != "" {
		*addr = "127.0.0.1:" + p
	}
	store, e := repository.Open("biocuration.db")
	if e != nil {
		log.Fatal(e)
	}
	defer store.Close()
	srv := httpapi.New(application.New(store))
	log.Printf("监听 %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, srv.Handler()))
}
