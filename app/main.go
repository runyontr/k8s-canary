package main

import (
	"flag"
	"github.com/go-kit/kit/log"
	"github.com/runyontr/k8s-canary/app/service"
	"github.com/runyontr/k8s-canary/app/transport"
	"github.com/sirupsen/logrus"
	"net"
	"net/http"
	"os"
)

func init() {
	//Log some initial environment variables

	logrus.Infof("Environment Variables:")
	env := os.Environ()
	for _, e := range env {
		logrus.Infof(e)
	}
}

func main() {
	//customizations
	httpAddr := flag.String("http.addr", ":8080", "Address to host server")
	version := flag.Int("version",1,"Version of app")

	flag.Parse()

	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(logrus.StandardLogger().Writer())
		// provides "ts=2017-10-17T20:45:30.175042963Z" with each log
		logger = log.With(logger, "ts", log.DefaultTimestampUTC)

		// provides caller=file:linenumber for each log statement.
		logger = log.With(logger, "caller", log.DefaultCaller)
	}

	infoService, err := service.New(*version)

	if err != nil {
		logrus.Fatalf("Error creating service: %v", err)
	}

	m := transport.MakeInfoServiceHandler(infoService, logger)

	mux := http.NewServeMux()
	mux.Handle("/v1/", m)

	httpListener, err := net.Listen("tcp", *httpAddr)
	if err != nil {
		logrus.Fatalf("Error creating http listener: %v", err)
	}

	logrus.Infof("Starting server @ %v", *httpAddr)
	logrus.Errorf("Server finished running: %v",
		http.Serve(httpListener, mux))
}
