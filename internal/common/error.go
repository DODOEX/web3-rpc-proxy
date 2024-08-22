package common

import (
	"encoding/json"
	"log"
	"runtime"
	"strconv"
	"strings"

	"github.com/DODOEX/web3rpcproxy/utils/helpers"
	"github.com/duke-git/lancet/v2/slice"
)

type HTTPErrors interface {
	QueryStatus() QueryStatus
	Message() string
	Error() string
	StatusCode() int
	Body() []byte
	String() string
}

func IsHTTPErrors(err error) bool {
	_, ok := err.(HTTPErrors)
	return ok
}

type httpError struct {
	Status  int    `json:"code"`
	Name    string `json:"error"`
	Msg     string `json:"message"`
	Details error  `json:"details,omitempty"`
	file    string // 文件名
	line    int    // 行号
}

func (e httpError) QueryStatus() QueryStatus {
	switch e.Name {
	case "Upstream Server Error":
		return Fail
	case "Bad Request":
		fallthrough
	case "Too Many Requests":
		fallthrough
	case "Not Found":
		fallthrough
	case "Forbidden":
		return Reject
	case "Timeout":
		return Timeout
	case "Intercept":
		return Intercept
	}

	if e.Details != nil && strings.Contains(e.Details.Error(), "deadline exceeded") {
		return Timeout
	}

	return Error
}

func (e httpError) Message() string {
	return e.Msg
}

func (e httpError) Error() string {
	if e.Details == nil {
		return helpers.Concat(e.Name, ": ", e.Message())
	}
	return e.Details.Error()
}

func (e httpError) StatusCode() int {
	return e.Status
}

func (e httpError) Body() []byte {
	data, err := json.Marshal(e)
	if err != nil {
		log.Fatal(err)
	}
	return data
}

func (e httpError) String() string {
	return e.Error() + " \n" + e.file + ":" + strconv.Itoa(e.line)
}

func NewHttpError(status int, name, msg string, errs ...error) httpError {
	errs = slice.Compact(errs)
	if len(errs) <= 0 {
		return httpError{
			Status:  status,
			Name:    name,
			Msg:     msg,
			Details: nil,
		}
	}

	msg = errs[0].Error()

	return httpError{
		Status:  status,
		Name:    name,
		Msg:     msg,
		Details: errs[0],
	}
}

func UpstreamServerError(msg string, errs ...error) httpError {
	err := NewHttpError(200, "Upstream Server Error", msg, errs...)
	_, file, line, _ := runtime.Caller(1)
	err.file, err.line = file, line
	return err
}

func BadRequestError(msg string, errs ...error) httpError {
	err := NewHttpError(400, "Bad Request", msg, errs...)
	_, file, line, _ := runtime.Caller(1)
	err.file, err.line = file, line
	return err
}

func ForbiddenError(msg string, errs ...error) httpError {
	err := NewHttpError(403, "Forbidden", msg, errs...)
	_, file, line, _ := runtime.Caller(1)
	err.file, err.line = file, line
	return err
}

func NotFoundError(msg string, errs ...error) httpError {
	err := NewHttpError(404, "Not Found", msg, errs...)
	_, file, line, _ := runtime.Caller(1)
	err.file, err.line = file, line
	return err
}

func TimeoutError(msg string, errs ...error) httpError {
	err := NewHttpError(408, "Timeout", msg, errs...)
	_, file, line, _ := runtime.Caller(1)
	err.file, err.line = file, line
	return err
}

func TooManyRequestsError(msg string, errs ...error) httpError {
	err := NewHttpError(429, "Too Many Requests", msg, errs...)
	_, file, line, _ := runtime.Caller(1)
	err.file, err.line = file, line
	return err
}

func InternalServerError(msg string, errs ...error) httpError {
	err := NewHttpError(500, "Internal Server Error", msg, errs...)
	_, file, line, _ := runtime.Caller(1)
	err.file, err.line = file, line
	return err
}
