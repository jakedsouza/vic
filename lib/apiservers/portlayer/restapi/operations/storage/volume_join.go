package storage

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the generate command

import (
	"net/http"

	middleware "github.com/go-openapi/runtime/middleware"
)

// VolumeJoinHandlerFunc turns a function with the right signature into a volume join handler
type VolumeJoinHandlerFunc func(VolumeJoinParams) middleware.Responder

// Handle executing the request and returning a response
func (fn VolumeJoinHandlerFunc) Handle(params VolumeJoinParams) middleware.Responder {
	return fn(params)
}

// VolumeJoinHandler interface for that can handle valid volume join params
type VolumeJoinHandler interface {
	Handle(VolumeJoinParams) middleware.Responder
}

// NewVolumeJoin creates a new http.Handler for the volume join operation
func NewVolumeJoin(ctx *middleware.Context, handler VolumeJoinHandler) *VolumeJoin {
	return &VolumeJoin{Context: ctx, Handler: handler}
}

/*VolumeJoin swagger:route POST /storage/volumes/{name} storage volumeJoin

Attach a volume to a container

*/
type VolumeJoin struct {
	Context *middleware.Context
	Handler VolumeJoinHandler
}

func (o *VolumeJoin) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	route, _ := o.Context.RouteInfo(r)
	var Params = NewVolumeJoinParams()

	if err := o.Context.BindValidRequest(r, route, &Params); err != nil { // bind params
		o.Context.Respond(rw, r, route.Produces, route, err)
		return
	}

	res := o.Handler.Handle(Params) // actually handle the request

	o.Context.Respond(rw, r, route.Produces, route, res)

}