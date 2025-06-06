package exercise

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type (
	// Exercise  是一次生成的所有题目，暂时只有选择题
	// 单条记录多次生成题目会有多个Exercises对象
	Exercise struct {
		ID         primitive.ObjectID `bson:"_id" json:"id"`
		UserId     string             `bson:"user_id" json:"userId"`                             // 归属的用户ID
		LogId      string             `bson:"log_id" json:"logId"`                               // 批改记录的ID
		Question   *Question          `bson:"question" json:"question"`                          // 生成的题目
		History    *History           `bson:"history" json:"history"`                            // 用户做题记录
		Like       int64              `bson:"like" json:"like"`                                  // 点赞, -1是不喜欢该题，1是喜欢该题
		CreateTime time.Time          `bson:"create_time" json:"createTime"`                     // 创建时间
		UpdateTime time.Time          `bson:"update_time" json:"updateTime"`                     // 更新时间
		DeleteTime time.Time          `bson:"delete_time,omitempty" json:"deleteTime,omitempty"` // 删除时间
		Status     int64              `bson:"status" json:"status"`
	}

	// Question 一组问题, 抽离出来方便扩充其他体型
	Question struct {
		ChoiceQuestions []*ChoiceQuestion `bson:"choice_questions" json:"choiceQuestions"` // 选择题列表
	}

	// ChoiceQuestion 是一道完整的选择题
	ChoiceQuestion struct {
		Id          string    `bson:"id" json:"id"`                   // 题目id，如 "Q01"
		Question    string    `bson:"question" json:"question"`       // 问题描述
		Explanation string    `bson:"explanation" json:"explanation"` // 题目解答
		Options     []*Option `bson:"options" json:"options"`         // 题目选项
	}

	// Option 是一道选择题中的选项
	Option struct {
		Option  string `bson:"option" json:"option"`   // 选项，如'A'
		Content string `bson:"content" json:"content"` // 选项内容
		Score   int64  `bson:"score" json:"score"`     // 选项对应得分
	}

	// History 一组题目的总记录
	History struct {
		Records []*Records `bson:"records" json:"records"`
	}

	// Records 是用户做的一组题目的记录
	Records struct {
		Records    []*Record `bson:"records" json:"records"`        // 作答记录
		Score      int64     `bson:"score" json:"score"`            // 总得分
		CreateTime time.Time `bson:"create_time" json:"createTime"` // 提交时间
	}

	// Record 一道题的记录
	Record struct {
		Id     string `bson:"id" json:"id"`         // 题目Id
		Option string `bson:"option" json:"option"` // 选择内容
		Score  int64  `bson:"score" json:"score"`   // 得分
	}
)
