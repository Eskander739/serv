package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"
)

var db = dbInitialization()
var Job = make(chan JobAndWork)

func giveTaskById(w http.ResponseWriter, r *http.Request) {
	Id := mux.Vars(r)["task-id"]
	fmt.Println(Id)
	var data = DelAndGet{Writer: w, Db: db, Id: Id}

	if r.Method == http.MethodGet {
		methodGet(data)

	} else if r.Method == http.MethodDelete {
		methodDelete(data)
	}

	errClose := r.Body.Close()
	if errClose != nil {
		panic(errClose)
	}

}
func createTask(w http.ResponseWriter, r *http.Request) {

	decoder, errReadAll := ioutil.ReadAll(r.Body)
	if errReadAll != nil {
		panic(errReadAll)
	}

	var method = r.Method
	var IdTask = uuid()
	var httpRequestData = PostWorkData{Decoder: decoder, Method: method, Db: db}
	var dataLocal = Data{methodPost(httpRequestData), db, IdTask}
	Job <- JobAndWork{Job: dataLocal}

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
