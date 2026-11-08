package tutor_test

import (
	"fmt"
	"log"

	"github.com/altnovatrail/spec/types/admissionv0"

	"github.com/altnovatrail/tutor"
	"github.com/altnovatrail/tutor/admission"
)

// api makes the library once, when it starts: spec's admission table, the facts of its deployment,
// the least stage of its learners and the platform's keys, which it reads from Secret Manager. Before
// an attempt it asks which models would read the learner's work, and whether any would.
func Example() {
	table, err := admission.Platform()
	if err != nil {
		log.Fatal(err)
	}
	facts, err := admission.ReadFacts(table, []byte("anthropic:\n  data_processing_agreement: [adult]\n  minimal_retention: [adult]\n"))
	if err != nil {
		log.Fatal(err)
	}
	lib, err := tutor.New(tutor.Config{Table: table, Facts: facts, Least: admission.StageAuthor, Keys: map[admissionv0.Provider]string{"anthropic": "the platform's key"}})
	if err != nil {
		log.Fatal(err)
	}
	route := tutor.Route{AI: admissionv0.AiRoutePlatformAi, Category: admissionv0.AgeCategoryAdult}
	entry, answer, err := lib.Choose(admissionv0.CapabilityHandwritingReading, route)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(answer.Admitted, entry.Stage, entry.Models[0].Name(), "+", entry.Models[1].Name())

	child := tutor.Route{AI: admissionv0.AiRoutePlatformAi, Category: admissionv0.AgeCategoryUnder13}
	_, answer, _ = lib.Choose(admissionv0.CapabilityHandwritingReading, child)
	fmt.Println(answer.Admitted, *answer.Reason, *answer.Fact)
	// Output:
	// true author claude-opus-5-5 high + claude-haiku-5-5 medium
	// false fact_missing data_processing_agreement
}
