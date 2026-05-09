package traefik_headers_rules

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
)

// Rule holds configuration for a conditional header application.
type Rule struct {
	Expression string `json:"expression,omitempty"`
	SetHeader  string `json:"setHeader,omitempty"`
	SetValue   string `json:"setValue,omitempty"`
}

// Config holds the plugin configuration.
type Config struct {
	RequestRules  []Rule `json:"requestRules,omitempty"`
	ResponseRules []Rule `json:"responseRules,omitempty"`
	LogLevel      string `json:"logLevel,omitempty"`
}

// CreateConfig creates and initializes the plugin configuration.
func CreateConfig() *Config {
	return &Config{
		RequestRules:  make([]Rule, 0),
		ResponseRules: make([]Rule, 0),
		LogLevel:      "INFO",
	}
}

type rule struct {
	node      Node
	setHeader string
	setValue  string
}

type headersRules struct {
	name          string
	next          http.Handler
	requestRules  []rule
	responseRules []rule
	debug         bool
}

// New creates and returns a new middleware plugin instance.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	reqRules, err := compileRules(config.RequestRules)
	if err != nil {
		return nil, fmt.Errorf("error compiling request rules: %w", err)
	}
	resRules, err := compileRules(config.ResponseRules)
	if err != nil {
		return nil, fmt.Errorf("error compiling response rules: %w", err)
	}

	debug := false
	if config.LogLevel == "DEBUG" {
		debug = true
	}

	return &headersRules{
		name:          name,
		next:          next,
		requestRules:  reqRules,
		responseRules: resRules,
		debug:         debug,
	}, nil
}

func compileRules(cfgRules []Rule) ([]rule, error) {
	var compiled []rule
	for _, r := range cfgRules {
		node, err := parseExpression(r.Expression)
		if err != nil {
			return nil, fmt.Errorf("error parsing expression %q: %w", r.Expression, err)
		}

		compiled = append(compiled, rule{
			node:      node,
			setHeader: r.SetHeader,
			setValue:  r.SetValue,
		})
	}
	return compiled, nil
}

func (h *headersRules) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Apply Request Rules (status is 0 since it's not applicable)
	for i, r := range h.requestRules {
		if r.node.Eval(req, 0, req.Header) {
			if h.debug {
				fmt.Printf("[headers-rules] (Request) plugin=%s rule=%d matched! Setting %s: %s\n", h.name, i, r.setHeader, r.setValue)
			}
			req.Header.Set(r.setHeader, r.setValue)
		}
	}

	if len(h.responseRules) > 0 {
		wrappedWriter := &responseWriter{
			writer: rw,
			req:    req,
			rules:  h.responseRules,
			debug:  h.debug,
			plugin: h.name,
		}
		h.next.ServeHTTP(wrappedWriter, req)
	} else {
		h.next.ServeHTTP(rw, req)
	}
}

type responseWriter struct {
	writer      http.ResponseWriter
	req         *http.Request
	rules       []rule
	wroteHeader bool
	debug       bool
	plugin      string
}

func (r *responseWriter) Header() http.Header {
	return r.writer.Header()
}

func (r *responseWriter) Write(bytes []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.writer.Write(bytes)
}

func (r *responseWriter) WriteHeader(statusCode int) {
	if !r.wroteHeader {
		for i, rule := range r.rules {
			if rule.node.Eval(r.req, statusCode, r.writer.Header()) {
				if r.debug {
					fmt.Printf("[headers-rules] (Response) plugin=%s rule=%d matched! Setting %s: %s\n", r.plugin, i, rule.setHeader, rule.setValue)
				}
				r.writer.Header().Set(rule.setHeader, rule.setValue)
			}
		}
		r.wroteHeader = true
		r.writer.WriteHeader(statusCode)
	}
}

func (r *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.writer.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("%T is not a http.Hijacker", r.writer)
	}
	return hijacker.Hijack()
}

func (r *responseWriter) Flush() {
	if flusher, ok := r.writer.(http.Flusher); ok {
		flusher.Flush()
	}
}
