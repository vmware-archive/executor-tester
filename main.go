package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/nu7hatch/gouuid"
	datadog "github.com/xb95/go-datadog-api"

	"github.com/cloudfoundry-incubator/runtime-schema/bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/storeadapter/workerpool"
)

var etcdMachines = flag.String("etcdMachines", "", "comma-separated list of http:// etcd endpoints")
var runOnceCount = flag.Int("runOnceCount", 1, "number of RunOnces to create")
var guid = flag.String("guid", "", "RunOnce guid (defaults to random guid)")
var timeout = flag.Duration("timeout", 10*time.Second, "maximum runtime of the RunOnce (0 for no limit)")
var script = flag.String("script", `echo hello, world`, "script to run in RunOnce")
var memoryMB = flag.Int("memoryMB", 0, "memory limit of the RunOnce (0 for no limit)")
var diskMB = flag.Int("diskMB", 0, "disk limit of the RunOnce (0 for no limit)")
var stack = flag.String("stack", "lucid64", "stack")
var logGuid = flag.String("logGuid", "", "guid for logs (empty for no logs)")
var logSourceName = flag.String("logSourceName", "TST", "source name for logs")

var emitStates = flag.Bool("emitStates", false, "emit state of RunOnces to datadog")

type RunOnceResult struct {
	Guid     string
	Duration time.Duration
	Failed   bool
}

func main() {
	flag.Parse()

	datadogApiKey := os.Getenv("DATADOG_API_KEY")
	datadogAppKey := os.Getenv("DATADOG_APP_KEY")

	var datadogClient *datadog.Client

	if datadogApiKey != "" {
		datadogClient = datadog.NewClient(datadogApiKey, datadogAppKey)
	}

	store := etcdstoreadapter.NewETCDStoreAdapter(
		strings.Split(*etcdMachines, ","),
		workerpool.NewWorkerPool(50),
	)

	err := store.Connect()
	if err != nil {
		log.Fatalln("failed to connect to store:", err)
	}

	if *emitStates {
		go emitRunOnceStates(datadogClient, store)
		select {}
	}

	bbs := bbs.New(store)

	completed, stop, errs := bbs.WatchForCompletedRunOnce()

	runOnceStartTimes := make(map[string]time.Time)

	startAll := time.Now()

	if datadogClient != nil {
		event, err := datadogClient.PostEvent(&datadog.Event{
			Title: "diego_runonce_stampede_start",
			Text:  "started the stampede",
			Tags:  []string{fmt.Sprintf("count:%d", *runOnceCount)},
		})

		log.Println("posted start event:", event, err)

		defer func() {
			event, err := datadogClient.PostEvent(&datadog.Event{
				Title: "diego_runonce_stampede_stop",
				Text:  "stopped the stampede",
				Tags: []string{
					fmt.Sprintf("count:%d", *runOnceCount),
					fmt.Sprintf("duration:%s", time.Since(startAll)),
				},
			})

			log.Println("posted stop event:", event, err)
		}()
	}

	for i := 0; i < *runOnceCount; i++ {
		go func() {
			runOnceGuid := *guid

			if runOnceGuid == "" {
				randomGuid, err := uuid.NewV4()
				if err != nil {
					log.Fatalln("somehow failed to create a guid:", err)
				}

				runOnceGuid = randomGuid.String()
			}

			runOnceStartTimes[runOnceGuid] = time.Now()

			runOnce := models.RunOnce{
				Guid: runOnceGuid,
				Actions: []models.ExecutorAction{
					{
						models.RunAction{
							Script:  *script,
							Timeout: *timeout,
						},
					},
				},
				Stack:    *stack,
				MemoryMB: *memoryMB,
				DiskMB:   *diskMB,
			}

			if *logGuid != "" {
				runOnce.Log = models.LogConfig{
					Guid:       *logGuid,
					SourceName: *logSourceName,
				}
			}

			err = bbs.DesireRunOnce(runOnce)
			if err != nil {
				log.Fatalln("failed to desire RunOnce:", err)
			}

			log.Println("desired RunOnce:", runOnce.Guid)
		}()
	}

	seenRunOnces := 0

	for {
		if seenRunOnces == *runOnceCount {
			break
		}

		select {
		case completedRunOnce := <-completed:
			startedAt, found := runOnceStartTimes[completedRunOnce.Guid]
			if !found {
				continue
			}

			log.Println("done:", seenRunOnces, RunOnceResult{
				Guid:     completedRunOnce.Guid,
				Duration: time.Since(startedAt),
				Failed:   completedRunOnce.Failed,
			})

			seenRunOnces++
		case err := <-errs:
			log.Println("watch error:", err)
		}
	}

	close(stop)
}

func emitRunOnceStates(datadogClient *datadog.Client, store *etcdstoreadapter.ETCDStoreAdapter) {
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
