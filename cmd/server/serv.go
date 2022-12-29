package main

import (
	"github.com/serv/internal/handler"
	"github.com/serv/internal/worker"

	"github.com/gorilla/mux"
	"log"
	"net/http"
)

func StartServer() {
	router := mux.NewRouter()
	var Job = make(chan worker.JobAndWork)
	for i := 1; i < 6; i += 1 {
		go worker.Worker(Job, i)
	}

	router.HandleFunc("/task", handler.CreateTask)
	router.HandleFunc("/tasks/{task-id}", handler.GiveTaskById)
	listenError := http.ListenAndServe(":8000", router)
	if listenError != nil {
		log.Fatal(listenError)
	}

}
