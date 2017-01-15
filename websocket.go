package main

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"code.cloudfoundry.org/bytefmt"
	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/process"
)

var upgrader = websocket.Upgrader{}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		var conn, _ = upgrader.Upgrade(w, r, nil)

		go func(conn *websocket.Conn) {
			memTicker := time.NewTicker(1 * time.Second)

			defer func() {
				memTicker.Stop()
				conn.Close()
			}()

			for {
				select {
				case <-memTicker.C:
					// Get memory stats
					v, _ := mem.VirtualMemory()
					// Map to hold process name -> memory usage percentage pair
					procRAMMap := make(map[string]float32)
					// Get all process IDs
					pids, _ := process.Pids()
					// Iterate to get the process name and memory usage
					for _, pid := range pids {
						p, _ := process.NewProcess(pid)
						processName, _ := p.Name()
						processMem, _ := p.MemoryPercent()
						procRAMMap[processName] = processMem
					}
					// Get the top 3 ram consumers
					top3RamConsumers := sortMemoryUsage(procRAMMap, 3)
					// Get storage stats
					diskUsageStat, _ := disk.Usage("/")
					// Send json payload to client
					conn.WriteJSON(struct {
						FreeMemory              string   `json:"freeMemory"`
						UsedMemory              string   `json:"usedMemory"`
						TotalMemory             string   `json:"totalMemory"`
						UsedPercentMemory       string   `json:"usedPercentMemory"`
						TopThreeMemoryConsumers PairList `json:"topThreeMemoryConsumers"`
						FreeDisk                string   `json:"freeDisk"`
						UsedDisk                string   `json:"usedDisk"`
						TotalDisk               string   `json:"totalDisk"`
						UsedPercentDisk         string   `json:"usedPercentDisk"`
					}{
						FreeMemory:              bytefmt.ByteSize(v.Free),
						UsedMemory:              bytefmt.ByteSize(v.Used),
						TotalMemory:             bytefmt.ByteSize(v.Total),
						UsedPercentMemory:       fmt.Sprintf("%f%%", v.UsedPercent),
						TopThreeMemoryConsumers: top3RamConsumers,
						FreeDisk:                bytefmt.ByteSize(diskUsageStat.Free),
						UsedDisk:                bytefmt.ByteSize(diskUsageStat.Used),
						TotalDisk:               bytefmt.ByteSize(diskUsageStat.Total),
						UsedPercentDisk:         fmt.Sprintf("%f%%", diskUsageStat.UsedPercent),
					})
				}
			}
		}(conn)

	})

	http.ListenAndServe(":3000", nil)
}

// sortMemoryUsage returns the top nth memory consuming processes
func sortMemoryUsage(processToMemoryMap map[string]float32, nthLimit int) PairList {
	pl := make(PairList, len(processToMemoryMap))
	i := 0
	for k, v := range processToMemoryMap {
		pl[i] = Pair{k, v}
		i++
	}
	sort.Sort(sort.Reverse(pl))
	return pl[:nthLimit]

}

// Pair is a key-value structure
type Pair struct {
	Key   string
	Value float32
}

// PairList is an alias for a slice of Pairs
type PairList []Pair

// helper functions to implement sorting
func (p PairList) Len() int           { return len(p) }
func (p PairList) Less(i, j int) bool { return p[i].Value < p[j].Value }
func (p PairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
