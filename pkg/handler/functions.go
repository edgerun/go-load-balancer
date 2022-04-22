package handler

import "go.uber.org/zap"

type FunctionState struct {
	Zone      string
	Functions map[string]Weights
}

func NewFunctionState(zone string) *FunctionState {
	return &FunctionState{
		Zone:      zone,
		Functions: make(map[string]Weights),
	}
}

type EtcdFunctionStateManager struct {
	FunctionState *FunctionState
}

func NewEtcdFunctionStateManager(functionState *FunctionState) *EtcdFunctionStateManager {
	// TODO load all available weights from etcd at startup
	return &EtcdFunctionStateManager{
		functionState,
	}
}

func (manager *EtcdFunctionStateManager) HandleWeightUpdate(update *WeightUpdate) {
	state := manager.FunctionState
	state.Functions[update.Function] = update.Weights
}

func UpdateFunctionState(state *FunctionState) {
	etcdClient, err := NewEtcdClientFromEnv()
	if err != nil {
		panic(err)
	}
	freshState, err := etcdClient.GetFunctionState(state.Zone)
	if err != nil {
		zap.S().Error(err)
		return
	}

	if freshState == nil {
		zap.S().Debugf("No state found for zone %s", state.Zone)
		return
	}

	for function, _ := range freshState.Functions {
		state.Functions[function] = freshState.Functions[function]
	}

	zap.S().Info("Updated Functionstate: ", state)
}
