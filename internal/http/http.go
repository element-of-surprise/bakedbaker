/*
Package http provides an HTTP frontend for the agent baker service. This is a simple
HTTP server that routes requests to the appropriate backend agent baker service.

Usage is simple:

	verMap, err := versions.New()
	if err != nil {
		panic(err)
	}

	serv, err := New(verMap)
	if err != nil {
		panic(err)
	}

	panic(serv.ListenAndServe(*addr))
*/
package http

import (
	"fmt"
	"path"
	"reflect"
	"time"

	"github.com/element-of-surprise/bakedbaker/internal/versions"
	"github.com/go-json-experiment/json"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

// VersionedReq is a request that includes an Agent Baker version.
type VersionedReq[T any] struct {
	// ABVersion is the Agent Baker version. This must be set to a valid version
	// or "latest".
	ABVersion versions.Version
	// Req is the request to be sent to the agent baker service.
	Req T
}

// Server provides an HTTP frontend that routes requests to the appropriate
// backend agent baker service.
type Server struct {
	app *fiber.App

	mapping versions.Mapping
}

// Option is an option for the New() constructor. This is
// currently unused.
type Option func(*Server) error

// New creates a new Server.
func New(mapping versions.Mapping, options ...Option) (*Server, error) {
	s := &Server{mapping: mapping}

	for _, o := range options {
		if err := o(s); err != nil {
			return nil, err
		}
	}

	conf := fiber.Config{
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	app := fiber.New(conf)
	app.Use(compress.New())

	// These handle all the current endpoints.
	app.Post("/getnodebootstrapdata", s.bootstrapData)
	app.Post("/getlatestsigimageconfig", s.latestConfig)
	app.Post("/getdistrosigimageconfig", s.distroConfig)
	app.Get("/healthz", s.healthz)

	s.app = app
	return s, nil
}

// ListenAndServe starts the server on the given address. This is a blocking call.
// It returns an error if the server fails to start. addr should be a string in the
// format "host:port".
func (s *Server) ListenAndServe(addr string) error {
	return s.app.Listen(addr)
}

// okContentTypeHeader is the content type header for a successful response to healthz.
// This provides a static value that never has to be reallocated.
var okContentTypeHeader = []string{"MIMETextPlainCharsetUTF8"}

// healthz is a handler for the /healthz endpoint. It returns a 200 OK status code.
func (s *Server) healthz(c *fiber.Ctx) error {
	headers := c.GetRespHeaders()
	headers[fiber.HeaderContentType] = okContentTypeHeader
	c.SendStatus(fiber.StatusOK)
	return nil
}

// versionedRequest returns the AgentBaker version to use, the config to use, and an error.
// This is generic and can be used for any request. This handles raw JSON requests or ones
// that are wrapped in a VersionedReq. If a raw request, the version will be versions.Latest.
func versionedRequest[T any](body []byte) (versions.Version, T, error) {
	var emptyT T // Used when we return an error

	if len(body) == 0 {
		return "", emptyT, fmt.Errorf("empty body")
	}

	var config T
	var versioned VersionedReq[T]

	// If this errors, this is some JSON error and not that we don't have the right fields.
	if err := json.Unmarshal(body, &versioned); err != nil {
		return "", emptyT, fmt.Errorf("could not unmarshal our the body content to VersionedReq: %w", err)
	}

	// If we don't have a .Req, then this is either a request for latest (using non-versioned request type)
	// or a mistake. We determine if it is a mistake by checking if .ABVersion is set.
	if reflect.ValueOf(versioned.Req).IsZero() {
		if versioned.ABVersion != "" {
			return "", emptyT, fmt.Errorf("must provide .Req if .ABVersion is set")
		}

		// Let's try again directly against the config.
		if err := json.Unmarshal(body, &config); err != nil {
			return "", emptyT, fmt.Errorf("could not unmarshal our the body content to GetNodeBootstrapDataRequest: %w", err)
		}
		if reflect.ValueOf(config).IsZero() {
			return "", emptyT, fmt.Errorf("must provide a valid request")
		}
		return versions.Latest, config, nil
	}

	if versioned.ABVersion == "" {
		return "", emptyT, fmt.Errorf("must provide a version")
	}
	return versioned.ABVersion, versioned.Req, nil
}

// sendToAgentBaker sends the request to the agent baker service and returns the response the client.
func sendToAgentBaker(c *fiber.Ctx, base string, body []byte) error {
	agent := fiber.Post(path.Join(base, c.Path()))
	agent = agent.Body(body)
	c.Request().Header.VisitAll(func(key, value []byte) {
		// TODO: consider using unsafe to avoid the string conversion.
		// Would need to test that this is safe, because fasthttp might do something funky.
		agent.Request().Header.Add(string(key), string(value))
	})

	status, body, errs := agent.Bytes()

	if len(errs) > 0 {
		return fmt.Errorf("could not send the request to the agent: %w", errs[0])
	}
	if status != fiber.StatusOK {
		return fmt.Errorf("the agent returned a non-200 status code: %d", status)
	}

	return c.Send(body)
}

func (s *Server) bootstrapData(c *fiber.Ctx) error {
	ver, config, err := versionedRequest[datamodel.GetNodeBootstrapDataRequest](c.Body())
	if err != nil {
		return err
	}

	base := s.mapping.Base(ver)
	if base == "" {
		return fmt.Errorf("could not find agent baker version(%s) in our mapping: %w", ver, err)
	}

	// Re-encode the config to send to agent baker.
	out, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("could not marshal the config to send to agent baker: %w", err)
	}

	return sendToAgentBaker(c, base, out)
}

func (s *Server) latestConfig(c *fiber.Ctx) error {
	ver, config, err := versionedRequest[datamodel.GetLatestSigImageConfigRequest](c.Body())
	if err != nil {
		return err
	}

	base := s.mapping.Base(ver)
	if base == "" {
		return fmt.Errorf("could not find agent baker version(%s) in our mapping: %w", ver, err)
	}

	// Re-encode the config to send to agent baker.
	out, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("could not marshal the config to send to agent baker: %w", err)
	}

	return sendToAgentBaker(c, base, out)
}

func (s *Server) distroConfig(c *fiber.Ctx) error {
	ver, config, err := versionedRequest[datamodel.GetLatestSigImageConfigRequest](c.Body())
	if err != nil {
		return err
	}

	base := s.mapping.Base(ver)
	if base == "" {
		return fmt.Errorf("could not find agent baker version(%s) in our mapping: %w", ver, err)
	}

	// Re-encode the config to send to agent baker.
	out, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("could not marshal the config to send to agent baker: %w", err)
	}

	return sendToAgentBaker(c, base, out)
}
