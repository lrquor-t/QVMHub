package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"qvmhub/service"
)

// GetLXCStats 获取 LXC 容器实时资源使用（读采集器内存缓存）。
func GetLXCStats(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "容器名称不能为空"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "ok",
		"data":    service.GetCachedLXCStats(name),
	})
}

// GetLXCStatsHistory 获取 LXC 容器资源使用历史（按日期范围，复用 VM 历史查询）。
func GetLXCStatsHistory(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "容器名称不能为空"})
		return
	}
	startStr := c.Query("start")
	endStr := c.Query("end")
	if startStr == "" || endStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请指定查询时间范围（start, end），格式: 2006-01-02 或 2006-01-02T15:04:05",
		})
		return
	}

	var start, end time.Time
	var err error
	start, err = time.ParseInLocation("2006-01-02", startStr, time.Local)
	if err != nil {
		start, err = time.ParseInLocation("2006-01-02T15:04:05", startStr, time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "start 日期格式错误"})
			return
		}
	}
	end, err = time.ParseInLocation("2006-01-02", endStr, time.Local)
	if err != nil {
		end, err = time.ParseInLocation("2006-01-02T15:04:05", endStr, time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "end 日期格式错误"})
			return
		}
	}
	if end.Hour() == 0 && end.Minute() == 0 && end.Second() == 0 {
		end = end.Add(24*time.Hour - time.Second)
	}

	records, err := service.QueryVMStatsHistory(name, start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "查询历史记录失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": records})
}
