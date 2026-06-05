package clients

import (
	"context"
	"fmt"
	"home_automation/internal/logger"
	"home_automation/internal/utils"
	"strings"
	"time"

	"github.com/carlmjohnson/requests"
)

// type iBricksResponse struct {
// 	Response *ResponseValues `xml:"Response"`
// }

// type ResponseValues struct {
// 	Text  string      `xml:"Text"`
// 	Value int         `xml:"Value"`
// 	P1    string      `xml:"P1"`
// 	P2    interface{} `xml:"P2"`
// }

const (
	ERROR_PREFIX = "[ERR]"
)

type IBricksClient struct {
	url  string
	port int
}

func InitIBricksClient(config *utils.Config) *IBricksClient {
	return &IBricksClient{
		url:  config.IBricks.URL,
		port: config.IBricks.Port,
	}
}

func (iBricks *IBricksClient) SetMemo(memoName string, memoValue interface{}) error {
	returnBody := ""
	requestUrl := fmt.Sprintf("http://%s:%d/M2M/Core-HTTP/CallFunction.aspx", iBricks.url, iBricks.port)
	reqBuilder := requests.URL(requestUrl).
		Param("name", "SetMemoExt").
		Param("p1", memoName).
		Param("p2", fmt.Sprintf("%v", memoValue)).
		ToString(&returnBody)
	err := reqBuilder.Fetch(context.Background())

	if err != nil {
		logger.Error("Failed to set memo %s to value %v: %s", memoName, memoValue, err)
		return err
	}
	if strings.HasPrefix(returnBody, ERROR_PREFIX) {
		logger.Error("Failed to set memo %s to value %v: %s", memoName, memoValue, returnBody)
		return fmt.Errorf("%s", returnBody)
	}
	return nil
}

func (iBricks *IBricksClient) TriggerShutterPosition() error {
	returnBody := ""
	requestUrl := fmt.Sprintf("http://%s:%d/M2M/Core-HTTP/CallFunction.aspx", iBricks.url, iBricks.port)
	reqBuilder := requests.URL(requestUrl).
		Param("name", "TriggerShutterPositionExt").
		ToString(&returnBody)
	err := reqBuilder.Fetch(context.Background())

	if err != nil {
		logger.Error("Failed to trigger shutter position: %s", err)
	}
	if strings.HasPrefix(returnBody, ERROR_PREFIX) {
		logger.Error("Failed to trigger shutter position:: %s", returnBody)
		return fmt.Errorf("%s", returnBody)
	}
	return nil
}

func (iBricks *IBricksClient) StartSendingHeartbeat(frequency int) {
	go func() {
		// Send initial heartbeat to let ibricks now we're here, then every frequency minute
		iBricks.SetMemo(MemoHeartbeatTimestamp, time.Now().Unix())
		for range time.Tick(time.Minute * time.Duration(frequency)) {
			iBricks.SetMemo(MemoHeartbeatTimestamp, time.Now().Unix())
		}
	}()
}
