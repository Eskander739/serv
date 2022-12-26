package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

const (
	username = "root"
	password = "root"
	hostname = "localhost:3306"
	dbname   = "requests"
)

func dsn() string {
	return fmt.Sprintf("%s:%s@tcp(%s)/%s", username, password, hostname, dbname)

}

func dbConnection() (*sql.DB, error) {
	var dbFirstName = fmt.Sprintf("%s:%s@tcp(%s)/", username, password, hostname)
	db, err := sql.Open("mysql", dbFirstName)
	if err != nil {
		log.Printf("Error %s when opening DB\n", err)
		return nil, err
	}

	ctx, cancelfunc := context.WithTimeout(context.Background(), time.Second*5)
	defer cancelfunc()
	res, err := db.ExecContext(ctx, `CREATE DATABASE IF NOT EXISTS requests`)
	if err != nil {
		log.Printf("Error %s when creating DB\n", err)
		return nil, err
	}
	no, err := res.RowsAffected()
	if err != nil {
		log.Printf("Error %s when fetching rows", err)
		return nil, err
	}
	log.Printf("rows affected %d\n", no)

	db.Close()
	db, err = sql.Open("mysql", dsn())
	if err != nil {
		log.Printf("Error %s when opening DB", err)
		return nil, err
	}
	//defer db.Close()

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(20)
	db.SetConnMaxLifetime(time.Minute * 5)

	ctx, cancelfunc = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelfunc()
	err = db.PingContext(ctx)
	if err != nil {
		log.Printf("Errors %s pinging DB", err)
		return nil, err
	}
	log.Printf("Connected to DB %s successfully\n", dbname)
	return db, nil
}

func dbInitialization() *sql.DB {
	/*
		Метод возвращает инстанс и создает таблицу если её не существует
	*/

	db, dateBaseError := dbConnection()
	if dateBaseError != nil {
		panic(dateBaseError)
	}
	var services = Services{SqlDb: db}
	ErrorReqAndResponse := services.ReqAndResponse()
	if ErrorReqAndResponse != nil {
		panic(ErrorReqAndResponse)
	}

	ErrorTasks := services.Tasks()
	if ErrorTasks != nil {
		panic(ErrorTasks)
	}

	return db
}

func (c Services) CacheLRU() (MainReq, error) {
	/*
		Проверяет, существует ли подобный request в БД, если да, возвращает request+response
	*/

	var headers = fmt.Sprintf("%s", c.Req.Headers)
	var body = fmt.Sprintf("%s", c.Req.Body)
	var requests, ErrorfetchRequests = c.fetchRequests()
	if ErrorfetchRequests != nil {
		return MainReq{}, ErrorfetchRequests
	}
	for _, requestIter := range requests {
		var methodLocal = (requestIter["Method"]).(string)
		var urlLocal = (requestIter["Url"]).(string)

		var headerstLocal = map[string]string{}
		var _ = json.Unmarshal(requestIter["HeadersReq"].([]byte), &headerstLocal)

		var bodyLocal = map[string]string{}
		var unmarshalError = json.Unmarshal(requestIter["Body"].([]byte), &bodyLocal)

		if unmarshalError != nil {
			return MainReq{}, unmarshalError
		}

		var headerstLocalString = fmt.Sprintf("%s", headerstLocal)
		var bodyLocalString = fmt.Sprintf("%s", bodyLocal)
		if methodLocal == c.Req.Method {
			if urlLocal == c.Req.Url {
				if GetMD5Hash(headerstLocalString) == GetMD5Hash(headers) {
					if GetMD5Hash(bodyLocalString) == GetMD5Hash(body) {
						return jsonResp(requestIter), nil
					} else if len(body) == 0 && bodyLocal == nil {
						return jsonResp(requestIter), nil
					}

				} else if len(headers) == 0 && headerstLocal == nil {
					if GetMD5Hash(bodyLocalString) == GetMD5Hash(body) {
						return jsonResp(requestIter), nil
					} else if len(body) == 0 && bodyLocal == nil {
						return jsonResp(requestIter), nil
					}

				}

			}

		}

	}

	return MainReq{}, nil
}

