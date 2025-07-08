package stateless

type Evaluate struct {
	Title        string       `json:"title"`
	Text         [][]string   `json:"text"`
	EssayInfo    EssayInfo    `json:"essayInfo"`
	AIEvaluation AIEvaluation `json:"aiEvaluation"`
}

type EssayInfo struct {
	EssayType string   `json:"essayType"`
	Grade     int      `json:"grade"`
	Counting  Counting `json:"counting"`
}

type Counting struct {
	AdjAdvNum         int `json:"adjAdvNum"`
	CharNum           int `json:"charNum"`
	DieciNum          int `json:"dieciNum"`
	Fluency           int `json:"fluency"`
	GrammarMistakeNum int `json:"grammarMistakeNum"`
	HighlightSentsNum int `json:"highlightSentsNum"`
	IdiomNum          int `json:"idiomNum"`
	NounTypeNum       int `json:"nounTypeNum"`
	ParaNum           int `json:"paraNum"`
	SentNum           int `json:"sentNum"`
	UniqueWordNum     int `json:"uniqueWordNum"`
	VerbTypeNum       int `json:"verbTypeNum"`
	WordNum           int `json:"wordNum"`
	WrittenMistakeNum int `json:"writtenMistakeNum"`
}

type AIEvaluation struct {
	ModelVersion           ModelVersion           `json:"modelVersion"`
	OverallEvaluation      OverallEvaluation      `json:"overallEvaluation"`      // 总评
	FluencyEvaluation      FluencyEvaluation      `json:"fluencyEvaluation"`      // 流畅度评价
	WordSentenceEvaluation WordSentenceEvaluation `json:"wordSentenceEvaluation"` // 好词好句评价
	ExpressionEvaluation   ExpressionEvaluation   `json:"expressionEvaluation"`   // 逻辑表达评价
	SuggestionEvaluation   SuggestionEvaluation   `json:"suggestionEvaluation"`   // 建议
	ParagraphEvaluations   []ParagraphEvaluation  `json:"paragraphEvaluations"`   // 段落点评
	ScoreEvaluation        ScoreEvaluation        `json:"scoreEvaluations"`       // 分数点评
}

type ModelVersion struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type OverallEvaluation struct {
	Description         string `json:"description"`
	TopicRelevanceScore int    `json:"topicRelevanceScore"`
}

type FluencyEvaluation struct {
	FluencyDescription string `json:"fluencyDescription"`
	FluencyScore       int    `json:"fluencyScore"`
}

type WordSentenceEvaluation struct {
	SentenceEvaluations [][]SentenceEvaluation `json:"sentenceEvaluations"`
	WordSentenceScore   int                    `json:"wordSentenceScore"`
}

type SentenceEvaluation struct {
	IsGoodSentence  bool              `json:"isGoodSentence"`
	Label           string            `json:"label"`
	Type            map[string]string `json:"type"`
	WordEvaluations []WordEvaluation  `json:"wordEvaluations"`
}

type WordEvaluation struct {
	Span    []int             `json:"span"`
	Type    map[string]string `json:"type"`
	Ori     string            `json:"ori,omitempty"`
	Revised string            `json:"revised,omitempty"`
}

type ExpressionEvaluation struct {
	ExpressDescription string `json:"expressDescription"`
	ExpressionScore    int    `json:"expressionScore"`
}

type SuggestionEvaluation struct {
	SuggestionDescription string `json:"suggestionDescription"`
}

type ParagraphEvaluation struct {
	ParagraphIndex int    `json:"paragraphIndex"`
	Comment        string `json:"comment"`
}

type ScoreEvaluation struct {
	All        int `json:"all"`
	Appearance int `json:"appearance"`
	Content    int `json:"content"`
	Expression int `json:"expression"`
	Structure  int `json:"structure"`
}
