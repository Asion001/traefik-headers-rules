package traefik_headers_rules

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type Node interface {
	Eval(req *http.Request, status int, headers http.Header) bool
}

type andNode struct {
	left  Node
	right Node
}

func (n *andNode) Eval(req *http.Request, status int, headers http.Header) bool {
	return n.left.Eval(req, status, headers) && n.right.Eval(req, status, headers)
}

type orNode struct {
	left  Node
	right Node
}

func (n *orNode) Eval(req *http.Request, status int, headers http.Header) bool {
	return n.left.Eval(req, status, headers) || n.right.Eval(req, status, headers)
}

type notNode struct {
	node Node
}

func (n *notNode) Eval(req *http.Request, status int, headers http.Header) bool {
	return !n.node.Eval(req, status, headers)
}

type methodNode struct {
	methods []string
}

func (n *methodNode) Eval(req *http.Request, status int, headers http.Header) bool {
	for _, m := range n.methods {
		if req.Method == m {
			return true
		}
	}
	return false
}

type pathNode struct {
	regexps []*regexp.Regexp
}

func (n *pathNode) Eval(req *http.Request, status int, headers http.Header) bool {
	for _, r := range n.regexps {
		if r.MatchString(req.URL.Path) {
			return true
		}
	}
	return false
}

type headerNode struct {
	name    string
	regexps []*regexp.Regexp
}

func (n *headerNode) Eval(req *http.Request, status int, headers http.Header) bool {
	values := headers.Values(n.name)
	if len(values) == 0 {
		return false
	}
	for _, v := range values {
		for _, r := range n.regexps {
			if r.MatchString(v) {
				return true
			}
		}
	}
	return false
}

type statusNode struct {
	statuses []int
}

func (n *statusNode) Eval(req *http.Request, status int, headers http.Header) bool {
	if status == 0 {
		return false
	}
	for _, s := range n.statuses {
		if status == s {
			return true
		}
	}
	return false
}

// Minimal Parser

type tokenType int

const (
	tokEOF tokenType = iota
	tokAnd
	tokOr
	tokNot
	tokLParen
	tokRParen
	tokComma
	tokIdent
	tokString
)

type token struct {
	typ tokenType
	val string
}

func lex(input string) ([]token, error) {
	var tokens []token
	i := 0
	for i < len(input) {
		c := input[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '&' && i+1 < len(input) && input[i+1] == '&':
			tokens = append(tokens, token{typ: tokAnd})
			i += 2
		case c == '|' && i+1 < len(input) && input[i+1] == '|':
			tokens = append(tokens, token{typ: tokOr})
			i += 2
		case c == '!':
			tokens = append(tokens, token{typ: tokNot})
			i++
		case c == '(':
			tokens = append(tokens, token{typ: tokLParen})
			i++
		case c == ')':
			tokens = append(tokens, token{typ: tokRParen})
			i++
		case c == ',':
			tokens = append(tokens, token{typ: tokComma})
			i++
		case c == '`' || c == '"' || c == '\'':
			quote := c
			i++
			start := i
			for i < len(input) && input[i] != quote {
				i++
			}
			if i >= len(input) {
				return nil, fmt.Errorf("unclosed string literal")
			}
			tokens = append(tokens, token{typ: tokString, val: input[start:i]})
			i++
		default:
			start := i
			for i < len(input) && isAlphaNum(input[i]) {
				i++
			}
			if start == i {
				return nil, fmt.Errorf("unexpected character: %c", c)
			}
			tokens = append(tokens, token{typ: tokIdent, val: input[start:i]})
		}
	}
	tokens = append(tokens, token{typ: tokEOF})
	return tokens, nil
}

func isAlphaNum(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_'
}

type parser struct {
	tokens []token
	pos    int
}

func parseExpression(input string) (Node, error) {
	tokens, err := lex(input)
	if err != nil {
		return nil, err
	}
	p := &parser{tokens: tokens}
	node, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.current().typ != tokEOF {
		return nil, fmt.Errorf("unexpected token at end: %v", p.current().val)
	}
	return node, nil
}

func (p *parser) current() token {
	if p.pos >= len(p.tokens) {
		return token{typ: tokEOF}
	}
	return p.tokens[p.pos]
}

func (p *parser) advance() {
	p.pos++
}

func (p *parser) parseOr() (Node, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.current().typ == tokOr {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &orNode{left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseAnd() (Node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.current().typ == tokAnd {
		p.advance()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &andNode{left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseUnary() (Node, error) {
	if p.current().typ == tokNot {
		p.advance()
		node, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &notNode{node: node}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (Node, error) {
	tok := p.current()
	if tok.typ == tokLParen {
		p.advance() // consume (
		node, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.current().typ != tokRParen {
			return nil, fmt.Errorf("expected ')', got %v", p.current().typ)
		}
		p.advance() // consume )
		return node, nil
	}

	if tok.typ == tokIdent {
		return p.parseFuncCall()
	}

	return nil, fmt.Errorf("unexpected token in expression: %v", tok.val)
}

func (p *parser) parseFuncCall() (Node, error) {
	funcName := p.current().val
	p.advance() // consume ident

	if p.current().typ != tokLParen {
		return nil, fmt.Errorf("expected '(' after function name %s", funcName)
	}
	p.advance() // consume (

	var args []string
	for p.current().typ != tokRParen && p.current().typ != tokEOF {
		if p.current().typ != tokString {
			return nil, fmt.Errorf("expected string argument")
		}
		args = append(args, p.current().val)
		p.advance()

		if p.current().typ == tokComma {
			p.advance()
		} else if p.current().typ != tokRParen {
			return nil, fmt.Errorf("expected ',' or ')'")
		}
	}

	if p.current().typ != tokRParen {
		return nil, fmt.Errorf("expected ')' closing %s", funcName)
	}
	p.advance() // consume )

	switch strings.ToLower(funcName) {
	case "method":
		return &methodNode{methods: args}, nil
	case "path":
		var regexps []*regexp.Regexp
		for _, arg := range args {
			re, err := regexp.Compile(arg)
			if err != nil {
				return nil, fmt.Errorf("invalid path regex %q: %v", arg, err)
			}
			regexps = append(regexps, re)
		}
		return &pathNode{regexps: regexps}, nil
	case "header":
		if len(args) < 2 {
			return nil, fmt.Errorf("Header() requires at least 2 arguments: name and match regex")
		}
		var regexps []*regexp.Regexp
		for _, arg := range args[1:] {
			re, err := regexp.Compile(arg)
			if err != nil {
				return nil, fmt.Errorf("invalid header regex %q: %v", arg, err)
			}
			regexps = append(regexps, re)
		}
		return &headerNode{name: args[0], regexps: regexps}, nil
	case "status":
		var statuses []int
		for _, arg := range args {
			s, err := strconv.Atoi(arg)
			if err != nil {
				return nil, fmt.Errorf("invalid status code %q: %v", arg, err)
			}
			statuses = append(statuses, s)
		}
		return &statusNode{statuses: statuses}, nil
	default:
		return nil, fmt.Errorf("unknown function: %s", funcName)
	}
}
