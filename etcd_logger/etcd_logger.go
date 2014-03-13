package etcd_logger

import (
	"encoding/json"
	"fmt"
	"github.com/cloudfoundry/gunk/urljoiner"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	datadog "github.com/xb95/go-datadog-api"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

func EmitRunOnceStates(datadogClient *datadog.Client, store *etcdstoreadapter.ETCDStoreAdapter, etcdMachines []string) {
	for {
		now := time.Now().Unix()
		all, err := store.ListRecursively("/v1/run_once")
		if err != nil {
			log.Println("failed to get all RunOnces:", err)
			time.Sleep(1 * time.Second)
			continue
		}

		metrics := []datadog.Metric{}

		for i, etcdMachine := range etcdMachines {
			stats := map[string]int{}
			resp, err := http.Get(urljoiner.Join(etcdMachine, "/v2/stats/store"))
			if err != nil {
				log.Println("failed to fetch stats:", err)
				continue
			}
			data, _ := ioutil.ReadAll(resp.Body)
			resp.Body.Close()

			json.Unmarshal(data, &stats)

			metrics = append(metrics, datadog.Metric{
				Metric: fmt.Sprintf("etcd_watchers_%d", i),
				Points: []datadog.DataPoint{
					datadog.DataPoint(
						[2]float64{
							float64(now),
							float64(stats["watchers"]),
						},
					),
				},
			})
		}

		for _, state := range []string{"pending", "claimed", "running", "completed", "resolving"} {
			runOnces, found := all.Lookup(state)
			if !found {
				log.Println("failed to find RunOnces in", state, "state")
				time.Sleep(1 * time.Second)
				continue
			}

			metrics = append(metrics, datadog.Metric{
				Metric: "diego_runonce_" + state,
				Points: []datadog.DataPoint{
					datadog.DataPoint(
						[2]float64{
							float64(now),
							float64(len(runOnces.ChildNodes)),
						},
					),
				},
			})
		}

		executors, err := store.ListRecursively("/v1/executor")
		if err != nil {
			log.Println("failed to get all Executors:", err)
			time.Sleep(1 * time.Second)
			continue
		}

		metrics = append(metrics, datadog.Metric{
			Metric: "executors_maintaining_presence",
			Points: []datadog.DataPoint{
				datadog.DataPoint(
					[2]float64{
						float64(now),
						float64(len(executors.ChildNodes)),
					},
				),
			},
		})

		err = datadogClient.PostMetrics(metrics)
		if err != nil {
			log.Println("failed to post metrics:", err)
		}

		time.Sleep(1 * time.Second)
	}
}
