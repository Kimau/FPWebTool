package main

import (
	"log"
	"net/http"
)

func WebServeStaticFolder(addr string, fileRoot string) {
	log.Println("Listening on ", addr)
	log.Fatal(http.ListenAndServe(addr, http.FileServer(http.Dir(fileRoot))))
}
