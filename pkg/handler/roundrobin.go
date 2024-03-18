package handler

import (
	"context"
	"edgebench/go-load-balancer/pkg/env"
	"encoding/json"
	"errors"
	"fmt"
	"go.etcd.io/etcd/mvcc/mvccpb"
	"go.uber.org/zap"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)
import "go.etcd.io/etcd/clientv3"

func NewWeightUpdate(function string, weights Weights) *WeightUpdate {
	return &WeightUpdate{
		Function: function,
		Weights:  weights,
	}
}

type EtcdClient struct {
	Client *clientv3.Client
}

func (c EtcdClient) GetWithPrefix(pattern string) (*clientv3.GetResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	etcdCli := c.Client
	return etcdCli.Get(ctx, pattern, clientv3.WithPrefix())
}

func (c EtcdClient) GetFunctionState(zone string) (*FunctionState, error) {
	gresp, err := c.GetWithPrefix(fmt.Sprintf("golb/function/%s/", zone))
	if err != nil {
		panic(err)
	}
	zap.S().Debug(gresp)
	state := NewFunctionState(zone)
	for _, kv := range gresp.Kvs {
		function, err := parseEtcdFunctionKey(string(kv.Key))
		if err != nil {
			zap.S().Error(err)
			continue
		}
		weights, err := parseWeights(string(kv.Value))
		if err != nil {
			zap.S().Error(err)
			continue
		}

		state.Functions[function] = weights
	}
	return state, nil
}

func NewEtcdClient(url string) (*EtcdClient, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{url},
		DialTimeout: 1 * time.Second,
	})
	if err != nil {
		return nil, err
	} else {
		return &EtcdClient{
			Client: cli,
		}, nil
	}
}

func NewEtcdClientFromEnv() (*EtcdClient, error) {
	etcdUrl, found := env.OsEnv.Lookup("eb_go_lb_etcd_host")
	if !found {
		etcdUrl = "localhost:2379"
	}
	zap.S().Infow("Connect to etcd", "etcdUrl", etcdUrl)
	return NewEtcdClient(etcdUrl)
}

type WeightUpdater interface {
	GetUpdates(string) chan *WeightUpdate
}

type EtcdWeightUpdater struct {
	etcdClient *EtcdClient
	Zone       string
}

func parseWeights(msg string) (Weights, error) {
	weights := Weights{}
	err := json.Unmarshal([]byte(msg), &weights)
	if err != nil {
		return Weights{}, err
	}
	return weights, nil
}

func parseEtcdFunctionKey(key string) (string, error) {
	split := strings.Split(key, "/")
	if len(split) != 4 {
		return "", errors.New(fmt.Sprintf("key does not adhere to format 'golb/function/<zone>/<function>': %s", key))
	}
	return split[3], nil
}

func (updater EtcdWeightUpdater) GetUpdates(key string) chan *WeightUpdate {
	updates := make(chan *WeightUpdate)
	go func() {

		//updates <- NewWeightUpdate("nginx", []string{"10.0.0.1"}, []float64{0.3})
		ch := updater.etcdClient.Client.Watch(context.TODO(), key, clientv3.WithPrefix())
		for resp := range ch {
			for _, event := range resp.Events {
				switch event.Type {
				case mvccpb.PUT:
					function, err := parseEtcdFunctionKey(string(event.Kv.Key))
					if err != nil {
						zap.S().Error(err)
					}
					weights, err := parseWeights(string(event.Kv.Value))
					if err != nil {
						zap.S().Errorf("error parsing weight update: %s", err)
						continue
					}
					zap.S().Debugw("put", "function", function, "weights", weights)
					updates <- NewWeightUpdate(function, weights)
				}
			}
		}
	}()
	return updates
}

func NewEtcdWeightUpdater(client *EtcdClient) WeightUpdater {
	return EtcdWeightUpdater{
		etcdClient: client,
	}
}

type WeightedRoundRobinHandler struct {
	functionState              *FunctionState
	wrrInstancesWithoutGateway map[string]*WRR
	wrrInstances               map[string]*WRR
	ForwardMessage             string
	NodeName                   string
	Gateways                   map[string]bool
}

func (handler *WeightedRoundRobinHandler) HandleWeightUpdate(update *WeightUpdate) {
	zap.S().Debugf("WRR - Got weight update: %s - ips: %s, weights: %s", update.Function, update.Weights.Ips, update.Weights.Weights)
	state := handler.functionState
	state.Functions[update.Function] = update.Weights

	wrr, err := NewWRR(update.Weights)
	val, ok := handler.wrrInstances[update.Function]

	if ok {
		wrr, err = NewWRRWithLast(update.Weights, val.Last)
	}

	if err != nil {
		zap.S().Error(err)
	} else {
		handler.wrrInstances[update.Function] = wrr

		wrrWithoutGateway, _ := NewWRR(filterGateway(update.Weights))
		val, ok := handler.wrrInstancesWithoutGateway[update.Function]
		if ok {
			wrrWithoutGateway, _ = NewWRRWithLast(filterGateway(update.Weights), val.Last)
		}
		handler.wrrInstancesWithoutGateway[update.Function] = wrrWithoutGateway
	}
}

