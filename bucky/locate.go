package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

import . "github.com/jjneely/buckytools"
import "github.com/jjneely/buckytools/hashing"

func init() {
	usage := "[options] <metric list>"
	short := "Determine location in cluster for metrics."
	long := `Output to STDOUT a Graphite host for each given metric key.  That
host is where the given metric lives according to the hash ring.  We leave out
the instance and assume that all instances use the same data store on the
graphite node.

Metrics may be listed on the command line as arguments or, if the first
argument is "-" we read the list from a JSON array on STDIN.  Using -j will
produce a JSON map/hash on STDOUT of metric => host.

Use -s to query the hash ring only on the host given by -h or in the BUCKYHOST
environment variable.  Without -s, we verify the health of the cluster before
calculating metric locations.`

	c := NewCommand(locateCommand, "locate", usage, short, long)
	SetupHostname(c)
	SetupSingle(c)
	SetupJSON(c)
}

func buildHashRing(rings []*JSONRingType) *hashing.HashRing {
	hr := hashing.NewHashRing()
	for _, n := range rings[0].Nodes {
		// ports are already removed
		fields := strings.Split(n, ":")
		if len(fields) < 2 {
			hr.AddNode(hashing.NewNode(fields[0], ""))
		} else {
			hr.AddNode(hashing.NewNode(fields[0], fields[1]))
		}
	}

	return hr
}

// LocateSliceMetrics takes a slice of metric ken names and derives the location
// of each metric in the cluster by using the consistent hash algorithm.  It
// returns a map of metric => server.
func LocateSliceMetrics(metrics []string) map[string]string {
	rings := GetRings()
	if !IsHealthy(rings) {
		log.Fatal("Cluster is inconsistent. Use the servers command to investigate.")
	}

	hr := buildHashRing(rings)
	result := make(map[string]string)
	for _, key := range metrics {
		// XXX: we toss away instance info here due to our assumption that a
		// graphite node has one whisper db store
		result[key] = hr.GetNode(key).Server
	}

	return result
}

func LocateJSONMetrics(fd io.Reader) map[string]string {
	// Read the JSON from the file-like object
	blob, err := ioutil.ReadAll(fd)
	metrics := make([]string, 0)

	err = json.Unmarshal(blob, &metrics)
	if err != nil {
		log.Fatal("Error unmarshalling JSON data: %s", err)
	}

	return LocateSliceMetrics(metrics)
}

// locateCommand runs this subcommand.
func locateCommand(c Command) int {
	var list map[string]string
	if c.Flag.NArg() == 0 {
		log.Fatal("At least one argument is required.")
	} else if c.Flag.Arg(0) != "-" {
		list = LocateSliceMetrics(c.Flag.Args())
	} else {
		list = LocateJSONMetrics(os.Stdin)
	}

	if JSONOutput {
		blob, err := json.Marshal(list)
		if err != nil {
			log.Printf("%s", err)
		} else {
			os.Stdout.Write(blob)
			os.Stdout.Write([]byte("\n"))
		}
	} else {
		for k, v := range list {
			fmt.Printf("%s => %s\n", k, v)
		}
	}

	return 0
}
