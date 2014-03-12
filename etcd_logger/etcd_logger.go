package etcd_logger

import (
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	datadog "github.com/xb95/go-datadog-api"
	"log"
	"time"
)

func EmitRunOnceStates(datadogClient *datadog.Client, store *etcdstoreadapter.ETCDStoreAdapter) {
	for {
		all, err := store.ListRecursively("/v1/run_once")
		if err != nil {
			log.Println("failed to get all RunOnces:", err)
			time.Sleep(1 * time.Second)
			continue
		}

		metrics := []datadog.Metric{}

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
							float64(time.Now().Unix()),
							float64(len(runOnces.ChildNodes)),
						},
					),
				},
			})
		}

		err = datadogClient.PostMetrics(metrics)
		if err != nil {
			log.Println("failed to post metrics:", err)
		}

		time.Sleep(1 * time.Second)
	}
}
