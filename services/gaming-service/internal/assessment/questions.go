// Package assessment holds the static question bank for the learning style
// assessment. Questions target the four Felder-Silverman dimensions:
//
//	active_reflective  – negative = active,      positive = reflective
//	sensing_intuitive  – negative = sensing,     positive = intuitive
//	visual_verbal      – negative = visual,      positive = verbal
//	sequential_global  – negative = sequential,  positive = global
//
// Three questions per dimension, presented in interleaved order so the user
// doesn't notice a block structure.
package assessment

// Option is one answer choice in an assessment question.
// Weight is in [-1.0, 1.0] and represents how strongly this choice pushes
// toward the positive pole of the question's dimension.
type Option struct {
	Key    string  `json:"key"`
	Text   string  `json:"text"`
	Weight float64 `json:"-"` // never sent to the client
}

// Question is a single assessment item.
type Question struct {
	ID        string   `json:"id"`
	Dimension string   `json:"dimension"` // felder-silverman dial name
	Stem      string   `json:"stem"`
	Options   []Option `json:"options"`
}

// bank is the ordered list of assessment questions (12 total, 3 per dimension).
// Order is intentionally interleaved across dimensions.
var bank = []Question{
	// ── active_reflective ──────────────────────────────────────────────────────
	{
		ID:        "ar-1",
		Dimension: "active_reflective",
		Stem:      "When you encounter a new concept, you prefer to…",
		Options: []Option{
			{Key: "A", Text: "Jump straight into practice problems and learn as you go.", Weight: -1.0},
			{Key: "B", Text: "Think it through carefully before attempting any problems.", Weight: +1.0},
		},
	},
	// ── sensing_intuitive ─────────────────────────────────────────────────────
	{
		ID:        "si-1",
		Dimension: "sensing_intuitive",
		Stem:      "You find it easier to remember…",
		Options: []Option{
			{Key: "A", Text: "Specific facts, formulas, and step-by-step procedures.", Weight: -1.0},
			{Key: "B", Text: "Underlying principles and how ideas connect to each other.", Weight: +1.0},
		},
	},
	// ── visual_verbal ─────────────────────────────────────────────────────────
	{
		ID:        "vv-1",
		Dimension: "visual_verbal",
		Stem:      "When studying, you get more out of…",
		Options: []Option{
			{Key: "A", Text: "Diagrams, charts, graphs, and colour-coded notes.", Weight: -1.0},
			{Key: "B", Text: "Written summaries, definitions, and detailed explanations.", Weight: +1.0},
		},
	},
	// ── sequential_global ─────────────────────────────────────────────────────
	{
		ID:        "sg-1",
		Dimension: "sequential_global",
		Stem:      "You prefer to learn new material by…",
		Options: []Option{
			{Key: "A", Text: "Progressing step-by-step through topics in order.", Weight: -1.0},
			{Key: "B", Text: "Getting the big picture first, then filling in the details.", Weight: +1.0},
		},
	},
	// ── active_reflective ──────────────────────────────────────────────────────
	{
		ID:        "ar-2",
		Dimension: "active_reflective",
		Stem:      "You feel most confident in a topic when you have…",
		Options: []Option{
			{Key: "A", Text: "Worked through many practice problems yourself.", Weight: -1.0},
			{Key: "B", Text: "Fully understood the theory behind it.", Weight: +1.0},
		},
	},
	// ── sensing_intuitive ─────────────────────────────────────────────────────
	{
		ID:        "si-2",
		Dimension: "sensing_intuitive",
		Stem:      "When you see a new formula, you first want to…",
		Options: []Option{
			{Key: "A", Text: "See it applied to a concrete, worked example.", Weight: -1.0},
			{Key: "B", Text: "Understand where it comes from and what it means.", Weight: +1.0},
		},
	},
	// ── visual_verbal ─────────────────────────────────────────────────────────
	{
		ID:        "vv-2",
		Dimension: "visual_verbal",
		Stem:      "You remember content best when…",
		Options: []Option{
			{Key: "A", Text: "You can picture it as a diagram or mental image.", Weight: -1.0},
			{Key: "B", Text: "You have read or written about it in your own words.", Weight: +1.0},
		},
	},
	// ── sequential_global ─────────────────────────────────────────────────────
	{
		ID:        "sg-2",
		Dimension: "sequential_global",
		Stem:      "When solving a complex problem, you tend to…",
		Options: []Option{
			{Key: "A", Text: "Follow a logical sequence from the first step to the last.", Weight: -1.0},
			{Key: "B", Text: "Jump around between sub-parts until the full picture clicks.", Weight: +1.0},
		},
	},
	// ── active_reflective ──────────────────────────────────────────────────────
	{
		ID:        "ar-3",
		Dimension: "active_reflective",
		Stem:      "In a group study session, you typically…",
		Options: []Option{
			{Key: "A", Text: "Drive the discussion and try ideas out loud right away.", Weight: -1.0},
			{Key: "B", Text: "Listen carefully and process before sharing your thoughts.", Weight: +1.0},
		},
	},
	// ── sensing_intuitive ─────────────────────────────────────────────────────
	{
		ID:        "si-3",
		Dimension: "sensing_intuitive",
		Stem:      "You are more comfortable with courses that emphasise…",
		Options: []Option{
			{Key: "A", Text: "Factual knowledge and well-defined procedures.", Weight: -1.0},
			{Key: "B", Text: "Abstract theories and open-ended problems.", Weight: +1.0},
		},
	},
	// ── visual_verbal ─────────────────────────────────────────────────────────
	{
		ID:        "vv-3",
		Dimension: "visual_verbal",
		Stem:      "If you don't understand something, you tend to…",
		Options: []Option{
			{Key: "A", Text: "Sketch it out or look for a diagram that shows it.", Weight: -1.0},
			{Key: "B", Text: "Re-read the explanation or find a clearer written description.", Weight: +1.0},
		},
	},
	// ── sequential_global ─────────────────────────────────────────────────────
	{
		ID:        "sg-3",
		Dimension: "sequential_global",
		Stem:      "You feel most lost when a topic…",
		Options: []Option{
			{Key: "A", Text: "Skips steps or jumps ahead without clear transitions.", Weight: -1.0},
			{Key: "B", Text: "Gives endless details without ever showing the overall purpose.", Weight: +1.0},
		},
	},
}

