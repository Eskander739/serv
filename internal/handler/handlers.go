package handler

import (
	"encoding/json"
	"fmt"
	"github.com/Eskander739/serv/internal/methods"
	"github.com/Eskander739/serv/internal/service"
	"github.com/Eskander739/serv/internal/worker"
	text_helper "github.com/Eskander739/serv/pkg/text-helper"
	"github.com/gorilla/mux"

	"io/ioutil"
	"net/http"
)

var db = service.DbInitialization()
var Job = make(chan worker.JobAndWork)

func GiveTaskById(w http.ResponseWriter, r *http.Request) {
	Id := mux.Vars(r)["task-id"]
	fmt.Println(Id)
	var data = methods.MethodData{Writer: w, Db: db, Id: Id}

	if r.Method == http.MethodGet {
		methods.MethodGet(data)

	} else if r.Method == http.MethodDelete {
		methods.MethodDelete(data)
	}

	errClose := r.Body.Close()
	if errClose != nil {
		panic(errClose)
	}

}

func CreateTask(w http.ResponseWriter, r *http.Request) {

	decoder, errReadAll := ioutil.ReadAll(r.Body)
	if errReadAll != nil {
		panic(errReadAll)
	}

	var method = r.Method
	var IdTask = text_helper.Uuid()
	var httpRequestData = methods.PostWorkData{Decoder: decoder, Method: method, Db: db}
	var dataLocal = worker.WorkerData{Body: methods.MethodPost(methods.PostWorkData(httpRequestData)), Db: db, IdTask: IdTask}
	Job <- worker.JobAndWork{Job: dataLocal}

	var TaskData = map[string]string{"Id": IdTask, "Status": "Task started"}
	var idTaskData, jsonError = json.MarshalIndent(TaskData, "", "   ")
	_, writeError := w.Write(idTaskData)
	if writeError != nil {
		panic(writeError)
	}
	if jsonError != nil {
		panic(jsonError)
	}

	errClose := r.Body.Close()
	if errClose != nil {
		panic(errClose)
	}
}