func (c Services) HttpRequest() (*http.Response, error) {
	/*
		Отправляет запрос по указанному url
	*/

	var ErrorHttpRequest error
	if c.Req.Method == "GET" && len(c.Req.Body) == 0 {
		req, reqError := http.NewRequest(c.Req.Method, c.Req.Url, nil)

		if reqError != nil {
			ErrorHttpRequest = reqError
		}

		if c.Req.Headers != nil {
			for key, value := range c.Req.Headers {
				req.Header.Add(key, value)
			}
		}
		resp, doError := c.Client.Do(req)

		if doError != nil {
			ErrorHttpRequest = doError
		}

		return resp, ErrorHttpRequest

	} else if c.Req.Method == "POST" {
		out, jsonError := json.Marshal(c.Req.Body)

		if jsonError != nil {
			ErrorHttpRequest = jsonError
		}

		req, reqError := http.NewRequest(c.Req.Method, c.Req.Url, bytes.NewBuffer(out))

		if reqError != nil {
			ErrorHttpRequest = reqError
		}

		if c.Req.Headers != nil {
			for key, value := range c.Req.Headers {
				req.Header.Add(key, value)
			}
		}
		resp, doError2 := c.Client.Do(req)

		if doError2 != nil {
			ErrorHttpRequest = doError2
		}

		return resp, ErrorHttpRequest

	} else if c.Req.Method != "" && c.Req.Url != "" {
		resp := &http.Response{Status: "400 Bad Request", StatusCode: 400}
		return resp, nil
	} else {
		resp := &http.Response{Status: "400 Bad Request", StatusCode: 400}
		return resp, nil
	}
}

