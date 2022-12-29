package worker

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/serv/internal/service"
)

type WorkerData struct {
	Body   service.MainRequest
	Db     *sql.DB
	IdTask string
}

type JobAndWork struct {
	Job WorkerData
}

func PostWorker(data WorkerData) {
	var body = data.Body
	var sqlInstance = data.Db
	var idTask = data.IdTask
	var dataToWatch, jsonError = json.MarshalIndent(body, "", "   ")
	var services = service.Services{SqlDb: sqlInstance, Table: "req_and_response"}
	ErrorAddInfoTask := services.AddInfoTask(idTask, dataToWatch)
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
