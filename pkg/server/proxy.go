package server

import (
	"edgebench/go-load-balancer/pkg/handler"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

// Boilerplate code taken from: https://hackernoon.com/writing-a-reverse-proxy-in-just-one-line-with-go-c1edfa78c84b

type ReverseProxyServer struct {
	handler handler.Handler
}

func NewReverseProxyServer(handler handler.Handler) *ReverseProxyServer {
	return &ReverseProxyServer{
		handler: handler,
	}
}

// Get env var or default
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// Get the port to listen on
func getListenAddress() string {
	port := getEnv("eb_go_lb_listen_port", "8079")
	return ":" + port
}

func (server *ReverseProxyServer) Run() {
	// start server
	http.HandleFunc("/", server.handler.Handle)
	if err := http.ListenAndServe(getListenAddress(), nil); err != nil {
		panic(err)
	}
}

// Serve a reverse proxy for a given url
func serveReverseProxy(target string, res http.ResponseWriter, req *http.Request) {
	// parse the url
	targetUrl, _ := url.Parse(target)

	// create the reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(targetUrl)

	// Update the headers to allow for SSL redirection
	req.URL.Host = targetUrl.Host
	req.URL.Scheme = targetUrl.Scheme
	req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
	req.Host = targetUrl.Host

	// Note that ServeHttp is non blocking and uses a go routine under the hood
	proxy.ServeHTTP(res, req)
}

//// Given a request send it to the appropriate url
//func handleRequestAndRedirect(res http.ResponseWriter, req *http.Request) {
//	requestPayload := parseRequestBody(req)
//	url := getProxyUrl(requestPayload.ProxyCondition)
//
//	logRequestPayload(requestPayload, url)
//
//	serveReverseProxy(url, res, req)
//}
