# Essays

> What the executor wrote in 20 synthetic essays in English, and what in them is correct by the plan (T82; R44). On them the measurement E4 measures how Claude assesses an essay by the author's criteria — assessment by the rubric — and writes feedback (R41 spec; О-22). How the call and the report are built — [architecture.md](architecture.md), section 3.4, "Essays".
>
> The essays are the file `eval/essays.json` in the format `tutor-essays` v0 (R44). This is synthetic content under Apache-2.0: the essays were written by the executor, not by learners, on made-up topics and without names (П-20). There are no files of pages: the code draws the strokes in memory, from the file and the seed. The rubric, the counts and the essays below are the same file: a test of `cmd/tutor-eval` builds them from the file and compares them with this document byte for byte, so an essay is edited in both places.
>
> **The author's proofreading** confirms the planned truth as an adult's assessment: whether each criterion is met as stated for the essay. Where an adult would assess otherwise, that is a remark on the essay.

## How to read

- **Rubric** — one for all the essays, six criteria. The text of a criterion is in English, as a module would give it: the explanation language of the essays module is English (R41 spec). The model gets the criteria and the topic as a block of the task's data (R02 spec).
- **Topic** — the task statement, in English. The topics are made up and general, not about the learner's life, as П-20 recommends.
- **Truth** — whether the criterion is met, by the plan. Five criteria of the six the program computes itself and compares with the plan when the file is checked, `tutor-eval check`. "On topic" is held only by the plan and this proofreading.
- **No disputable cases** — a divergence from the truth must be the model's error, not a dispute about a rule. The check of the file holds the rules, and what the program does not see, the plan and the proofreading hold:
  - **sentences** — where the criterion is met there are 5–9 of them, where it is not met 3–4, long ones, joined by commas. A sentence ends with a full stop, a question mark or an exclamation mark, and after the mark comes a space or the end of the text;
  - **capitals and stops** — not met if a sentence starts with a small letter that is not written like its own capital: a b d e f g h i m n q r t y. For c, o, s, w and others the capital differs only in height, and in handwriting this is disputable. The second case is the last sentence with no mark at the end. There is no missing full stop inside the text: otherwise how many sentences there are would be disputable. The program sees one only if the next word starts with a capital, and before a small letter the plan and the proofreading hold it;
  - **linking words** — met if there are at least two different linking words: because, but, so, then, first, next, finally, also, however, later, when. Not met — not one, and not even and, or, after, before, while, until, if, although or since. The check rejects everything between these cases. The program counts words, not meaning: "so bright" and "the first customers" link no thoughts. That each essay where the criterion is met has at least two linking words even without such words, the proofreading holds;
  - **spelling** — not met if 2–3 common words are misspelled. Linking words are never misspelled;
  - **on topic** — not met if the whole essay is about another subject, and its title too;
  - **no names** — a capital letter inside a sentence stands only in "I", in the text and in the topic. So the check keeps the essays free of names (П-20). The title is written with capitals, and only the proofreading finds a name in it;
  - **characters** — Latin letters, an apostrophe inside a word, a space, a comma and the end marks. The text has 40–80 words, the title 1–6.
- **Shares** — each criterion is not met in 5 essays of 20: in one of the dev part and in four of the test part. In all, 30 criteria of 120 are not met. All six are met by 4 essays — one in the dev part and three in the test part — and the others have one to three not met. Met criteria are the majority: the main measure of О-22 is a criterion that the model called not met although it is met.
- **Split** — essays 1–5 are the dev part, 6–20 the test part (T84).

## How a page is built

- **Sheet** — 1000 × 1400 units, a handwriting sheet of the `spec` format: for a task with a rubric the canvas is `handwriting_sheet` with ruled lines. The title is the first line, and the text below it is wrapped by words, 28–40 characters to a line. Each essay takes its line length from the seed, so their handwriting is of different sizes too.
- **Handwriting** — `tutor`'s synthetic handwriting, where Latin letters are strokes of its own (R43). Between words there is a gap of two spaces: at half a space, the words of the lines with calculations ran together.
- **Crop** — by the crop rule of the eval protocol at 32 px, as in all the measurements: only the ink. The scale is set by the median of the heights of the stroke bundles, and in a text that is the height of a small letter: it is drawn D′ high, and the capitals almost twice as high (В36). Where the picture at 32 px does not fit in 1568 tokens, the rule reduces D′. On the 20 essays D′ is 23–32 px, a crop 864–1216 × 671–1330 px, 936–1564 tokens.
- **To view** the crops — what the model will see — run `go run ./cmd/tutor-eval essays -png synthetic/png/essays`. The directory is in `synthetic/`, outside git. *2026-11-07 (T107; R71): the essays are samples of the eval protocol, and `draw` saves the crops as it draws them: `go run ./cmd/tutor-eval draw -essays eval/essays.json -dir synthetic/png/drawn-essays -png synthetic/png/essays`, into a directory with no essays in it yet.*

