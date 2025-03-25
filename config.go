package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	check "github.com/andreimerlescu/checkfs"
	"github.com/andreimerlescu/checkfs/directory"
	"github.com/andreimerlescu/checkfs/file"
	"github.com/andreimerlescu/figs"
)

func init() {
	cfigs = figs.New()
	cfigs.NewString(kDir, ".", "Directory to scan for ocr.*.txt files")
	cfigs.NewString(kPort, "18004", "HTTP port to use 1000-65534")
	cfigs.NewString(kCacheDir, filepath.Join(".", "cache"), "Path to the search cache index directory")
	cfigs.NewString(kErrorLog, filepath.Join(".", "error.log"), "Path to the error log")
	cfigs.NewString(kAccessLog, filepath.Join(".", "access.log"), "Path to the Gin access log file")
	cfigs.NewString(kRunMode, "local", "Run mode of search. Modes: "+strings.Join(runModes, ", "))
	cfigs.NewBool(kStdoutAccessLogs, false, "Write Gin access logs to STDOUT")
	cfigs.NewString(kReaderDomain, "idoread.com", "Domain name of the project excluding protocol path or query from the URL")
	cfigs.NewInt(kWorkers, 17, "Number of workers to use to build the cache index")
	cfigs.NewBool(kBoost, false, "When enabled, the runtime.GOMAXPROCS(0) is overridden in worker concurrency offering 200% boost in concurrent limits")
	cfigs.NewInt(kMaxOpenFiles, 500, "Maximum number of open files allowed during index building to prevent 'too many open files' errors; adjust based on system limits (ulimit -n)")
	cfigs.NewBool(kRateLimitEnabled, true, "When enabled, the rate limit enforcement is enabled")
	cfigs.NewFloat64(kRateLimitRequestsPerSecond, 3.0, "Rate limit requests per second allowed")
	cfigs.NewInt(kRateLimitTTL, 6.0, "Seconds to keep rate limit data")
	cfigs.NewInt(kPerIPSearchLimit, 3.0, "Number of searches per IP allowed")
	cfigs.NewFloat64(kJaroThreshold, 0.71, "1.0 means exact match 0.0 means no match; default is 0.71")
	cfigs.NewFloat64(kJaroWinklerThreshold, 0.71, "using the JaroWinkler method, define the threshold that is tolerated; default is 0.71")
	cfigs.NewFloat64(kJaroWinklerBoostThreshold, 0.7, "weight applied to common prefixes in matched strings comparing dictionary terms, page word data, and search query params")
	cfigs.NewInt(kJaroWinklerPrefixSize, 3, "length of a jarrow weighted prefix string")
	cfigs.NewInt(kUkkonenICost, 1, "insert cost ; when adding a char to find a match ; increase the score by this number ; default = 1")
	cfigs.NewInt(kUkkonenSCost, 2, "substitution cost ; when replacing a char increase the score by this number ; default = 2")
	cfigs.NewInt(kUkkonenDCost, 1, "delete cost ; when removing a char to find a match ; increase the score by this number ; default = 1")
	cfigs.NewInt(kUkkonenMaxSubs, 2, "maximum number of substitutions allowed for a word to be considered a match ; higher value = lower accurate ; lower value = higher accuracy ; min = 0; default = 2")
	cfigs.NewInt(kWagnerFisherICost, 1, "insert cost ; when adding a char to find a match ; increase the score by this number ; default = 1")
	cfigs.NewInt(kWagnerFisherSCost, 2, "substitution cost ; when replacing a char increase the score by this number ; default = 2")
	cfigs.NewInt(kWagnerFisherDCost, 1, "delete cost ; when removing a char to find a match ; increase the score by this number ; default = 1")
	cfigs.NewInt(kWagnerFisherMaxSubs, 2, "maximum number of substitutions allowed for a word to be considered a match ; higher value = lower accurate ; lower value = higher accuracy ; min = 0; default = 2")
	cfigs.NewInt(kHammingMaxSubs, 2, "maximum number of substitutions allowed for a word to be considered a match ; higher value = lower accuracy ; min = 1 ; default = 2")

	// CSP
	cfigs.NewBool(kCSPEnabled, false, "Enable Content Security Policy (CSP) Enforcement")
	cfigs.NewString(kCSPDomains, "", "CSP: Comma separated list of domains to whitelist content security policy for public web routes")
	cfigs.NewString(kCSPWebSocketDomains, "", "CSP: Comma separated list of domains to whitelist for web sockets")
	cfigs.NewString(kCSPThirdPartyStyleDomains, "", "CSP: Comma separated list of domains to whitelist for stylesheets")
	cfigs.NewString(kCSPThirdPartyScriptDomains, "", "CSP: Comma separated list of domains to whitelist for scripts")
	cfigs.NewBool(kCSPScriptUnsafeInlineEnabled, false, "CSP: permit 'unsafe-inline' in scripts")
	cfigs.NewBool(kCSPScriptUnsafeEvalEnabled, false, "CSP: permit 'unsafe-eval' in scripts")
	cfigs.NewBool(kCSPChildScriptUnsafeInlineEnabled, false, "CSP: permit 'unsafe-inline' in child scripts")
	cfigs.NewBool(kCSPStyleUnsafeInlineEnabled, false, "CSP: permit 'unsafe-inline' in stylesheets")
	cfigs.NewBool(kCSPUpgradeRequests, false, "CSP: upgrade insecure requests automatically")
	cfigs.NewBool(kCSPBlockMixedContent, false, "CSP: block mixed content")
	cfigs.NewString(kCSPReportURI, "/csp-incident/report", "Endpoint on public server to mount CSP violation report handler")

	// CORS Policy
	cfigs.NewBool(kCORSEnabled, false, "Enable Cross Origin Request Signing")
	cfigs.NewBool(kCORSAllowCredentials, true, "CORS allow credentials")
	cfigs.NewString(kCORSAllowOrigin, "*", "CORS allowed origin")
	cfigs.NewString(kCORSAllowMethods, "GET", "CORS allowed methods")
	cfigs.NewString(kCORSAllowHeaders, "*", "CORS allowed headers")

	// IP Banning
	cfigs.NewInt(kCleanupIPBanListEvery, 963, "Cleanup banned IP addresses very n-minutes")
	cfigs.NewInt(kIPBanDuration, 1776, "Minutes to persist a ban on an IP address")
	cfigs.NewString(kIPBanFile, filepath.Join(".", "database", "ip.bin"), "Path to the ip.bin binary file that will maintain the IP ban list for the instance")

	cfigs.NewBool(kMiddlewareEnableAdsTXT, false, "Enable serving /ads.txt")
	cfigs.NewString(kMiddlewareAdsTXTPath, filepath.Join(".", "public", "ads.txt"), "Path to the ads.txt file")
	cfigs.NewBool(kMiddlewareEnableRobotsTXT, false, "Enable serving /robots.txt")
	cfigs.NewString(kMiddlewareRobotsTXTPath, filepath.Join(".", "public", "robots.txt"), "Path to the robots.txt file")
	cfigs.NewBool(kMiddlewareEnableSecurityTXT, false, "Enable serving /security.txt")
	cfigs.NewString(kMiddlewareSecurityTXTPath, filepath.Join(".", "public", "security.txt"), "Path to the security.txt file")
	cfigs.NewBool(kMiddlewareEnabledIPBanList, false, "Enable IP Ban list")
	cfigs.NewBool(kMiddlewareEnabledTLSHandshake, false, "Enable TLS Handshake (TLS)")
	cfigs.NewBool(kEnablePing, true, "Enable /ping endpoint")
	cfigs.NewBool(kMiddlewareEnableOnlineUsers, false, "Enable online user list")
	cfigs.NewString(kAutoBanHitPaths, "/cgi-bin|/admin.php|/wp-login.php", "Pipe separated list of paths when hit result in an insta-ban of the IP")
	cfigs.NewString(kAutoBanHitPathContains, "cgi-bin|.php?", "Pipe separated list of substrings to look for in any path that results in an insta-ban of the IP")
	cfigs.NewBool(kForceHTTPS, false, "Auto-upgrade http to https for all web requests")
	cfigs.NewInt(kMaxSearches, 17, "Maximum concurrent searches permitted on the system")
}