func removeWeight(slice []int, s int) []int {
	return append(slice[:s], slice[s+1:]...)
}

func removeIp(slice []string, s int) []string {
	return append(slice[:s], slice[s+1:]...)
}

func filterGateway(weights Weights) Weights {
	index := -1
	for i, ip := range weights.Ips {
		if strings.Contains(ip, "10.0.") {
			index = i
			break
		}
	}

	if index != -1 {
		return Weights{
			Ips:     removeIp(weights.Ips, index),
			Weights: removeWeight(weights.Weights, index),
		}
	} else {
		return weights
	}

}

func NewWeightedRoundRobinHandler(gateways map[string]bool, functionState *FunctionState) *WeightedRoundRobinHandler {
	wrrInstances := make(map[string]*WRR)
	wrrInstancesWithoutGateway := make(map[string]*WRR)
	for function, weights := range functionState.Functions {
		wrr, err := NewWRR(weights)
		if err != nil {
			zap.S().Info(err)
			continue
		}
		wrrWithoutGateway, _ := NewWRR(filterGateway(weights))
		wrrInstances[function] = wrr
		wrrInstancesWithoutGateway[function] = wrrWithoutGateway
	}

	nodeName, found := env.OsEnv.Lookup("eb_go_lb_node_name")
	if !found {
		nodeName = env.OsEnv.Get("HOSTNAME")
	}
	return &WeightedRoundRobinHandler{
		functionState:              functionState,
		wrrInstances:               wrrInstances,
		wrrInstancesWithoutGateway: wrrInstancesWithoutGateway,
		ForwardMessage:             fmt.Sprintf("X-Forwarded-Host-%s", nodeName),
		NodeName:                   nodeName,
		Gateways:                   gateways,
	}
}

func (handler *WeightedRoundRobinHandler) selectIp(req *http.Request) (string, error) {
	// Algorithm based on http://kb.linuxvirtualserver.org/wiki/Weighted_Round-Robin_Scheduling

	// parse url for function
	// localhost:8080/function/nginx
	// localhost:8080/function/responder/static?time=10
	uri := req.URL.RequestURI()
	split := strings.Split(uri, "/")
	if len(split) != 3 && len(split) != 4 {
		return "", fmt.Errorf("invalid URL format: %s\n", uri)
	}

	if split[1] != "function" {
		return "", fmt.Errorf("not a function call: %s\n", uri)
	}

	function := split[2]

	// get weights
	var wrr *WRR
	var found bool
	forwarded := len(req.Header.Get("X-Forwarded-For")) >= 1
	if !forwarded {
		wrr, found = handler.wrrInstances[function]
		if !found {
			return "", fmt.Errorf("no servers found for function: %s", function)
		}
	} else {
		wrr, found = handler.wrrInstancesWithoutGateway[function]
		if !found {
			return "", fmt.Errorf("no servers found for function: %s", function)
		}
	}

	ip := wrr.Next()

	return ip, nil
}

func (handler *WeightedRoundRobinHandler) Handle(res http.ResponseWriter, req *http.Request) {
	var target string
	ip, err := handler.selectIp(req)
	if err == nil {
		target = fmt.Sprintf("http://%s", ip)
	} else {
		text := fmt.Sprintf("error selecting server: %s - in %s", err, handler.functionState.Zone)
		zap.S().Info(text)
		res.WriteHeader(404)
		res.Write([]byte(text))
		return
	}
	parsedUrl, _ := url.Parse(target)

	req.Header.Set("X-Forwarded-For", handler.NodeName)
	res.Header().Set("X-Forwarded-For", handler.NodeName)

	// Update the headers to allow for SSL redirection
	req.URL.Host = parsedUrl.Host

	if _, found := handler.Gateways[parsedUrl.Host]; !found {
		uri := req.URL.RequestURI()
		split := strings.Split(uri, "/")
		params := ""
		if len(split) == 4 {
			params = split[3]
		}
		target = fmt.Sprintf("http://%s/%s", parsedUrl.Host, params)
		parsedUrl2, _ := url.Parse(target)

		req.URL = parsedUrl2

		req.RequestURI = target
	} else {
		// ignore
	}
	proxy := httputil.NewSingleHostReverseProxy(parsedUrl)
	//req.URL.Scheme = parsedUrl.Scheme
	req.Header.Set(handler.ForwardMessage, fmt.Sprintf("%.7f", float64(time.Now().UnixNano())/float64(1000000000)))
	res.Header().Set(handler.ForwardMessage, fmt.Sprintf("%.7f", float64(time.Now().UnixNano())/float64(1000000000)))
	res.Header().Set("X-Final-Host", parsedUrl.Host)
	req.Header.Set("X-Final-Host", parsedUrl.Host)
	req.Host = parsedUrl.Host
	zap.S().Debug("direct request ", req.RequestURI, " to ", target)
	// Note that ServeHttp is non blocking and uses a go routine under the hood
	proxy.ServeHTTP(res, req)
}
