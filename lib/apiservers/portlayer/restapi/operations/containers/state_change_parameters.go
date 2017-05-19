package containers

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"io"
	"net/http"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"

	strfmt "github.com/go-openapi/strfmt"
)

// NewStateChangeParams creates a new StateChangeParams object
// with the default values initialized.
func NewStateChangeParams() StateChangeParams {
	var ()
	return StateChangeParams{}
}

// StateChangeParams contains all the bound params for the state change operation
// typically these are obtained from a http.Request
//
// swagger:parameters StateChange
type StateChangeParams struct {

	// HTTP Request Object
	HTTPRequest *http.Request

	/*
	  Required: true
	  In: path
	*/
	Handle string
	/*
	  Required: true
	  In: body
	*/
	State string
}

// BindRequest both binds and validates a request, it assumes that complex things implement a Validatable(strfmt.Registry) error interface
// for simple values it will use straight method calls
func (o *StateChangeParams) BindRequest(r *http.Request, route *middleware.MatchedRoute) error {
	var res []error
	o.HTTPRequest = r

	rHandle, rhkHandle, _ := route.Params.GetOK("handle")
	if err := o.bindHandle(rHandle, rhkHandle, route.Formats); err != nil {
		res = append(res, err)
	}

	if runtime.HasBody(r) {
		defer r.Body.Close()
		var body string
		if err := route.Consumer.Consume(r.Body, &body); err != nil {
			if err == io.EOF {
				res = append(res, errors.Required("state", "body"))
			} else {
				res = append(res, errors.NewParseError("state", "body", "", err))
			}

		} else {

			if len(res) == 0 {
				o.State = body
			}
		}

	} else {
		res = append(res, errors.Required("state", "body"))
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (o *StateChangeParams) bindHandle(rawData []string, hasKey bool, formats strfmt.Registry) error {
	var raw string
	if len(rawData) > 0 {
		raw = rawData[len(rawData)-1]
	}

	o.Handle = raw

	return nil
}