package pjerrors

import "fmt"

type HttpErrors struct{
	Code string `json:"code"`
	Message string `json:"message"`
}


func(err *HttpErrors) Error() string{
	return fmt.Sprintf("HttpError: code: %s: message: %s", err.Code, err.Message)
}