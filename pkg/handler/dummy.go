package handler

import (
	"fmt"
	"go.uber.org/zap"
	"net/http"
)

type DummyHandler struct {
}

func NewDummyHandler() *DummyHandler {
	return &DummyHandler{}
}

func (DummyHandler) selectIp(req *http.Request) (string, error) {
	return "ip", nil
}

func (DummyHandler) Handle(res http.ResponseWriter, req *http.Request) {
	zap.S().Info("Received request")
	var urlHost = fmt.Sprintf("URL: %s, Host: %s", req.URL, req.Host)
	zap.S().Info(urlHost)
	res.WriteHeader(200)
	res.Write([]byte(fmt.Sprintf("Dummy response, %s", urlHost)))
}

func (DummyHandler) HandleWeightUpdate(update *WeightUpdate) {
	zap.S().Info("got weight update: %s - ips: %s, weights: ", update.Function, update.Weights.Ips, update.Weights.Weights)
}
