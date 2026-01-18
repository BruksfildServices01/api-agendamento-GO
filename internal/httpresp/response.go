package httpresp

import "github.com/gin-gonic/gin"

type ListResponse[T any] struct {
	Data  []T `json:"data"`
	Total int `json:"total"`
}

func OK(c *gin.Context, data any) {
	c.JSON(200, data)
}

func List[T any](c *gin.Context, data []T) {
	c.JSON(200, ListResponse[T]{
		Data:  data,
		Total: len(data),
	})
}
