package gofast

import (
	"bytes"
	"log"
	"net/http"
)

// Handler is implements http.Handler and provide logger changing method.
type Handler interface {
	http.Handler
	SetLogger(logger *log.Logger)
}

// NewHandler returns the default Handler implementation. This default Handler
// act as the "web server" component in fastcgi specification, which connects
// fastcgi "application" through the network/address and passthrough I/O as
// specified.
//
// An extra Middleware (if nil, will be ignored) can be provided to modify
// the *Request or rewrite the response stream.
//
func NewHandler(middleware Middleware, network, address string) Handler {
	sessionHandler := BasicSession
	if middleware != nil {
		sessionHandler = middleware(sessionHandler)
	}
	return &defaultHandler{
		sessionHandler: sessionHandler,
		newClient: SimpleClientFactory(
			SimpleConnFactory(network, address),
			0,
		),
	}
}

// defaultHandler implements Handler
type defaultHandler struct {
	sessionHandler SessionHandler
	newClient      ClientFactory
	logger         *log.Logger
}

// SetLogger implements Handler
func (h *defaultHandler) SetLogger(logger *log.Logger) {
	h.logger = logger
}

// ServeHTTP implements http.Handler
func (h *defaultHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// TODO: separate dial logic to pool client / connection
	c, err := h.newClient()
	if err != nil {
		http.Error(w, "failed to connect to FastCGI application", http.StatusBadGateway)
		log.Printf("gofast: unable to connect to FastCGI application. %s",
			err.Error())
		return
	}

	// handle the session
	resp, err := h.sessionHandler(c, c.NewRequest(r))
	if err != nil {
		http.Error(w, "failed to process request", http.StatusInternalServerError)
		log.Printf("gofast: unable to process request %s",
			err.Error())
		return
	}
	errBuffer := new(bytes.Buffer)
	resp.WriteTo(w, errBuffer)

	if errBuffer.Len() > 0 {
		log.Printf("gofast: error stream from application process %s",
			errBuffer.String())
		return
	}

	// signal to close the client
	// or the pool to return the client
	if err = c.Close(); err != nil {
		log.Printf("gofast: error closeing client: %s",
			err.Error())
	}
}