// All returns a copy of the full ordered question list.
func All() []Question {
	out := make([]Question, len(bank))
	copy(out, bank)
	return out
}

// ByIndex returns the question at position idx (0-based).
// Returns nil if idx is out of range.
func ByIndex(idx int) *Question {
	if idx < 0 || idx >= len(bank) {
		return nil
	}
	q := bank[idx]
	return &q
}

// TotalCount is the number of assessment questions.
const TotalCount = 12

// Dimensions lists all Felder-Silverman dial names used in the bank.
var Dimensions = []string{
	"active_reflective",
	"sensing_intuitive",
	"visual_verbal",
	"sequential_global",
}

// ComputeDials averages the chosen-option weights across all questions for each
// dimension and returns a map[dimension]dial_value in [-1.0, 1.0].
// answers maps question ID to the chosen option key ("A" or "B").
func ComputeDials(answers map[string]string) map[string]float64 {
	sums := map[string]float64{}
	counts := map[string]int{}

	for _, q := range bank {
		chosenKey, ok := answers[q.ID]
		if !ok {
			continue
		}
		for _, opt := range q.Options {
			if opt.Key == chosenKey {
				sums[q.Dimension] += opt.Weight
				counts[q.Dimension]++
				break
			}
		}
	}

	dials := make(map[string]float64, len(Dimensions))
	for _, dim := range Dimensions {
		if counts[dim] == 0 {
			dials[dim] = 0.0
			continue
		}
		val := sums[dim] / float64(counts[dim])
		// clamp to [-1, 1]
		if val > 1.0 {
			val = 1.0
		} else if val < -1.0 {
			val = -1.0
		}
		dials[dim] = val
	}
	return dials
}