## Rubric

| ID | Text of the criterion | What it means | Who checks the truth |
|---|---|---|---|
| `title` | It has a title on its own first line. | the essay has a title, on a line of its own, the first | the program: a title is there — 1 to 6 words, with a capital letter, with no mark at the end |
| `sentences` | It has at least five sentences. | the essay has no fewer than five sentences | the program: sentences are counted by the marks at their ends |
| `capitals-and-stops` | Every sentence begins with a capital letter and ends with a full stop, a question mark or an exclamation mark. | every sentence starts with a capital letter and ends with a full stop, a question mark or an exclamation mark | the program |
| `linking-words` | It links its ideas with words such as because, but, so, then or first. | the thoughts are linked by words such as because, but, so, then or first | the program, by a closed list of linking words |
| `on-topic` | It keeps to the topic of the task. | the essay keeps to the topic of the task | only the plan and the author's proofreading |
| `spelling` | Common words are spelled correctly. | common words are spelled without mistakes | the program: the misspelled words are listed in the file and stand in the text |

## How many essays of each kind

| Criterion | Not met | Dev | Test |
|---|---|---|---|
| `title` | 5 | 1 | 4 |
| `sentences` | 5 | 1 | 4 |
| `capitals-and-stops` | 5 | 1 | 4 |
| `linking-words` | 5 | 1 | 4 |
| `on-topic` | 5 | 1 | 4 |
| `spelling` | 5 | 1 | 4 |
| **all met** | 4 | 1 | 3 |

Dev and Test are the numbers of essays 1–5 and of essays 6–20.

## The essays one by one

### 1. The Bread Robot

- **Part:** dev.
- **Topic:** Imagine a robot that bakes bread. Write a short essay about one day of its work.
- **Words** — 69, **sentences** — 6.
- **Not met:** none.

> The Bread Robot
>
> The robot wakes up at four in the morning because the ovens need time to get hot. First it mixes flour, water and salt in a big bowl. Then it shapes the dough into long loaves and round buns. The kitchen smells sweet and warm. When the sun comes up, the first customers knock on the door. The robot is tired, but it is happy to see them smile.

### 2. Under the Waves

- **Part:** dev.
- **Topic:** Imagine a town under the sea. Write a short essay about what a visitor sees there.
- **Words** — 74, **sentences** — 3.
- **Not met:**
  - `sentences` — 3 sentences.

> Under the Waves
>
> When I swim down to the town under the sea, I see houses made of shells, streets of soft white sand and gardens full of green weed that waves slowly in the water. The fish carry letters from house to house, and a crab sweeps the square every morning with a tiny broom. I would like to stay there for a long time, but I have to go back up because I need air.

### 3. My First Morning

- **Part:** dev.
- **Topic:** Imagine an island that nobody has found yet. Write a short essay about your first morning there.
- **Words** — 68, **sentences** — 8.
- **Not met:**
  - `spelling` — misspelled: realy instead of really, beautifull instead of beautiful.

> My First Morning
>
> I wake up on a quiet beach. The sand is warm and the sky is pink. First I walk to the trees to look for water. I find a small river with clear, cold water. Then I see a bird with long blue feathers. It is realy beautifull. I sit on a rock and wait for the sun. Nobody knows about this place, but I am not afraid.

### 4. no title

- **Part:** dev.
- **Topic:** Imagine a tree that can talk. Write a short essay about what it tells you.
- **Words** — 57, **sentences** — 6.
- **Not met:**
  - `title` — there is no title;
  - `on-topic` — the whole essay is about something else.

> The bicycle race starts at nine in the morning. Hundreds of riders wait at the line because the streets are closed for them. When the whistle blows, they rush forward. The crowd cheers loudly, but the wind is strong. Then the riders climb a steep hill near the river. The winner finishes in less than two hours.

### 5. The Flying Umbrella

- **Part:** dev.
- **Topic:** Imagine an umbrella that can fly. Write a short essay about where it goes.
- **Words** — 52, **sentences** — 7.
- **Not met:**
  - `capitals-and-stops` — sentence 5 starts with a small letter: "the umbrella …";
  - `linking-words` — not one linking word, not even and or or.

