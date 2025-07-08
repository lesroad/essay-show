package util

import (
	"encoding/json"
	"essay-show/biz/infrastructure/util/log"
)

type StreamType string

var (
	STInit     StreamType = "init"
	STPart     StreamType = "part"
	STComplete StreamType = "complete"
	STError    StreamType = "error"
)

type StreamMessage struct {
	Type    StreamType `json:"type"`              // 消息类型
	Message string     `json:"message,omitempty"` // 文本消息
	Data    any        `json:"data,omitempty"`    // 数据内容
}

func SendStreamMessage(resultChan chan<- string, msgType StreamType, message string, data any) {
	msg := StreamMessage{
		Type:    msgType,
		Message: message,
		Data:    data,
	}
	if jsonData, err := json.Marshal(msg); err == nil {
		select {
		case resultChan <- string(jsonData):
		default:
			log.Error("流式消息通道已满，跳过消息: %s", msgType)
		}
	} else {
		log.Error("流式消息JSON序列化失败: %v", err)
	}
}
