package assessment_test

import (
	"testing"

	"github.com/teacherslounge/gaming-service/internal/assessment"
)

func TestAll_CountAndUniqueness(t *testing.T) {
	questions := assessment.All()

	if len(questions) != assessment.TotalCount {
		t.Errorf("want %d questions, got %d", assessment.TotalCount, len(questions))
	}

	ids := make(map[string]struct{}, len(questions))
	for _, q := range questions {
		if q.ID == "" {
			t.Error("question has empty ID")
		}
		if _, dup := ids[q.ID]; dup {
			t.Errorf("duplicate question ID %q", q.ID)
		}
		ids[q.ID] = struct{}{}
	}
}

func TestAll_EachQuestionHasTwoOptions(t *testing.T) {
	for _, q := range assessment.All() {
		if len(q.Options) != 2 {
			t.Errorf("question %q: want 2 options, got %d", q.ID, len(q.Options))
		}
		keys := map[string]bool{}
		for _, o := range q.Options {
			if o.Key == "" || o.Text == "" {
				t.Errorf("question %q: option missing key or text", q.ID)
			}
			keys[o.Key] = true
		}
		if !keys["A"] || !keys["B"] {
			t.Errorf("question %q: options must have keys A and B", q.ID)
		}
	}
}

func TestAll_DimensionCoverage(t *testing.T) {
	counts := map[string]int{}
	for _, q := range assessment.All() {
		counts[q.Dimension]++
	}
	for _, dim := range assessment.Dimensions {
		if counts[dim] < 3 {
			t.Errorf("dimension %q has only %d questions (want >= 3)", dim, counts[dim])
		}
	}
}

func TestByIndex(t *testing.T) {
	for i := 0; i < assessment.TotalCount; i++ {
		q := assessment.ByIndex(i)
		if q == nil {
			t.Errorf("ByIndex(%d) returned nil", i)
		}
	}
	if assessment.ByIndex(-1) != nil {
		t.Error("ByIndex(-1) should return nil")
	}
	if assessment.ByIndex(assessment.TotalCount) != nil {
		t.Errorf("ByIndex(%d) should return nil", assessment.TotalCount)
	}
}

func TestComputeDials_AllA(t *testing.T) {
	answers := map[string]string{}
	for _, q := range assessment.All() {
		answers[q.ID] = "A"
	}
	dials := assessment.ComputeDials(answers)
	for _, dim := range assessment.Dimensions {
		val, ok := dials[dim]
		if !ok {
			t.Errorf("dimension %q missing from dials", dim)
			continue
		}
		if val != -1.0 {
			t.Errorf("all-A: dimension %q want -1.0, got %f", dim, val)
		}
	}
}

func TestComputeDials_AllB(t *testing.T) {
	answers := map[string]string{}
	for _, q := range assessment.All() {
		answers[q.ID] = "B"
	}
	dials := assessment.ComputeDials(answers)
	for _, dim := range assessment.Dimensions {
		val, ok := dials[dim]
		if !ok {
			t.Errorf("dimension %q missing from dials", dim)
			continue
		}
		if val != 1.0 {
			t.Errorf("all-B: dimension %q want 1.0, got %f", dim, val)
		}
	}
}

func TestComputeDials_Mixed(t *testing.T) {
	// Give one dimension all A answers and another all B.
	answers := map[string]string{}
	for _, q := range assessment.All() {
		switch q.Dimension {
		case "active_reflective":
			answers[q.ID] = "A" // should give -1.0
		case "sensing_intuitive":
			answers[q.ID] = "B" // should give +1.0
		default:
			answers[q.ID] = "A"
		}
	}
	dials := assessment.ComputeDials(answers)
	if dials["active_reflective"] != -1.0 {
		t.Errorf("active_reflective: want -1.0, got %f", dials["active_reflective"])
	}
	if dials["sensing_intuitive"] != 1.0 {
		t.Errorf("sensing_intuitive: want +1.0, got %f", dials["sensing_intuitive"])
	}
}

func TestComputeDials_MissingAnswers(t *testing.T) {
	// Empty answers map: every dimension should default to 0.0.
	dials := assessment.ComputeDials(map[string]string{})
	for _, dim := range assessment.Dimensions {
		if dials[dim] != 0.0 {
			t.Errorf("empty answers: dimension %q want 0.0, got %f", dim, dials[dim])
		}
	}
}
