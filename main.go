package main

import (
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"time"
)

var (
	RMaps        RouteMapController
	logger       *slog.Logger
	listenport   string
	configString = flag.String("c", "", `json config string Example: '[ {"route": "myhostname.domain", "deployment-prefix": "cf", "job": "router"}, {"route": "myhostname2.domain", "deployment-prefix": "tanzu-hub", "job": "controler"} ]'`)
	configFile   = flag.String("f", "", "json config file")
	boshClient   = flag.String("client", "ops_manager", "client id for bosh director oauth")
	boshSecret   = flag.String("secret", "", "secret for bosh director client oauth")
	boshHost     = flag.String("host", "", "bosh director hostname or IP")
	debug        = flag.Bool("d", false, "enable debug logging")

	BRPDefaultTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 30 * time.Second,
			//KeepAlive: 30 * time.Second,
		}).Dial,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		TLSHandshakeTimeout: 10 * time.Second,
		//DisableKeepAlives:   true,
	}
)

func init() {
	listenport = os.Getenv("PORT")
	if listenport == "" {
		listenport = "8080"
	}
}

func main() {
	flag.Parse()
	lvl := new(slog.LevelVar)
	lvl.Set(slog.LevelInfo)

	logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level:     lvl,
		AddSource: true,
	}))

	logger.Debug("debug logging disabled")
	if *debug {
		lvl.Set(slog.LevelDebug)
		logger.Debug("debug logging enabled", "a", "b")
	}

	logger.Info("Building route maps", "configFile", *configFile, "configString", *configString)
	var err error
	if *configString != "" {
		RMaps, err = LoadRouteMapsFromString(*configString)
	} else if *configFile != "" {
		RMaps, err = LoadRouteMapsFromFile(*configFile)
	} else {
		logger.Error("no config specified")
		os.Exit(1)
	}
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	go RMaps.RouterSyncer(*boshClient, *boshSecret, *boshHost)

	logger.Info("starting http server")
	proxy := &httputil.ReverseProxy{
		Transport: roundTripper(rt),
		Director:  RMaps.RouteMapDirector,
	}
	logger.Error("shutting down server", "error", http.ListenAndServe(fmt.Sprintf(":%s", listenport), proxy))
}

func rt(req *http.Request) (*http.Response, error) {
	logger.Debug("request received", "url", req.URL)
	//req.Header.Set("Host", backend)
	defer logger.Debug("request complete", "url", req.URL)

	var InsecureTransport http.RoundTripper = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 30 * time.Second,
			//KeepAlive: 30 * time.Second,
		}).Dial,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         req.Host,
		},
		TLSHandshakeTimeout: 10 * time.Second,
		//DisableKeepAlives:   true,
	}

	if req.ContentLength > 0 {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			logger.Error("failed to read body", "error", err)
			return &http.Response{}, err
		}
		rdr := io.NopCloser(bytes.NewBuffer(b))
		//fmt.Printf("%s\n", b)
		req.Body = rdr
	}
	return InsecureTransport.RoundTrip(req)
	//return http.DefaultTransport.RoundTrip(req)
}

// roundTripper makes func signature a http.RoundTripper
type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
