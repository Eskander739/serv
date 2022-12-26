package main

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
)

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

func GetMD5Hash(text string) string {
	/*
		Генерирует хэш из строки
	*/

	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}

func jsonResp(data map[string]any) MainReq {
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

	var mainReq = MainReq{Id: data["IdReq"].(string), Request: req, Response: resp}

	return mainReq

}