func loadConfigs() error {
	if fn := os.Getenv(configEnvKey); len(fn) > 0 {
		if err := check.File(fn, file.Options{Exists: true}); err == nil {
			if err = cfigs.Parse(fn); err != nil {
				return err
			} else {
				goto check
			}
		}
	} else if err := check.File(configFile, file.Options{Exists: true}); err == nil {
		if err = cfigs.Parse(configFile); err != nil {
			return err
		} else {
			goto check
		}
	} else {
		if err = cfigs.Parse(""); err != nil {
			return err
		} else {
			goto check
		}
	}
check:
	// verify the error log is writable
	if len(*cfigs.String(kErrorLog)) == 0 {
		return errors.New("configurable error-log must be a path to the error.log but was blank")
	}
	// verify cache directory is writable
	if err := check.Directory(*cfigs.String(kCacheDir), directory.Options{RequireWrite: true, Exists: true}); err != nil {
		return err
	}
	// verify apario-writer database directory
	if err := check.Directory(*cfigs.String(kDir), directory.Options{Exists: true}); err != nil {
		return err
	}
	portInt, portErr := strconv.Atoi(*cfigs.String(kPort))
	if portErr != nil {
		return portErr
	}
	if portInt < 1000 || portInt > 65535 {
		return errors.New("cannot bind to port below 1000 or above 65535")
	}
	if len(*cfigs.String(kReaderDomain)) == 0 {
		return errors.New("cannot omit the reader-domain configurable")
	}
	return nil
}
