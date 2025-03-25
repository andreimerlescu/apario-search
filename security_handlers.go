package main

import (
	"github.com/gin-gonic/gin"
	"log"
	"net"
	"net/http"
	"strings"
)

// handlerCORS writes new headers to the gin.Context and uses the config properties
// kCORSAllowOrigin, kCORSAllowMethods, kCORSAllowHeaders and kCORSAllowCredentials
func handlerCORS(c *gin.Context) {
	var allow_creds string
	if *cfg.Bool(kCORSAllowCredentials) == true {
		allow_creds = "true"
	} else {
		allow_creds = "false"
	}
	c.Writer.Header().Set("Access-Control-Allow-Origin", *cfg.String(kCORSAllowOrigin))
	c.Writer.Header().Set("Access-Control-Allow-Methods", *cfg.String(kCORSAllowMethods))
	c.Writer.Header().Set("Access-Control-Allow-Headers", *cfg.String(kCORSAllowHeaders))
	c.Writer.Header().Set("Access-Control-Allow-Credentials", allow_creds)

	if c.Request.Method == "OPTIONS" {
		c.AbortWithStatus(http.StatusOK)
		return
	}
}

// handlerForceHttps upgrades HTTP:// with HTTPS:// on URLs with a HTTP 301 on the redirect
func handlerForceHttps(c *gin.Context) {
	if *cfg.Bool(kForceHTTPS) {
		r_url := c.Request.URL
		r_url.Scheme = "https"
		r_url.Host = c.Request.Host
		c.Redirect(http.StatusMovedPermanently, r_url.String())
		return
	}
}

// handlerTlsHandshake checks the gin.Context if the TLS.HandshakeComplete is false, and if it is,
// the gin.Context is canceled with an error
func handlerTlsHandshake(c *gin.Context) {
	if c.Request.TLS != nil && c.Request.TLS.HandshakeComplete == false {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "TLS handshake misconfiguration",
		})
		c.Abort()
		return
	}
}

