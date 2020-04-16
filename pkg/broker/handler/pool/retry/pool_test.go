/*
Copyright 2020 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package retry

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/config/memory"
	"github.com/google/knative-gcp/pkg/broker/handler/pool"
)

// TODO could reuse some fanout UT test function here. Needing a semi e2e test, perhaps could be combined with fanout semi e2e test, or reuse some function.
// TODO making some fanout UT test/e2e test function shared.
func TestWatchAndSync(t *testing.T) {
	testProject := "test-project"
	signal := make(chan struct{})
	targets := memory.NewEmptyTargets()
	p, err := StartSyncPool(context.Background(), targets,
		pool.WithProjectID(testProject),
		pool.WithSyncSignal(signal),
	)
	if err != nil {
		t.Errorf("unexpected error from starting sync pool: %v", err)
	}
	assertHandlers(t, p, targets)

	t.Run("adding some brokers with their targets", func(t *testing.T) {
		// Add some brokers with their targets.
		for i := 0; i < 2; i++ {
			for j := 0; j < 2; j++ {
				ns := fmt.Sprintf("ns-%d", i)
				bn := fmt.Sprintf("broker-%d", j)
				targets.MutateBroker(ns, bn, func(bm config.BrokerMutation) {
					bm.SetAddress("address")
					bm.SetDecoupleQueue(&config.Queue{
						Topic:        fmt.Sprintf("t-%d-%d", i, j),
						Subscription: fmt.Sprintf("sub-%d-%d", i, j),
					})
					bm.UpsertTargets(makeTarget("old", bn, ns))
				})
			}
		}
		signal <- struct{}{}
		// Wait a short period for the handlers to be updated.
		<-time.After(time.Second)
		assertHandlers(t, p, targets)
	})

	t.Run("delete and adding targets in brokers", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			for j := 0; j < 2; j++ {
				ns := fmt.Sprintf("ns-%d", i)
				bn := fmt.Sprintf("broker-%d", j)
				targets.MutateBroker(ns, bn, func(bm config.BrokerMutation) {
					bm.DeleteTargets(makeTarget("old", bn, ns))
					bm.UpsertTargets(makeTarget("new", bn, ns))
				})
			}
		}
		signal <- struct{}{}
		// Wait a short period for the handlers to be updated.
		<-time.After(time.Second)
		assertHandlers(t, p, targets)
	})

	t.Run("deleting all brokers with their targets", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			for j := 0; j < 2; j++ {
				ns := fmt.Sprintf("ns-%d", i)
				bn := fmt.Sprintf("broker-%d", j)
				targets.MutateBroker(ns, bn, func(bm config.BrokerMutation) {
					bm.DeleteTargets(makeTarget("new", bn, ns))
					bm.Delete()
				})
			}
		}
		signal <- struct{}{}
		// Wait a short period for the handlers to be updated.
		<-time.After(time.Second)
		assertHandlers(t, p, targets)
	})
}

func assertHandlers(t *testing.T, p *SyncPool, targets config.Targets) {
	t.Helper()
	gotHandlers := make(map[string]bool)
	wantHandlers := make(map[string]bool)

	p.pool.Range(func(key, value interface{}) bool {
		gotHandlers[key.(string)] = true
		return true
	})

	targets.RangeAllTargets(func(t *config.Target) bool {
		wantHandlers[t.Key()] = true
		return true
	})

	if diff := cmp.Diff(wantHandlers, gotHandlers); diff != "" {
		t.Errorf("handlers map (-want,+got): %v", diff)
	}
}

func makeTarget(name, brokerName, namespace string) *config.Target {
	return &config.Target{
		Id:      "uid",
		Address: "consumer.example.com",
		Broker:  brokerName,
		FilterAttributes: map[string]string{
			"app": "zzz",
		},
		Name:      name,
		Namespace: namespace,
		RetryQueue: &config.Queue{
			Topic:        "topic",
			Subscription: "sub",
		},
	}
}