package http

import (
	"encoding/json"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// Headers `tailscale serve` adds with the requesting user. It replaces any copies a client
// sends, and the web container is only reachable through it (or from the host itself), so
// they can be trusted. See docs/TAILSCALE.md.
const (
	headerLogin      = "Tailscale-User-Login"
	headerName       = "Tailscale-User-Name"
	headerProfilePic = "Tailscale-User-Profile-Pic"
)

var wordDecoder = new(mime.WordDecoder)

// identity reads the Tailscale user; non-ASCII names arrive RFC 2047 encoded.
func identity(c *gin.Context) domain.Identity {
	name := c.GetHeader(headerName)
	if decoded, err := wordDecoder.DecodeHeader(name); err == nil {
		name = decoded
	}
	return domain.Identity{Login: c.GetHeader(headerLogin), Name: name, ProfilePic: c.GetHeader(headerProfilePic)}
}

// AdminMe tells the console who is signed in and whether they are an admin. It is open to
// everyone so the console can explain why access is denied.
func (handler *Handler) AdminMe(c *gin.Context) {
	if handler.admin == nil {
		handler.unavailable(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": handler.admin.Session(identity(c))})
}

// requireAdmin admits only Tailscale users on the admin allow list.
func (handler *Handler) requireAdmin(c *gin.Context) {
	if handler.admin == nil {
		handler.unavailable(c)
		c.Abort()
		return
	}
	if !handler.admin.Session(identity(c)).Admin {
		c.AbortWithStatusJSON(http.StatusForbidden, errorResponse{Error: apiError{
			Code: "forbidden", Message: "the admin console needs an allowed Tailscale user", RequestID: requestid.Get(c),
		}})
		return
	}
	c.Next()
}

func (handler *Handler) unavailable(c *gin.Context) {
	c.JSON(http.StatusServiceUnavailable, errorResponse{Error: apiError{Code: "not_available", Message: "the admin console requires a database", RequestID: requestid.Get(c)}})
}

// AdminDataStatus reports imports and coverage, including full error messages.
func (handler *Handler) AdminDataStatus(c *gin.Context) {
	if handler.dataStatus == nil {
		handler.unavailable(c)
		return
	}
	status, err := handler.dataStatus.Status(c.Request.Context())
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": status})
}

// AdminSystem reports database size, disk space and backups.
func (handler *Handler) AdminSystem(c *gin.Context) {
	status, err := handler.admin.System(c.Request.Context())
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": status})
}

// requireSameOrigin guards state-changing admin requests against cross-site request
// forgery: tailscale serve attaches the admin's identity to any request their browser
// makes, including one a page on another site triggers. Browsers mark cross-site requests
// in Sec-Fetch-Site and Origin, and a JSON body cannot be sent cross-site without a CORS
// preflight, which the router does not allow for POST.
func (handler *Handler) requireSameOrigin(c *gin.Context) {
	forbid := func(message string) {
		c.AbortWithStatusJSON(http.StatusForbidden, errorResponse{Error: apiError{Code: "forbidden", Message: message, RequestID: requestid.Get(c)}})
	}
	if site := c.GetHeader("Sec-Fetch-Site"); site != "" && site != "same-origin" {
		forbid("cross-site requests are not allowed")
		return
	}
	if origin := c.GetHeader("Origin"); origin != "" {
		parsed, err := url.Parse(origin)
		if err != nil || hostname(parsed.Host) != hostname(c.Request.Host) {
			forbid("cross-site requests are not allowed")
			return
		}
	}
	if !strings.HasPrefix(c.ContentType(), "application/json") {
		c.AbortWithStatusJSON(http.StatusUnsupportedMediaType, errorResponse{Error: apiError{Code: "invalid_request", Message: "send JSON", RequestID: requestid.Get(c)}})
		return
	}
	c.Next()
}

// hostname drops the port: nginx forwards Host without it, while Origin keeps it.
func hostname(host string) string {
	if name, _, err := net.SplitHostPort(host); err == nil {
		return strings.ToLower(name)
	}
	return strings.ToLower(host)
}

// AdminJobs lists recent jobs requested from the console, newest first.
func (handler *Handler) AdminJobs(c *gin.Context) {
	jobs, err := handler.admin.Jobs(c.Request.Context())
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": jobs})
}

// AdminEnqueueJob queues a job, e.g. {"kind": "sync"} or {"kind": "seasons", "params":
// {"seasons": "115S2", "cities": ["a"]}}, recording who asked for it.
func (handler *Handler) AdminEnqueueJob(c *gin.Context) {
	var request struct {
		Kind   domain.JobKind  `json:"kind"`
		Params json.RawMessage `json:"params"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: apiError{Code: "invalid_request", Message: "body must be {\"kind\": ...}", RequestID: requestid.Get(c)}})
		return
	}
	job, err := handler.admin.Enqueue(c.Request.Context(), request.Kind, request.Params, identity(c).Login)
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": job})
}

// AdminOpenings lists line and platform opening dates for editing.
func (handler *Handler) AdminOpenings(c *gin.Context) {
	openings, err := handler.admin.Openings(c.Request.Context())
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": openings})
}

// AdminSaveOpening sets ({"openedOn": "2020-01-31"}) or removes ({"openedOn": null}) the
// opening date of a line or platform, then returns every date.
func (handler *Handler) AdminSaveOpening(c *gin.Context) {
	var edit domain.OpeningEdit
	if err := c.ShouldBindJSON(&edit); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: apiError{Code: "invalid_request", Message: "body must be {kind, targetId, openedOn, source}", RequestID: requestid.Get(c)}})
		return
	}
	if err := handler.admin.SaveOpening(c.Request.Context(), edit, identity(c).Login); err != nil {
		handler.writeError(c, err)
		return
	}
	handler.AdminOpenings(c)
}
