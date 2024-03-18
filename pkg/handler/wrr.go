package handler

import (
	"errors"
	"math"
	"sync"
)

// Taken from https://github.com/jjnp/traefik/blob/df39dad2e9ebacfca3e7b39df038814dafa98be3/pkg/server/loadbalancer/custom/wrr_provider.go

type WRR struct {
	servers []string
	weights []int
	mtx     sync.Mutex
	Last    int
	cw      int
	max     int
	gcd     int
	n       int
}

func NewWRR(weights Weights) (*WRR, error) {
	wrr := WRR{
		mtx: sync.Mutex{},
	}
	servers := []string{}
	actualWeights := []int{}
	for i := range weights.Ips {
		servers = append(servers, weights.Ips[i])
		actualWeights = append(actualWeights, weights.Weights[i])
	}

	max, errmax := max(actualWeights)
	gcd, errgcd := gcd(actualWeights)
	if errmax != nil || errgcd != nil {
		return &wrr, errors.New("error calculating initial values for wrr")
	}
	wrr.max = max
	wrr.gcd = gcd
	wrr.cw = 0
	wrr.Last = -1
	wrr.servers = servers
	wrr.weights = actualWeights
	wrr.n = len(servers)
	return &wrr, nil
}

func NewWRRWithLast(weights Weights, last int) (*WRR, error) {
	wrr := WRR{
		mtx: sync.Mutex{},
	}
	servers := []string{}
	actualWeights := []int{}
	for i := range weights.Ips {
		servers = append(servers, weights.Ips[i])
		actualWeights = append(actualWeights, weights.Weights[i])
	}

	max, errmax := max(actualWeights)
	gcd, errgcd := gcd(actualWeights)
	if errmax != nil || errgcd != nil {
		return &wrr, errors.New("error calculating initial values for wrr")
	}
	wrr.max = max
	wrr.gcd = gcd
	wrr.cw = 0
	wrr.Last = last
	wrr.servers = servers
	wrr.weights = actualWeights
	wrr.n = len(servers)
	return &wrr, nil
}

func gcd(ns []int) (int, error) {
	max_possible, err := min(ns)
	if err != nil {
		return -1, err
	}
	gcd := 1
	// We move downward, because this way we can potentially break out of the loop earlier
	// I benchmarked it and it's about twice as fast
	for i := max_possible; i >= 1; i-- {
		valid := true
		for _, n := range ns {
			if n%i != 0 {
				valid = false
				break
			}
		}
		if valid && i > 1 {
			gcd = i
			break
		}
	}
	return gcd, nil
}

func min(ns []int) (int, error) {
	if ns == nil {
		return -1, errors.New("cannot calculate min of nil or empty slice")
	}
	min := math.MaxInt
	for _, cur := range ns {
		if cur < min {
			min = cur
		}
	}
	return min, nil
}

func max(ns []int) (int, error) {
	if ns == nil {
		return -1, errors.New("cannot calculate max of nil or empty slice")
	}
	max := 0
	for _, cur := range ns {
		if cur > max {
			max = cur
		}
	}
	return max, nil
}

func (w *WRR) Next() string {
	w.mtx.Lock()
	defer w.mtx.Unlock()
	for true {
		w.Last = (w.Last + 1) % w.n
		if w.Last == 0 {
			w.cw -= w.gcd
			if w.cw <= 0 {
				w.cw = w.max
			}
		}
		if w.weights[w.Last] >= w.cw {
			return w.servers[w.Last]
		}
	}
	panic("reached a theoretically unreachable state trying to calculate the next server in WRR.Next() method")
}
