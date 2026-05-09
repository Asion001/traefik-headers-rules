package traefik_headers_rules

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"regexp"
)

// Rule holds configuration for a conditional header application.
type Rule struct {
	CheckHeader string `json:"checkHeader,omitempty"`
	CheckRegex  string `json:"checkRegex,omitempty"`
	CheckMethod string `json:"checkMethod,omitempty"`
	CheckPath   string `json:"checkPath,omitempty"`
	CheckStatus int    `json:"checkStatus,omitempty"`
	SetHeader   string `json:"setHeader,omitempty"`
	SetValue    string `json:"setValue,omitempty"`
}

// Config holds the plugin configuration.
type Config struct {
	RequestRules  []Rule `json:"requestRules,omitempty"`
	ResponseRules []Rule `json:"responseRules,omitempty"`
}

// CreateConfig creates and initializes the plugin configuration.
func CreateConfig() *Config {
	return &Config{
		RequestRules:  make([]Rule, 0),
		ResponseRules: make([]Rule, 0),
	}
}

type rule struct {
	checkHeader string
	checkRegex  *regexp.Regexp
	checkMethod string
	checkPath   *regexp.Regexp
	checkStatus int
	setHeader   string
	setValue    string
}

type headersRules struct {
	name          string
	next          http.Handler
	requestRules  []rule
	responseRules []rule
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
	}, nil
}

func compileRules(cfgRules []Rule) ([]rule, error) {
	var compiled []rule
	for _, r := range cfgRules {
		var headerRegex *regexp.Regexp
		var pathRegex *regexp.Regexp
		var err error

		if r.CheckRegex != "" {
			headerRegex, err = regexp.Compile(r.CheckRegex)
			if err != nil {
				return nil, fmt.Errorf("error compiling checkRegex %q: %w", r.CheckRegex, err)
			}
		}

		if r.CheckPath != "" {
			pathRegex, err = regexp.Compile(r.CheckPath)
			if err != nil {
				return nil, fmt.Errorf("error compiling checkPath %q: %w", r.CheckPath, err)
			}
		}

		compiled = append(compiled, rule{
			checkHeader: r.CheckHeader,
			checkRegex:  headerRegex,
			checkMethod: r.CheckMethod,
			checkPath:   pathRegex,
			checkStatus: r.CheckStatus,
			setHeader:   r.SetHeader,
			setValue:    r.SetValue,
		})
	}
	return compiled, nil
}

func matchRule(r rule, req *http.Request, status int, headers http.Header) bool {
	if r.checkStatus > 0 && status != r.checkStatus {
		return false
	}
	if r.checkMethod != "" && req.Method != r.checkMethod {
		return false
	}
	if r.checkPath != nil && !r.checkPath.MatchString(req.URL.Path) {
		return false
	}
	if r.checkHeader != "" {
		values := headers.Values(r.checkHeader)
		if len(values) == 0 {
			return false
		}
		if r.checkRegex != nil {
			matched := false
			for _, v := range values {
				if r.checkRegex.MatchString(v) {
					matched = true
					break
				}
			}
			if !matched {
				return false
			}
		}
	}
	return true
}

func (h *headersRules) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Apply Request Rules (status is 0 since it's not applicable)
	for _, r := range h.requestRules {
		if matchRule(r, req, 0, req.Header) {
			req.Header.Set(r.setHeader, r.setValue)
		}
	}

	if len(h.responseRules) > 0 {
		wrappedWriter := &responseWriter{
			writer: rw,
			req:    req,
			rules:  h.responseRules,
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
		for _, rule := range r.rules {
			if matchRule(rule, r.req, statusCode, r.writer.Header()) {
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
