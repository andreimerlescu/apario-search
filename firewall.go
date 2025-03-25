package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

func patchServerWithBannedIp(ip net.IP) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered from panic:", r)
		}
	}()
	requestedAt := time.Now().UTC()
	ipBanningSemaphore.Acquire()
	if since := time.Since(requestedAt).Seconds(); since > 1.7 {
		log.Printf("took %.0f seconds to acquire sem_banned_ip_patch queue position", since)
	}
	defer ipBanningSemaphore.Release()
	// TODO: add the option to use firewall-cmd, ufw or iptables to block the IP address from the server with a comment
	log.Printf("need to patch the server with banning the ip %v", ip)
}

func addIpToBanList(ip net.IP) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered from panic:", r)
		}
	}()
	ipBanLocker.Lock()
	defer ipBanLocker.Unlock()

	ipBanned = append(ipBanned, ip)
	patchServerWithBannedIp(ip)
}

func addIPToList(ip net.IP) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered from panic:", r)
		}
	}()
	ipListLocker.RLock()
	counter, found := ipWatchList[ip.String()]
	ipListLocker.RUnlock()
	if !found {
		ipListLocker.Lock()
		ipWatchList[ip.String()] = &atomic.Int64{}
		counter = ipWatchList[ip.String()]
		ipListLocker.Unlock()
	}
	newCount := counter.Add(1)

	if newCount >= 6 {
		addIpToBanList(ip)
	}
}

func ipInBanList(ip net.IP) bool {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered from panic:", r)
		}
	}()
	ipBanLocker.RLock()
	defer ipBanLocker.RUnlock()

	for _, banned := range ipBanned {
		if ip.Equal(banned) {
			return true
		}
	}
	return false
}

func scheduleIpBanListCleanup(ctx context.Context) {
	ticker1 := time.NewTicker(time.Duration(*cfigs.Int(kCleanupIPBanListEvery)) * time.Minute)
	ticker2 := time.NewTicker(3 * time.Minute) // every 3 minutes save to disk
	ticker3 := time.NewTicker(6 * time.Minute) // every 6 minutes restore from disk
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker1.C:
			performIpBanListCleanup(ctx)
		case <-ticker2.C:
			performIpBanFsSync(ctx)
		case <-ticker3.C:
			performIpBanFsLoad(ctx)
		}
	}
}

func performIpBanFsLoad(ctx context.Context) {
	ipFile, openErr := os.OpenFile(*cfigs.String(kIPBanFile), os.O_RDONLY, 0600)
	if openErr != nil {
		log.Printf("Error opening IP ban file: %v", openErr)
		return
	}
	defer ipFile.Close()

	if ctx.Err() != nil {
		log.Println("Operation cancelled:", ctx.Err())
		return
	}

	var results StoredIPs
	decoder := json.NewDecoder(ipFile)
	if err := decoder.Decode(&results); err != nil {
		log.Printf("Error decoding IP ban list: %v", err)
		return
	}

	ipBanLocker.Lock()
	defer ipBanLocker.Unlock()
	for key, entry := range results.Entries {
		if ctx.Err() != nil {
			log.Println("Operation cancelled during processing:", ctx.Err())
			return
		}
		ip := net.ParseIP(key)
		if ip != nil {
			ipBanned = append(ipBanned, ip)
			updateWatchCounter(ip, entry.Counter)
		}
	}
}

func performIpBanFsSync(ctx context.Context) {
	ipBanLocker.RLock()
	var results StoredIPs
	results.Entries = make(map[string]SavedIP)
	for _, ip := range ipBanned {
		if ctx.Err() != nil {
			log.Println("Operation cancelled before file operations:", ctx.Err())
			ipBanLocker.RUnlock()
			return
		}
		ipStr := ip.String()
		results.Entries[ipStr] = SavedIP{IP: ip, Counter: getCurrentCounter(ipStr)}
	}
	ipBanLocker.RUnlock()

	ipFile, openErr := os.OpenFile(*cfigs.String(kIPBanFile), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if openErr != nil {
		log.Printf("Error opening IP ban file for writing: %v", openErr)
		return
	}
	defer ipFile.Close()

	encoder := json.NewEncoder(ipFile)
	if err := encoder.Encode(results); err != nil {
		log.Printf("Error encoding IP ban list: %v", err)
	}
}

func performIpBanListCleanup(ctx context.Context) {
	var results StoredIPs
	ipBanLocker.RLock()
	ips := slices.Clone(ipBanned)
	ipBanLocker.RUnlock()
	duration := time.Duration(*cfigs.Int(kIPBanDuration)) * time.Millisecond
	timeoutCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()
	breakFor := atomic.Bool{}
	ctxCanceled := atomic.Bool{}
	tryLockCount := atomic.Int64{}

	if len(results.Entries) == 0 || results.Entries == nil {
		results.mu = &sync.RWMutex{}
		breakFor.Store(false)
		for {
			select {
			case <-ctx.Done():
				breakFor.Store(true)
				ctxCanceled.Store(true)
				break
			case <-timeoutCtx.Done():
				breakFor.Store(true)
				break
			case <-time.Tick(10 * time.Millisecond):
				count := tryLockCount.Add(1)
				if count < int64(*cfigs.Int(kIPBanDuration)) && !breakFor.Load() && results.mu.TryLock() { // max 170ms to unlock
					results.mu.Lock()
					results.Entries = make(map[string]SavedIP)
					results.mu.Unlock()
					breakFor.Store(true)
					break
				}
			}
			if breakFor.Load() {
				break
			}
		}
		if ctxCanceled.Load() {
			log.Printf("failing because non-timeout ctx was canceled")
			return
		}
	}

	for _, ip := range ips {
		ipListLocker.RLock()
		counter, found := ipWatchList[ip.String()]
		ipListLocker.RUnlock()
		if !found {
			ipListLocker.Lock()
			ipWatchList[ip.String()] = &atomic.Int64{}
			counter = ipWatchList[ip.String()]
			ipListLocker.Unlock()
		}
		results.mu.Lock()
		results.Entries[ip.String()] = SavedIP{
			IP:      ip,
			Counter: counter.Load(),
		}
		results.mu.Unlock()
	}
}

func Sha256(in string) (checksum string) {
	hash := sha256.New()
	hash.Write([]byte(in))
	checksum = hex.EncodeToString(hash.Sum(nil))
	return checksum
}

func updateWatchCounter(ip net.IP, count int64) {
	ipListLocker.Lock()
	defer ipListLocker.Unlock()

	ipStr := ip.String()
	counter, found := ipWatchList[ipStr]
	if !found {
		// Initialize the counter if it does not exist
		ipWatchList[ipStr] = new(atomic.Int64)
		counter = ipWatchList[ipStr]
	}
	counter.Store(count) // Set the counter to the specific value
}

func getCurrentCounter(ipStr string) int64 {
	ipListLocker.RLock()
	defer ipListLocker.RUnlock()

	if counter, found := ipWatchList[ipStr]; found {
		return counter.Load()
	}
	return 0
}
