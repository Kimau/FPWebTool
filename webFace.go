package main

import (
	"log"
	"net/http"
)

type WebFace struct {
	Addr   string
	Router *http.ServeMux

	OutMsg             chan string
	InMsg              chan string
	GlobalTemplateData map[string]string
}

func MakeWebFace(addr string, fileroot string) *WebFace {
	w := &WebFace{
		Addr:   addr,
		Router: http.NewServeMux(),

		OutMsg:             make(chan string),
		InMsg:              make(chan string),
		GlobalTemplateData: make(map[string]string),
	}

	w.Router.Handle("/", http.FileServer(http.Dir(fileroot)))

	go w.HostLoop()

	return w
}

func (wf *WebFace) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Write([]byte("This is a Test"))
}

func (wf *WebFace) HostLoop() {
	defer log.Println("Stopped Listening")

	log.Println("Listening on " + wf.Addr)
	err := http.ListenAndServe(wf.Addr, wf.Router)
	if err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

//===
