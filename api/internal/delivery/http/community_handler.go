package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Community describes one community, identified by its ID.
func (handler *Handler) Community(c *gin.Context) {
	if handler.communities == nil {
		handler.unavailableWithoutDatabase(c, "communities")
		return
	}
	detail, err := handler.communities.Community(c.Request.Context(), c.Param("id"))
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": detail})
}

// CommunityRanking lists a district's communities with the most recent sales.
func (handler *Handler) CommunityRanking(c *gin.Context) {
	if handler.communities == nil {
		handler.unavailableWithoutDatabase(c, "communities")
		return
	}
	ranking, err := handler.communities.Ranking(c.Request.Context(), c.DefaultQuery("city", "a"), c.Query("district"))
	if err != nil {
		handler.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": ranking})
}
