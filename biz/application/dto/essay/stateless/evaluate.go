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
	PolishingEvaluation    []PolishingEvaluation  `json:"polishingEvaluations"`   // 润色
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
	Comment  string   `json:"comment"`
	Comments Comments `json:"comments"`
	Scores   Scores   `json:"scores"`
}

type Comments struct {
	Appearance  string `json:"appearance"`
	Content     string `json:"content"`
	Expression  string `json:"expression"`
	Structure   string `json:"structure,omitempty"`   // 结构-初中
	Development string `json:"development,omitempty"` // 发展-高中
}
type Scores struct {
	All         int `json:"all"`
	Appearance  int `json:"appearance"`
	Content     int `json:"content"`
	Expression  int `json:"expression"`
	Structure   int `json:"structure,omitempty"`
	Development int `json:"development,omitempty"`
}

type PolishingEvaluation struct {
	ParagraphIndex int `json:"paragraphIndex"`
	Edits          []struct {
		Op            string `json:"op"`
		Reason        string `json:"reason"`
		Original      string `json:"original"`
		Revised       string `json:"revised,omitempty"`
		SentenceIndex int    `json:"sentenceIndex"`
		Span          []int  `json:"span"`
	} `json:"edits"`
}
