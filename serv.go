package main

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

func main() {
	router := mux.NewRouter()
	var Job = make(chan JobAndWork)
	for i := 1; i < 6; i += 1 {
		go Worker(Job, i)
	}

	router.HandleFunc("/task", createTask)
	router.HandleFunc("/tasks/{task-id}", giveTaskById)
	listenError := http.ListenAndServe(":8000", router)
	if listenError != nil {
		log.Fatal(listenError)
	}

}
