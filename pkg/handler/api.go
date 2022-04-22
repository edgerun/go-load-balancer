package handler

import (
	"fmt"
	"net/http"
	"strings"
)

type Handler interface {
	Handle(res http.ResponseWriter, req *http.Request)
	HandleWeightUpdate(update *WeightUpdate)
}

type HandlerType int

const (
	Dummy HandlerType = iota
	WeightedRR
)

func NewHandlerType(handlerType string) HandlerType {
	switch strings.ToLower(handlerType) {
	case "dummy":
		return Dummy
	case "wrr":
		return WeightedRR
	default:
		err := fmt.Sprintf("Error unknown handlertype: %s", handlerType)
		panic(err)
	}
}

type WeightUpdate struct {
	Zone     string
	Function string
	Weights  Weights
}

type Weights struct {
	Ips     []string `json:"ips"`
	Weights []int    `json:"weights"`
}
