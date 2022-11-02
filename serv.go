package main

// нужно добавить go модули
import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type Response struct {
	Headers map[string]any
	Length  int
	Status  int
}
type Request struct {
	Headers map[string]string `json:"headers"`
	Body    map[string]string `json:"body"`
	Method  string            `json:"method"`
	Url     string            `json:"url"`
}

type MainReq struct {
	Id       string
	Response Response
	Request  Request
}

type Services struct {
	Req    Request
	Client *http.Client
	SqlDb  *sql.DB
	Id     string
	Table  string
}

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

func (c Services) CacheLRU() ([]byte, error) {
	/*
		Проверяет, существует ли подобный request в БД, если да, возвращает request+response
	*/

	var headers = fmt.Sprintf("%s", c.Req.Headers)
	var body = fmt.Sprintf("%s", c.Req.Body)
	var requests, ErrorfetchRequests = c.fetchRequests()
	if ErrorfetchRequests != nil {
		return nil, ErrorfetchRequests
	}
	for _, requestIter := range requests {
		var methodLocal = (requestIter["Method"]).(string)
		var urlLocal = (requestIter["Url"]).(string)

		var headerstLocal = map[string]string{}
		var _ = json.Unmarshal(requestIter["HeadersReq"].([]byte), &headerstLocal)

		var bodyLocal = map[string]string{}
		var unmarshalError = json.Unmarshal(requestIter["Body"].([]byte), &bodyLocal)

		if unmarshalError != nil {
			return nil, unmarshalError
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

	return nil, nil
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

func uuid() string {
	/*
		Генератор уникальных id
	*/

	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		log.Fatal(err)
	}
	uuid := fmt.Sprintf("%x-%x-%x-%x-%x",

		b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
	return uuid
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

func GetMD5Hash(text string) string {
	/*
		Генерирует хэш из строки
	*/

	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}

func jsonResp(data map[string]any) []byte {
	/*
		Генерирует Json ответ
	*/

	var headersReq = map[string]string{}
	var unmarshalError = json.Unmarshal(data["HeadersReq"].([]byte), &headersReq)

	if unmarshalError != nil {
		log.Fatal(unmarshalError)
	}

	var headersResp = map[string]any{}
	var unmarshalError2 = json.Unmarshal(data["HeadersResp"].([]byte), &headersResp)

	if unmarshalError2 != nil {
		log.Fatal(unmarshalError2)
	}

	var body = map[string]string{}
	var unmarshalError3 = json.Unmarshal(data["Body"].([]byte), &body)

	if unmarshalError3 != nil {
		log.Fatal(unmarshalError3)
	}

	var req = Request{Headers: headersReq,
		Body:   body,
		Method: (data["Method"]).(string), Url: (data["Url"]).(string)}

	var resp = Response{Headers: headersResp,
		Length: (data["Length"]).(int),
		Status: (data["Status"]).(int)}
	var result = map[string]any{"id": data["IdReq"], "request": req, "response": resp}
	var jsonResult, jsonError = json.MarshalIndent(result, "", "    ")

	if jsonError != nil {
		log.Fatal(jsonError)
	}

	return jsonResult

}

func (c Services) searchById() ([]byte, error) {
	/*
		Делает поиск внутри БД по id, если id совпадают, возвращает сохраненный с ним request+response
	*/

	var requests, ErrorfetchRequests = c.fetchRequests()

	for _, requestIter := range requests {
		if requestIter["IdReq"] == c.Id {

			return jsonResp(requestIter), ErrorfetchRequests
		}

	}
	return nil, ErrorfetchRequests

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

func methodPost(decoder []byte, sqlInstance *sql.DB, idTask string) {

	client := &http.Client{Timeout: 5 * time.Second}
	var jsonReq = Request{}

	var unmarshalError = json.Unmarshal(decoder, &jsonReq)

	if unmarshalError != nil {
		panic(unmarshalError)
	}

	var body = map[string]string{}
	var headers = map[string]string{}
	if jsonReq.Body == nil {
		body = nil
	} else {
		body = jsonReq.Body
	}

	if jsonReq.Headers == nil {
		headers = nil

	} else {
		headers = jsonReq.Headers
	}
	var method = jsonReq.Method
	var url = jsonReq.Url

	var jsR = Request{Method: jsonReq.Method, Url: url, Headers: jsonReq.Headers, Body: jsonReq.Body}
	var services = Services{Req: jsR, SqlDb: sqlInstance, Client: client, Table: "req_and_response"}
	var cacheLRU, ErrorCacheLRU = services.CacheLRU()
	if ErrorCacheLRU != nil {
		panic(ErrorCacheLRU)
	}

	if cacheLRU == nil {

		var uuidForReq = uuid()

		var httpResponse, ErrorHttpRequest = services.HttpRequest()
		if ErrorHttpRequest != nil {
			panic(ErrorHttpRequest)
		}

		if httpResponse.StatusCode != 400 {
			var contentType = httpResponse.Header["Content-Type"][0]
			var secondHeaders = map[string]any{"Content-Length": httpResponse.ContentLength, "Content-Type": contentType}
			var headersData = MainReq{Id: uuidForReq, Request: Request{Headers: headers, Body: body, Method: method, Url: url},
				Response: Response{Headers: secondHeaders, Length: int(httpResponse.ContentLength), Status: httpResponse.StatusCode}}
			ErrorAddInfo := services.addInfo(headersData)
			if ErrorAddInfo != nil {
				panic(ErrorAddInfo)
			}

			var dataToWatch, jsonError = json.MarshalIndent(headersData, "", "   ")
			ErrorAddInfoTask := services.addInfoTask(idTask, dataToWatch)
			if ErrorAddInfoTask != nil {
				panic(ErrorAddInfoTask)
			}
			if jsonError != nil {
				panic(jsonError)
			}

		}

	} else {

		ErrorAddInfoTask := services.addInfoTask(idTask, cacheLRU)
		if ErrorAddInfoTask != nil {
			panic(ErrorAddInfoTask)
		}

	}
}

type ReqId struct {
	Id string
}

func methodGet(decoder []byte, sqlInstance *sql.DB, idTask string) {
	/*
		Метод возвращает request по id
	*/

	var jsonReq = ReqId{}

	unmarshalError := json.Unmarshal(decoder, &jsonReq)
	if unmarshalError != nil {
		panic(unmarshalError)
	}

	if jsonReq.Id == "" {
		var services = Services{SqlDb: sqlInstance, Table: "tasks"}
		var errorData = map[string]string{"Error": "ID field is empty"}

		var dataToWatch, jsonError = json.MarshalIndent(errorData, "", "   ")
		ErrorAddInfoTask := services.addInfoTask(idTask, dataToWatch)
		if ErrorAddInfoTask != nil {
			panic(ErrorAddInfoTask)
		}
		if jsonError != nil {
			panic(jsonError)
		}

	} else {
		var services = Services{SqlDb: sqlInstance, Id: jsonReq.Id, Table: "req_and_response"}
		var text, ErrorsearchById = services.searchById()
		if ErrorsearchById != nil {
			panic(ErrorsearchById)
		}

		if text == nil {
			var errorData = map[string]string{"Error": "Request not found"}

			var dataToWatch, jsonError = json.MarshalIndent(errorData, "", "   ")
			ErrorAddInfoTask := services.addInfoTask(idTask, dataToWatch)
			if ErrorAddInfoTask != nil {
				panic(ErrorAddInfoTask)
			}
			if jsonError != nil {
				panic(jsonError)
			}

		} else {
			ErrorAddInfoTask := services.addInfoTask(idTask, text)
			if ErrorAddInfoTask != nil {
				panic(ErrorAddInfoTask)
			}
		}
	}

}

func methodGetTask(sqlInstance *sql.DB, idTask string, rw http.ResponseWriter) {
	/*
		Метод возвращает request по id
	*/
	var services = Services{SqlDb: sqlInstance, Table: "tasks"}
	var text, ErrorsearchById = services.searchByIdTask(idTask)
	if ErrorsearchById != nil {
		panic(ErrorsearchById)
	}

	if text == nil {
		var errorData = map[string]string{"Error": "Request not found"}

		var dataToWatch, jsonError = json.MarshalIndent(errorData, "", "   ")
		_, WriteError := rw.Write(dataToWatch)
		if WriteError != nil {
			panic(WriteError)
		}
		if jsonError != nil {
			panic(jsonError)
		}

	} else {
		_, WriteError := rw.Write(text)
		if WriteError != nil {
			panic(WriteError)
		}

	}
}

func methodDelete(decoder []byte, sqlInstance *sql.DB, idTask string) {
	/*
		Метод удаляет request по id
	*/

	var jsonReq = ReqId{}

	unmarshalError := json.Unmarshal(decoder, &jsonReq)
	if unmarshalError != nil {
		panic(unmarshalError)
	}

	if jsonReq.Id == "" {
		var services = Services{SqlDb: sqlInstance}
		var errorData = map[string]string{"Error": "ID field is empty"}

		var dataToWatch, jsonError = json.MarshalIndent(errorData, "", "   ")
		ErrorAddInfoTask := services.addInfoTask(idTask, dataToWatch)
		if ErrorAddInfoTask != nil {
			panic(ErrorAddInfoTask)
		}
		if jsonError != nil {
			panic(jsonError)
		}
	} else {
		var id = jsonReq.Id
		var services = Services{Id: id, SqlDb: sqlInstance, Table: "req_and_response"}
		var removed, ErrorRemoveInfo = services.removeInfo()
		if ErrorRemoveInfo != nil {
			panic(ErrorRemoveInfo)
		}

		if removed {
			var errorData = map[string]string{"OK": "Request deleted"}

			var dataToWatch, jsonError = json.MarshalIndent(errorData, "", "   ")
			ErrorAddInfoTask := services.addInfoTask(idTask, dataToWatch)
			if ErrorAddInfoTask != nil {
				panic(ErrorAddInfoTask)
			}
			if jsonError != nil {
				panic(jsonError)
			}

		} else if !removed {
			var errorData = map[string]string{"Error": "Request not found"}

			var dataToWatch, jsonError = json.MarshalIndent(errorData, "", "   ")
			ErrorAddInfoTask := services.addInfoTask(idTask, dataToWatch)
			if ErrorAddInfoTask != nil {
				panic(ErrorAddInfoTask)
			}
			if jsonError != nil {
				panic(jsonError)
			}
		}

	}

}

type idTaskReq struct {
	IdTask string
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

func Worker(mainChan chan WKData, StatusChan chan string, i int) {
	fmt.Println("Запущена горутина: ", i)
	for {

		var mainData = <-mainChan

		var idTask = mainData.Id
		var decoder = mainData.Decoder
		var sqlInstance = mainData.Db
		var methodWK = mainData.Method
		fmt.Println("Работает горутина номер ", i)
		// зачем здесь обработка http методов GET и DELETE
		// что эти эти методы должны делать и почему они в воркере?
		// воркер про http часть ничего не должен здать, воркер просто выполняет задаяние
		if methodWK == http.MethodGet {
			StatusChan <- "Task started"

			var jsonReq = ReqId{}

			unmarshalError := json.Unmarshal(decoder, &jsonReq)
			if unmarshalError != nil {
				panic(unmarshalError)
			}

			if jsonReq.Id == "" {
				var services = Services{SqlDb: sqlInstance, Table: "tasks"}
				var errorData = map[string]string{"Error": "ID field is empty"}

				var dataToWatch, jsonError = json.MarshalIndent(errorData, "", "   ")
				ErrorAddInfoTask := services.addInfoTask(idTask, dataToWatch)
				if ErrorAddInfoTask != nil {
					panic(ErrorAddInfoTask)
				}
				if jsonError != nil {
					panic(jsonError)
				}

			} else {
				var services = Services{SqlDb: sqlInstance, Id: jsonReq.Id, Table: "req_and_response"}
				var text, ErrorsearchById = services.searchById()
				if ErrorsearchById != nil {
					panic(ErrorsearchById)
				}

				if text == nil {
					var errorData = map[string]string{"Error": "Request not found"}

					var dataToWatch, jsonError = json.MarshalIndent(errorData, "", "   ")
					ErrorAddInfoTask := services.addInfoTask(idTask, dataToWatch)
					if ErrorAddInfoTask != nil {
						panic(ErrorAddInfoTask)
					}
					if jsonError != nil {
						panic(jsonError)
					}

				} else {
					ErrorAddInfoTask := services.addInfoTask(idTask, text)
					if ErrorAddInfoTask != nil {
						panic(ErrorAddInfoTask)
					}
				}
			}
		} else if methodWK == http.MethodPost {
			StatusChan <- "Task started"

			client := &http.Client{Timeout: 5 * time.Second}
			var jsonReq = Request{}

			var unmarshalError = json.Unmarshal(decoder, &jsonReq)

			if unmarshalError != nil {
				panic(unmarshalError)
			}

			var body = map[string]string{}
			var headers = map[string]string{}
			if jsonReq.Body == nil {
				body = nil
			} else {
				body = jsonReq.Body
			}

			if jsonReq.Headers == nil {
				headers = nil

			} else {
				headers = jsonReq.Headers
			}
			var method = jsonReq.Method
			var url = jsonReq.Url

			var jsR = Request{Method: jsonReq.Method, Url: url, Headers: jsonReq.Headers, Body: jsonReq.Body}
			var services = Services{Req: jsR, SqlDb: sqlInstance, Client: client, Table: "req_and_response"}
			var cacheLRU, ErrorCacheLRU = services.CacheLRU()
			if ErrorCacheLRU != nil {
				panic(ErrorCacheLRU)
			}

			if cacheLRU == nil {

				var uuidForReq = uuid()

				var httpResponse, ErrorHttpRequest = services.HttpRequest()
				if ErrorHttpRequest != nil {
					panic(ErrorHttpRequest)
				}

				if httpResponse.StatusCode != 400 {
					var contentType = httpResponse.Header["Content-Type"][0]
					var secondHeaders = map[string]any{"Content-Length": httpResponse.ContentLength, "Content-Type": contentType}
					var headersData = MainReq{Id: uuidForReq, Request: Request{Headers: headers, Body: body, Method: method, Url: url},
						Response: Response{Headers: secondHeaders, Length: int(httpResponse.ContentLength), Status: httpResponse.StatusCode}}
					ErrorAddInfo := services.addInfo(headersData)
					if ErrorAddInfo != nil {
						panic(ErrorAddInfo)
					}

					var dataToWatch, jsonError = json.MarshalIndent(headersData, "", "   ")
					ErrorAddInfoTask := services.addInfoTask(idTask, dataToWatch)
					if ErrorAddInfoTask != nil {
						panic(ErrorAddInfoTask)
					}
					if jsonError != nil {
						panic(jsonError)
					}

				}

			} else {

				ErrorAddInfoTask := services.addInfoTask(idTask, cacheLRU)
				if ErrorAddInfoTask != nil {
					panic(ErrorAddInfoTask)
				}

			}

		} else if methodWK == http.MethodDelete {
			StatusChan <- "Task started"
			var jsonReq = ReqId{}

			unmarshalError := json.Unmarshal(decoder, &jsonReq)
			if unmarshalError != nil {
				panic(unmarshalError)
			}

			if jsonReq.Id == "" {
				var services = Services{SqlDb: sqlInstance}
				var errorData = map[string]string{"Error": "ID field is empty"}

				var dataToWatch, jsonError = json.MarshalIndent(errorData, "", "   ")
				ErrorAddInfoTask := services.addInfoTask(idTask, dataToWatch)
				if ErrorAddInfoTask != nil {
					panic(ErrorAddInfoTask)
				}
				if jsonError != nil {
					panic(jsonError)
				}
			} else {
				var id = jsonReq.Id
				var services = Services{Id: id, SqlDb: sqlInstance, Table: "req_and_response"}
				var removed, ErrorRemoveInfo = services.removeInfo()
				if ErrorRemoveInfo != nil {
					panic(ErrorRemoveInfo)
				}

				if removed {
					var errorData = map[string]string{"OK": "Request deleted"}

					var dataToWatch, jsonError = json.MarshalIndent(errorData, "", "   ")
					ErrorAddInfoTask := services.addInfoTask(idTask, dataToWatch)
					if ErrorAddInfoTask != nil {
						panic(ErrorAddInfoTask)
					}
					if jsonError != nil {
						panic(jsonError)
					}

				} else if !removed {
					var errorData = map[string]string{"Error": "Request not found"}

					var dataToWatch, jsonError = json.MarshalIndent(errorData, "", "   ")
					ErrorAddInfoTask := services.addInfoTask(idTask, dataToWatch)
					if ErrorAddInfoTask != nil {
						panic(ErrorAddInfoTask)
					}
					if jsonError != nil {
						panic(jsonError)
					}
				}

			}
		}

	}

}

type WKData struct {
	Decoder []byte
	Method  string
	Db      *sql.DB
	Id      string
}

func main() {
	db := dbInitialization()
	var StatusChan = make(chan string)
	var mainChan = make(chan WKData)
	for i := 1; i != 6; i += 1 {
		go Worker(mainChan, StatusChan, i)
	}

	var server = func(w http.ResponseWriter, r *http.Request) {

		decoder, errReadAll := ioutil.ReadAll(r.Body)
		if errReadAll != nil {
			panic(errReadAll)
		}

		var Req = idTaskReq{}
		unmarshalError := json.Unmarshal(decoder, &Req)
		if unmarshalError != nil {
			panic(unmarshalError)
		}
		if r.Method == http.MethodGet && Req.IdTask != "" {
			methodGetTask(db, Req.IdTask, w)

		} else {
			var idTask = uuid()
			mainChan <- WKData{decoder, r.Method, db, idTask}
			var TaskData = map[string]string{"idTask": idTask, "Status": <-StatusChan} // здесь зачем статус ждать? зачем это нужно, также у тебя тут блокироваться будет и потеряется вся асинхронность
			var idTaskData, jsonError = json.MarshalIndent(TaskData, "", "   ")
			_, writeError := w.Write(idTaskData)
			if writeError != nil {
				panic(writeError)
			}
			if jsonError != nil {
				panic(jsonError)
			}

		}
		errClose := r.Body.Close()
		if errClose != nil {
			panic(errClose)
		}

	}

	http.HandleFunc("/", server)
	listenError := http.ListenAndServe(":8000", nil)
	if listenError != nil {
		log.Fatal(listenError)
	}

}
