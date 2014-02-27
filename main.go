package main

import (
	"flag"
	"log"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/runtime-schema/bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/storeadapter/workerpool"
	"github.com/nu7hatch/gouuid"
)

var etcdMachines = flag.String("etcdMachines", "", "comma-separated list of http:// etcd endpoints")
var guid = flag.String("guid", "", "RunOnce guid (defaults to random guid)")
var timeout = flag.Duration("timeout", 10*time.Second, "maximum runtime of the RunOnce (0 for no limit)")
var script = flag.String("script", `echo hello, world`, "script to run in RunOnce")
var memoryMB = flag.Int("memoryMB", 0, "memory limit of the RunOnce (0 for no limit)")
var diskMB = flag.Int("diskMB", 0, "disk limit of the RunOnce (0 for no limit)")
var stack = flag.String("stack", "default", "stack")
var logGuid = flag.String("logGuid", "", "guid for logs (empty for no logs)")
var logSourceName = flag.String("logSourceName", "TST", "source name for logs")

func main() {
	flag.Parse()

	runOnceGuid := *guid

	if runOnceGuid == "" {
		randomGuid, err := uuid.NewV4()
		if err != nil {
			log.Fatalln("somehow failed to create a guid:", err)
		}

		runOnceGuid = randomGuid.String()
	}

	store := etcdstoreadapter.NewETCDStoreAdapter(strings.Split(*etcdMachines, ","), workerpool.NewWorkerPool(1))

	err := store.Connect()
	if err != nil {
		log.Fatalln("failed to connect to store:", err)
	}

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

	bbs := bbs.New(store)

	completed, stop, errs := bbs.WatchForCompletedRunOnce()

	err = bbs.DesireRunOnce(runOnce)
	if err != nil {
		log.Fatalln("failed to desire RunOnce:", err)
	}

	log.Println("desired RunOnce:", string(runOnce.ToJSON()))

dance:
	for {
		select {
		case done := <-completed:
			if done.Guid == runOnceGuid {
				log.Println("completed RunOnce:", string(done.ToJSON()))
				break dance
			}
		case err := <-errs:
			log.Println("watch error:", err)
		}
	}

	close(stop)
}
