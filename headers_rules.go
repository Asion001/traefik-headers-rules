package traefik_headers_rules

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
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
	logLevel      string
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

	return &headersRules{
		name:          name,
		next:          next,
		requestRules:  reqRules,
		responseRules: resRules,
		logLevel:      config.LogLevel,
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
		matched := r.node.Eval(req, 0, req.Header)
		if h.logLevel == "VERBOSE" {
			os.Stdout.WriteString(fmt.Sprintf("[headers-rules] (Request) plugin=%s rule=%d matched=%v\n", h.name, i, matched))
		}
		if matched {
			if h.logLevel == "DEBUG" || h.logLevel == "VERBOSE" {
				os.Stdout.WriteString(fmt.Sprintf("[headers-rules] (Request) plugin=%s Setting %s: %s\n", h.name, r.setHeader, r.setValue))
			}
			req.Header.Set(r.setHeader, r.setValue)
		}
	}

	if len(h.responseRules) > 0 {
		wrappedWriter := &responseWriter{
			writer:   rw,
			req:      req,
			rules:    h.responseRules,
			logLevel: h.logLevel,
			plugin:   h.name,
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
	logLevel    string
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
			matched := rule.node.Eval(r.req, statusCode, r.writer.Header())
			if r.logLevel == "VERBOSE" {
				os.Stdout.WriteString(fmt.Sprintf("[headers-rules] (Response) plugin=%s rule=%d matched=%v\n", r.plugin, i, matched))
			}
			if matched {
				if r.logLevel == "DEBUG" || r.logLevel == "VERBOSE" {
					os.Stdout.WriteString(fmt.Sprintf("[headers-rules] (Response) plugin=%s Setting %s: %s\n", r.plugin, rule.setHeader, rule.setValue))
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
