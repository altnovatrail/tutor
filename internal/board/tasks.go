package board

// Task is one of the board's tasks: what the person solves and what the model is told is right.
type Task struct {
	ID        string   `json:"id"`
	Statement string   `json:"statement"`
	Key       string   `json:"-"`
	Solution  []string `json:"-"`
}

// Tasks are the five tasks of the board: two linear equations, two chains of calculations and
// an olympiad task, as in the measurement's plan (docs/architecture.md, section 3.1). They are
// fixed, so that every try of a task is a try of the same thing.
var Tasks = []Task{
	{
		ID:        "equation-1",
		Statement: "Реши уравнение 3x + 1 = 7.",
		Key:       "x = 2",
		Solution:  []string{"3x + 1 = 7", "3x = 6", "x = 2"},
	},
	{
		ID:        "equation-2",
		Statement: "Реши уравнение 2(x − 3) = 10.",
		Key:       "x = 8",
		Solution:  []string{"2(x − 3) = 10", "x − 3 = 5", "x = 8"},
	},
	{
		ID:        "chain-1",
		Statement: "Вычисли 17 + 49 − 14 × 2.",
		Key:       "38",
		Solution:  []string{"14 × 2 = 28", "17 + 49 = 66", "66 − 28 = 38"},
	},
	{
		ID:        "chain-2",
		Statement: "Вычисли (47 + 19) : 6.",
		Key:       "11",
		Solution:  []string{"47 + 19 = 66", "66 : 6 = 11"},
	},
	{
		ID:        "posts",
		Statement: "Вдоль дорожки длиной 20 м через каждые 5 м стоят столбы, первый — в начале, последний — в конце. Сколько всего столбов?",
		Key:       "5",
		Solution:  []string{"20 : 5 = 4 — столько промежутков", "4 + 1 = 5 — столбов на один больше, чем промежутков", "Ответ: 5 столбов"},
	},
}

func taskByID(id string) *Task {
	for i := range Tasks {
		if Tasks[i].ID == id {
			return &Tasks[i]
		}
	}
	return nil
}
