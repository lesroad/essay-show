package stateless

import (
	"encoding/json"
	"essay-show/biz/application/dto/essay/show"
)

type ExportEvaluate struct {
	Title        string             `json:"title"`
	Text         [][]string         `json:"text"`
	EssayInfo    EssayInfo          `json:"essayInfo"`
	AIEvaluation ExportAIEvaluation `json:"aiEvaluation"`
}

type ExportAIEvaluation struct {
	OverallEvaluation      OverallEvaluation       `json:"overallEvaluation"`
	WordSentenceEvaluation *WordSentenceEvaluation `json:"wordSentenceEvaluation,omitempty"`
	SuggestionEvaluation   *SuggestionEvaluation   `json:"suggestionEvaluation,omitempty"`
	ParagraphEvaluations   []ParagraphEvaluation   `json:"paragraphEvaluations,omitempty"`
	ScoreEvaluation        *ScoreEvaluation        `json:"scoreEvaluations,omitempty"`
	PolishingEvaluation    []PolishingEvaluation   `json:"polishingEvaluation,omitempty"`
}

func BuildExportEvaluateData(response string, excludeOptions *show.EvaluateExcludeOptions) (*ExportEvaluate, error) {
	var evaluateResult Evaluate
	if err := json.Unmarshal([]byte(response), &evaluateResult); err != nil {
		return nil, err
	}

	exportResult := &ExportEvaluate{
		Title:     evaluateResult.Title,
		Text:      evaluateResult.Text,
		EssayInfo: evaluateResult.EssayInfo,
		AIEvaluation: ExportAIEvaluation{
			OverallEvaluation: evaluateResult.AIEvaluation.OverallEvaluation,
		},
	}

	if !(excludeOptions.GetWrittenMistake() || excludeOptions.GetWordSentence()) {
		exportResult.AIEvaluation.WordSentenceEvaluation = &evaluateResult.AIEvaluation.WordSentenceEvaluation
	}
	if !excludeOptions.GetParagraph() {
		exportResult.AIEvaluation.ParagraphEvaluations = evaluateResult.AIEvaluation.ParagraphEvaluations
	}
	if !excludeOptions.GetPolishing() {
		exportResult.AIEvaluation.PolishingEvaluation = evaluateResult.AIEvaluation.PolishingEvaluation
	}
	if !excludeOptions.GetScore() {
		exportResult.AIEvaluation.ScoreEvaluation = &evaluateResult.AIEvaluation.ScoreEvaluation
	}
	if !excludeOptions.GetSuggestion() {
		exportResult.AIEvaluation.SuggestionEvaluation = &evaluateResult.AIEvaluation.SuggestionEvaluation
	}
	return exportResult, nil
}

func (e *ExportEvaluate) ToJson() string {
	data, _ := json.Marshal(e)
	return string(data)
}
