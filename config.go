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
	"github.com/andreimerlescu/configurable"
)

func init() {
	cfg = configurable.New()
	cfg.NewString(kDir, ".", "Directory to scan for ocr.*.txt files")
	cfg.NewString(kPort, "18004", "HTTP port to use 1000-65534")
	cfg.NewString(kCacheDir, filepath.Join(".", "cache"), "Path to the search cache index directory")
	cfg.NewString(kErrorLog, filepath.Join(".", "error.log"), "Path to the error log")
	cfg.NewString(kAccessLog, filepath.Join(".", "access.log"), "Path to the Gin access log file")
	cfg.NewString(kRunMode, "local", "Run mode of search. Modes: "+strings.Join(runModes, ", "))
	cfg.NewBool(kStdoutAccessLogs, false, "Write Gin access logs to STDOUT")
	cfg.NewString(kReaderDomain, "idoread.com", "Domain name of the project excluding protocol path or query from the URL")
	cfg.NewInt(kWorkers, 17, "Number of workers to use to build the cache index")
	cfg.NewBool(kBoost, false, "When enabled, the runtime.GOMAXPROCS(0) is overridden in worker concurrency offering 200% boost in concurrent limits")
	cfg.NewInt(kMaxOpenFiles, 500, "Maximum number of open files allowed during index building to prevent 'too many open files' errors; adjust based on system limits (ulimit -n)")
	cfg.NewBool(kRateLimitEnabled, true, "When enabled, the rate limit enforcement is enabled")
	cfg.NewFloat64(kRateLimitRequestsPerSecond, 3.0, "Rate limit requests per second allowed")
	cfg.NewInt(kRateLimitTTL, 6.0, "Seconds to keep rate limit data")
	cfg.NewInt(kPerIPSearchLimit, 3.0, "Number of searches per IP allowed")
	cfg.NewFloat64(kJaroThreshold, 0.71, "1.0 means exact match 0.0 means no match; default is 0.71")
	cfg.NewFloat64(kJaroWinklerThreshold, 0.71, "using the JaroWinkler method, define the threshold that is tolerated; default is 0.71")
	cfg.NewFloat64(kJaroWinklerBoostThreshold, 0.7, "weight applied to common prefixes in matched strings comparing dictionary terms, page word data, and search query params")
	cfg.NewInt(kJaroWinklerPrefixSize, 3, "length of a jarrow weighted prefix string")
	cfg.NewInt(kUkkonenICost, 1, "insert cost ; when adding a char to find a match ; increase the score by this number ; default = 1")
	cfg.NewInt(kUkkonenSCost, 2, "substitution cost ; when replacing a char increase the score by this number ; default = 2")
	cfg.NewInt(kUkkonenDCost, 1, "delete cost ; when removing a char to find a match ; increase the score by this number ; default = 1")
	cfg.NewInt(kUkkonenMaxSubs, 2, "maximum number of substitutions allowed for a word to be considered a match ; higher value = lower accurate ; lower value = higher accuracy ; min = 0; default = 2")
	cfg.NewInt(kWagnerFisherICost, 1, "insert cost ; when adding a char to find a match ; increase the score by this number ; default = 1")
	cfg.NewInt(kWagnerFisherSCost, 2, "substitution cost ; when replacing a char increase the score by this number ; default = 2")
	cfg.NewInt(kWagnerFisherDCost, 1, "delete cost ; when removing a char to find a match ; increase the score by this number ; default = 1")
	cfg.NewInt(kWagnerFisherMaxSubs, 2, "maximum number of substitutions allowed for a word to be considered a match ; higher value = lower accurate ; lower value = higher accuracy ; min = 0; default = 2")
	cfg.NewInt(kHammingMaxSubs, 2, "maximum number of substitutions allowed for a word to be considered a match ; higher value = lower accuracy ; min = 1 ; default = 2")

	// CSP
	cfg.NewBool(kCSPEnabled, false, "Enable Content Security Policy (CSP) Enforcement")
	cfg.NewString(kCSPDomains, "", "CSP: Comma separated list of domains to whitelist content security policy for public web routes")
	cfg.NewString(kCSPWebSocketDomains, "", "CSP: Comma separated list of domains to whitelist for web sockets")
	cfg.NewString(kCSPThirdPartyStyleDomains, "", "CSP: Comma separated list of domains to whitelist for stylesheets")
	cfg.NewString(kCSPThirdPartyScriptDomains, "", "CSP: Comma separated list of domains to whitelist for scripts")
	cfg.NewBool(kCSPScriptUnsafeInlineEnabled, false, "CSP: permit 'unsafe-inline' in scripts")
	cfg.NewBool(kCSPScriptUnsafeEvalEnabled, false, "CSP: permit 'unsafe-eval' in scripts")
	cfg.NewBool(kCSPChildScriptUnsafeInlineEnabled, false, "CSP: permit 'unsafe-inline' in child scripts")
	cfg.NewBool(kCSPStyleUnsafeInlineEnabled, false, "CSP: permit 'unsafe-inline' in stylesheets")
	cfg.NewBool(kCSPUpgradeRequests, false, "CSP: upgrade insecure requests automatically")
	cfg.NewBool(kCSPBlockMixedContent, false, "CSP: block mixed content")
	cfg.NewString(kCSPReportURI, "/csp-incident/report", "Endpoint on public server to mount CSP violation report handler")

	// CORS Policy
	cfg.NewBool(kCORSEnabled, false, "Enable Cross Origin Request Signing")
	cfg.NewBool(kCORSAllowCredentials, true, "CORS allow credentials")
	cfg.NewString(kCORSAllowOrigin, "*", "CORS allowed origin")
	cfg.NewString(kCORSAllowMethods, "GET", "CORS allowed methods")
	cfg.NewString(kCORSAllowHeaders, "*", "CORS allowed headers")

	// IP Banning
	cfg.NewInt(kCleanupIPBanListEvery, 963, "Cleanup banned IP addresses very n-minutes")
	cfg.NewInt(kIPBanDuration, 1776, "Minutes to persist a ban on an IP address")
	cfg.NewString(kIPBanFile, filepath.Join(".", "database", "ip.bin"), "Path to the ip.bin binary file that will maintain the IP ban list for the instance")

	cfg.NewBool(kMiddlewareEnableAdsTXT, false, "Enable serving /ads.txt")
	cfg.NewString(kMiddlewareAdsTXTPath, filepath.Join(".", "public", "ads.txt"), "Path to the ads.txt file")
	cfg.NewBool(kMiddlewareEnableRobotsTXT, false, "Enable serving /robots.txt")
	cfg.NewString(kMiddlewareRobotsTXTPath, filepath.Join(".", "public", "robots.txt"), "Path to the robots.txt file")
	cfg.NewBool(kMiddlewareEnableSecurityTXT, false, "Enable serving /security.txt")
	cfg.NewString(kMiddlewareSecurityTXTPath, filepath.Join(".", "public", "security.txt"), "Path to the security.txt file")
	cfg.NewBool(kMiddlewareEnabledIPBanList, false, "Enable IP Ban list")
	cfg.NewBool(kMiddlewareEnabledTLSHandshake, false, "Enable TLS Handshake (TLS)")
	cfg.NewBool(kEnablePing, true, "Enable /ping endpoint")
	cfg.NewBool(kMiddlewareEnableOnlineUsers, false, "Enable online user list")
	cfg.NewString(kAutoBanHitPaths, "/cgi-bin|/admin.php|/wp-login.php", "Pipe separated list of paths when hit result in an insta-ban of the IP")
	cfg.NewString(kAutoBanHitPathContains, "cgi-bin|.php?", "Pipe separated list of substrings to look for in any path that results in an insta-ban of the IP")
	cfg.NewBool(kForceHTTPS, false, "Auto-upgrade http to https for all web requests")
	cfg.NewInt(kMaxSearches, 17, "Maximum concurrent searches permitted on the system")
}

