package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	strfmt "github.com/go-openapi/strfmt"

	"github.com/go-openapi/errors"
)

// TaskBindResponse task bind response
// swagger:model TaskBindResponse
type TaskBindResponse struct {

	// handle
	// Required: true
	Handle interface{} `json:"handle"`
}

// Validate validates this task bind response
func (m *TaskBindResponse) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateHandle(formats); err != nil {
		// prop
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *TaskBindResponse) validateHandle(formats strfmt.Registry) error {

	return nil
}