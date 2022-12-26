package main

import (
	"encoding/json"
	"net/http"
	"time"
)

func methodGet(data DelAndGet) {
	var db = data.Db
	var Id = data.Id
	var writer = data.Writer
	var services = Services{SqlDb: db, Table: "tasks"}
	var text, ErrorsearchById = services.searchByIdTask(Id)
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

func methodPost(data PostWorkData) MainReq {
	var decoder = data.Decoder
	var sqlInstance = data.Db

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

	if cacheLRU.Id == "" {

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

			return headersData

		}

	} else {
		return cacheLRU
	}
	return MainReq{}

}

func methodDelete(data DelAndGet) {

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

		var services = Services{Id: Id, SqlDb: sqlInstance, Table: "req_and_response"}
		var removed, ErrorRemoveInfo = services.removeInfo()
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
