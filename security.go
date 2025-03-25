package main

import "github.com/gin-gonic/gin"

func middlewareCSP() gin.HandlerFunc {
	return func(c *gin.Context) {
		handlerContentSecurityPolicy(c)
		c.Next()
	}
}

func middlewareOnlineCounter() gin.HandlerFunc {
	return func(c *gin.Context) {
		handlerOnlineCounter(c)
		c.Next()
	}
}

func middlewareForceHTTPS() gin.HandlerFunc {
	return func(c *gin.Context) {
		handlerForceHttps(c)
		c.Next()
	}
}

func middlewareTLSHandshake() gin.HandlerFunc {
	return func(c *gin.Context) {
		handlerTlsHandshake(c)
		c.Next()
	}
}

func middlewareEnforceIPBan() gin.HandlerFunc {
	return func(c *gin.Context) {
		handlerEnforceIPBanList(c)
		c.Next()
	}
}

func middlewareCORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		handlerCORS(c)
		c.Next()
	}
}

func middlewareCountHits() gin.HandlerFunc {
	return func(c *gin.Context) {
		handlerAddHit(c)
		c.Next()
	}
}

func middlewareNoRouteLinter() gin.HandlerFunc {
	return func(c *gin.Context) {
		handlerNoRouteLinter(c)
		c.Next()
	}
}