// handlerContentSecurityPolicy enforces a Content Security Policy (CSP) on gin.Context and relies
// on the config properties kCSPDomains, kCSPWebSocketDomains, and the other kCSP... values
func handlerContentSecurityPolicy(c *gin.Context) {
	var domains []string
	if len(*cfg.String(kCSPDomains)) > 1 {
		// parse the flag/config CSV values and sanitize the string
		_domains := strings.Split(*cfg.String(kCSPDomains), ",")
		for _, domain := range _domains {
			domain = strings.ReplaceAll(domain, " ", "")
			if len(domain) > 0 {
				domains = append(domains, domain)
			}
		}
	}

	var wsDomains []string
	if len(*cfg.String(kCSPWebSocketDomains)) > 1 {
		// parse the flag/config CSV values and sanitize the string
		_domains := strings.Split(*cfg.String(kCSPWebSocketDomains), ",")
		for _, domain := range _domains {
			domain = strings.ReplaceAll(domain, " ", "")
			if len(domain) > 0 {
				wsDomains = append(wsDomains, domain)
			}
		}
	}

	var thirdPartyStyles []string
	if len(*cfg.String(kCSPThirdPartyScriptDomains)) > 1 {
		// parse the flag/config CSV values and sanitize the string
		_domains := strings.Split(*cfg.String(kCSPThirdPartyStyleDomains), ",")
		for _, domain := range _domains {
			domain = strings.ReplaceAll(domain, " ", "")
			if len(domain) > 0 {
				thirdPartyStyles = append(thirdPartyStyles, domain)
			}
		}
	}
	// List of domains allowed for thirdParty usage
	var thirdParty []string
	if len(*cfg.String(kCSPThirdPartyScriptDomains)) > 1 {
		// parse the flag/config CSV values and sanitize the string
		_domains := strings.Split(*cfg.String(kCSPThirdPartyScriptDomains), ",")
		for _, domain := range _domains {
			domain = strings.ReplaceAll(domain, " ", "")
			if len(domain) > 0 {
				thirdParty = append(thirdParty, domain)
			}
		}
	}

	// Script execution protection policy defaults
	scriptUnsafeInline := ""
	scriptUnsafeEval := ""
	childUnsafeInline := ""
	styleUnsafeInline := ""
	upgradeInsecure := ""
	blockMixed := ""

	// Depending on config flags, set the policy defaults
	if *cfg.Bool(kCSPScriptUnsafeInlineEnabled) {
		scriptUnsafeInline = "'unsafe-inline'"
	}
	if *cfg.Bool(kCSPScriptUnsafeEvalEnabled) {
		scriptUnsafeEval = "'unsafe-eval'"
	}
	if *cfg.Bool(kCSPChildScriptUnsafeInlineEnabled) {
		childUnsafeInline = "'unsafe-inline'"
	}
	if *cfg.Bool(kCSPStyleUnsafeInlineEnabled) {
		styleUnsafeInline = "'unsafe-inline'"
	}
	if *cfg.Bool(kCSPUpgradeRequests) {
		upgradeInsecure = "upgrade-insecure-requests;"
	}
	if *cfg.Bool(kCSPBlockMixedContent) {
		blockMixed = "block-all-mixed-content;"
	}

	c.Writer.Header().Set("Content-Security-Policy",
		"default-src 'self' "+strings.Join(domains, " ")+" "+strings.Join(thirdParty, " ")+"; "+
			"font-src 'self' data: "+strings.Join(domains, " ")+"; "+
			"img-src 'self' data: blob: "+strings.Join(domains, " ")+" "+strings.Join(thirdParty, " ")+"; "+
			"object-src 'self' "+strings.Join(domains, " ")+" "+strings.Join(thirdParty, " ")+"; "+
			"script-src 'self' "+scriptUnsafeInline+" "+scriptUnsafeEval+" "+strings.Join(domains, " ")+" "+strings.Join(thirdParty, " ")+"; "+
			"frame-src 'self' "+strings.Join(domains, " ")+" "+strings.Join(thirdParty, " ")+";"+
			"child-src 'self' "+childUnsafeInline+" blob: data: "+strings.Join(domains, " ")+" "+strings.Join(thirdParty, " ")+"; "+
			"style-src data: "+styleUnsafeInline+" "+strings.Join(domains, " ")+" "+strings.Join(thirdPartyStyles, " ")+"; "+
			"connect-src 'self' blob: "+strings.Join(domains, " ")+" "+strings.Join(wsDomains, " ")+"; "+
			"report-uri "+*cfg.String(kCSPReportURI)+"; "+
			upgradeInsecure+
			blockMixed)

	// Process the next Gin middleware
}

// handlerEnforceIPBanList uses filteredIp and ipInBanList to terminate the gin.Context if violating
func handlerEnforceIPBanList(c *gin.Context) {
	ip := FilteredIP(c)
	if len(ip) == 0 {
		log.Printf("invalid ip address detected")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	if ipInBanList(net.ParseIP(ip)) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
}

// handlerNoRouteLinter uses the kAutoBanHitPaths and kAutoBanHitPathContains config
// properties in order to verify the gin.Context of the request is not requesting a Path
// that is explicitly being filtered out by the WAF policy
func handlerNoRouteLinter(c *gin.Context) {
	requestedURL := c.Request.URL.Path

	var endpointIs []string
	if len(*cfg.String(kAutoBanHitPaths)) > 2 {
		endpointIs = strings.Split(*cfg.String(kAutoBanHitPaths), "|")
	}

	var endpointContains []string
	if len(*cfg.String(kAutoBanHitPathContains)) > 2 {
		endpointContains = strings.Split(*cfg.String(kAutoBanHitPathContains), "|")
	}

	ip := FilteredIP(c)
	if len(ip) == 0 {
		c.Data(http.StatusNotFound, "text/plain", []byte("The truth can never be concealed forever."))
		return
	}

	nip := net.ParseIP(ip)

	if ipInBanList(nip) {
		c.Data(http.StatusForbidden, "text/plain", []byte("403"))
		return
	}

	for _, endpoint := range endpointIs {
		if strings.EqualFold(requestedURL, endpoint) {
			addIpToBanList(nip)
			c.Data(http.StatusNotFound, "text/plain", []byte("404"))
			return
		}
	}

	for _, endpoint := range endpointContains {
		if strings.Contains(requestedURL, endpoint) {
			addIpToBanList(nip)
			c.Data(http.StatusNotFound, "text/plain", []byte("404"))
			return
		}
	}

	c.Data(http.StatusNotFound, "text/plain", []byte("404"))
	return
}
