package main

import (
	"database/sql"
	"net/http"
)

type Services struct {
	Req    Request
	Client *http.Client
	SqlDb  *sql.DB
	Id     string
	Table  string
}

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

type PostWorkData struct {
	Decoder []byte
	Method  string
	Db      *sql.DB
	Id      string
}

type Data struct {
	Body   MainReq
	Db     *sql.DB
	IdTask string
}

type DelAndGet struct {
	Writer http.ResponseWriter
	Db     *sql.DB
	Id     string
}

type JobAndWork struct {
	Job Data
}
