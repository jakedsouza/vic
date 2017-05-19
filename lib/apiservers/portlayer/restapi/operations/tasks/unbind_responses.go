package tasks

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"net/http"

	"github.com/go-openapi/runtime"

	"github.com/vmware/vic/lib/apiservers/portlayer/models"
)

/*UnbindOK OK

swagger:response unbindOK
*/
type UnbindOK struct {

	/*
	  In: Body
	*/
	Payload *models.TaskUnbindResponse `json:"body,omitempty"`
}

// NewUnbindOK creates UnbindOK with default headers values
func NewUnbindOK() *UnbindOK {
	return &UnbindOK{}
}

// WithPayload adds the payload to the unbind o k response
func (o *UnbindOK) WithPayload(payload *models.TaskUnbindResponse) *UnbindOK {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the unbind o k response
func (o *UnbindOK) SetPayload(payload *models.TaskUnbindResponse) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *UnbindOK) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(200)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}

/*UnbindNotFound No such task

swagger:response unbindNotFound
*/
type UnbindNotFound struct {

	/*
	  In: Body
	*/
	Payload *models.Error `json:"body,omitempty"`
}

// NewUnbindNotFound creates UnbindNotFound with default headers values
func NewUnbindNotFound() *UnbindNotFound {
	return &UnbindNotFound{}
}

// WithPayload adds the payload to the unbind not found response
func (o *UnbindNotFound) WithPayload(payload *models.Error) *UnbindNotFound {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the unbind not found response
func (o *UnbindNotFound) SetPayload(payload *models.Error) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *UnbindNotFound) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(404)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}

/*UnbindInternalServerError Deactivating task failed

swagger:response unbindInternalServerError
*/
type UnbindInternalServerError struct {

	/*
	  In: Body
	*/
	Payload *models.Error `json:"body,omitempty"`
}

// NewUnbindInternalServerError creates UnbindInternalServerError with default headers values
func NewUnbindInternalServerError() *UnbindInternalServerError {
	return &UnbindInternalServerError{}
}

// WithPayload adds the payload to the unbind internal server error response
func (o *UnbindInternalServerError) WithPayload(payload *models.Error) *UnbindInternalServerError {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the unbind internal server error response
func (o *UnbindInternalServerError) SetPayload(payload *models.Error) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *UnbindInternalServerError) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(500)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}