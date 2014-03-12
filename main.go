package main

import (
	"flag"
	"github.com/pivotal-cf-experimental/executor-tester/etcd_logger"
	"github.com/pivotal-cf-experimental/executor-tester/stampede"
	datadog "github.com/xb95/go-datadog-api"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/runtime-schema/bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/storeadapter/workerpool"
)

var etcdMachines = flag.String("etcdMachines", "", "comma-separated list of http:// etcd endpoints")
var runOnceCount = flag.Int("runOnceCount", 1, "number of RunOnces to create")
var timeout = flag.Duration("timeout", 10*time.Second, "maximum runtime of the RunOnce (0 for no limit)")
var script = flag.String("script", `echo hello, world`, "script to run in RunOnce")
var memoryMB = flag.Int("memoryMB", 0, "memory limit of the RunOnce (0 for no limit)")
var diskMB = flag.Int("diskMB", 0, "disk limit of the RunOnce (0 for no limit)")
var stack = flag.String("stack", "lucid64", "stack")
var logGuid = flag.String("logGuid", "", "guid for logs (empty for no logs)")
var logSourceName = flag.String("logSourceName", "TST", "source name for logs")

var emitStates = flag.Bool("emitStates", false, "emit state of RunOnces to datadog")

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
		// logging mode
		etcd_logger.EmitRunOnceStates(datadogClient, store)
	} else {
		// stampede mode

		runOnce := models.RunOnce{
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
		stampede.RunonceStampede(bbs.New(store), datadogClient, runOnce, *runOnceCount)
	}

}
