package main

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/treeder/gotils/v2"
	"golang.org/x/time/rate"
)

type Server struct {
	target  *url.URL
	proxy   *httputil.ReverseProxy
	wsProxy *WebsocketProxy
	myTransport
}

func (cfg *ConfigData) NewServer() (*Server, error) {
	url, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, err
	}
	wsurl, err := url.Parse(cfg.WSURL)
	if err != nil {
		return nil, err
	}
	s := &Server{target: url, proxy: httputil.NewSingleHostReverseProxy(url), wsProxy: NewProxy(wsurl)}
	s.myTransport.requestsPerMinuteLimit = cfg.RPM
	s.myTransport.minGasPrice = cfg.MinGasPrice
	s.myTransport.blockRangeLimit = cfg.BlockRangeLimit
	s.myTransport.url = cfg.URL
	s.matcher, err = newMatcher(cfg.Allow)
	if err != nil {
		return nil, err
	}
	s.visitors = make(map[string]*rate.Limiter)
	s.noLimitIPs = make(map[string]struct{})
	for _, ip := range cfg.NoLimit {
		s.noLimitIPs[ip] = struct{}{}
	}
	s.proxy.Transport = &s.myTransport
	s.wsProxy.Transport = &s.myTransport

	return s, nil
}

func (p *Server) HomePage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	homepage := []byte{}
	if _, err := io.Copy(w, bytes.NewReader(homepage)); err != nil {
		gotils.L(ctx).Error().Printf("Failed to serve homepage: %v", err)
		return
	}
}

func (p *Server) RPCProxy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-rpc-proxy", "rpc-proxy")
	p.proxy.ServeHTTP(w, r)
}

func (p *Server) WSProxy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-rpc-proxy", "rpc-proxy")
	p.wsProxy.ServeHTTP(w, r)
}

func hexAddr(arg string) (interface{}, error) {
	if !common.IsHexAddress(arg) {
		return nil, fmt.Errorf("not a hex address: %s", arg)
	}
	return arg, nil
}

func isHexHash(s string) bool {
	if hasHexPrefix(s) {
		s = s[2:]
	}
	return len(s) == 2*common.HashLength && isHex(s)
}

func hexHash(arg string) (interface{}, error) {
	if !isHexHash(arg) {
		return nil, fmt.Errorf("not a hex hash: %s", arg)
	}
	return arg, nil
}

func boolOrFalse(arg string) (interface{}, error) {
	if arg == "" {
		return false, nil
	}
	v, err := strconv.ParseBool(arg)
	if err != nil {
		return nil, fmt.Errorf("not a bool: %v", err)
	}
	return v, nil
}

func hexNumOrLatest(arg string) (interface{}, error) {
	return hexNumOr(arg, "latest", "pending", "earliest")
}

func hexNumOrZero(arg string) (interface{}, error) {
	return hexNumOr(arg, "0x0")
}

// hexNumOr reforms decimal integers as '0x' prefixed hex and returns
// or for empty, otherwise an error is returned.
func hexNumOr(arg string, or string, allow ...string) (interface{}, error) {
	if arg == "" {
		return or, nil
	}
	for _, a := range allow {
		if arg == a {
			return arg, nil
		}
	}
	i, ok := new(big.Int).SetString(arg, 0)
	if !ok {
		return nil, fmt.Errorf("not an integer: %s", arg)
	}
	return fmt.Sprintf("0x%X", i), nil
}

// hasHexPrefix validates str begins with '0x' or '0X'.
func hasHexPrefix(str string) bool {
	return len(str) >= 2 && str[0] == '0' && (str[1] == 'x' || str[1] == 'X')
}

// isHexCharacter returns bool of c being a valid hexadecimal.
func isHexCharacter(c byte) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}

// isHex validates whether each byte is valid hexadecimal string.
func isHex(str string) bool {
	if len(str)%2 != 0 {
		return false
	}
	for _, c := range []byte(str) {
		if !isHexCharacter(c) {
			return false
		}
	}
	return true
}
