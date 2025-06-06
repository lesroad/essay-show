package exercise

import (
	"encoding/json"
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/consts"
	"essay-show/biz/infrastructure/mapper/exercise"
	"essay-show/biz/infrastructure/mapper/log"
	"essay-show/biz/infrastructure/util"
)

func GenerateExercise(grade int64, l *log.Log) (*exercise.Exercise, error) {
	m, err := parseLog(l)
	if err != nil {
		return nil, err
	}
	resp, err := generateByHttp(grade, m)
	if err != nil {
		return nil, err
	}
	e, err := parseExercise(resp)
	if err != nil {
		return nil, err
	}
	return e, nil
}

// 将log的Response转换为Json格式
func parseLog(l *log.Log) (map[string]any, error) {
	m := make(map[string]any)
	err := json.Unmarshal([]byte(l.Response), &m)
	return m, err
}

// 将map形式的resp解析成exercise
func parseExercise(resp map[string]any) (*exercise.Exercise, error) {
	// 选择题数组
	cqs := make([]*exercise.ChoiceQuestion, 0)

	// 题目数组
	questions := resp["result"].([]any)
	for _, question := range questions {
		q := question.(map[string]any)
		cq := &exercise.ChoiceQuestion{Options: make([]*exercise.Option, 0)}
		for k, v := range q {
			switch k {
			case "question":
				cq.Question = v.(string)
			case "explaion":
				fallthrough
			case "explanation":
				cq.Explanation = v.(string)
			case "id":
				cq.Id = v.(string)
			default:
				detailQuestion := v.(map[string]any)
				opt := &exercise.Option{
					Option:  k,
					Content: detailQuestion["content"].(string),
					Score:   int64(detailQuestion["score"].(float64)),
				}
				cq.Options = append(cq.Options, opt)
			}
		}
		cqs = append(cqs, cq)
	}

	// 题目列表
	q := &exercise.Question{
		ChoiceQuestions: cqs,
	}
	// 作答记录
	records := make([]*exercise.Records, 0)
	h := &exercise.History{
		Records: records,
	}
	// 练习
	e := &exercise.Exercise{
		Question: q,
		History:  h,
		Like:     0,
		Status:   0,
	}
	return e, nil
}

func generateByHttp(grade int64, m map[string]any) (map[string]any, error) {
	header := make(map[string]string)
	header["Content-Type"] = consts.ContentTypeJson
	header["Charset"] = consts.CharSetUTF8

	body := buildBody(grade, m)

	client := util.GetHttpClient()
	resp, err := client.SendRequest(consts.Post, config.GetConfig().Api.GenerateExercisesURL, header, body)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func buildBody(grade int64, m map[string]any) map[string]any {
	body := make(map[string]any)

	essay := ""
	paragraphs := m["text"].([]any)
	for _, paragraph := range paragraphs {
		paragraph := paragraph.([]any)
		for _, sentence := range paragraph {
			essay += sentence.(string)
		}
	}

	body["grade"] = grade
	body["title"] = m["title"]
	body["essay"] = essay
	body["result"] = m
	return body
}
