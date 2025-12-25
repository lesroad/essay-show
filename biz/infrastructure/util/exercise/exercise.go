package exercise

import (
	"context"
	"encoding/json"
	"essay-show/biz/infrastructure/config"
	"essay-show/biz/infrastructure/consts"
	"essay-show/biz/infrastructure/repository/exercise"
	"essay-show/biz/infrastructure/repository/log"
	"essay-show/biz/infrastructure/util"
	logx "essay-show/biz/infrastructure/util/log"
)

func GenerateExercise(ctx context.Context, grade int64, l *log.Log) (*exercise.Exercise, error) {
	m, err := parseLog(l)
	if err != nil {
		return nil, err
	}
	resp, err := generateByHttp(ctx, grade, m)
	if err != nil {
		return nil, err
	}
	e, err := parseExercise(resp)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func GenerateExerciseStream(ctx context.Context, grade int64, l *log.Log, resultChan chan<- string) (*exercise.Exercise, error) {
	// 创建下游JSON字符串通道
	downstreamChan := make(chan string, 100)
	defer close(downstreamChan)

	m, err := parseLog(l)
	if err != nil {
		return nil, err
	}

	header := make(map[string]string)
	header["Content-Type"] = consts.ContentTypeJson
	header["Charset"] = consts.CharSetUTF8

	body := buildBody(grade, m)
	client := util.GetHttpClient()
	url := config.GetConfig().Api.AlgorithmURL + "/generate_exercises_stream"

	go client.SendRequestStream(ctx, consts.Post, url, header, body, downstreamChan)

	cqs := make([]*exercise.ChoiceQuestion, 0)
	for jsonMessage := range downstreamChan {
		// 解析JSON消息
		var data map[string]any
		if parseErr := json.Unmarshal([]byte(jsonMessage), &data); parseErr != nil {
			logx.Error("解析下游JSON消息失败: %v", parseErr)
			continue
		}

		if msgType, ok := data["type"].(string); ok && msgType == "end" {
			break
		}

		cq, _ := parseExerciseFromStream(data)
		cqs = append(cqs, cq)

		// 返回部分数据
		util.SendStreamMessage(resultChan, util.STPart, "", cq)
	}

	// 构建最终练习对象
	que := &exercise.Question{
		ChoiceQuestions: cqs,
	}

	records := make([]*exercise.Records, 0)
	h := &exercise.History{
		Records: records,
	}

	e := &exercise.Exercise{
		Question: que,
		History:  h,
		Like:     0,
		Status:   0,
	}

	return e, nil
}

// 将log的Response转换为Json格式
func parseLog(l *log.Log) (map[string]any, error) {
	m := make(map[string]any)
	err := json.Unmarshal([]byte(l.Response), &m)
	return m, err
}

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

func parseExerciseFromStream(result map[string]any) (*exercise.ChoiceQuestion, error) {
	var q map[string]any
	json.Unmarshal([]byte(result["content"].(string)), &q)

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

	return cq, nil
}

func generateByHttp(ctx context.Context, grade int64, m map[string]any) (map[string]any, error) {
	header := make(map[string]string)
	header["Content-Type"] = consts.ContentTypeJson
	header["Charset"] = consts.CharSetUTF8

	body := buildBody(grade, m)

	client := util.GetHttpClient()
	resp, err := client.SendRequest(ctx, consts.Post, config.GetConfig().Api.AlgorithmURL+"/generate_exercises", header, body)
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