func loadConfigs() error {
	if fn := os.Getenv(configEnvKey); len(fn) > 0 {
		if err := check.File(fn, file.Options{Exists: true}); err == nil {
			if err = cfg.Parse(fn); err != nil {
				return err
			} else {
				goto check
			}
		}
	} else if err := check.File(configFile, file.Options{Exists: true}); err == nil {
		if err = cfg.Parse(configFile); err != nil {
			return err
		} else {
			goto check
		}
	} else {
		if err = cfg.Parse(""); err != nil {
			return err
		} else {
			goto check
		}
	}
check:
	// verify the error log is writable
	if len(*cfg.String(kErrorLog)) == 0 {
		return errors.New("configurable error-log must be a path to the error.log but was blank")
	}
	// verify cache directory is writable
	if err := check.Directory(*cfg.String(kCacheDir), directory.Options{RequireWrite: true, Exists: true}); err != nil {
		return err
	}
	// verify apario-writer database directory
	if err := check.Directory(*cfg.String(kDir), directory.Options{Exists: true}); err != nil {
		return err
	}
	portInt, portErr := strconv.Atoi(*cfg.String(kPort))
	if portErr != nil {
		return portErr
	}
	if portInt < 1000 || portInt > 65535 {
		return errors.New("cannot bind to port below 1000 or above 65535")
	}
	if len(*cfg.String(kReaderDomain)) == 0 {
		return errors.New("cannot omit the reader-domain configurable")
	}
	return nil
}
