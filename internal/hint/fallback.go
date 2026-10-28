package hint

import "github.com/altnovatrail/tutor/internal/sample"

// The causes of a fallback text, as the log names them: the tutor's question did not come — the
// call failed or its answer was off the schema —, or it did not pass its checks, the leak check or
// the check of its form (concepts 10 spec; R39). A hint that both leaks and breaks its form falls
// back for the leak.
const (
	FallbackFailed = "failed"
	FallbackLeak   = "leak"
	FallbackForm   = "form"
)

// rungTemplates are the templates of the first two rungs of the ladder in each language: what the
// learner reads when the tutor's question did not come or did not pass its checks (the format text
// of spec, section 19, «Fallback texts»). A module has them in its own language; the measurement
// has a hint in two, so it has them in both. The two differ, so that a fallback never repeats the
// one before it. They hold no number and name nothing a task might have as its answer, so that they
// pass the leak check of every task, and they keep the form a hint keeps (R39).
var rungTemplates = map[sample.Lang][2]string{
	sample.LangRussian: {
		"Проверь работу шаг за шагом. В каком месте что-то пошло не так?",
		"Вспомни правило, которое нужно в этой задаче. Где в работе оно нарушено?",
	},
	sample.LangEnglish: {
		"Check your work step by step. Where did something go wrong?",
		"Think of the rule this task needs. Where in your work is it broken?",
	},
}

// Fallback is the fallback text of a rung in a language. At rungs 1 and 2 it is the template of the
// rung: the banks of the measurement have neither the text of a trap value nor a trap's advice,
// which come before it. At rung 3 it is what rung 2 gives: they have no simpler case either (R31).
func Fallback(rung Rung, lang sample.Lang) string {
	t := rungTemplates[lang]
	if rung == 1 {
		return t[0]
	}
	return t[1]
}
