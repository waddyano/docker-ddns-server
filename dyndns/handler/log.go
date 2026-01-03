package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/waddyano/docker-ddns-server/dyndns/model"
)

// CreateLogEntry simply adds a log entry to the database.
func (h *Handler) CreateLogEntry(log *model.Log) (err error) {
	if err = h.DB.Create(log).Error; err != nil {
		return err
	}

	return nil
}

// ShowLogs fetches all log entries from all hosts and renders them to the website.
func (h *Handler) ShowLogs(c echo.Context) (err error) {
	if !h.AuthAdmin {
		return c.JSON(http.StatusUnauthorized, &Error{UNAUTHORIZED})
	}

	countString := ""
	logCount, err := h.countLogs(0)
	if err == nil {
		countString = " (" + strconv.FormatInt(logCount, 10) + " entries)"
	}

	logs := new([]model.Log)
	if err = h.DB.Preload("Host").Limit(30).Order("created_at desc").Find(logs).Error; err != nil {
		return c.JSON(http.StatusBadRequest, &Error{err.Error()})
	}

	return c.Render(http.StatusOK, "listlogs", echo.Map{
		"logs":  logs,
		"title": h.Title,
		"count": countString,
	})
}

func (h *Handler) countLogs(hostId uint) (int64, error) {
	logCount := int64(0)
	var err error
	if hostId == 0 {
		err = h.DB.Model(&model.Log{}).Count(&logCount).Error
	} else {
		err = h.DB.Model(&model.Log{}).Where("host_id = ?", hostId).Count(&logCount).Error
	}
	return logCount, err
}

func (h *Handler) deleteOldRecordsKeepMinimum(host uint, cutoffDate time.Time, minToKeep int) error {
	// First, count total records
	var totalCount int64
	if err := h.DB.Model(&model.Log{}).Where("host_id = ?", host).Count(&totalCount).Error; err != nil {
		return err
	}

	// If we have fewer than or equal to minimum, don't delete anything
	if totalCount <= int64(minToKeep) {
		return nil
	}

	// Find the cutoff ID: the ID of the Nth newest record (where N = minToKeep)
	var cutoffRecord model.Log
	if err := h.DB.Model(&model.Log{}).
		Where("host_id = ?", host).
		Order("created_at DESC").
		Offset(minToKeep - 1).
		Limit(1).
		First(&cutoffRecord).Error; err != nil {
		return err
	}

	// Delete records that are BOTH:
	// 1. Older than the cutoff date, AND
	// 2. Older than our Nth newest record (to ensure we keep at least minToKeep)
	return h.DB.Unscoped().Where("host_id = ? AND created_at < ? AND created_at < ?",
		host,
		cutoffDate,
		cutoffRecord.CreatedAt,
	).Delete(&model.Log{}).Error
}

func (h *Handler) BackgroundClearLogs() (res string, err error) {
	sixtyDaysAgo := time.Now().AddDate(0, 0, -60)

	hosts := new([]model.Host)
	if err = h.DB.Find(hosts).Error; err != nil {
		return "", fmt.Errorf("error fetching hosts: %s", err.Error())
	}

	countBefore, _ := h.countLogs(0)
	for _, host := range *hosts {
		hostCountBefore, _ := h.countLogs(host.ID)
		err = h.deleteOldRecordsKeepMinimum(host.ID, sixtyDaysAgo, 10)
		if err != nil {
			return "", fmt.Errorf("error deleting logs for host %s: %s", host.Hostname, err.Error())
		}
		hostCountAfter, _ := h.countLogs(host.ID)
		fmt.Printf("host %s: %d entries before, %d after\n", host.Hostname, hostCountBefore, hostCountAfter)
	}
	countAfter, _ := h.countLogs(0)

	// fast enough on my DB (300ms)
	err = h.DB.Exec("VACUUM").Error
	if err != nil {
		return "", fmt.Errorf("error running VACUUM: %s", err.Error())
	}

	return fmt.Sprintf("%d entries before, %d after", countBefore, countAfter), nil
}

// ShowHostLogs fetches all log entries of a specific host by "id" and renders them to the website.
func (h *Handler) ShowHostLogs(c echo.Context) (err error) {
	if !h.AuthAdmin {
		return c.JSON(http.StatusUnauthorized, &Error{UNAUTHORIZED})
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 0)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &Error{err.Error()})
	}

	countString := ""
	logCount, err := h.countLogs(uint(id))
	if err == nil {
		countString = " (" + strconv.FormatInt(logCount, 10) + " entries)"
	}

	logs := new([]model.Log)
	if err = h.DB.Preload("Host").Where(&model.Log{HostID: uint(id)}).Order("created_at desc").Limit(30).Find(logs).Error; err != nil {
		return c.JSON(http.StatusBadRequest, &Error{err.Error()})
	}

	return c.Render(http.StatusOK, "listlogs", echo.Map{
		"logs":  logs,
		"title": h.Title,
		"count": countString,
	})
}