> The Flying Umbrella
>
> My red umbrella opens by itself on a windy day. It lifts me off the ground. We float over the roofs of the town. The people below look very small. the umbrella carries me to a hill far away. There it lands softly in the grass. It closes with a happy click.

### 6. The Lighthouse Cat

- **Part:** test.
- **Topic:** Imagine a cat that lives in a lighthouse. Write a short essay about one night of its life.
- **Words** — 67, **sentences** — 6.
- **Not met:** none.

> The Lighthouse Cat
>
> Every evening the cat climbs the long stairs of the lighthouse. At the top it sits next to the big lamp because the glass is warm. It watches the dark sea and the little boats. When a storm comes, the waves crash against the rocks. The cat is not scared, so it stays by the light all night. In the morning it sleeps on a soft blanket.

### 7. no title

- **Part:** test.
- **Topic:** Imagine a library where the books whisper at night. Write a short essay about what you hear there.
- **Words** — 68, **sentences** — 6.
- **Not met:**
  - `title` — there is no title.

> The library is quiet in the day, but at night the books begin to whisper. I hide behind a tall shelf and listen. An old book of maps talks about mountains and deserts. A small book of poems sings a soft song. Then a book about the sea tells a story of a lost ship. I stay until the sun rises because every book has something to say.

### 8. The Shy Dragon

- **Part:** test.
- **Topic:** Imagine a dragon that is afraid of the dark. Write a short essay about how it finds its courage.
- **Words** — 66, **sentences** — 9.
- **Not met:**
  - `linking-words` — not one linking word, not even and or or;
  - `spelling` — misspelled: freind instead of friend, beleive instead of believe.

> The Shy Dragon
>
> The little dragon lives in a deep cave. It does not like the dark. Every night it hides under its wings. One evening a small owl visits the cave. The owl is a kind freind. It shows the dragon the stars through a hole in the roof. The dragon can't beleive its eyes. It breathes a tiny flame. Its cave is full of warm light now.

### 9. A Balloon Ride

- **Part:** test.
- **Topic:** Imagine a snowman who wants to see the summer. Write a short essay about its wish.
- **Words** — 70, **sentences** — 3.
- **Not met:**
  - `sentences` — 3 sentences;
  - `on-topic` — the whole essay is about something else.

> A Balloon Ride
>
> Early in the morning we climb into the basket of a huge yellow balloon, and the pilot lights a fire that roars above our heads. Slowly we rise over fields, farms and a river that shines like silver in the sun, so the cows below look like tiny toys. When we land in a soft meadow, everyone laughs because the basket tips over and we roll out onto the grass.

### 10. Above the Clouds

- **Part:** test.
- **Topic:** Imagine a city in the clouds. Write a short essay about a day you spend there.
- **Words** — 64, **sentences** — 6.
- **Not met:**
  - `capitals-and-stops` — the last sentence has no mark at the end.

> Above the Clouds
>
> The city in the clouds has houses as light as feathers. People walk on bridges made of mist. In the morning I eat breakfast on a soft white hill. Then I ride a bird to the market because there are no cars. The fruit there tastes like rain and honey. When evening comes, the clouds turn orange and gold, so the whole city glows

### 11. The Walking Shoe

- **Part:** test.
- **Topic:** Imagine a shoe that can walk on its own. Write a short essay about where it goes.
- **Words** — 66, **sentences** — 7.
- **Not met:** none.

> The Walking Shoe
>
> One morning a brown shoe decides to go for a walk. First it hops down the steps of the house. Then it walks along the street and looks in the shop windows. A dog barks at it, but the shoe is brave. It visits the park because it wants to see the ducks. At sunset it comes home, tired and dusty. It sleeps well that night.

### 12. no title

- **Part:** test.
- **Topic:** Imagine a garden on the roof of a tall building. Write a short essay about what grows there.
- **Words** — 59, **sentences** — 6.
- **Not met:**
  - `title` — there is no title;
  - `capitals-and-stops` — sentence 3 starts with a small letter: "bees come …";
  - `spelling` — misspelled: wonderfull instead of wonderful, evry instead of every.

> High above the busy streets there is a garden on a roof. Tomatoes grow in big pots, and beans climb up long sticks. bees come from far away because the flowers are so bright. In the evening the wind is cool, but the plants are strong. The view of the city is wonderfull. I water the garden evry day.

### 13. The Fishing Boat

- **Part:** test.
- **Topic:** Imagine a clock that runs backwards. Write a short essay about a day when it changes everything.
- **Words** — 61, **sentences** — 6.
- **Not met:**
  - `on-topic` — the whole essay is about something else.

