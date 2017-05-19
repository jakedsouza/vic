package logging

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"net/http"

	"github.com/go-openapi/runtime"

	"github.com/vmware/vic/lib/apiservers/portlayer/models"
)

/*LoggingJoinOK OK

swagger:response loggingJoinOK
*/
type LoggingJoinOK struct {

	/*
	  In: Body
	*/
	Payload *models.LoggingJoinResponse `json:"body,omitempty"`
}

// NewLoggingJoinOK creates LoggingJoinOK with default headers values
func NewLoggingJoinOK() *LoggingJoinOK {
	return &LoggingJoinOK{}
}

// WithPayload adds the payload to the logging join o k response
func (o *LoggingJoinOK) WithPayload(payload *models.LoggingJoinResponse) *LoggingJoinOK {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the logging join o k response
func (o *LoggingJoinOK) SetPayload(payload *models.LoggingJoinResponse) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *LoggingJoinOK) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(200)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}

/*LoggingJoinNotFound VirtualDevice not found

swagger:response loggingJoinNotFound
*/
type LoggingJoinNotFound struct {

	/*
	  In: Body
	*/
	Payload *models.Error `json:"body,omitempty"`
}

// NewLoggingJoinNotFound creates LoggingJoinNotFound with default headers values
func NewLoggingJoinNotFound() *LoggingJoinNotFound {
	return &LoggingJoinNotFound{}
}

// WithPayload adds the payload to the logging join not found response
func (o *LoggingJoinNotFound) WithPayload(payload *models.Error) *LoggingJoinNotFound {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the logging join not found response
func (o *LoggingJoinNotFound) SetPayload(payload *models.Error) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *LoggingJoinNotFound) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(404)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}

/*LoggingJoinInternalServerError Adding a VirtualDevice failed

swagger:response loggingJoinInternalServerError
*/
type LoggingJoinInternalServerError struct {

	/*
	  In: Body
	*/
	Payload *models.Error `json:"body,omitempty"`
}

// NewLoggingJoinInternalServerError creates LoggingJoinInternalServerError with default headers values
func NewLoggingJoinInternalServerError() *LoggingJoinInternalServerError {
	return &LoggingJoinInternalServerError{}
}

// WithPayload adds the payload to the logging join internal server error response
func (o *LoggingJoinInternalServerError) WithPayload(payload *models.Error) *LoggingJoinInternalServerError {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the logging join internal server error response
func (o *LoggingJoinInternalServerError) SetPayload(payload *models.Error) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *LoggingJoinInternalServerError) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(500)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}