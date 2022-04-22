package handler

import (
	"edgebench/go-load-balancer/pkg/env"
	"fmt"
	"go.uber.org/zap"
)

func NewHandler(handlerType HandlerType, functionState *FunctionState) Handler {
	switch handlerType {
	case Dummy:
		return NewDummyHandler()
	case WeightedRR:
		gateways, err := readGatewaysFromEnv()
		if err != nil {
			panic(fmt.Sprintf("error looking up gateways: %s", err))
		}
		zap.S().Info("Read gateways: ")
		return NewWeightedRoundRobinHandler(gateways, functionState)
	default:
		err := fmt.Sprintf("error unknown handlertype: %d", handlerType)
		panic(err)
	}

}

func readGatewaysFromEnv() (map[string]bool, error) {
	fields, found, err := env.OsEnv.LookupFields("eb_go_lb_gateways")
	if !found {
		return make(map[string]bool), nil
	}

	if err != nil {
		return nil, err
	}

	gateways := make(map[string]bool)
	for _, field := range fields {
		gateways[field] = true
	}
	return gateways, nil
}
