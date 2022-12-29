package methods

import (
	"database/sql"
	"encoding/json"
	"github.com/Eskander739/serv/internal/service"
	text_helper "github.com/Eskander739/serv/pkg/text-helper"

	"net/http"
	"time"
)

type MethodData struct {
	Writer http.ResponseWriter
	Db     *sql.DB
	Id     string
}

type PostWorkData struct {
	Decoder []byte
	Method  string
	Db      *sql.DB
	Id      string
}

func MethodGet(data MethodData) {
	var db = data.Db
	var Id = data.Id
	var writer = data.Writer
	var services = service.Services{SqlDb: db, Table: "tasks"}
	var text, ErrorsearchById = services.SearchByIdTask(Id)
	if ErrorsearchById != nil {
		panic(ErrorsearchById)
	}

	if text == nil {
		var errorData = map[string]string{"Error": "Request not found"}

		var dataToWatch, jsonError = json.MarshalIndent(errorData, "", "   ")
		_, WriteError := writer.Write(dataToWatch)
		if WriteError != nil {
			panic(WriteError)
		}
		if jsonError != nil {
			panic(jsonError)
		}

	} else {
		_, WriteError := writer.Write(text)
		if WriteError != nil {
			panic(WriteError)
		}

	}
}

func MethodPost(data PostWorkData) service.MainRequest {
	var decoder = data.Decoder
	var sqlInstance = data.Db

	client := &http.Client{Timeout: 5 * time.Second}
	var jsonReq = service.Request{}

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

	var jsR = service.Request{Method: jsonReq.Method, Url: url, Headers: jsonReq.Headers, Body: jsonReq.Body}
	var services = service.Services{Req: jsR, SqlDb: sqlInstance, Client: client, Table: "req_and_response"}
	var cacheLRU, ErrorCacheLRU = services.CacheLRU()
	if ErrorCacheLRU != nil {
		panic(ErrorCacheLRU)
	}

	if cacheLRU.Id == "" {

		var uuidForReq = text_helper.Uuid()

		var httpResponse, ErrorHttpRequest = services.HttpRequest()
		if ErrorHttpRequest != nil {
			panic(ErrorHttpRequest)
		}

		if httpResponse.StatusCode != 400 {
			var contentType = httpResponse.Header["Content-Type"][0]
			var secondHeaders = map[string]any{"Content-Length": httpResponse.ContentLength, "Content-Type": contentType}
			var headersData = service.MainRequest{Id: uuidForReq, Request: service.Request{Headers: headers, Body: body, Method: method, Url: url},
				Response: service.Response{Headers: secondHeaders, Length: int(httpResponse.ContentLength), Status: httpResponse.StatusCode}}
			ErrorAddInfo := services.AddInfo(headersData)
			if ErrorAddInfo != nil {
				panic(ErrorAddInfo)
			}

			return headersData

		}

	} else {
		return cacheLRU
	}
	return service.MainRequest{}

}

func MethodDelete(data MethodData) {

	var Id = data.Id
	var sqlInstance = data.Db
	var writer = data.Writer

	if Id == "" {
		var errorData = map[string]string{"Error": "ID field is empty"}
		var dataToWatch, jsonError = json.MarshalIndent(errorData, "", "   ")
		_, WriteError := writer.Write(dataToWatch)
		if WriteError != nil {
			panic(WriteError)
		}
		if jsonError != nil {
			panic(jsonError)
		}

	} else {

		var services = service.Services{Id: Id, SqlDb: sqlInstance, Table: "req_and_response"}
		var removed, ErrorRemoveInfo = services.RemoveInfo()
		if ErrorRemoveInfo != nil {
			panic(ErrorRemoveInfo)
		}

		if removed {
			var errorData = map[string]string{"OK": "Request deleted"}

			var dataToWatch, jsonError = json.MarshalIndent(errorData, "", "   ")
			_, WriteError := writer.Write(dataToWatch)
			if WriteError != nil {
				panic(WriteError)
			}
			if jsonError != nil {
				panic(jsonError)
			}

		} else if !removed {
			var errorData = map[string]string{"Error": "Request not found"}

			var dataToWatch, jsonError = json.MarshalIndent(errorData, "", "   ")
			_, WriteError := writer.Write(dataToWatch)
			if WriteError != nil {
				panic(WriteError)
			}
			if jsonError != nil {
				panic(jsonError)
			}
		}

	}

}
