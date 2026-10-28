package runner

// Reader is a configuration of the runner as a kind of reading reads with it: the configuration,
// the output tokens a call is expected to take, for the estimate, and whether it reads once whatever
// the repeats, as Opus does in the reduced matrix (R08; R28). The reading of a sheet and that of the
// workbook have their readers so (T68, T79).
type Reader struct {
	Config
	Output int
	Once   bool
}

// ReadersOf are the readers of a kind, one for each configuration of the runner, in their order,
// with the output tokens the kind expects of each by its name and the names of those that read
// once. A configuration with no output here is a mistake of the code, and ReadersOf panics on it
// rather than estimate its calls at nothing or repeat it unasked.
func ReadersOf(kind string, output map[string]int, once ...string) []Reader {
	var out []Reader
	for _, c := range Configs() {
		n, ok := output[c.Name]
		if !ok {
			panic(kind + ": no reader for the configuration " + c.Name)
		}
		rd := Reader{Config: c, Output: n}
		for _, name := range once {
			rd.Once = rd.Once || name == c.Name
		}
		out = append(out, rd)
	}
	return out
}

// ReaderRecord is a reader as the record of a run tells it: its model and prices, and whether it
// reads once.
type ReaderRecord struct {
	Name       string  `json:"name"`
	Model      string  `json:"model"`
	Effort     string  `json:"effort"`
	NoThinking bool    `json:"no_thinking,omitempty"`
	Once       bool    `json:"once,omitempty"`
	InputUSD   float64 `json:"input_usd_per_mtok"`
	OutputUSD  float64 `json:"output_usd_per_mtok"`
}

// Record is the reader as the record of a run tells it.
func (rd Reader) Record() ReaderRecord {
	return ReaderRecord{Name: rd.Name, Model: rd.Model.ID, Effort: rd.Effort, NoThinking: rd.NoThinking, Once: rd.Once, InputUSD: rd.Model.Input, OutputUSD: rd.Model.Output}
}
