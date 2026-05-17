package util

import "essay-show/biz/infrastructure/consts"

func GetGradeType(grade *int64) string {
	if grade == nil {
		return ""
	}
	if *grade < 10 {
		return "mid"
	} else if *grade < 13 {
		return "high"
	} else {
		return "hsk"
	}
}

func IsSupportHomeworkTopic(topic int64) bool {
	return topic == consts.TopicTypeCustom || topic == consts.TopicTypeLibrary || topic == consts.TopicTypeWeb
}
