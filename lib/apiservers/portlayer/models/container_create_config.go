package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	strfmt "github.com/go-openapi/strfmt"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/swag"
)

// ContainerCreateConfig container create config
// swagger:model ContainerCreateConfig
type ContainerCreateConfig struct {

	// annotations
	Annotations map[string]string `json:"annotations,omitempty"`

	// image
	Image string `json:"image,omitempty"`

	// image store
	ImageStore *ImageStore `json:"imageStore,omitempty"`

	// layer
	Layer string `json:"layer,omitempty"`

	// memory m b
	MemoryMB int64 `json:"memoryMB,omitempty"`

	// name
	Name string `json:"name,omitempty"`

	// network disabled
	NetworkDisabled bool `json:"networkDisabled,omitempty"`

	// num cpus
	NumCpus int64 `json:"numCPUs,omitempty"`

	// repo name
	RepoName string `json:"repoName,omitempty"`
}

// Validate validates this container create config
func (m *ContainerCreateConfig) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateImageStore(formats); err != nil {
		// prop
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *ContainerCreateConfig) validateImageStore(formats strfmt.Registry) error {

	if swag.IsZero(m.ImageStore) { // not required
		return nil
	}

	if m.ImageStore != nil {

		if err := m.ImageStore.Validate(formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("imageStore")
			}
			return err
		}
	}

	return nil
}