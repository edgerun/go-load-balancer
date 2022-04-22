package util

import (
	"edgebench/go-load-balancer/pkg/env"
	"fmt"
	"go.uber.org/zap"
	"log"
	"os"
	"strconv"
	"time"
)

func createProdLogger() (*zap.Logger, error) {
	// code based on https://github.com/uber-go/zap/issues/586
	os.Mkdir("logs", os.ModePerm)

	config := zap.NewProductionConfig()
	year, month, day := time.Now().Date()
	path := "logs/" + strconv.Itoa(year) + "-" + strconv.Itoa(int(month)) + "-" + strconv.Itoa(day) + ".json"
	config.OutputPaths = []string{
		fmt.Sprintf(path),
		"stdout",
	}

	config.EncoderConfig.LevelKey = "level"
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.CallerKey = "caller"
	config.EncoderConfig.MessageKey = "message"

	return config.Build()
}

func InitZapLogger() *zap.SugaredLogger {
	mode, ok := env.OsEnv.Lookup("eb_go_lb_mode")
	text := ""
	if !ok {
		mode = "dev"
		text = "no eb_go_lb_mode set, default to dev"
	}

	var plainlogger *zap.Logger
	var logger *zap.SugaredLogger
	var err error
	if mode == "prod" {
		plainlogger, err = createProdLogger()
		text = "prod logger set"
	} else if mode == "dev" {
		plainlogger, err = zap.NewDevelopment()
		text = "dev logger set"
	} else {
		plainlogger, err = zap.NewDevelopment()
		text = fmt.Sprintf("unknown eb_go_lb_mode '%s'. default to dev", mode)
	}

	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	logger = plainlogger.Sugar()
	zap.ReplaceGlobals(plainlogger)
	logger.Info(text)
	return logger
}
