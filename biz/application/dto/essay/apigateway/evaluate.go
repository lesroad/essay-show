package apigateway

// StreamMessage 流式消息的通用结构
type StreamMessage struct {
	Type    string         `json:"type" mapstructure:"type"`       // progress, complete, error
	Message string         `json:"message" mapstructure:"message"` // 消息描述
	Data    map[string]any `json:"data" mapstructure:"data"`       // 具体数据
}

type EssayContent struct {
	Title     string     `json:"title" mapstructure:"title"`
	Text      [][]string `json:"text" mapstructure:"text"`
	EssayInfo EssayInfo  `json:"essay_info" mapstructure:"essay_info"`
}

type AllContent struct {
	Title        string       `json:"title" mapstructure:"title"`
	Text         [][]string   `json:"text" mapstructure:"text"`
	EssayInfo    EssayInfo    `json:"essayInfo" mapstructure:"essayInfo"`
	AIEvaluation AIEvaluation `json:"aiEvaluation" mapstructure:"aiEvaluation"`
}

type EssayInfo struct {
	EssayType string   `json:"essayType" mapstructure:"essayType"`
	Grade     int      `json:"grade" mapstructure:"grade"`
	Counting  Counting `json:"counting" mapstructure:"counting"`
}

type Counting struct {
	AdjAdvNum         int `json:"adjAdvNum" mapstructure:"adjAdvNum"`
	CharNum           int `json:"charNum" mapstructure:"charNum"`
	DieciNum          int `json:"dieciNum" mapstructure:"dieciNum"`
	Fluency           int `json:"fluency" mapstructure:"fluency"`
	GrammarMistakeNum int `json:"grammarMistakeNum" mapstructure:"grammarMistakeNum"`
	HighlightSentsNum int `json:"highlightSentsNum" mapstructure:"highlightSentsNum"`
	IdiomNum          int `json:"idiomNum" mapstructure:"idiomNum"`
	NounTypeNum       int `json:"nounTypeNum" mapstructure:"nounTypeNum"`
	ParaNum           int `json:"paraNum" mapstructure:"paraNum"`
	SentNum           int `json:"sentNum" mapstructure:"sentNum"`
	UniqueWordNum     int `json:"uniqueWordNum" mapstructure:"uniqueWordNum"`
	VerbTypeNum       int `json:"verbTypeNum" mapstructure:"verbTypeNum"`
	WordNum           int `json:"wordNum" mapstructure:"wordNum"`
	WrittenMistakeNum int `json:"writtenMistakeNum" mapstructure:"writtenMistakeNum"`
}

type AIEvaluation struct {
	ModelVersion           ModelVersion           `json:"modelVersion" mapstructure:"modelVersion"`
	WordSentenceEvaluation WordSentenceEvaluation `json:"wordSentenceEvaluation" mapstructure:"wordSentenceEvaluation"` // 好词好句评价+语法检查
	SuggestionEvaluation   SuggestionEvaluation   `json:"suggestionEvaluation" mapstructure:"suggestionEvaluation"`     // 建议
	ScoreEvaluation        ScoreEvaluation        `json:"scoreEvaluations" mapstructure:"scoreEvaluations"`             // 分数点评
	ParagraphEvaluations   []ParagraphEvaluation  `json:"paragraphEvaluations" mapstructure:"paragraphEvaluations"`     // 段落点评
	PolishingEvaluation    []PolishingEvaluation  `json:"polishingEvaluation" mapstructure:"polishingEvaluation"`       // 润色
}

type ModelVersion struct {
	Name    string `json:"name" mapstructure:"name"`
	Version string `json:"version" mapstructure:"version"`
}

type WordSentenceEvaluation struct {
	SentenceEvaluations [][]SentenceEvaluation `json:"sentenceEvaluations" mapstructure:"sentenceEvaluations"`
	WordSentenceScore   int                    `json:"wordSentenceScore" mapstructure:"wordSentenceScore"`
}

type SentenceEvaluation struct {
	IsGoodSentence  bool              `json:"isGoodSentence" mapstructure:"isGoodSentence"`
	Label           string            `json:"label" mapstructure:"label"`
	Type            map[string]string `json:"type" mapstructure:"type"`
	WordEvaluations []WordEvaluation  `json:"wordEvaluations" mapstructure:"wordEvaluations"`
}

type WordEvaluation struct {
	Span    []int             `json:"span" mapstructure:"span"`
	Type    map[string]string `json:"type" mapstructure:"type"`
	Ori     string            `json:"ori" mapstructure:"ori"`
	Revised string            `json:"revised" mapstructure:"revised"`
}

type SuggestionEvaluation struct {
	SuggestionDescription string `json:"suggestionDescription" mapstructure:"suggestionDescription"`
}

type ParagraphEvaluation struct {
	ParagraphIndex int    `json:"paragraphIndex" mapstructure:"paragraphIndex"`
	Comment        string `json:"comment" mapstructure:"comment"`
}

type ScoreEvaluation struct {
	Comment  string   `json:"comment" mapstructure:"comment"`
	Comments Comments `json:"comments" mapstructure:"comments"`
	Scores   Scores   `json:"scores" mapstructure:"scores"`
}

type Comments struct {
	Appearance  string `json:"appearance" mapstructure:"appearance"`
	Content     string `json:"content" mapstructure:"content"`
	Expression  string `json:"expression" mapstructure:"expression"`
	Structure   string `json:"structure" mapstructure:"structure"`     // 结构-初中
	Development string `json:"development" mapstructure:"development"` // 发展-高中
}

type Scores struct {
	All         int `json:"all" mapstructure:"all"`
	Appearance  int `json:"appearance" mapstructure:"appearance"`
	Content     int `json:"content" mapstructure:"content"`
	Expression  int `json:"expression" mapstructure:"expression"`
	Structure   int `json:"structure" mapstructure:"structure"`
	Development int `json:"development" mapstructure:"development"`
}

type PolishingEvaluation struct {
	ParagraphIndex int `json:"paragraphIndex" mapstructure:"paragraphIndex"`
	Edits          []struct {
		Op            string `json:"op" mapstructure:"op"`
		Reason        string `json:"reason" mapstructure:"reason"`
		Original      string `json:"original" mapstructure:"original"`
		Revised       string `json:"revised" mapstructure:"revised"`
		SentenceIndex int    `json:"sentenceIndex" mapstructure:"sentenceIndex"`
		Span          []int  `json:"span" mapstructure:"span"`
	} `json:"edits" mapstructure:"edits"`
}
