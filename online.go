package main

import (
	"github.com/andreimerlescu/sema"
	"github.com/gin-gonic/gin"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type onlineEntry struct {
	UserAgent     string        `json:"ua"`
	IP            net.IP        `json:"ip"`
	FirstAction   time.Time     `json:"fa"`
	LastAction    time.Time     `json:"la"`
	DeleteOn      time.Time     `json:"do"`
	Hits          *atomic.Int64 `json:"h"`
	LastPath      string        `json:"lp"`
	Authenticated bool          `json:"au"`
	Administrator bool          `json:"ad"`
	Username      string        `json:"un"`
}

type SavedIP struct {
	IP      net.IP ` json:"ip"`
	Counter int64  `json:"c"`
}
type StoredIPs struct {
	Entries map[string]SavedIP `json:"e"`
	mu      *sync.RWMutex
}

var (
	// Security
	ipListLocker = &sync.RWMutex{}
	ipWatchList  = map[string]*atomic.Int64{}

	ipBanLocker = &sync.RWMutex{}
	ipBanned    []net.IP

	onlineList   = map[string]onlineEntry{} // map[ip]online_entry{}
	onlineLocker = sync.RWMutex{}

	onlineCounter      = atomic.Int64{}
	ipBanningSemaphore = sema.New(1)
)

func handlerOnlineCounter(c *gin.Context) {
	ip := FilteredIP(c)
	onlineLocker.RLock()
	entry, exists := onlineList[ip]
	onlineLocker.RUnlock()
	nip := net.IP(ip)
	if !exists {
		onlineLocker.Lock()
		onlineList[nip.String()] = onlineEntry{
			UserAgent:     c.Request.Header.Get("User-Agent"),
			IP:            nip,
			FirstAction:   time.Now().UTC(),
			LastAction:    time.Now().UTC(),
			DeleteOn:      time.Now().UTC().Add(time.Duration(*cfigs.Int(kShowOnlineLastMinutes)) * time.Minute),
			Hits:          &atomic.Int64{},
			LastPath:      c.Request.URL.Path,
			Authenticated: false,
			Administrator: false,
			Username:      "",
		}
		onlineLocker.Unlock()
	} else {
		entry.Hits.Add(1)
		entry.LastPath = c.Request.URL.Path
		entry.LastAction = time.Now().UTC()
		entry.DeleteOn = time.Now().UTC().Add(time.Duration(*cfigs.Int(kShowOnlineLastMinutes)) * time.Minute)
		entry.UserAgent = c.Request.Header.Get("User-Agent")

		onlineLocker.Lock()
		onlineList[nip.String()] = entry
		onlineLocker.Unlock()
	}
}