func (c Services) addInfo(data MainReq) error {
	/*
		Добавляет запрос в БД
	*/

	var ErrorAddInfo error
	var headersResp, reqErr = json.Marshal(data.Response.Headers)
	if reqErr != nil {
		ErrorAddInfo = reqErr
	}
	var headersReq, respErr = json.Marshal(data.Request.Headers)
	if respErr != nil {
		ErrorAddInfo = respErr
	}

	var body, bodyErr = json.Marshal(data.Request.Body)
	if bodyErr != nil {
		ErrorAddInfo = bodyErr
	}

	records := `INSERT INTO req_and_response(IdReq, HeadersResp, Length, Status, HeadersReq, Body, Method, Url) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	query, prepareError := c.SqlDb.Prepare(records)
	if prepareError != nil {
		ErrorAddInfo = prepareError
	}

	_, execError := query.Exec(data.Id, headersResp, data.Response.Length, data.Response.Status, headersReq, body, data.Request.Method, data.Request.Url)
	if execError != nil {
		ErrorAddInfo = execError
	}
	return ErrorAddInfo
}

func (c Services) addInfoTask(idTask string, data []byte) error {
	/*
		Добавляет запрос в БД
	*/

	var ErrorAddInfo error

	records := `INSERT INTO tasks(IdReq, RequestData) VALUES (?, ?)`
	query, prepareError := c.SqlDb.Prepare(records)
	if prepareError != nil {
		ErrorAddInfo = prepareError
	}

	_, execError := query.Exec(idTask, data)
	if execError != nil {
		ErrorAddInfo = execError
	}
	return ErrorAddInfo
}

func (c Services) ReqAndResponse() error {
	/*
		Создает таблицу req_and_response
	*/

	users_table := `CREATE TABLE IF NOT EXISTS req_and_response (
      id INTEGER NOT NULL PRIMARY KEY auto_increment,
      IdReq TEXT,
      HeadersResp BLOB,
      Length INT,
      Status INT,
      HeadersReq BLOB,
      Body BLOB,
      Method TEXT,
      Url TEXT)`

	ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelfunc()
	res, err := c.SqlDb.ExecContext(ctx, users_table)
	if err != nil {
		log.Printf("Error %s when creating product table", err)
		panic(err)
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		log.Printf("Error %s when getting rows affected", err)
		panic(err)
		return err
	}
	log.Printf("Rows affected when creating table: %d", rows)
	return nil

}

func (c Services) Tasks() error {
	/*
		Создает таблицу req_and_response
	*/

	users_table := `CREATE TABLE IF NOT EXISTS tasks (
      id INTEGER NOT NULL PRIMARY KEY auto_increment,
      IdReq TEXT,
      RequestData BLOB)`

	ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelfunc()
	res, err := c.SqlDb.ExecContext(ctx, users_table)
	if err != nil {
		log.Printf("Error %s when creating product table", err)
		panic(err)
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		log.Printf("Error %s when getting rows affected", err)
		panic(err)
		return err
	}
	log.Printf("Rows affected when creating table: %d", rows)
	return nil

}

func (c Services) fetchRequests() ([]map[string]any, error) {
	/*
		Выкачивает всю инфу из БД
	*/

	if c.Table == "req_and_response" {
		var results []map[string]any
		record, queryError := c.SqlDb.Query("SELECT * FROM req_and_response")

		if queryError != nil {
			return results, queryError
		}

		defer func(record *sql.Rows) {
			err := record.Close()
			if err != nil {
				panic(err)
			}

		}(record)

		for record.Next() {
			var res = map[string]any{}
			var id int
			var IdReq string
			var HeadersResp []byte
			var Length int
			var Status int
			var HeadersReq []byte
			var Body []byte
			var Method string
			var Url string
			scanError := record.Scan(&id, &IdReq, &HeadersResp, &Length, &Status, &HeadersReq, &Body, &Method, &Url)

			if scanError != nil {
				return results, scanError
			}

			res["id"] = id
			res["IdReq"] = IdReq
			res["HeadersResp"] = HeadersResp
			res["Length"] = Length
			res["Status"] = Status
			res["HeadersReq"] = HeadersReq
			res["Body"] = Body
			res["Method"] = Method
			res["Url"] = Url
			results = append(results, res)
		}

		return results, nil

	} else if c.Table == "tasks" {
		var results []map[string]any
		record, queryError := c.SqlDb.Query("SELECT * FROM tasks")

		if queryError != nil {
			return results, queryError
		}

		defer func(record *sql.Rows) {
			err := record.Close()
			if err != nil {
				panic(err)
			}

		}(record)

		for record.Next() {
			var res = map[string]any{}
			var id int
			var IdReq string
			var RequestData []byte

			scanError := record.Scan(&id, &IdReq, &RequestData)

			if scanError != nil {
				return results, scanError
			}

			res["id"] = id
			res["IdReq"] = IdReq
			res["RequestData"] = RequestData
			results = append(results, res)
		}

		return results, nil
	} else {
		return nil, nil
	}

}

func (c Services) removeInfo() (bool, error) {
	/*
		Удаляет запрос из Бд по id
	*/

	var idDb, ErrorIdFromDb = c.IdFromDb()
	if idDb == -1 {
		return false, ErrorIdFromDb
	} else {

		var deleteReq = fmt.Sprintf("delete from req_and_response where id = %d", idDb)

		_, execError := c.SqlDb.Exec(deleteReq)

		if execError != nil {
			log.Fatal(execError)
		}

		return true, ErrorIdFromDb

	}

}

func (c Services) searchById() (MainReq, error) {
	/*
		Делает поиск внутри БД по id, если id совпадают, возвращает сохраненный с ним request+response
	*/

	var requests, ErrorfetchRequests = c.fetchRequests()

	for _, requestIter := range requests {
		if requestIter["IdReq"] == c.Id {

			return jsonResp(requestIter), ErrorfetchRequests
		}

	}
	return MainReq{}, ErrorfetchRequests

}

func (c Services) searchByIdTask(idTask string) ([]byte, error) {
	/*
		Делает поиск внутри БД по id, если id совпадают, возвращает сохраненный с ним request+response
	*/

	var requests, ErrorfetchRequests = c.fetchRequests()

	for _, requestIter := range requests {
		if requestIter["IdReq"] == idTask {

			return requestIter["RequestData"].([]byte), ErrorfetchRequests
		}

	}
	return nil, ErrorfetchRequests

}

func (c Services) IdFromDb() (int, error) {
	/*
		Делает поиск внутри БД по id, если id совпадают, возвращает id строки из БД
	*/

	var requests, ErrorfetchRequests = c.fetchRequests()

	for _, requestIter := range requests {
		if requestIter["IdReq"] == c.Id {
			return requestIter["id"].(int), ErrorfetchRequests
		}

	}

	return -1, ErrorfetchRequests

}
