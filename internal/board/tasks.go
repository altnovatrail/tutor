package board

// Task is one of the board's tasks: what the person solves and what the model is told is right.
type Task struct {
	ID        string   `json:"id"`
	Statement string   `json:"statement"`
	Key       string   `json:"-"`
	Solution  []string `json:"-"`
}

// Tasks are the eleven tasks of the board. The first five are two linear equations, two chains of
// calculations and an olympiad task, as in the measurement's plan (docs/architecture.md, section
// 3.1). The other six are olympiad tasks solved in free form, for epic E4: two on a number line,
// two with a drawing, a row of objects or a scheme of segments, and two with the working at the
// side (section 3.4). They are fixed, so that every try of a task is a try of the same thing, and
// each is a copy of its task in eval/bank.json or eval/free-bank.json, which a test of
// cmd/tutor-eval keeps equal.
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
	{
		ID:        "grasshopper",
		Statement: "Кузнечик сидит на числовой прямой в точке 3. Он прыгнул три раза вправо, каждый раз на 4, а потом один раз влево на 5. В какой точке он оказался?",
		Key:       "10",
		Solution:  []string{"числовая прямая: прыжки вправо 3 → 7 → 11 → 15 и прыжок влево 15 → 10", "3 + 4 + 4 + 4 = 15", "15 − 5 = 10", "Ответ: в точке 10"},
	},
	{
		ID:        "backwards",
		Statement: "Задумали число. К нему прибавили 7, потом вычли 12 и получили 15. Какое число задумали?",
		Key:       "20",
		Solution:  []string{"числовая прямая, с конца: прыжок вправо 15 → 27 и прыжок влево 27 → 20", "15 + 12 = 27", "27 − 7 = 20", "Ответ: 20"},
	},
	{
		ID:        "houses",
		Statement: "В ряд стоят домики. Домик с флажком — четвёртый слева и шестой справа. Сколько домиков в ряду?",
		Key:       "9",
		Solution:  []string{"рисунок: ряд домиков — 3 слева от домика с флажком, он сам и 5 справа", "3 + 1 + 5 = 9", "Ответ: 9 домиков"},
	},
	{
		ID:        "stamps",
		Statement: "У двух братьев вместе 20 марок. У старшего на 6 марок больше, чем у младшего. Сколько марок у младшего?",
		Key:       "7",
		Solution:  []string{"схема: отрезок младшего и отрезок старшего, длиннее на 6, — вместе 20", "20 − 6 = 14 — это два отрезка младшего", "14 : 2 = 7", "Ответ: 7 марок"},
	},
	{
		ID:        "pages",
		Statement: "Страницы книги пронумеровали подряд, с 1 по 75. Сколько всего цифр написали?",
		Key:       "141",
		Solution:  []string{"с 1 по 9 — 9 страниц по одной цифре", "с 10 по 75 — 75 − 9 = 66 страниц по две цифры", "66 × 2 = 132", "9 + 132 = 141", "Ответ: 141 цифра"},
	},
	{
		ID:        "album",
		Statement: "Тетрадь стоит 14 рублей, а альбом — в 3 раза дороже. Сколько стоят вместе 2 альбома и 5 тетрадей?",
		Key:       "154",
		Solution:  []string{"14 × 3 = 42 — стоит альбом", "42 × 2 = 84 — стоят 2 альбома", "14 × 5 = 70 — стоят 5 тетрадей", "84 + 70 = 154", "Ответ: 154 рубля"},
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
