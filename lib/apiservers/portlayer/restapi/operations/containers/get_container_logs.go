package containers

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the generate command

import (
	"net/http"

	middleware "github.com/go-openapi/runtime/middleware"
)

// GetContainerLogsHandlerFunc turns a function with the right signature into a get container logs handler
type GetContainerLogsHandlerFunc func(GetContainerLogsParams) middleware.Responder

// Handle executing the request and returning a response
func (fn GetContainerLogsHandlerFunc) Handle(params GetContainerLogsParams) middleware.Responder {
	return fn(params)
}

// GetContainerLogsHandler interface for that can handle valid get container logs params
type GetContainerLogsHandler interface {
	Handle(GetContainerLogsParams) middleware.Responder
}

// NewGetContainerLogs creates a new http.Handler for the get container logs operation
func NewGetContainerLogs(ctx *middleware.Context, handler GetContainerLogsHandler) *GetContainerLogs {
	return &GetContainerLogs{Context: ctx, Handler: handler}
}

/*GetContainerLogs swagger:route GET /containers/{id}/logs containers getContainerLogs

Gets the container logs

Gets the container logs by id

*/
type GetContainerLogs struct {
	Context *middleware.Context
	Handler GetContainerLogsHandler
}

func (o *GetContainerLogs) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	route, _ := o.Context.RouteInfo(r)
	var Params = NewGetContainerLogsParams()

	if err := o.Context.BindValidRequest(r, route, &Params); err != nil { // bind params
		o.Context.Respond(rw, r, route.Produces, route, err)
		return
	}

	res := o.Handler.Handle(Params) // actually handle the request

	o.Context.Respond(rw, r, route.Produces, route, res)

}