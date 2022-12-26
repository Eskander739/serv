package main

import (
	"encoding/json"
	"fmt"
)

func PostWorker(data Data) {
	var body = data.Body
	var sqlInstance = data.Db
	var idTask = data.IdTask
	var dataToWatch, jsonError = json.MarshalIndent(body, "", "   ")
	var services = Services{SqlDb: sqlInstance, Table: "req_and_response"}
	ErrorAddInfoTask := services.addInfoTask(idTask, dataToWatch)
	if ErrorAddInfoTask != nil {
		panic(ErrorAddInfoTask)
	}
	if jsonError != nil {
		panic(jsonError)
	}
}

func Worker(DataAndJobChan chan JobAndWork, i int) {
	fmt.Println("Запущена горутина: ", i)
	for {
		var Job = (<-DataAndJobChan).Job
		PostWorker(Job)
		fmt.Println("Работает горутина номер ", i)

	}

}
