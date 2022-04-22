package main

import (
	"edgebench/go-load-balancer/pkg/env"
	"edgebench/go-load-balancer/pkg/handler"
	"edgebench/go-load-balancer/pkg/server"
	"edgebench/go-load-balancer/pkg/util"
	"fmt"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	logger := util.InitZapLogger()
	defer logger.Sync()

	startReverseProxy()

	waitForSignal()
}

func waitForSignal() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	zap.S().Info("received stop")
}

func startReverseProxy() {
	zone, ok := env.OsEnv.Lookup("eb_go_lb_zone")
	if ok {
		zap.S().Info("Start go-load-balancer in zone ", zone)
	} else {
		panic("No zone provided")
	}

	var handlerType handler.HandlerType
	zap.S().Info("read HANDLER_TYPE")
	if val, ok := env.OsEnv.Lookup("eb_go_lb_handler_type"); ok {
		zap.S().Info("instantiate '", val, "' handler")
		handlerType = handler.NewHandlerType(val)
	} else {
		zap.S().Info("no eb_go_lb_handler_type provided, default to 'dummy'")
		handlerType = handler.Dummy
	}

	functionState := handler.NewFunctionState(zone)
	handler.UpdateFunctionState(functionState)
	handlerImpl := handler.NewHandler(handlerType, functionState)
	go func() {
		server.NewReverseProxyServer(handlerImpl).Run()
	}()

	go func() {
		etcdClient, err := handler.NewEtcdClientFromEnv()
		if err != nil {
			panic(err)
		}
		weightUpdater := handler.NewEtcdWeightUpdater(etcdClient)
		ch := weightUpdater.GetUpdates(fmt.Sprintf("golb/function/%s", zone))
		for ev := range ch {
			zap.S().Debug(ev)
			handlerImpl.HandleWeightUpdate(ev)
			zap.S().Debug("updated")
		}

	}()
}