> The Fishing Boat
>
> The little fishing boat leaves the port before dawn. The sailors pull the ropes, and the sail fills with wind. First they sail past the rocks where the seals sleep. Then they throw their nets into the cold water. The work is hard, but the sea is calm today. When the nets are full, the boat turns back to the port.

### 14. The Singing Whale

- **Part:** test.
- **Topic:** Imagine a whale that sings to ships. Write a short essay about its song.
- **Words** — 62, **sentences** — 4.
- **Not met:**
  - `sentences` — 4 sentences;
  - `linking-words` — not one linking word, not even and or or.

> The Singing Whale
>
> Far out in the ocean lives a giant blue whale with a voice as deep as thunder. Every night it swims beside the big ships, singing a slow song about the moon, the stars, the warm southern seas. The sailors on the decks stop their work to listen, their eyes full of tears. The song stays in their hearts for many years.

### 15. no title

- **Part:** test.
- **Topic:** Imagine a door in the side of a mountain. Write a short essay about what is behind it.
- **Words** — 61, **sentences** — 6.
- **Not met:**
  - `title` — there is no title;
  - `spelling` — misspelled: diffrent instead of different, peple instead of people.

> At the foot of the mountain there is a small green door. I knock three times, and it opens slowly. Behind it is a long tunnel with glowing stones on the walls. I follow the tunnel because I hear music. At the end I find a hall full of diffrent instruments. There are no peple, but the instruments play by themselves.

### 16. The Snail's Journey

- **Part:** test.
- **Topic:** Imagine a bus that can fly over the city. Write a short essay about a trip on it.
- **Words** — 56, **sentences** — 7.
- **Not met:**
  - `capitals-and-stops` — sentence 4 starts with a small letter: "its shell …";
  - `linking-words` — not one linking word, not even and or or;
  - `on-topic` — the whole essay is about something else.

> The Snail's Journey
>
> A small snail starts a long journey across the garden. It moves very slowly over the wet stones. The sun is hot on its back. its shell feels heavy. A beetle passes by with a friendly nod. The snail rests under a big leaf. By evening it reaches the pond at the end of the garden.

### 17. The Fallen Star

- **Part:** test.
- **Topic:** Imagine a small star that falls into a field. Write a short essay about what happens next.
- **Words** — 72, **sentences** — 7.
- **Not met:** none.

> The Fallen Star
>
> Late one night a small star falls into a field of wheat. It glows softly, but it cannot fly back to the sky. A fox finds it first and sniffs it carefully. Then a sleepy owl lands beside them. The star is sad because it misses its friends. The fox and the owl carry it to the top of a hill. When the wind blows, the star rises back into the sky.

### 18. The Ice Bridge

- **Part:** test.
- **Topic:** Imagine a bridge made of ice. Write a short essay about crossing it.
- **Words** — 64, **sentences** — 3.
- **Not met:**
  - `sentences` — 3 sentences;
  - `spelling` — misspelled: slipery instead of slippery, beatiful instead of beautiful.

> The Ice Bridge
>
> The bridge over the frozen river is made of clear blue ice, and it shines like glass in the winter sun. I walk across it very carefully because the surface is slipery, but the view is so beatiful that I forget to be scared. When I reach the other side, I look back and see my footprints sparkling on the ice like tiny stars.

### 19. no title

- **Part:** test.
- **Topic:** Imagine a painter who paints with the wind. Write a short essay about one of the pictures.
- **Words** — 65, **sentences** — 3.
- **Not met:**
  - `title` — there is no title;
  - `sentences` — 3 sentences;
  - `on-topic` — the whole essay is about something else.

> In a quiet village there is a turtle that runs a tiny shop full of buttons, ribbons and old keys. Every customer has to wait a long time because the turtle walks very slowly from the shelves to the counter. Nobody minds the wait, so the shop is always busy, and children love to watch the turtle count the coins with its small green feet.

### 20. The Changing Map

- **Part:** test.
- **Topic:** Imagine a map that changes every night. Write a short essay about where it takes you.
- **Words** — 59, **sentences** — 7.
- **Not met:**
  - `capitals-and-stops` — sentence 3 starts with a small letter: "the path …", sentence 6 starts with a small letter: "each night …";
  - `linking-words` — not one linking word, not even and or or.

> The Changing Map
>
> My old map shows a different land every morning. Today it shows a desert with three red rocks. the path on the map leads to a hidden well. I walk for hours in the hot sand. The well is full of cold, sweet water. each night the map changes again. Tomorrow it might show a jungle full of parrots.
